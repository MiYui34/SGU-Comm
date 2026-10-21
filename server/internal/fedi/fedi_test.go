package fedi_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"

	"kallisti.chat/server/internal/fedi"
	"kallisti.chat/server/internal/fedi/fedifake"
)

func TestParseInstance(t *testing.T) {
	for in, want := range map[string]string{
		"mastodon.social":                  "mastodon.social",
		"  Mastodon.Social  ":              "mastodon.social",
		"https://mastodon.social/@me":      "mastodon.social",
		"https://mastodon.social:443/":     "mastodon.social",
		"mastodon.social/about?x=1#y":      "mastodon.social",
		"@alice@Mastodon.Social":           "mastodon.social",
		"alice@mastodon.social":            "mastodon.social",
		"https://user:pw@mastodon.social/": "mastodon.social",
		"bücher.example":                   "xn--bcher-kva.example",
		"social.example.":                  "social.example",
	} {
		got, err := fedi.ParseInstance(in, false)
		if err != nil || got.Host != want || got.Base != "https://"+want {
			t.Errorf("%q: %+v %v", in, got, err)
		}
	}
	for _, in := range []string{
		"", "localhost", "http://mastodon.social", "mastodon.social:8443", "127.0.0.1", "[::1]",
		"exa mple.org", "ftp://mastodon.social", "-bad-.example", "a..b",
	} {
		if got, err := fedi.ParseInstance(in, false); err == nil {
			t.Errorf("%q: accepted as %+v", in, got)
		}
	}
	// Development (FEDI_INSECURE_DEV_INSTANCES): local test instances.
	got, err := fedi.ParseInstance("http://127.0.0.1:9000/", true)
	if err != nil || got.Host != "127.0.0.1:9000" || got.Base != "http://127.0.0.1:9000" {
		t.Fatalf("dev: %+v %v", got, err)
	}
}

func TestPKCE(t *testing.T) {
	v := fedi.NewVerifier()
	if len(v) < 43 || v == fedi.NewVerifier() {
		t.Fatalf("verifier %q", v)
	}
	// Computed with Python (hashlib, urlsafe_b64encode without padding).
	if got := fedi.Challenge("dBjftJeZ4CVP-mJ92K9qkY4-ci5a7GMNPTzYJ2E2GzM"); got != "QRIkrs9GbiUvBEkmDI7Y7hfY2vRdAQNYqjsK4E92QRs" {
		t.Fatalf("challenge %q", got)
	}
}

func TestSuggestName(t *testing.T) {
	for in, want := range map[string]string{
		"alice":                                  "alice",
		"Alice_99":                               "Alice_99",
		"al ice!":                                "alice",
		"a":                                      "",
		"!!":                                     "",
		"superuser":                              "",
		"averyveryveryveryveryverylongusername1": "averyveryveryveryveryverylonguse",
	} {
		if got := fedi.SuggestName(in); got != want {
			t.Errorf("%q: %q", in, got)
		}
	}
}

func TestAuthorizeURL(t *testing.T) {
	inst := fedi.Instance{Host: "social.example", Base: "https://social.example"}
	u, _ := url.Parse(fedi.AuthorizeURL(inst, "cid", "https://k.example/api/fedi/callback", "profile", "st", "ch"))
	q := u.Query()
	if u.Scheme != "https" || u.Host != "social.example" || u.Path != "/oauth/authorize" ||
		q.Get("client_id") != "cid" || q.Get("response_type") != "code" || q.Get("scope") != "profile" ||
		q.Get("state") != "st" || q.Get("code_challenge") != "ch" || q.Get("code_challenge_method") != "S256" ||
		q.Get("redirect_uri") != "https://k.example/api/fedi/callback" {
		t.Fatalf("url %s", u)
	}
}

// testClient talks to a fake instance on httptest's TLS server (loopback).
func testClient(srv *httptest.Server) *fedi.Client {
	return fedi.NewClient(func(netip.AddrPort) bool { return true }, srv.Client().Transport.(*http.Transport).TLSClientConfig, false)
}

func fakeInstance(t *testing.T, f *fedifake.Instance) (*httptest.Server, fedi.Instance) {
	srv := httptest.NewTLSServer(f)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	return srv, fedi.Instance{Host: u.Host, Base: srv.URL}
}

