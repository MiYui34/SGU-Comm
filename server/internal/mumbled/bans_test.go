package mumbled

import (
	"net/netip"
	"testing"
	"time"
)

func TestBans(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	ip := netip.MustParseAddr("10.1.2.3")
	for _, c := range []struct {
		name string
		ban  BanEntry
		want bool
	}{
		{"same IP", BanEntry{Prefix: netip.MustParsePrefix("10.1.2.3/32"), Start: now}, true},
		{"range", BanEntry{Prefix: netip.MustParsePrefix("10.1.0.0/16"), Start: now}, true},
		{"other range", BanEntry{Prefix: netip.MustParsePrefix("10.2.0.0/16"), Start: now}, false},
		{"certificate", BanEntry{Hash: "abc", Start: now}, true},
		{"other certificate", BanEntry{Hash: "xyz", Start: now}, false},
		{"expired", BanEntry{Hash: "abc", Start: now.Add(-2 * time.Hour), Duration: time.Hour}, false},
		{"still running", BanEntry{Hash: "abc", Start: now.Add(-30 * time.Minute), Duration: time.Hour}, true},
	} {
		if got := c.ban.Matches(ip, "abc", now); got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}
}

func TestBanAddressesAsMumbleSendsThem(t *testing.T) {
	// Mumble sends 16 bytes (IPv4 mapped into IPv6) and a mask out of 128.
	v4 := netip.MustParseAddr("192.168.1.0").As16()
	p, ok := banPrefix(v4[:], 120)
	if !ok || p != netip.MustParsePrefix("192.168.1.0/24") {
		t.Fatalf("prefix %v %v", p, ok)
	}
	addr, mask := banAddress(p)
	if len(addr) != 16 || mask != 120 {
		t.Fatalf("back: %x %d", addr, mask)
	}
	v6 := netip.MustParseAddr("2001:db8::1").As16()
	if p, ok := banPrefix(v6[:], 64); !ok || p.Bits() != 64 || !p.Addr().Is6() {
		t.Fatalf("v6 prefix %v", p)
	}
	if _, ok := banPrefix([]byte{1, 2, 3}, 32); ok {
		t.Fatal("accepted a short address")
	}
}
