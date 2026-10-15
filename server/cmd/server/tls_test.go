package main

import (
	"crypto/tls"
	"net"
	"os"
	"path/filepath"
	"testing"

	"kallisti.chat/server/internal/certs"
	"kallisti.chat/server/internal/config"
)

func TestMumbleUsesTheConfiguredCertificate(t *testing.T) {
	dir := t.TempDir()
	certPEM, keyPEM, _ := certs.GenerateServer("chat.example")
	os.WriteFile(filepath.Join(dir, "c.pem"), certPEM, 0o600)
	os.WriteFile(filepath.Join(dir, "k.pem"), keyPEM, 0o600)
	cfg, err := mumbleTLS(&config.Builtin{CertDir: dir, CertFile: filepath.Join(dir, "c.pem"), KeyFile: filepath.Join(dir, "k.pem")})
	if err != nil {
		t.Fatal(err)
	}
	a, b := net.Pipe()
	defer a.Close()
	go tls.Server(b, cfg).Handshake()
	c := tls.Client(a, &tls.Config{InsecureSkipVerify: true})
	if err := c.Handshake(); err != nil {
		t.Fatal(err)
	}
	if cn := c.ConnectionState().PeerCertificates[0].Subject.CommonName; cn != "chat.example" {
		t.Fatalf("served %q", cn)
	}
}
