// Package clientip finds the client's IP address of a request, and the
// key that rate and connection limits count it under.
package clientip

import (
	"net"
	"net/http"
	"net/netip"
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

// Prefix is the key of all rate and connection limits: one IPv6 /64 counts
// as one address (one user usually has all of it). IPv4, also mapped into
// IPv6, stays the full address. Identity, bans and logs keep the address.
func Prefix(ip netip.Addr) netip.Addr {
	ip = ip.Unmap()
	if ip.Is6() {
		p, _ := ip.Prefix(64)
		return p.Addr()
	}
	return ip
}

// Key is Prefix for an address as text; other text stays as it is.
func Key(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	return Prefix(addr).String()
}
