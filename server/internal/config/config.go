// Package config reads the server settings from environment variables.
package config

import (
	"cmp"
	"errors"
	"slices"
	"strconv"
	"strings"
)

type Config struct {
	MumbleHost string
	MumblePort int
	ListenPort int
	// "*" allows every origin (development only).
	AllowedOrigins []string
	// Most Mumble servers use self-signed certificates.
	RejectUnauthorized bool
	MaxConnections     int
	// SQLite file for text channels and messages.
	DBPath string
	// Text channel names, in display order.
	TextChannels []string
	// Connections from one IP. All web users reach Murmur from our IP, so
	// one client must not use them all (Murmur's autoban would hit everyone).
	MaxPerIP int
	// Use the first X-Forwarded-For address as the client IP. Only behind a
	// reverse proxy that sets it: clients can fake the header.
	TrustForwardedFor bool
	// Uploaded files, and the size limit per file.
	UploadDir      string
	UploadMaxBytes int64
	// Bytes one user may upload per 24 hours.
	UploadQuotaBytes int64
}

// list splits a comma-separated value, trims and drops empty and repeated
// entries.
func list(value string) []string {
	var out []string
	for _, v := range strings.Split(value, ",") {
		if v = strings.TrimSpace(v); v != "" && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

func positive(value string, fallback int) (int, bool) {
	if value == "" {
		return fallback, true
	}
	n, err := strconv.Atoi(value)
	return n, err == nil && n > 0
}

func parsePort(value string, fallback int) (int, bool) {
	if value == "" {
		return fallback, true
	}
	n, err := strconv.Atoi(value)
	return n, err == nil && n > 0 && n < 65536
}

// Parse reads the settings. getenv is os.Getenv in production.
func Parse(getenv func(string) string) (Config, error) {
	c := Config{MumbleHost: getenv("MUMBLE_HOST"), RejectUnauthorized: getenv("MUMBLE_INSECURE_TLS") != "1"}
	if c.MumbleHost == "" {
		return Config{}, errors.New("MUMBLE_HOST is required")
	}
	c.AllowedOrigins = list(getenv("ALLOWED_ORIGINS"))
	if len(c.AllowedOrigins) == 0 {
		return Config{}, errors.New("ALLOWED_ORIGINS is required (use * to allow every origin)")
	}
	var ok bool
	if c.MumblePort, ok = parsePort(getenv("MUMBLE_PORT"), 64738); !ok {
		return Config{}, errors.New("MUMBLE_PORT must be a port number")
	}
	if c.ListenPort, ok = parsePort(getenv("LISTEN_PORT"), 8080); !ok {
		return Config{}, errors.New("LISTEN_PORT must be a port number")
	}
	if c.MaxConnections, ok = positive(getenv("MAX_CONNECTIONS"), 100); !ok {
		return Config{}, errors.New("MAX_CONNECTIONS must be a positive integer")
	}
	if c.MaxPerIP, ok = positive(getenv("MAX_CONNECTIONS_PER_IP"), 10); !ok {
		return Config{}, errors.New("MAX_CONNECTIONS_PER_IP must be a positive integer")
	}
	c.TrustForwardedFor = getenv("TRUST_FORWARDED_FOR") == "1"
	c.UploadDir = cmp.Or(getenv("UPLOAD_DIR"), "uploads")
	mb, ok := positive(getenv("UPLOAD_MAX_MB"), 10)
	if !ok {
		return Config{}, errors.New("UPLOAD_MAX_MB must be a positive integer")
	}
	c.UploadMaxBytes = int64(mb) << 20
	quota, ok := positive(getenv("UPLOAD_QUOTA_MB"), 100)
	if !ok {
		return Config{}, errors.New("UPLOAD_QUOTA_MB must be a positive integer")
	}
	c.UploadQuotaBytes = int64(quota) << 20
	c.DBPath = cmp.Or(getenv("DB_PATH"), "kallisti.db")
	if c.TextChannels = list(getenv("TEXT_CHANNELS")); len(c.TextChannels) == 0 {
		c.TextChannels = []string{"general"}
	}
	return c, nil
}

// OriginAllowed reports whether a browser origin may connect. An empty
// origin (non-browser client) only passes with "*".
func OriginAllowed(allowed []string, origin string) bool {
	return slices.Contains(allowed, "*") || (origin != "" && slices.Contains(allowed, origin))
}
