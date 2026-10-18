package clientip

import (
	"net/http/httptest"
	"testing"
)

func TestOf(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.5:4321"
	r.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
	if got := Of(r, false); got != "10.0.0.5" {
		t.Errorf("untrusted: got %q", got)
	}
	// The last entry is the one our proxy added: earlier ones can be faked.
	if got := Of(r, true); got != "10.0.0.1" {
		t.Errorf("trusted: got %q", got)
	}
	r.Header.Del("X-Forwarded-For")
	if got := Of(r, true); got != "10.0.0.5" {
		t.Errorf("trusted without header: got %q", got)
	}
}

func TestKey(t *testing.T) {
	if Key("2001:db8:1:2:3:4:5:6") != Key("2001:db8:1:2:ffff::1") {
		t.Error("IPv6 addresses of one /64 must share a limit")
	}
	if Key("2001:db8:1:2::1") == Key("2001:db8:1:3::1") {
		t.Error("different /64s must not share a limit")
	}
	for in, want := range map[string]string{
		"192.0.2.1":        "192.0.2.1",
		"::ffff:192.0.2.1": "192.0.2.1", // the same IPv4 host
		"not an address":   "not an address",
	} {
		if got := Key(in); got != want {
			t.Errorf("Key(%q) = %q, want %q", in, got, want)
		}
	}
	if Key("::ffff:192.0.2.1") == Key("::ffff:192.0.2.2") {
		t.Error("IPv4-in-IPv6 addresses must not share a limit")
	}
}
