// Package fedi talks to fediverse instances through the Mastodon API
// (Mastodon, Pleroma/Akkoma, GoToSocial, Pixelfed, Friendica, ...) for
// fediverse logins: register an app, OAuth (authorization code with PKCE),
// and read the logged-in account. Instances are named by users, so every
// request goes through a guarded client (see NewClient).
package fedi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"golang.org/x/net/idna"

	"kallisti.chat/server/internal/avatarimg"
	"kallisti.chat/server/internal/names"
	"kallisti.chat/server/internal/netguard"
)

const (
	timeout      = 10 * time.Second
	maxRedirects = 3
	maxBody      = 64 << 10
	// maxAvatar: the most bytes of an avatar picture downloaded.
	maxAvatar = 2 << 20
)

var (
	ErrInstance = errors.New("this is not a fediverse server address")
	// ErrUnreachable: no connection, a refused address, or not an answer of
	// the Mastodon API.
	ErrUnreachable = errors.New("cannot reach this server, or it does not offer Mastodon logins")
)

// Instance is a fediverse server: its host (lower-case, ASCII; the key of
// its accounts) and the base URL of its API.
type Instance struct{ Host, Base string }

// ParseInstance reads what a user typed: a host, a URL on it, or a handle
// (@user@host). dev (FEDI_INSECURE_DEV_INSTANCES) also takes http:// and
// local hosts with ports, for test instances.
func ParseInstance(input string, dev bool) (Instance, error) {
	s := strings.TrimSpace(input)
	if !strings.Contains(s, "/") && strings.Contains(s, "@") {
		s = s[strings.LastIndex(s, "@")+1:] // a handle
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" || u.Scheme != "https" && !(dev && u.Scheme == "http") {
		return Instance{}, ErrInstance
	}
	scheme, host, port := u.Scheme, u.Hostname(), u.Port()
	if dev {
		if port != "" {
			host = net.JoinHostPort(host, port)
		}
		return Instance{Host: strings.ToLower(host), Base: scheme + "://" + strings.ToLower(host)}, nil
	}
	if port != "" && port != "443" {
		return Instance{}, ErrInstance
	}
	host, err = idna.Lookup.ToASCII(strings.TrimSuffix(host, "."))
	if err != nil || !strings.Contains(host, ".") || strings.Contains(host, "..") || net.ParseIP(host) != nil || len(host) > 253 {
		return Instance{}, ErrInstance
	}
	return Instance{Host: host, Base: "https://" + host}, nil
}

// NewVerifier is a PKCE code verifier (RFC 7636): 43 characters.
func NewVerifier() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// Challenge is the verifier's S256 code challenge.
func Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// SuggestName makes a Kallisti name from a remote user name: only allowed
// characters, at most 32; "" when nothing usable is left.
func SuggestName(username string) string {
	var b strings.Builder
	n := 0
	for _, r := range username {
		if n == 32 {
			break
		}
		if names.AllowedRune(r) {
			b.WriteRune(r)
			n++
		}
	}
	if s := b.String(); names.Check(s) == "" {
		return s
	}
	return ""
}

// Scopes: "profile" (Mastodon 4.3 and later) gives only the account; older
// servers know "read:accounts". Apps are registered with both where the
// server allows it.
const (
	scopeProfile  = "profile"
	scopeAccounts = "read:accounts"
)

// Scope is the scope to ask for with an app's scopes; fallback after an
// instance refused "profile".
func Scope(appScopes string, fallback bool) string {
	for _, s := range strings.Fields(appScopes) {
		if s == scopeProfile && !fallback {
			return scopeProfile
		}
	}
	return scopeAccounts
}

// AuthorizeURL is where the user approves the login, on the instance.
func AuthorizeURL(inst Instance, clientID, redirectURI, scope, state, challenge string) string {
	q := url.Values{
		"client_id": {clientID}, "redirect_uri": {redirectURI}, "response_type": {"code"}, "scope": {scope},
		"state": {state}, "code_challenge": {challenge}, "code_challenge_method": {"S256"},
	}
	return inst.Base + "/oauth/authorize?" + q.Encode()
}

// Client makes the requests to instances.
type Client struct {
	http      *http.Client
	allowHTTP bool
}

// publicHTTPS: production instances, on public addresses and port 443.
func publicHTTPS(ap netip.AddrPort) bool { return ap.Port() == 443 && netguard.Public(ap.Addr()) }

// NewClient: allow decides the addresses dialed (nil: public ones on port
// 443), after DNS and on every redirect. Tests allow their local server
// (and give its certificate in tlsConfig); allowHTTP is for development
// only (FEDI_INSECURE_DEV_INSTANCES). No proxy: the guard must see the
// instance's address.
func NewClient(allow func(netip.AddrPort) bool, tlsConfig *tls.Config, allowHTTP bool) *Client {
	if allow == nil {
		allow = publicHTTPS
	}
	dialer := &net.Dialer{Timeout: timeout, Control: netguard.Control(allow)}
	c := &Client{allowHTTP: allowHTTP}
	c.http = &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:                  nil,
			DialContext:            dialer.DialContext,
			TLSClientConfig:        tlsConfig,
			DisableKeepAlives:      true,
			TLSHandshakeTimeout:    timeout,
			ResponseHeaderTimeout:  timeout,
			MaxResponseHeaderBytes: 64 << 10,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > maxRedirects {
				return errors.New("too many redirects")
			}
			if req.URL.Scheme != "https" && !(c.allowHTTP && req.URL.Scheme == "http") {
				return errors.New("redirect away from https")
			}
			return nil
		},
	}
	return c
}

