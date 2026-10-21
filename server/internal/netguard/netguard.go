// Package netguard keeps the server's own requests to hosts that users
// name (link previews, fediverse instances) on public addresses. The check
// runs on the address really dialed, so DNS answers and redirects are
// covered too.
package netguard

import (
	"errors"
	"net/netip"
	"slices"
	"syscall"
)

var blocked = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"), // carrier-grade NAT (also VPNs)
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::/96"),        // IPv4-compatible
	netip.MustParsePrefix("2001::/32"),    // Teredo: IPv4 inside
	netip.MustParsePrefix("2002::/16"),    // 6to4: IPv4 inside
	netip.MustParsePrefix("64:ff9b::/96"), // NAT64: IPv4 behind it
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("fec0::/10"),
}

// Public: a global address (no local network, loopback or cloud metadata).
func Public(ip netip.Addr) bool {
	if ip.Is4In6() {
		return false
	}
	return ip.IsGlobalUnicast() && !ip.IsPrivate() &&
		!slices.ContainsFunc(blocked, func(p netip.Prefix) bool { return p.Contains(ip) })
}

var ErrRefused = errors.New("not a public address")

// Control is a net.Dialer Control that refuses the addresses allow does not
// pass. The Transport must not use a proxy: its address would be checked.
func Control(allow func(netip.AddrPort) bool) func(network, address string, c syscall.RawConn) error {
	return func(_, address string, _ syscall.RawConn) error {
		ap, err := netip.ParseAddrPort(address)
		if err != nil || !allow(ap) {
			return ErrRefused
		}
		return nil
	}
}
