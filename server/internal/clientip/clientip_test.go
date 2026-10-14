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
	if got := Of(r, true); got != "203.0.113.7" {
		t.Errorf("trusted: got %q", got)
	}
	r.Header.Del("X-Forwarded-For")
	if got := Of(r, true); got != "10.0.0.5" {
		t.Errorf("trusted without header: got %q", got)
	}
}
