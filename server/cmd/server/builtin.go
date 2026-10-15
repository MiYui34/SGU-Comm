package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"

	"kallisti.chat/server/internal/bridge"
	"kallisti.chat/server/internal/certs"
	"kallisti.chat/server/internal/config"
	"kallisti.chat/server/internal/mumbled"
	"kallisti.chat/server/internal/password"
	"kallisti.chat/server/internal/sidecar"
	"kallisti.chat/server/internal/store"
)

// builtin is the built-in Mumble server: native clients (Mumble, the
// desktop app) connect to its port, web clients in-process through the
// bridge.
type builtin struct {
	srv *mumbled.Server
	db  *mumbled.DB
	ln  net.Listener
	udp *net.UDPConn
}

// accountExists: on a database error the name counts as taken.
func accountExists(db *store.Store) func(name string) bool {
	return func(name string) bool {
		_, err := db.AccountByName(name)
		return !errors.Is(err, store.ErrNotFound)
	}
}

// startBuiltin starts the server. With a hub, voice channels get stored
// chats (publicBase: the server's address, for upload links).
func startBuiltin(cfg *config.Builtin, db *store.Store, hub *sidecar.Hub, publicBase string) (*builtin, error) {
	mdb, err := mumbled.OpenDB(db.DB())
	if err != nil {
		return nil, fmt.Errorf("Mumble tables: %w", err)
	}
	if err := mdb.InitLobby(); err != nil {
		return nil, fmt.Errorf("lobby: %w", err)
	}
	if err := mdb.InitRoles(); err != nil {
		return nil, fmt.Errorf("roles: %w", err)
	}
	// SuperUser's password, from the environment (not visible in ps).
	if pw := os.Getenv("MUMBLED_SUPERUSER_PASSWORD"); pw != "" {
		hash, err := password.Hash(pw)
		if err != nil {
			return nil, err
		}
		if err := mdb.SetPassword(0, hash); err != nil {
			return nil, err
		}
		os.Unsetenv("MUMBLED_SUPERUSER_PASSWORD")
	}
	tlsConfig, err := mumbleTLS(cfg)
	if err != nil {
		return nil, err
	}
	var onText func(from uint32, name string, registered bool, channel uint32, text string, images []mumbled.Image)
	var onRoles func()
	var onMoved func(session, from, to uint32)
	if hub != nil {
		onRoles, onMoved = hub.RolesChanged, hub.UserMoved
		onText = func(from uint32, name string, registered bool, channel uint32, text string, images []mumbled.Image) {
			imgs := make([]sidecar.Image, len(images))
			for i, img := range images {
				imgs[i] = sidecar.Image{Type: img.Type, Data: img.Data}
			}
			hub.MumbleText(from, name, registered, channel, text, imgs)
		}
	}
	srv, err := mumbled.New(mumbled.Config{
		Welcome: cfg.Welcome, RootName: cfg.ServerName, Password: cfg.Password, DB: mdb,
		MaxUsers: cfg.MaxUsers, MaxPerIP: cfg.MaxPerIP,
		ReservedName: accountExists(db), // names of web accounts are theirs only
		WebAccountExists: func(name, hash string) bool {
			acc, err := db.AccountByName(name)
			return err == nil && accountCertHash(acc) == hash
		},
		OnChannelText:  onText,
		OnRolesChanged: onRoles,
		OnUserMoved:    onMoved,
	})
	if err != nil {
		return nil, err
	}
	if hub != nil {
		hub.SetVoice(voiceServer{srv}, publicBase) // before anyone connects
	}
	ln, err := tls.Listen("tcp", cfg.Listen, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("Mumble port: %w", err)
	}
	udpAddr, err := net.ResolveUDPAddr("udp", cfg.Listen)
	if err != nil {
		ln.Close()
		return nil, err
	}
	udp, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		ln.Close()
		return nil, fmt.Errorf("Mumble UDP port: %w", err)
	}
	go srv.Serve(ln)
	go srv.ServeUDP(udp)
	return &builtin{srv: srv, db: mdb, ln: ln, udp: udp}, nil
}

// mumbleTLS: the configured certificate files (reloaded when renewed), or
// a self-signed certificate made on the first start (CERT_DIR).
func mumbleTLS(cfg *config.Builtin) (*tls.Config, error) {
	cert, err := certs.LoadOrCreateServer(cfg.CertDir)
	if err != nil {
		return nil, fmt.Errorf("server certificate in %s: %w", cfg.CertDir, err)
	}
	if cfg.CertFile == "" {
		return mumbled.TLSConfig(cert), nil
	}
	// Until the files exist (a new proxy), the self-signed one.
	files, err := certs.NewFileSource(cfg.CertFile, cfg.KeyFile, &cert)
	if err != nil {
		return nil, fmt.Errorf("Mumble certificate: %w", err)
	}
	return mumbled.TLSConfigFrom(files.GetCertificate), nil
}

// voiceServer is the built-in server as the sidecar's Voice.
type voiceServer struct{ *mumbled.Server }

func (v voiceServer) SyncTextChannels(want []sidecar.Mirror) ([]uint32, error) {
	mirrors := make([]mumbled.TextMirror, len(want))
	for i, w := range want {
		mirrors[i] = mumbled.TextMirror{Channel: w.Channel, Name: w.Name, Topic: w.Topic}
	}
	return v.Server.SyncTextChannels(mirrors)
}

// dial makes an in-process session for a web client.
func (b *builtin) dial(_ context.Context, up bridge.Upstream) (net.Conn, error) {
	client, server := net.Pipe()
	o := mumbled.Origin{}
	if ip, err := netip.ParseAddr(up.IP); err == nil {
		o.Host = ip.Unmap()
	}
	if up.Cert != nil && up.AccountName != "" {
		o.Account = &mumbled.WebAccount{Name: up.AccountName, CertHash: certHash(up.Cert.Certificate[0])}
	}
	go b.srv.ServeConn(server, o)
	return client, nil
}

// nameTaken: new web accounts may not take names of Mumble registrations.
func (b *builtin) nameTaken(key string) bool {
	r, err := b.db.UserByName(key)
	if err != nil {
		slog.Error("looking up a Mumble user", "err", err)
		return true
	}
	return r != nil
}

func (b *builtin) close() {
	b.ln.Close()
	b.udp.Close()
	b.srv.Close()
}