// do sends a request and reads a JSON answer (at most maxBody) into v.
func (c *Client) do(req *http.Request, v any) error {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Kallisti")
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxBody+1))
	if err != nil || len(body) > maxBody {
		return fmt.Errorf("%w: answer too large or cut", ErrUnreachable)
	}
	if res.StatusCode != http.StatusOK {
		return &StatusError{Code: res.StatusCode}
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	return nil
}

// StatusError: the instance answered with an error status.
type StatusError struct{ Code int }

func (e *StatusError) Error() string { return fmt.Sprintf("the server answered %d", e.Code) }

func (c *Client) post(ctx context.Context, u string, form url.Values, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return ErrInstance
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.do(req, v)
}

// App is our app on an instance.
type App struct{ ClientID, ClientSecret, Scopes string }

// RegisterApp registers the app, with "profile" where the server knows it.
func (c *Client) RegisterApp(ctx context.Context, inst Instance, name, redirectURI, website string) (App, error) {
	var err error
	for _, scopes := range []string{scopeAccounts + " " + scopeProfile, scopeAccounts} {
		var a struct {
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
		}
		form := url.Values{"client_name": {name}, "redirect_uris": {redirectURI}, "scopes": {scopes}}
		if website != "" {
			form.Set("website", website)
		}
		err = c.post(ctx, inst.Base+"/api/v1/apps", form, &a)
		var status *StatusError
		if errors.As(err, &status) && (status.Code == http.StatusBadRequest || status.Code == http.StatusUnprocessableEntity) {
			continue // an older server: without "profile"
		}
		if err != nil {
			return App{}, err
		}
		if a.ClientID == "" || a.ClientSecret == "" || len(a.ClientID) > 512 || len(a.ClientSecret) > 512 {
			return App{}, fmt.Errorf("%w: no app credentials", ErrUnreachable)
		}
		return App{ClientID: a.ClientID, ClientSecret: a.ClientSecret, Scopes: scopes}, nil
	}
	return App{}, fmt.Errorf("%w: %w", ErrUnreachable, err)
}

// Token trades the code from the callback for an access token.
func (c *Client) Token(ctx context.Context, inst Instance, app App, redirectURI, code, verifier string) (string, error) {
	var t struct {
		AccessToken string `json:"access_token"`
	}
	err := c.post(ctx, inst.Base+"/oauth/token", url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "client_id": {app.ClientID}, "client_secret": {app.ClientSecret},
		"redirect_uri": {redirectURI}, "code_verifier": {verifier},
	}, &t)
	if err == nil && t.AccessToken == "" {
		err = fmt.Errorf("%w: no access token", ErrUnreachable)
	}
	return t.AccessToken, err
}

// Account is the logged-in remote account.
type Account struct {
	ID       string // the instance's id: the key, with the host
	Username string
	Avatar   string // the URL of its avatar picture ("": none, see avatarURL)
}

// Handle is @user@host, to show.
func (a Account) Handle(inst Instance) string { return "@" + a.Username + "@" + inst.Host }

var (
	remoteID  = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)
	numericID = regexp.MustCompile(`^[0-9]{1,20}$`)
)

// VerifyCredentials reads the account the token belongs to.
func (c *Client) VerifyCredentials(ctx context.Context, inst Instance, token string) (Account, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, inst.Base+"/api/v1/accounts/verify_credentials", nil)
	if err != nil {
		return Account{}, ErrInstance
	}
	req.Header.Set("Authorization", "Bearer "+token)
	var a struct {
		ID           json.RawMessage `json:"id"`
		Username     string          `json:"username"`
		Avatar       string          `json:"avatar"`
		AvatarStatic string          `json:"avatar_static"` // not animated
	}
	if err := c.do(req, &a); err != nil {
		return Account{}, err
	}
	var id string // a string, or a number on some servers
	if json.Unmarshal(a.ID, &id) != nil && numericID.Match(a.ID) {
		id = string(a.ID)
	}
	if !remoteID.MatchString(id) || !validUsername(a.Username) {
		return Account{}, fmt.Errorf("%w: unexpected account", ErrUnreachable)
	}
	avatar := a.AvatarStatic
	if avatar == "" {
		avatar = a.Avatar
	}
	return Account{ID: id, Username: a.Username, Avatar: c.avatarURL(avatar)}, nil
}

// avatarURL is an avatar's URL if it can be downloaded (https) and is not
// an instance's default picture for accounts without one; else "".
func (c *Client) avatarURL(s string) string {
	u, err := url.Parse(s)
	if err != nil || u.Host == "" || u.Scheme != "https" && !(c.allowHTTP && u.Scheme == "http") ||
		strings.HasSuffix(u.Path, "/missing.png") || u.Path == "/images/avi.png" { // Mastodon's, Pleroma's
		return ""
	}
	return u.String()
}

// Avatar downloads an account's avatar picture (at most maxAvatar bytes)
// and makes an avatar of it (see avatarimg.Make).
func (c *Client) Avatar(ctx context.Context, avatarURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, avatarURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Kallisti")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, &StatusError{Code: res.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxAvatar+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxAvatar {
		return nil, avatarimg.ErrTooLarge
	}
	return avatarimg.Make(body)
}

// validUsername: user names as instances make them (letters, digits, _ . -),
// nothing to mislead in a handle (spaces, @, control or format characters).
func validUsername(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("_.-", r) {
			return false
		}
	}
	return true
}
