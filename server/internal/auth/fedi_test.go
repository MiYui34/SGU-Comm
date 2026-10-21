package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"

	"kallisti.chat/server/internal/fedi"
	"kallisti.chat/server/internal/fedi/fedifake"
)

// home is the web app's address (the server's public one) in fediSetup.
var home string

// fediSetup: a server with fediverse logins and a fake instance (on
// loopback, which only tests allow).
func fediSetup(t *testing.T, f *fedifake.Instance) (*Service, *httptest.Server, string) {
	t.Helper()
	a, srv := setup(t)
	inst := httptest.NewTLSServer(f)
	t.Cleanup(inst.Close)
	client := fedi.NewClient(func(netip.AddrPort) bool { return true }, inst.Client().Transport.(*http.Transport).TLSClientConfig, false)
	a.Fedi = &FediLogin{Client: client, Public: srv.URL, AppName: "Kallisti", Dev: true}
	home = srv.URL + "/"
	return a, srv, inst.URL
}

// browser keeps cookies and does not follow redirects.
type browser struct {
	t   *testing.T
	srv *httptest.Server
	c   *http.Client
}

func newBrowser(t *testing.T, srv *httptest.Server) *browser {
	jar, _ := cookiejar.New(nil)
	return &browser{t, srv, &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (b *browser) do(method, path string, body any) (*http.Response, map[string]any) {
	b.t.Helper()
	var rd *bytes.Reader
	if body != nil {
		j, _ := json.Marshal(body)
		rd = bytes.NewReader(j)
	} else {
		rd = bytes.NewReader(nil)
	}
	u := path
	if !strings.HasPrefix(path, "http") {
		u = b.srv.URL + path
	}
	req, _ := http.NewRequest(method, u, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := b.c.Do(req)
	if err != nil {
		b.t.Fatal(err)
	}
	defer resp.Body.Close()
	var v map[string]any
	json.NewDecoder(resp.Body).Decode(&v)
	return resp, v
}

// fediFlow starts a login (or a link) on the instance and approves it as
// user; the callback's answer.
func (b *browser) fediFlow(f *fedifake.Instance, start map[string]any, user string) *http.Response {
	b.t.Helper()
	resp, v := b.do("POST", "/api/fedi/start", start)
	if resp.StatusCode != http.StatusOK {
		b.t.Fatalf("start: %d %v", resp.StatusCode, v)
	}
	callback, err := f.Approve(v["url"].(string), user)
	if err != nil {
		b.t.Fatal(err)
	}
	resp, _ = b.do("GET", callback, nil)
	return resp
}

func wantRedirect(t *testing.T, resp *http.Response, want string) {
	t.Helper()
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != want {
		t.Fatalf("redirect: %d %q, want %q", resp.StatusCode, resp.Header.Get("Location"), want)
	}
}

func TestFediRegisterAndLogIn(t *testing.T) {
	f := &fedifake.Instance{}
	_, srv, inst := fediSetup(t, f)
	host := strings.TrimPrefix(inst, "https://")
	b := newBrowser(t, srv)
	if resp, v := b.do("GET", "/api/fedi", nil); resp.StatusCode != 200 || v["enabled"] != true {
		t.Fatalf("status: %d %v", resp.StatusCode, v)
	}
	wantRedirect(t, b.fediFlow(f, map[string]any{"instance": inst}, "Alice"), home+"?fedi=register")
	resp, v := b.do("GET", "/api/fedi/signup", nil)
	if resp.StatusCode != 200 || v["name"] != "Alice" || v["handle"] != "@Alice@"+host {
		t.Fatalf("signup: %d %v", resp.StatusCode, v)
	}
	if resp, _ := b.do("POST", "/api/fedi/signup", map[string]any{"name": "a b"}); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad name: %d", resp.StatusCode)
	}
	resp, v = b.do("POST", "/api/fedi/signup", map[string]any{"name": "alice"})
	if resp.StatusCode != http.StatusCreated || v["name"] != "alice" || sessionCookie(resp) == nil {
		t.Fatalf("create: %d %v", resp.StatusCode, v)
	}
	resp, v = b.do("GET", "/api/me", nil)
	if v["name"] != "alice" || v["password"] != false || v["fedi"].(map[string]any)["handle"] != "@Alice@"+host {
		t.Fatalf("me: %v", v)
	}
	// The sign-up is used up.
	if resp, _ := b.do("POST", "/api/fedi/signup", map[string]any{"name": "alice2"}); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("second sign-up: %d", resp.StatusCode)
	}
	// No password, no password login.
	if resp, _ := b.do("POST", "/api/login", map[string]any{"name": "alice", "password": ""}); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("empty password: %d", resp.StatusCode)
	}

	// Another browser logs in with the same remote account.
	b2 := newBrowser(t, srv)
	resp = b2.fediFlow(f, map[string]any{"instance": inst}, "Alice")
	wantRedirect(t, resp, home)
	if sessionCookie(resp) == nil {
		t.Fatal("no session")
	}
	if _, v := b2.do("GET", "/api/me", nil); v["name"] != "alice" {
		t.Fatalf("me after fedi login: %v", v)
	}
}

