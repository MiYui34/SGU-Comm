package netguard

import (
	"net/netip"
	"testing"
)

func TestPublic(t *testing.T) {
	for addr, want := range map[string]bool{
		"93.184.216.34": true, "2606:2800:220:1::1": true,
		"127.0.0.1": false, "10.1.2.3": false, "192.168.1.1": false,
		"172.16.0.1": false, "169.254.169.254": false, "100.100.1.1": false, "0.0.0.0": false,
		"::1": false, "fd00::1": false, "fe80::1": false, "::ffff:127.0.0.1": false, "::ffff:93.184.216.34": false,
		"64:ff9b::a00:1": false, "0.1.2.3": false, "240.0.0.1": false, "::7f00:1": false, "224.0.0.1": false,
		"ff02::1": false, "2002:a00:1::1": false, "2001:0:1::1": false, "192.88.99.1": false, "::": false,
	} {
		if got := Public(netip.MustParseAddr(addr)); got != want {
			t.Errorf("%s: %v", addr, got)
		}
	}
}

func TestControl(t *testing.T) {
	c := Control(func(ap netip.AddrPort) bool { return Public(ap.Addr()) })
	if c("tcp", "127.0.0.1:443", nil) == nil || c("tcp", "not an address", nil) == nil {
		t.Fatal("refused nothing")
	}
	if err := c("tcp", "93.184.216.34:443", nil); err != nil {
		t.Fatal(err)
	}
}
