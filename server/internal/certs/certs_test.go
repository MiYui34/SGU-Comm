package certs

import (
	"crypto/x509"
	"encoding/pem"
	"testing"
)

func TestGenerateAndLoad(t *testing.T) {
	certPEM, keyPEM, err := Generate("alice")
	if err != nil {
		t.Fatal(err)
	}
	cert, err := Load(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(certPEM)
	parsed, err := x509.ParseCertificate(block.Bytes)
	if err != nil || parsed.Subject.CommonName != "alice" {
		t.Fatalf("got %v %v", parsed, err)
	}
	if len(cert.Certificate) != 1 || cert.PrivateKey == nil {
		t.Fatal("certificate not loaded")
	}
	if parsed.NotAfter.Sub(parsed.NotBefore).Hours() < 24*365*10 {
		t.Error("certificate should be long-lived: Murmur ties the registration to it")
	}
}

func TestEachAccountGetsItsOwnCertificate(t *testing.T) {
	a, _, _ := Generate("alice")
	b, _, _ := Generate("alice")
	if string(a) == string(b) {
		t.Fatal("two certificates are the same")
	}
}

func TestServerCertificatesAreForServers(t *testing.T) {
	certPEM, keyPEM, err := GenerateServer("Kallisti")
	if err != nil {
		t.Fatal(err)
	}
	cert, err := Load(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.ExtKeyUsage) != 1 || parsed.ExtKeyUsage[0] != x509.ExtKeyUsageServerAuth {
		t.Fatalf("usage %v", parsed.ExtKeyUsage)
	}
}
