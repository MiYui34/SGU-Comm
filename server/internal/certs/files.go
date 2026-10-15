package certs

import (
	"crypto/tls"
	"log/slog"
	"os"
	"sync"
	"time"
)

// FileSource serves a certificate from PEM files (for example Let's
// Encrypt's, from a reverse proxy) and reloads them when they change. A
// broken write keeps the last good certificate.
type FileSource struct {
	certFile, keyFile string

	mu      sync.Mutex
	cert    *tls.Certificate
	changed time.Time // the newer modification time of the two files
}

// NewFileSource loads the files. If they cannot be read yet (the proxy has
// no certificate yet), it serves fallback until they can; without a
// fallback that is an error.
func NewFileSource(certFile, keyFile string, fallback *tls.Certificate) (*FileSource, error) {
	s := &FileSource{certFile: certFile, keyFile: keyFile}
	if err := s.load(); err != nil {
		if fallback == nil {
			return nil, err
		}
		slog.Warn("certificate files not usable yet, serving a self-signed certificate", "err", err)
		s.cert = fallback
	}
	return s, nil
}

func (s *FileSource) modified() (time.Time, error) {
	var latest time.Time
	for _, f := range []string{s.certFile, s.keyFile} {
		info, err := os.Stat(f)
		if err != nil {
			return time.Time{}, err
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}
	return latest, nil
}

func (s *FileSource) load() error {
	changed, err := s.modified()
	if err != nil {
		return err
	}
	cert, err := tls.LoadX509KeyPair(s.certFile, s.keyFile)
	if err != nil {
		return err
	}
	s.cert, s.changed = &cert, changed
	return nil
}

// GetCertificate is for tls.Config: it checks the files on each handshake.
func (s *FileSource) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if changed, err := s.modified(); err == nil && !changed.Equal(s.changed) {
		if err := s.load(); err != nil {
			slog.Warn("reloading the certificate failed, keeping the old one", "err", err)
			s.changed = changed // do not retry on every handshake
		}
	}
	return s.cert, nil
}
