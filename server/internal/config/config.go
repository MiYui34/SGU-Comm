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
	c.MaxConnections = 100
	if v := getenv("MAX_CONNECTIONS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return Config{}, errors.New("MAX_CONNECTIONS must be a positive integer")
		}
		c.MaxConnections = n
	}
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
