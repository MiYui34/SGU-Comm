// Command server is the Kallisti server: the web app, accounts, text
// channels, and voice through its built-in Mumble server (or a Murmur at
// MUMBLE_HOST).
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"syscall"
	"time"

	"kallisti.chat/server/internal/auth"
	"kallisti.chat/server/internal/bridge"
	"kallisti.chat/server/internal/certs"
	"kallisti.chat/server/internal/config"
	"kallisti.chat/server/internal/files"
	"kallisti.chat/server/internal/sidecar"
	"kallisti.chat/server/internal/site"
	"kallisti.chat/server/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Parse(os.Getenv)
	if err != nil {
		return err
	}
	if slices.Contains(cfg.AllowedOrigins, "*") {
		slog.Warn("ALLOWED_ORIGINS is *: every origin can connect (development only)")
	}

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open database %s: %w", cfg.DBPath, err)
	}
	defer db.Close()
	if err := db.EnsureChannels(cfg.TextChannels); err != nil {
		return fmt.Errorf("set up text channels: %w", err)
	}

	hub, err := sidecar.NewHub(db, time.Now)
	if err != nil {
		return fmt.Errorf("load text channels: %w", err)
	}
	accounts := &auth.Service{
		Store:             db,
		Now:               time.Now,
		AllowedOrigins:    cfg.AllowedOrigins,
		TrustForwardedFor: cfg.TrustForwardedFor,
	}
	dial, upstream := murmurDialer(cfg)
	if cfg.Builtin != nil {
		mumble, err := startBuiltin(cfg.Builtin, db)
		if err != nil {
			return err
		}
		defer mumble.close()
		dial, upstream = mumble.dial, "built-in on "+cfg.Builtin.Listen
		accounts.NameTaken = mumble.nameTaken
	}
	b := bridge.New(bridge.Options{
		Sidecar:           hub,
		UniqueGuestNames:  true,
		AllowedOrigins:    cfg.AllowedOrigins,
		MaxConnections:    cfg.MaxConnections,
		MaxPerIP:          cfg.MaxPerIP,
		TrustForwardedFor: cfg.TrustForwardedFor,
		Dial:              dial,
		Account: func(r *http.Request) (string, *tls.Certificate, bool) {
			acc, ok := accounts.AccountFor(r)
			if !ok {
				return "", nil, false
			}
			cert, err := certs.Load(acc.CertPEM, acc.KeyPEM)
			if err != nil {
				slog.Error("load account certificate", "account", acc.Name, "err", err)
				return "", nil, false
			}
			return acc.Name, &cert, true
		},
		// On a database error, refuse the guest rather than risk a taken name.
		IsAccountName: func(name string) bool {
			_, err := db.AccountByName(name)
			return !errors.Is(err, store.ErrNotFound)
		},
	})

	if err := os.MkdirAll(cfg.UploadDir, 0o700); err != nil {
		return fmt.Errorf("create upload directory: %w", err)
	}
	fileHandler := &files.Handler{
		Store:          db,
		Dir:            cfg.UploadDir,
		MaxBytes:       cfg.UploadMaxBytes,
		QuotaBytes:     cfg.UploadQuotaBytes,
		AllowedOrigins: cfg.AllowedOrigins,
		Now:            time.Now,
		Redeem:         hub.RedeemUploadToken,
	}
	go func() {
		for range time.Tick(time.Minute) {
			fileHandler.DeleteUnused()
			if _, err := db.DeleteExpiredSessions(time.Now()); err != nil {
				slog.Error("delete expired sessions", "err", err)
			}
		}
	}()
	mux := http.NewServeMux()
	mux.Handle("/api/upload", fileHandler)
	for _, path := range []string{"/api/accounts", "/api/login", "/api/logout", "/api/logout-all", "/api/password", "/api/me"} {
		mux.Handle(path, accounts)
	}
	mux.Handle("/files/", fileHandler)
	// WebSocket upgrades (the client uses /ws) go to the bridge, everything
	// else to the web app.
	mux.Handle("/", site.New(b, cfg.WebDir))

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.ListenPort))
	if err != nil {
		return fmt.Errorf("cannot listen on port %d: %w", cfg.ListenPort, err)
	}
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	slog.Info("server running", "listen", ln.Addr().String(), "mumble", upstream,
		"db", cfg.DBPath, "text_channels", cfg.TextChannels)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()

	select {
	case err := <-served:
		return err
	case <-ctx.Done():
	}
	slog.Info("shutting down")
	// First stop accepting. Shutdown does not wait for hijacked (WebSocket)
	// connections: the bridge closes and waits for those itself.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = srv.Shutdown(shutdownCtx)
	b.Close()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// murmurDialer connects web clients to Murmur. Accounts connect with their
// own client certificate, so Murmur can register them; guests without one.
func murmurDialer(cfg config.Config) (func(context.Context, bridge.Upstream) (net.Conn, error), string) {
	upstream := net.JoinHostPort(cfg.MumbleHost, strconv.Itoa(cfg.MumblePort))
	tlsConfig := &tls.Config{InsecureSkipVerify: !cfg.RejectUnauthorized}
	// SNI does not allow IP addresses.
	if net.ParseIP(cfg.MumbleHost) == nil {
		tlsConfig.ServerName = cfg.MumbleHost
	}
	return func(ctx context.Context, up bridge.Upstream) (net.Conn, error) {
		conf := tlsConfig.Clone()
		if up.Cert != nil {
			conf.Certificates = []tls.Certificate{*up.Cert}
		}
		d := &tls.Dialer{NetDialer: &net.Dialer{Timeout: 10 * time.Second}, Config: conf}
		return d.DialContext(ctx, "tcp", upstream)
	}, upstream
}
