// Package auth handles accounts: create, log in, log out, and finding the
// account behind a request (session cookie).
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"kallisti.chat/server/internal/password"
	"log/slog"
	"mime"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
	"golang.org/x/time/rate"

	"kallisti.chat/server/internal/certs"
	"kallisti.chat/server/internal/clientip"
	"kallisti.chat/server/internal/config"
	"kallisti.chat/server/internal/names"
	"kallisti.chat/server/internal/store"
)

const (
	CookieName      = "kallisti_session"
	sessionLifetime = 30 * 24 * time.Hour
	// Account creation and login attempts per client IP.
	perMinute = 10
	burst     = 10
)

type Service struct {
	Store             *store.Store
	Now               func() time.Time
	AllowedOrigins    []string
	TrustForwardedFor bool
	// NameTaken reports names (as names.Key) used elsewhere, for example by
	// users registered on the Mumble server. Nil: none.
	NameTaken func(key string) bool
	// MumbleLogin checks a Mumble registration's password (built-in server)
	// for names without a web account. Such logins get no session: the
	// client connects with the name and password. Nil: none.
	MumbleLogin func(host netip.Addr, name, password string) (string, bool)

	mu     sync.Mutex
	limits map[string]*rate.Limiter
}

type credentials struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (a *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.cors(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method == http.MethodPost {
		// Against cross-site forms (CSRF): a form cannot send JSON without a
		// CORS preflight, and browsers mark cross-site requests.
		ctype, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if ctype != "application/json" {
			writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "send JSON"})
			return
		}
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "cross-site requests are not allowed"})
			return
		}
	}
	switch {
	case r.URL.Path == "/api/accounts" && r.Method == http.MethodPost:
		a.create(w, r)
	case r.URL.Path == "/api/login" && r.Method == http.MethodPost:
		a.login(w, r)
	case r.URL.Path == "/api/logout" && r.Method == http.MethodPost:
		a.logout(w, r)
	case r.URL.Path == "/api/logout-all" && r.Method == http.MethodPost:
		a.logoutAll(w, r)
	case r.URL.Path == "/api/password" && r.Method == http.MethodPost:
		a.changePassword(w, r)
	case r.URL.Path == "/api/me" && r.Method == http.MethodGet:
		if acc, ok := a.AccountFor(r); ok {
			writeJSON(w, http.StatusOK, map[string]string{"name": acc.Name})
		} else {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not logged in"})
		}
	default:
		http.NotFound(w, r)
	}
}

// AccountFor returns the account of a request's session cookie.
func (a *Service) AccountFor(r *http.Request) (store.Account, bool) {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return store.Account{}, false
	}
	acc, err := a.Store.AccountBySession(tokenHash(c.Value), a.Now())
	return acc, err == nil
}

func (a *Service) create(w http.ResponseWriter, r *http.Request) {
	if !a.allow(r) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many attempts, try again later"})
		return
	}
	var c credentials
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&c) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	name := norm.NFC.String(strings.TrimSpace(c.Name))
	if msg := names.Check(name); msg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	if a.NameTaken != nil && a.NameTaken(names.Key(name)) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "this name is already taken"})
		return
	}
	if n := utf8.RuneCountInString(c.Password); n < 8 || n > 256 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "the password needs 8 to 256 characters"})
		return
	}
	release := password.Limit()
	hash, err := password.Hash(c.Password)
	release()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cannot create the account"})
		return
	}
	certPEM, keyPEM, err := certs.Generate(name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cannot create the account"})
		return
	}
	acc, err := a.Store.CreateAccount(store.Account{Name: name, PasswordHash: hash, CertPEM: certPEM, KeyPEM: keyPEM}, a.Now())
	if errors.Is(err, store.ErrNameTaken) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "this name is already taken"})
		return
	}
	if err != nil {
		slog.Error("create account", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cannot create the account"})
		return
	}
	if !a.startSession(w, r, acc) {
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": acc.Name})
}

func (a *Service) login(w http.ResponseWriter, r *http.Request) {
	if !a.allow(r) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many attempts, try again later"})
		return
	}
	var c credentials
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&c) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	acc, err := a.Store.AccountByName(c.Name)
	hash := acc.PasswordHash
	if err != nil {
		hash = password.Dummy // same work for unknown names
	}
	release := password.Limit()
	ok := password.Check(hash, c.Password)
	release()
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		slog.Error("login", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "please try again later"})
		return
	}
	if err != nil && a.MumbleLogin != nil {
		host, _ := netip.ParseAddr(clientip.Of(r, a.TrustForwardedFor))
		if name, ok := a.MumbleLogin(host.Unmap(), c.Name, c.Password); ok {
			// Not a web account: end a web session of this browser, or the
			// bridge would connect with that account instead.
			clearSession(w, r, a.Store)
			writeJSON(w, http.StatusOK, map[string]any{"name": name, "mumble": true})
			return
		}
	}
	if !ok || err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "wrong name or password"})
		return
	}
	if !a.startSession(w, r, acc) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": acc.Name})
}

