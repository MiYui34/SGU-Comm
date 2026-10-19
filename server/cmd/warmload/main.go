// warmload puts a load on a built-in server for the join warm-up
// measurements (tests/perf): registered Mumble users with avatars
// (random-noise PNGs of 50 to 120 KB) in one channel, the first of them
// talking without pause (Opus packets from an Ogg file). Runs until killed.
//
//	go run ./cmd/warmload -server 127.0.0.1:64820 -users 25 -tone ../tests/perf/tone.opus
package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"math/big"
	mrand "math/rand/v2"
	"os"
	"path/filepath"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

func main() {
	server := flag.String("server", "127.0.0.1:64820", "Mumble address")
	users := flag.Int("users", 25, "users")
	dir := flag.String("dir", filepath.Join(os.TempDir(), "kallisti-perf", "load"), "client certificates (kept between runs)")
	channel := flag.String("channel", "General", "channel")
	tone := flag.String("tone", "", "Ogg Opus file with 20 ms frames (required)")
	flag.Parse()
	if *tone == "" {
		log.Fatal("-tone is required")
	}
	if err := os.MkdirAll(*dir, 0o700); err != nil {
		log.Fatal(err)
	}
	raw, err := os.ReadFile(*tone)
	if err != nil {
		log.Fatal(err)
	}
	packets, err := oggPackets(raw)
	if err != nil {
		log.Fatalf("%s: %v", *tone, err)
	}
	var wg sync.WaitGroup
	for i := range *users {
		wg.Go(func() {
			var pk [][]byte
			if i == 0 {
				pk = packets
			}
			name := fmt.Sprintf("load%02d", i)
			for {
				err := runUser(*server, *dir, name, i, *channel, pk)
				log.Printf("%s: %v; again", name, err)
				time.Sleep(time.Second)
			}
		})
		time.Sleep(50 * time.Millisecond)
	}
	wg.Wait()
}

// cert loads the user's certificate from dir, or makes and saves one.
func cert(dir, name string) (tls.Certificate, error) {
	path := filepath.Join(dir, name+".pem")
	if b, err := os.ReadFile(path); err == nil {
		if c, err := tls.X509KeyPair(b, b); err == nil {
			return c, nil
		}
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(time.Now().UnixNano()), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(5, 0, 0)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	kb, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, err
	}
	b := append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb})...)
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return tls.Certificate{}, err
	}
	return tls.X509KeyPair(b, b)
}

// avatar: random noise compresses badly, so the side sets the size.
func avatar(i int) []byte {
	side := 130 + (i*13)%70
	img := image.NewNRGBA(image.Rect(0, 0, side, side))
	for p := range img.Pix {
		img.Pix[p] = byte(mrand.IntN(256))
		if p%4 == 3 {
			img.Pix[p] = 255
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

// runUser connects, joins the channel, registers itself, sets an avatar,
// and (with packets) talks; it returns when the connection ends.
func runUser(server, dir, name string, i int, channel string, packets [][]byte) error {
	c, err := cert(dir, name)
	if err != nil {
		return err
	}
	conn, err := tls.Dial("tcp", server, &tls.Config{InsecureSkipVerify: true, Certificates: []tls.Certificate{c}})
	if err != nil {
		return err
	}
	defer conn.Close()
	var wmu sync.Mutex
	write := func(b []byte) error {
		wmu.Lock()
		defer wmu.Unlock()
		_, err := conn.Write(b)
		return err
	}
	send := func(typ uint16, m proto.Message) { write(mumble.MessageFrame(typ, m)) }
	send(mumble.TypeVersion, &pb.Version{VersionV1: proto.Uint32(1<<16 | 4<<8), Release: proto.String("warmload")})
	send(mumble.TypeAuthenticate, &pb.Authenticate{Username: proto.String(name), Opus: proto.Bool(true)})
	done := make(chan struct{})
	defer close(done)
	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				send(mumble.TypePing, &pb.Ping{Timestamp: proto.Uint64(uint64(time.Now().Unix()))})
			}
		}
	}()
	channels := map[string]uint32{}
	var session uint32
	textured, talking := false, false
	header := make([]byte, 6)
	for {
		if _, err := io.ReadFull(conn, header); err != nil {
			return err
		}
		typ, body := binary.BigEndian.Uint16(header), make([]byte, binary.BigEndian.Uint32(header[2:]))
		if _, err := io.ReadFull(conn, body); err != nil {
			return err
		}
		switch typ {
		case mumble.TypeReject:
			var m pb.Reject
			proto.Unmarshal(body, &m)
			return fmt.Errorf("rejected: %s", m.GetReason())
		case mumble.TypeChannelState:
			var m pb.ChannelState
			if proto.Unmarshal(body, &m) == nil && m.Name != nil {
				channels[m.GetName()] = m.GetChannelId()
			}
		case mumble.TypeServerSync:
			var m pb.ServerSync
			proto.Unmarshal(body, &m)
			session = m.GetSession()
			send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(session), ChannelId: proto.Uint32(channels[channel])})
		case mumble.TypeUserState:
			var m pb.UserState
			if proto.Unmarshal(body, &m) != nil || m.GetSession() != session || session == 0 {
				continue
			}
			// Register first: the server keeps avatars of registered users.
			if m.UserId == nil && !textured {
				send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(session), UserId: proto.Uint32(0)})
			} else if !textured {
				textured = true
				send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(session), Texture: avatar(i)})
			}
			if m.ChannelId != nil && len(packets) > 0 && !talking {
				talking = true
				go talk(write, packets, done)
			}
		}
	}
}

// talk sends the packets in a loop, one each 20 ms, as one long spurt.
func talk(write func([]byte) error, packets [][]byte, done <-chan struct{}) {
	t := time.NewTicker(20 * time.Millisecond)
	defer t.Stop()
	for n := 0; ; n++ {
		select {
		case <-done:
			return
		case <-t.C:
		}
		if write(mumble.EncodeFrame(mumble.TypeUDPTunnel, voicePacket(n, packets[n%len(packets)]))) != nil {
			return
		}
	}
}
