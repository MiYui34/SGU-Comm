// Command server bridges browser WebSockets to one Mumble server.
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

	"kallisti.chat/server/internal/bridge"
	"kallisti.chat/server/internal/config"
	"kallisti.chat/server/internal/files"
	"kallisti.chat/server/internal/sidecar"
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

	upstream := net.JoinHostPort(cfg.MumbleHost, strconv.Itoa(cfg.MumblePort))
	tlsConfig := &tls.Config{InsecureSkipVerify: !cfg.RejectUnauthorized}
	// SNI does not allow IP addresses.
	if net.ParseIP(cfg.MumbleHost) == nil {
		tlsConfig.ServerName = cfg.MumbleHost
	}
	dialer := &tls.Dialer{NetDialer: &net.Dialer{Timeout: 10 * time.Second}, Config: tlsConfig}

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
	b := bridge.New(bridge.Options{
		Sidecar:           hub,
		AllowedOrigins:    cfg.AllowedOrigins,
		MaxConnections:    cfg.MaxConnections,
		MaxPerIP:          cfg.MaxPerIP,
		TrustForwardedFor: cfg.TrustForwardedFor,
		Dial: func(ctx context.Context) (net.Conn, error) {
			return dialer.DialContext(ctx, "tcp", upstream)
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
		}
	}()
	mux := http.NewServeMux()
	mux.Handle("/api/upload", fileHandler)
	mux.Handle("/files/", fileHandler)
	mux.Handle("/", b) // WebSocket bridge

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
