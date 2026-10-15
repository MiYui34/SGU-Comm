package certs

import (
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writePair(t *testing.T, dir, name string, at time.Time) {
	t.Helper()
	certPEM, keyPEM, err := GenerateServer(name)
	if err != nil {
		t.Fatal(err)
	}
	for file, data := range map[string][]byte{"c.pem": certPEM, "k.pem": keyPEM} {
		path := filepath.Join(dir, file)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(path, at, at)
	}
}

func commonName(t *testing.T, s *FileSource) string {
	t.Helper()
	c, err := s.GetCertificate(nil)
	if err != nil {
		t.Fatal(err)
	}
	x, _ := x509.ParseCertificate(c.Certificate[0])
	return x.Subject.CommonName
}

func TestFileSourceReloadsChangedFiles(t *testing.T) {
	dir := t.TempDir()
	start := time.Now().Add(-time.Hour)
	writePair(t, dir, "first", start)
	s, err := NewFileSource(filepath.Join(dir, "c.pem"), filepath.Join(dir, "k.pem"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := commonName(t, s); got != "first" {
		t.Fatalf("got %q", got)
	}
	// A renewal writes new files.
	writePair(t, dir, "renewed", start.Add(time.Minute))
	if got := commonName(t, s); got != "renewed" {
		t.Fatalf("after renewal: %q", got)
	}
	// A broken write keeps the last good certificate.
	os.WriteFile(filepath.Join(dir, "c.pem"), []byte("half"), 0o600)
	os.Chtimes(filepath.Join(dir, "c.pem"), start.Add(2*time.Minute), start.Add(2*time.Minute))
	if got := commonName(t, s); got != "renewed" {
		t.Fatalf("after a broken write: %q", got)
	}
}

func TestFileSourceNeedsReadableFiles(t *testing.T) {
	if _, err := NewFileSource("/nonexistent/c.pem", "/nonexistent/k.pem", nil); err == nil {
		t.Fatal("no error for missing files")
	}
}

func TestFileSourceWaitsForFilesWithAFallback(t *testing.T) {
	dir := t.TempDir()
	fallbackPEM, fallbackKey, _ := GenerateServer("self-signed")
	fallback, _ := Load(fallbackPEM, fallbackKey)
	s, err := NewFileSource(filepath.Join(dir, "c.pem"), filepath.Join(dir, "k.pem"), &fallback)
	if err != nil {
		t.Fatal(err)
	}
	if got := commonName(t, s); got != "self-signed" {
		t.Fatalf("before the files: %q", got)
	}
	writePair(t, dir, "issued", time.Now())
	if got := commonName(t, s); got != "issued" {
		t.Fatalf("after the files appeared: %q", got)
	}
}