func TestFediStateIsChecked(t *testing.T) {
	f := &fedifake.Instance{}
	_, srv, inst := fediSetup(t, f)
	b := newBrowser(t, srv)
	_, v := b.do("POST", "/api/fedi/start", map[string]any{"instance": inst})
	callback, _ := f.Approve(v["url"].(string), "mallory")

	// Another browser (no state cookie) gets nothing from the callback: an
	// attacker cannot log a victim in to the attacker's account.
	other := newBrowser(t, srv)
	resp, _ := other.do("GET", callback, nil)
	wantRedirect(t, resp, home+"?fedi=error&reason=expired")
	if sessionCookie(resp) != nil {
		t.Fatal("a session without the state cookie")
	}
	// A wrong state in the right browser fails too, and ends the flow.
	cb, _ := url.Parse(callback)
	q := cb.Query()
	q.Set("state", "wrong")
	resp, _ = b.do("GET", srv.URL+"/api/fedi/callback?"+q.Encode(), nil)
	wantRedirect(t, resp, home+"?fedi=error&reason=expired")
	resp, _ = b.do("GET", callback, nil)
	wantRedirect(t, resp, home+"?fedi=error&reason=expired")

	// The user said no on the instance.
	_, v = b.do("POST", "/api/fedi/start", map[string]any{"instance": inst})
	u, _ := url.Parse(v["url"].(string))
	resp, _ = b.do("GET", "/api/fedi/callback?error=access_denied&state="+u.Query().Get("state"), nil)
	wantRedirect(t, resp, home+"?fedi=error&reason=denied")
}

func TestFediStartRefusesBadInstances(t *testing.T) {
	a, srv := setup(t)
	a.Fedi = &FediLogin{Client: fedi.NewClient(nil, nil, false), Public: "https://app.example", AppName: "Kallisti"}
	b := newBrowser(t, srv)
	for _, inst := range []string{"", "localhost", "http://social.example", "127.0.0.1"} {
		if resp, _ := b.do("POST", "/api/fedi/start", map[string]any{"instance": inst}); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%q: %d", inst, resp.StatusCode)
		}
	}
	// Off: no fediverse routes.
	a.Fedi = nil
	if resp, _ := b.do("GET", "/api/fedi", nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("off: %d", resp.StatusCode)
	}
}