func (a *Service) logout(w http.ResponseWriter, r *http.Request) {
	clearSession(w, r, a.Store)
	w.WriteHeader(http.StatusNoContent)
}

func clearSession(w http.ResponseWriter, r *http.Request, s *store.Store) {
	if c, err := r.Cookie(CookieName); err == nil {
		s.DeleteSession(tokenHash(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

// logoutAll ends every session of the account, this one too.
func (a *Service) logoutAll(w http.ResponseWriter, r *http.Request) {
	acc, ok := a.AccountFor(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not logged in"})
		return
	}
	if err := a.Store.DeleteSessionsExcept(acc.ID, ""); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cannot log out"})
		return
	}
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

// changePassword needs the old password and ends all other sessions.
func (a *Service) changePassword(w http.ResponseWriter, r *http.Request) {
	acc, ok := a.AccountFor(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not logged in"})
		return
	}
	if !a.allow(r) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many attempts, try again later"})
		return
	}
	var c struct {
		Old string `json:"old"`
		New string `json:"new"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&c) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if n := utf8.RuneCountInString(c.New); n < 8 || n > 256 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "the password needs 8 to 256 characters"})
		return
	}
	release := password.Limit()
	ok = password.Check(acc.PasswordHash, c.Old)
	var hash string
	var err error
	if ok {
		hash, err = password.Hash(c.New)
	}
	release()
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "the current password is wrong"})
		return
	}
	cookie, _ := r.Cookie(CookieName)
	if err != nil || a.Store.SetPassword(acc.ID, hash) != nil || a.Store.DeleteSessionsExcept(acc.ID, tokenHash(cookie.Value)) != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cannot change the password"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *Service) startSession(w http.ResponseWriter, r *http.Request, acc store.Account) bool {
	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)
	if err := a.Store.CreateSession(tokenHash(token), acc.ID, a.Now().Add(sessionLifetime)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cannot log in"})
		return false
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionLifetime.Seconds()),
		HttpOnly: true,
		Secure:   a.secure(r),
		SameSite: http.SameSiteLaxMode,
	})
	return true
}

func (a *Service) secure(r *http.Request) bool {
	return r.TLS != nil || (a.TrustForwardedFor && r.Header.Get("X-Forwarded-Proto") == "https")
}

// cors allows the web app's origin to call the API with its cookie. With
// ALLOWED_ORIGINS=* (development) only localhost origins get credentials:
// a production server set to * by mistake is not open to every site.
func (a *Service) cors(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Vary", "Origin")
	origin := r.Header.Get("Origin")
	if origin == "" || !config.OriginAllowed(a.AllowedOrigins, origin) {
		return
	}
	if !slices.Contains(a.AllowedOrigins, origin) && !isLocalhost(origin) {
		return
	}
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", origin)
	h.Set("Access-Control-Allow-Credentials", "true")
	h.Set("Access-Control-Allow-Methods", "GET, POST")
	h.Set("Access-Control-Allow-Headers", "Content-Type")
}

func (a *Service) allow(r *http.Request) bool {
	key := limiterKey(clientip.Of(r, a.TrustForwardedFor))
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.limits == nil {
		a.limits = map[string]*rate.Limiter{}
	}
	l, ok := a.limits[key]
	if !ok {
		if len(a.limits) > 10_000 {
			// Keep memory bounded: drop idle limiters (full again), not all.
			for k, v := range a.limits {
				if v.Tokens() >= burst {
					delete(a.limits, k)
				}
			}
		}
		l = rate.NewLimiter(rate.Limit(perMinute)/60, burst)
		a.limits[key] = l
	}
	return l.Allow()
}

// limiterKey: one IPv6 user usually has a whole /64.
func limiterKey(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil || addr.Is4() || addr.Is4In6() {
		return ip
	}
	prefix, _ := addr.Prefix(64)
	return prefix.String()
}

func isLocalhost(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := u.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