func TestTheFlowAgainstAFakeInstance(t *testing.T) {
	ctx := context.Background()
	for _, old := range []bool{false, true} {
		f := &fedifake.Instance{NoProfileScope: old}
		srv, inst := fakeInstance(t, f)
		c := testClient(srv)
		redirect := "https://k.example/api/fedi/callback"
		app, err := c.RegisterApp(ctx, inst, "Kallisti", redirect, "https://k.example")
		if err != nil || app.ClientID == "" || app.ClientSecret == "" {
			t.Fatalf("register: %+v %v", app, err)
		}
		scope := fedi.Scope(app.Scopes, false)
		if want := map[bool]string{false: "profile", true: "read:accounts"}[old]; scope != want {
			t.Fatalf("old %v: scope %q", old, scope)
		}
		verifier := fedi.NewVerifier()
		callback, err := f.Approve(fedi.AuthorizeURL(inst, app.ClientID, redirect, scope, "st", fedi.Challenge(verifier)), "alice")
		if err != nil {
			t.Fatal(err)
		}
		cb, _ := url.Parse(callback)
		if cb.Query().Get("state") != "st" {
			t.Fatalf("callback %s", callback)
		}
		// A wrong verifier gets no token (PKCE).
		if _, err := c.Token(ctx, inst, app, redirect, cb.Query().Get("code"), fedi.NewVerifier()); err == nil {
			t.Fatal("token without the right verifier")
		}
		callback, _ = f.Approve(fedi.AuthorizeURL(inst, app.ClientID, redirect, scope, "st", fedi.Challenge(verifier)), "alice")
		cb, _ = url.Parse(callback)
		token, err := c.Token(ctx, inst, app, redirect, cb.Query().Get("code"), verifier)
		if err != nil {
			t.Fatal(err)
		}
		acc, err := c.VerifyCredentials(ctx, inst, token)
		if err != nil || acc.ID == "" || acc.Username != "alice" || acc.Handle(inst) != "@alice@"+inst.Host {
			t.Fatalf("verify: %+v %v", acc, err)
		}
	}
}

func TestOddAnswersAreRefused(t *testing.T) {
	ctx := context.Background()
	for name, h := range map[string]http.HandlerFunc{
		"not json": func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html>")) },
		"too big": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"id":"1","username":"` + strings.Repeat("a", 200_000) + `"}`))
		},
		"no id":        func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"username":"alice"}`)) },
		"null id":      func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"id":null,"username":"alice"}`)) },
		"float id":     func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"id":1e3,"username":"alice"}`)) },
		"bool id":      func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"id":true,"username":"alice"}`)) },
		"bad username": func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"id":"1","username":"a b<script>"}`)) },
		"error status": func(w http.ResponseWriter, r *http.Request) { http.Error(w, `{"id":"1","username":"a"}`, 500) },
	} {
		srv := httptest.NewTLSServer(h)
		u, _ := url.Parse(srv.URL)
		if acc, err := testClient(srv).VerifyCredentials(ctx, fedi.Instance{Host: u.Host, Base: srv.URL}, "t"); err == nil {
			t.Errorf("%s: %+v", name, acc)
		}
		srv.Close()
	}
	// Numeric ids (some servers) are fine.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":17,"username":"bob"}`))
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	if acc, err := testClient(srv).VerifyCredentials(ctx, fedi.Instance{Host: u.Host, Base: srv.URL}, "t"); err != nil || acc.ID != "17" {
		t.Fatalf("numeric id: %+v %v", acc, err)
	}
}

func TestTheGuardRefusesLocalAddresses(t *testing.T) {
	f := &fedifake.Instance{}
	srv, inst := fakeInstance(t, f)
	// The production guard, with the test server's certificate trusted: the
	// loopback address is refused when dialed.
	c := fedi.NewClient(nil, srv.Client().Transport.(*http.Transport).TLSClientConfig, false)
	_, err := c.RegisterApp(context.Background(), inst, "Kallisti", "https://k.example/cb", "")
	if err == nil || !errors.Is(err, fedi.ErrUnreachable) {
		t.Fatalf("loopback: %v", err)
	}
	if f.Requests() != 0 {
		t.Fatal("the request reached the server")
	}
}

func TestRedirectsStayOnHTTPS(t *testing.T) {
	plain := httptest.NewServer(&fedifake.Instance{})
	defer plain.Close()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plain.URL+r.URL.Path, http.StatusFound)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	if _, err := testClient(srv).VerifyCredentials(context.Background(), fedi.Instance{Host: u.Host, Base: srv.URL}, "t"); err == nil {
		t.Fatal("followed a redirect to http")
	}
}