func TestFediScopeFallback(t *testing.T) {
	f := &fedifake.Instance{ProfileDenied: true}
	_, srv, inst := fediSetup(t, f)
	b := newBrowser(t, srv)
	_, v := b.do("POST", "/api/fedi/start", map[string]any{"instance": inst})
	callback, _ := f.Approve(v["url"].(string), "carol")
	// invalid_scope: back to the instance, with read:accounts.
	resp, _ := b.do("GET", callback, nil)
	next, _ := url.Parse(resp.Header.Get("Location"))
	if resp.StatusCode != http.StatusFound || next.Query().Get("scope") != "read:accounts" {
		t.Fatalf("fallback: %d %s", resp.StatusCode, next)
	}
	callback, err := f.Approve(next.String(), "carol")
	if err != nil {
		t.Fatal(err)
	}
	resp, _ = b.do("GET", callback, nil)
	wantRedirect(t, resp, home+"?fedi=register")
}

func TestFediLinkAndUnlink(t *testing.T) {
	f := &fedifake.Instance{}
	_, srv, inst := fediSetup(t, f)
	b := newBrowser(t, srv)
	b.do("POST", "/api/accounts", map[string]any{"name": "bob", "password": "longpassword"})

	// Linking needs the current password (a stolen session cannot add a
	// way in) and a login.
	if resp, _ := b.do("POST", "/api/fedi/start", map[string]any{"instance": inst, "link": true, "password": "wrong password"}); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong password: %d", resp.StatusCode)
	}
	if resp, _ := newBrowser(t, srv).do("POST", "/api/fedi/start", map[string]any{"instance": inst, "link": true}); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("link without login: %d", resp.StatusCode)
	}
	link := map[string]any{"instance": inst, "link": true, "password": "longpassword"}
	wantRedirect(t, b.fediFlow(f, link, "bobby"), home+"?fedi=linked")
	if _, v := b.do("GET", "/api/me", nil); v["fedi"] == nil || v["password"] != true {
		t.Fatalf("me: %v", v)
	}
	// The remote account logs in to bob now.
	b2 := newBrowser(t, srv)
	wantRedirect(t, b2.fediFlow(f, map[string]any{"instance": inst}, "bobby"), home)
	if _, v := b2.do("GET", "/api/me", nil); v["name"] != "bob" {
		t.Fatalf("login after link: %v", v)
	}

	// A remote account linked elsewhere cannot be linked again.
	c := newBrowser(t, srv)
	c.do("POST", "/api/accounts", map[string]any{"name": "carol", "password": "longpassword"})
	wantRedirect(t, c.fediFlow(f, map[string]any{"instance": inst, "link": true, "password": "longpassword"}, "bobby"), home+"?fedi=error&reason=taken")

	// A link started in one session does not finish in another one.
	_, v := c.do("POST", "/api/fedi/start", map[string]any{"instance": inst, "link": true, "password": "longpassword"})
	callback, _ := f.Approve(v["url"].(string), "carol")
	c.do("POST", "/api/logout", map[string]any{})
	resp, _ := c.do("GET", callback, nil)
	wantRedirect(t, resp, home+"?fedi=error&reason=expired")

	if resp, _ := b.do("POST", "/api/fedi/unlink", map[string]any{}); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("unlink: %d", resp.StatusCode)
	}
}

// Unlink, but not the last way to log in.
func TestFediUnlinkKeepsAWayToLogIn(t *testing.T) {
	f := &fedifake.Instance{}
	_, srv, inst := fediSetup(t, f)
	d := newBrowser(t, srv)
	wantRedirect(t, d.fediFlow(f, map[string]any{"instance": inst}, "dave"), home+"?fedi=register")
	d.do("POST", "/api/fedi/signup", map[string]any{"name": "dave"})
	if resp, v := d.do("POST", "/api/fedi/unlink", map[string]any{}); resp.StatusCode != http.StatusConflict {
		t.Fatalf("unlink the only login: %d %v", resp.StatusCode, v)
	}
	// A first password needs no old one (but a fresh fediverse login).
	wantRedirect(t, d.fediFlow(f, map[string]any{"instance": inst, "purpose": "reauth"}, "dave"), home+"?fedi=reauth")
	if resp, v := d.do("POST", "/api/password", map[string]any{"old": "", "new": "davespassword"}); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("first password: %d %v", resp.StatusCode, v)
	}
	if resp, _ := d.do("POST", "/api/fedi/unlink", map[string]any{}); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("unlink with a password: %d", resp.StatusCode)
	}
	if resp, _ := d.do("POST", "/api/login", map[string]any{"name": "dave", "password": "davespassword"}); resp.StatusCode != http.StatusOK {
		t.Fatalf("password login: %d", resp.StatusCode)
	}
}

