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

	"github.com/mumble-discord/server/internal/bridge"
	"github.com/mumble-discord/server/internal/config"
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

	b := bridge.New(bridge.Options{
		AllowedOrigins: cfg.AllowedOrigins,
		MaxConnections: cfg.MaxConnections,
		Dial: func(ctx context.Context) (net.Conn, error) {
			return dialer.DialContext(ctx, "tcp", upstream)
		},
	})

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.ListenPort))
	if err != nil {
		return fmt.Errorf("cannot listen on port %d: %w", cfg.ListenPort, err)
	}
	srv := &http.Server{Handler: b, ReadHeaderTimeout: 10 * time.Second}
	slog.Info("bridge running", "listen", ln.Addr().String(), "mumble", upstream)

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
