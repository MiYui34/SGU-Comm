// Package clientip finds the client's IP address of a request.
package clientip

import (
	"net"
	"net/http"
	"strings"
)

// Of returns the remote address, or the first X-Forwarded-For entry when
// that header is trusted (set by our own reverse proxy).
func Of(r *http.Request, trustForwarded bool) string {
	if trustForwarded {
		first, _, _ := strings.Cut(r.Header.Get("X-Forwarded-For"), ",")
		if ip := strings.TrimSpace(first); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