func TestFediGuestAccountsLinkAndStay(t *testing.T) {
	f := &fedifake.Instance{}
	a, srv, inst := fediSetup(t, f)
	a.GuestAccounts = true
	var kept string
	a.OnPermanent = func(name string) { kept = name }
	b := newBrowser(t, srv)
	b.do("POST", "/api/guest", map[string]any{"name": "visitor"})
	wantRedirect(t, b.fediFlow(f, map[string]any{"instance": inst, "link": true}, "visitor"), home+"?fedi=linked")
	if _, v := b.do("GET", "/api/me", nil); v["temporary"] != false || kept != "visitor" {
		t.Fatalf("guest after link: %v %q", v, kept)
	}
}

func TestFediHandleIsOptIn(t *testing.T) {
	f := &fedifake.Instance{}
	_, srv, inst := fediSetup(t, f)
	host := strings.TrimPrefix(inst, "https://")
	b := newBrowser(t, srv)
	b.fediFlow(f, map[string]any{"instance": inst}, "erin")
	b.do("POST", "/api/fedi/signup", map[string]any{"name": "erin"})
	other := newBrowser(t, srv)
	if _, v := other.do("GET", "/api/fedi/handle?name=Erin", nil); v["handle"] != "" {
		t.Fatalf("before opting in: %v", v)
	}
	if resp, _ := other.do("POST", "/api/fedi/public", map[string]any{"on": true}); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("public without login: %d", resp.StatusCode)
	}
	if resp, _ := b.do("POST", "/api/fedi/public", map[string]any{"on": true}); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("public: %d", resp.StatusCode)
	}
	if _, v := other.do("GET", "/api/fedi/handle?name=Erin", nil); v["handle"] != "@erin@"+host {
		t.Fatalf("after opting in: %v", v)
	}
	if _, v := b.do("GET", "/api/me", nil); v["fedi"].(map[string]any)["public"] != true {
		t.Fatalf("me: %v", v)
	}
	b.do("POST", "/api/fedi/public", map[string]any{"on": false})
	if _, v := other.do("GET", "/api/fedi/handle?name=Erin", nil); v["handle"] != "" {
		t.Fatalf("after opting out: %v", v)
	}
}

func TestFediSignupNeedsTheBrowserCheck(t *testing.T) {
	f := &fedifake.Instance{}
	a, srv, inst := fediSetup(t, f)
	a.Captcha = func() string { return "normal" }
	b := newBrowser(t, srv)
	b.fediFlow(f, map[string]any{"instance": inst}, "frank")
	if resp, _ := b.do("POST", "/api/fedi/signup", map[string]any{"name": "frank"}); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("without the check: %d", resp.StatusCode)
	}
	challenge, bits := getChallenge(t, srv.URL)
	resp, v := b.do("POST", "/api/fedi/signup", map[string]any{"name": "frank", "challenge": challenge, "nonce": solve(challenge, bits)})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("with the check: %d %v", resp.StatusCode, v)
	}
}

// Instances may delete apps nobody used: until a login goes through, an
// app is registered again after a while.
func TestFediAppsAreKeptOnceUsed(t *testing.T) {
	f := &fedifake.Instance{}
	a, srv, inst := fediSetup(t, f)
	now := time.Now()
	a.Now = func() time.Time { return now }
	b := newBrowser(t, srv)
	b.do("POST", "/api/fedi/start", map[string]any{"instance": inst})
	registered := f.Requests()
	b.do("POST", "/api/fedi/start", map[string]any{"instance": inst})
	if f.Requests() != registered {
		t.Fatal("registered again at once")
	}
	now = now.Add(2 * time.Hour)
	b.fediFlow(f, map[string]any{"instance": inst}, "gina") // registers again, then is used
	used := f.Requests()
	now = now.Add(48 * time.Hour)
	b.do("POST", "/api/fedi/start", map[string]any{"instance": inst})
	if f.Requests() != used || used <= registered+1 {
		t.Fatalf("requests: %d %d %d", registered, used, f.Requests())
	}
}

