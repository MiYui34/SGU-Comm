package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kallisti.chat/server/internal/store"
)

func setup(t *testing.T) (*Service, *httptest.Server) {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	a := &Service{Store: s, Now: time.Now, AllowedOrigins: []string{"https://app.example"}}
	srv := httptest.NewServer(a)
	t.Cleanup(srv.Close)
	return a, srv
}

func post(t *testing.T, srv *httptest.Server, path string, body any, cookie *http.Cookie) (*http.Response, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", srv.URL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://app.example")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var v map[string]any
	json.NewDecoder(resp.Body).Decode(&v)
	return resp, v
}

func sessionCookie(resp *http.Response) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == CookieName {
			return c
		}
	}
	return nil
}

func requestWith(c *http.Cookie) *http.Request {
	r := httptest.NewRequest("GET", "/ws", nil)
	if c != nil {
		r.AddCookie(c)
	}
	return r
}

func TestCreateAccountLogsIn(t *testing.T) {
	a, srv := setup(t)
	resp, v := post(t, srv, "/api/accounts", map[string]string{"name": " Alice ", "password": "correct horse"}, nil)
	if resp.StatusCode != http.StatusCreated || v["name"] != "Alice" {
		t.Fatalf("create: %d %v", resp.StatusCode, v)
	}
	c := sessionCookie(resp)
	if c == nil || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie: %+v", c)
	}
	if resp.Header.Get("Access-Control-Allow-Credentials") != "true" || resp.Header.Get("Access-Control-Allow-Origin") != "https://app.example" {
		t.Errorf("CORS: %v", resp.Header)
	}
	acc, ok := a.AccountFor(requestWith(c))
	if !ok || acc.Name != "Alice" || len(acc.CertPEM) == 0 {
		t.Fatalf("account for cookie: %+v %v", acc, ok)
	}
	if strings.Contains(acc.PasswordHash, "correct horse") || !strings.HasPrefix(acc.PasswordHash, "$argon2id$") {
		t.Fatalf("password hash: %q", acc.PasswordHash)
	}
}

func TestLoginLogoutAndMe(t *testing.T) {
	a, srv := setup(t)
	post(t, srv, "/api/accounts", map[string]string{"name": "bob", "password": "longpassword"}, nil)
	if resp, _ := post(t, srv, "/api/login", map[string]string{"name": "BOB", "password": "wrong password"}, nil); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong password: %d", resp.StatusCode)
	}
	if resp, _ := post(t, srv, "/api/login", map[string]string{"name": "nobody", "password": "longpassword"}, nil); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unknown name: %d", resp.StatusCode)
	}
	resp, v := post(t, srv, "/api/login", map[string]string{"name": "BOB", "password": "longpassword"}, nil)
	c := sessionCookie(resp)
	if resp.StatusCode != http.StatusOK || v["name"] != "bob" || c == nil {
		t.Fatalf("login: %d %v", resp.StatusCode, v)
	}
	me := httptest.NewRecorder()
	a.ServeHTTP(me, func() *http.Request { r := httptest.NewRequest("GET", "/api/me", nil); r.AddCookie(c); return r }())
	if me.Code != 200 || !strings.Contains(me.Body.String(), `"bob"`) {
		t.Fatalf("me: %d %s", me.Code, me.Body.String())
	}
	if resp, _ := post(t, srv, "/api/logout", nil, c); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("logout: %d", resp.StatusCode)
	}
	if _, ok := a.AccountFor(requestWith(c)); ok {
		t.Fatal("session still valid after logout")
	}
}

func TestValidation(t *testing.T) {
	_, srv := setup(t)
	post(t, srv, "/api/accounts", map[string]string{"name": "carol", "password": "longpassword"}, nil)
	cases := []map[string]string{
		{"name": "x", "password": "longpassword"},                     // too short
		{"name": strings.Repeat("n", 33), "password": "longpassword"}, // too long
		{"name": "bad\nname", "password": "longpassword"},             // control character
		{"name": "superuser", "password": "longpassword"},             // Murmur's admin
		{"name": "dave", "password": "short"},                         // weak password
		{"name": "CAROL", "password": "longpassword"},                 // taken
	}
	for _, c := range cases {
		if resp, v := post(t, srv, "/api/accounts", c, nil); resp.StatusCode < 400 || v["error"] == nil {
			t.Errorf("%q: %d %v", c["name"], resp.StatusCode, v)
		}
	}
}

func TestLoginRateLimit(t *testing.T) {
	_, srv := setup(t)
	limited := false
	for i := 0; i < burst+3; i++ {
		resp, _ := post(t, srv, "/api/login", map[string]string{"name": "x", "password": "yyyyyyyy"}, nil)
		if resp.StatusCode == http.StatusTooManyRequests {
			limited = true
		}
	}
	if !limited {
		t.Fatal("no login attempt was limited")
	}
}

func TestPasswordHash(t *testing.T) {
	h, err := hashPassword("secret password")
	if err != nil {
		t.Fatal(err)
	}
	if !checkPassword(h, "secret password") || checkPassword(h, "other password") {
		t.Fatal("check failed")
	}
	h2, _ := hashPassword("secret password")
	if h == h2 {
		t.Fatal("no salt")
	}
}

func TestCrossSiteFormsAreRefused(t *testing.T) {
	_, srv := setup(t)
	// A plain HTML form can send text/plain with a JSON body, without CORS.
	req, _ := http.NewRequest("POST", srv.URL+"/api/login", strings.NewReader(`{"name":"x","password":"yyyyyyyy"}`))
	req.Header.Set("Content-Type", "text/plain")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain: %d, want 415", resp.StatusCode)
	}
	req, _ = http.NewRequest("POST", srv.URL+"/api/logout", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("cross-site: %d, want 403", resp.StatusCode)
	}
}

func TestLimiterKeys(t *testing.T) {
	if limiterKey("2001:db8:1:2:3:4:5:6") != limiterKey("2001:db8:1:2:ffff::1") {
		t.Error("IPv6 addresses of one /64 must share a limit")
	}
	if limiterKey("192.0.2.1") == limiterKey("192.0.2.2") {
		t.Error("IPv4 addresses must not share a limit")
	}
}
