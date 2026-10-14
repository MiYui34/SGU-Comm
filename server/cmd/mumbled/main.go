// Command mumbled runs the minimal built-in Mumble server (spike).
package main

import (
	"crypto/tls"
	"database/sql"
	"errors"
	"flag"
	"io/fs"
	"log"
	"net"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"kallisti.chat/server/internal/certs"
	"kallisti.chat/server/internal/mumbled"
	"kallisti.chat/server/internal/password"
)

func main() {
	listen := flag.String("listen", ":64738", "TCP and UDP address")
	data := flag.String("data", "mumbled-data", "folder for the server certificate and database")
	welcome := flag.String("welcome", "Welcome to Kallisti.", "welcome text")
	serverPassword := flag.String("password", "", "server password for guests")
	flag.Parse()

	cert, err := loadOrCreateCert(*data)
	if err != nil {
		log.Fatal(err)
	}
	ln, err := tls.Listen("tcp", *listen, mumbled.TLSConfig(cert))
	if err != nil {
		log.Fatal(err)
	}
	udpAddr, err := net.ResolveUDPAddr("udp", *listen)
	if err != nil {
		log.Fatal(err)
	}
	udp, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatal(err)
	}
	sqlDB, err := sql.Open("sqlite", filepath.Join(*data, "mumbled.db"))
	if err != nil {
		log.Fatal(err)
	}
	db, err := mumbled.OpenDB(sqlDB)
	if err != nil {
		log.Fatal(err)
	}
	// SuperUser's password, from the environment (not visible in ps).
	if pw := os.Getenv("MUMBLED_SUPERUSER_PASSWORD"); pw != "" {
		hash, err := password.Hash(pw)
		if err != nil {
			log.Fatal(err)
		}
		if err := db.SetPassword(0, hash); err != nil {
			log.Fatal(err)
		}
		os.Unsetenv("MUMBLED_SUPERUSER_PASSWORD")
	}
	s, err := mumbled.New(mumbled.Config{Welcome: *welcome, Password: *serverPassword, DB: db})
	if err != nil {
		log.Fatal(err)
	}
	go func() { log.Fatal(s.ServeUDP(udp)) }()
	log.Printf("listening on %s (TCP and UDP)", ln.Addr())
	log.Fatal(s.Serve(ln))
}

// loadOrCreateCert keeps one certificate, so clients can pin it.
func loadOrCreateCert(dir string) (tls.Certificate, error) {
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	certPEM, err := os.ReadFile(certPath)
	if errors.Is(err, fs.ErrNotExist) {
		var keyPEM []byte
		if certPEM, keyPEM, err = certs.GenerateServer("Kallisti"); err != nil {
			return tls.Certificate{}, err
		}
		if err = os.MkdirAll(dir, 0o700); err != nil {
			return tls.Certificate{}, err
		}
		if err = os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
			return tls.Certificate{}, err
		}
		if err = os.WriteFile(certPath, certPEM, 0o644); err != nil {
			return tls.Certificate{}, err
		}
	} else if err != nil {
		return tls.Certificate{}, err
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return tls.Certificate{}, err
	}
	return certs.Load(certPEM, keyPEM)
}