// The first password of a fediverse-only account needs a fresh login with
// the linked fediverse account, in the same session: a stolen session
// cannot set one and lock the owner out.
func TestFirstPasswordNeedsAFreshFediLogin(t *testing.T) {
	f := &fedifake.Instance{}
	_, srv, inst := fediSetup(t, f)
	d := newBrowser(t, srv)
	d.fediFlow(f, map[string]any{"instance": inst}, "dave")
	d.do("POST", "/api/fedi/signup", map[string]any{"name": "dave"})
	setPassword := func(b *browser) (*http.Response, map[string]any) {
		return b.do("POST", "/api/password", map[string]any{"old": "", "new": "davespassword"})
	}
	if resp, v := setPassword(d); resp.StatusCode != http.StatusForbidden || v["code"] != "reauth" {
		t.Fatalf("without a fresh login: %d %v", resp.StatusCode, v)
	}
	// Another fediverse account proves nothing.
	reauth := map[string]any{"instance": inst, "purpose": "reauth"}
	wantRedirect(t, d.fediFlow(f, reauth, "eve"), home+"?fedi=error&reason=other")
	if resp, _ := setPassword(d); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("after another account: %d", resp.StatusCode)
	}
	// The proof belongs to the session that got it.
	wantRedirect(t, d.fediFlow(f, reauth, "dave"), home+"?fedi=reauth")
	other := newBrowser(t, srv)
	other.fediFlow(f, map[string]any{"instance": inst}, "dave")
	if resp, _ := setPassword(other); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("another session: %d", resp.StatusCode)
	}
	if resp, v := setPassword(d); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("with a fresh login: %d %v", resp.StatusCode, v)
	}
}

func TestFediProofsAreSingleUse(t *testing.T) {
	var f FediLogin
	now := time.Now()
	f.prove("session", 1, now)
	if f.takeProof("session", 2, now) || f.takeProof("session", 1, now) {
		t.Fatal("a proof for another account, or used twice")
	}
	f.prove("session", 1, now)
	if !f.takeProof("session", 1, now) || f.takeProof("session", 1, now) {
		t.Fatal("not single use")
	}
}

func TestFediReauthExpiresAndNeedsALink(t *testing.T) {
	f := &fedifake.Instance{}
	a, srv, inst := fediSetup(t, f)
	now := time.Now()
	a.Now = func() time.Time { return now }
	d := newBrowser(t, srv)
	d.fediFlow(f, map[string]any{"instance": inst}, "dave")
	d.do("POST", "/api/fedi/signup", map[string]any{"name": "dave"})
	wantRedirect(t, d.fediFlow(f, map[string]any{"instance": inst, "purpose": "reauth"}, "dave"), home+"?fedi=reauth")
	now = now.Add(6 * time.Minute)
	if resp, v := d.do("POST", "/api/password", map[string]any{"new": "davespassword"}); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expired proof: %d %v", resp.StatusCode, v)
	}
	// Accounts with a password change it as before; a reauth needs a link.
	b := newBrowser(t, srv)
	b.do("POST", "/api/accounts", map[string]any{"name": "bob", "password": "longpassword"})
	if resp, _ := b.do("POST", "/api/fedi/start", map[string]any{"instance": inst, "purpose": "reauth"}); resp.StatusCode != http.StatusConflict {
		t.Fatalf("reauth without a link: %d", resp.StatusCode)
	}
	if resp, _ := b.do("POST", "/api/password", map[string]any{"old": "longpassword", "new": "newpassword"}); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("password change: %d", resp.StatusCode)
	}
}
