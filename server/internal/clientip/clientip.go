// Package clientip finds the client's IP address of a request.
package clientip

import (
	"net"
	"net/http"
	"strings"
)

// Of returns the remote address, or the last X-Forwarded-For entry when
// that header is trusted. The last entry is the one our own reverse proxy
// added (Caddy replaces the header, nginx appends to it); earlier entries
// come from the client and can be faked. Supports one proxy in front.
func Of(r *http.Request, trustForwarded bool) string {
	if trustForwarded {
		entries := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
		if ip := strings.TrimSpace(entries[len(entries)-1]); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
