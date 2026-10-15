package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func me(a *Service, c *http.Cookie) string {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/me", nil)
	r.AddCookie(c)
	a.ServeHTTP(w, r)
	return w.Body.String()
}

func TestGuestAccounts(t *testing.T) {
	a, srv := setup(t)
	if resp, _ := post(t, srv, "/api/guest", map[string]string{"name": "visitor"}, nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("without guest accounts: %d", resp.StatusCode)
	}
	a.GuestAccounts = true
	resp, v := post(t, srv, "/api/guest", map[string]string{"name": " visitor "}, nil)
	c := sessionCookie(resp)
	if resp.StatusCode != http.StatusCreated || v["name"] != "visitor" || v["temporary"] != true || c == nil {
		t.Fatalf("create: %d %v", resp.StatusCode, v)
	}
	acc, ok := a.AccountFor(requestWith(c))
	if !ok || !acc.Temporary || len(acc.CertPEM) == 0 {
		t.Fatalf("account %+v", acc)
	}
	if !strings.Contains(me(a, c), `"temporary":true`) {
		t.Fatalf("me: %s", me(a, c))
	}
	// No login into it: it has no password.
	for _, pw := range []string{"", "anything at all"} {
		if resp, _ := post(t, srv, "/api/login", map[string]string{"name": "visitor", "password": pw}, nil); resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("login with %q: %d", pw, resp.StatusCode)
		}
	}
	// Names: taken by the guest now, and the usual rules.
	if resp, _ := post(t, srv, "/api/guest", map[string]string{"name": "Visitor"}, nil); resp.StatusCode != http.StatusConflict {
		t.Fatalf("taken name: %d", resp.StatusCode)
	}
	if resp, _ := post(t, srv, "/api/guest", map[string]string{"name": ""}, nil); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty name: %d", resp.StatusCode)
	}
	// A password, without an old one, makes it a normal account.
	if resp, _ := post(t, srv, "/api/password", map[string]string{"new": "short"}, c); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("short password: %d", resp.StatusCode)
	}
	if resp, v := post(t, srv, "/api/password", map[string]string{"new": "a good password"}, c); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("set password: %d %v", resp.StatusCode, v)
	}
	if acc, _ := a.AccountFor(requestWith(c)); acc.Temporary {
		t.Fatal("still temporary")
	}
	if resp, _ := post(t, srv, "/api/login", map[string]string{"name": "visitor", "password": "a good password"}, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("login: %d", resp.StatusCode)
	}
	// From now on the old password is needed.
	if resp, _ := post(t, srv, "/api/password", map[string]string{"new": "another password"}, c); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no old password: %d", resp.StatusCode)
	}
}

func TestGuestAccountLimits(t *testing.T) {
	a, srv := setup(t)
	a.GuestAccounts = true
	post(t, srv, "/api/accounts", map[string]string{"name": "taken", "password": "a long password"}, nil)
	// Names that are taken use no guest token (but count as attempts).
	for range 3 {
		if resp, _ := post(t, srv, "/api/guest", map[string]string{"name": "taken"}, nil); resp.StatusCode != http.StatusConflict {
			t.Fatalf("taken: %d", resp.StatusCode)
		}
	}
	for i := range guestBurst {
		if resp, v := post(t, srv, "/api/guest", map[string]string{"name": "g" + string(rune('a'+i))}, nil); resp.StatusCode != http.StatusCreated {
			t.Fatalf("guest %d: %d %v", i, resp.StatusCode, v)
		}
	}
	if resp, _ := post(t, srv, "/api/guest", map[string]string{"name": "onemore"}, nil); resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("over the limit per address: %d", resp.StatusCode)
	}
	b, srv2 := setup(t)
	b.GuestAccounts, b.MaxGuests = true, 1
	post(t, srv2, "/api/guest", map[string]string{"name": "first"}, nil)
	if resp, _ := post(t, srv2, "/api/guest", map[string]string{"name": "second"}, nil); resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("over the limit of guests: %d", resp.StatusCode)
	}
}

func TestLoggingOutEndsAGuestAccount(t *testing.T) {
	a, srv := setup(t)
	a.GuestAccounts = true
	resp, _ := post(t, srv, "/api/guest", map[string]string{"name": "brief"}, nil)
	c := sessionCookie(resp)
	post(t, srv, "/api/logout", nil, c)
	// The cleanup takes it at its next round (when the user is offline).
	idle, _ := a.Store.IdleTemporaryAccounts(time.Now().Add(-time.Hour))
	if len(idle) != 1 || idle[0].Name != "brief" {
		t.Fatalf("idle %+v", idle)
	}
}
