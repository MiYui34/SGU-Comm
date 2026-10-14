// Package auth handles accounts: create, log in, log out, and finding the
// account behind a request (session cookie).
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/time/rate"

	"kallisti.chat/server/internal/certs"
	"kallisti.chat/server/internal/clientip"
	"kallisti.chat/server/internal/config"
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
	switch {
	case r.URL.Path == "/api/accounts" && r.Method == http.MethodPost:
		a.create(w, r)
	case r.URL.Path == "/api/login" && r.Method == http.MethodPost:
		a.login(w, r)
	case r.URL.Path == "/api/logout" && r.Method == http.MethodPost:
		a.logout(w, r)
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
	name := strings.TrimSpace(c.Name)
	if msg := checkName(name); msg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	if n := utf8.RuneCountInString(c.Password); n < 8 || n > 256 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "the password needs at least 8 characters"})
		return
	}
	hash, err := hashPassword(c.Password)
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
	acc, err := a.Store.AccountByName(strings.TrimSpace(c.Name))
	hash := acc.PasswordHash
	if err != nil {
		hash = dummyHash // same work for unknown names
	}
	if !checkPassword(hash, c.Password) || err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "wrong name or password"})
		return
	}
	if !a.startSession(w, r, acc) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": acc.Name})
}

func (a *Service) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(CookieName); err == nil {
		a.Store.DeleteSession(tokenHash(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
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

// cors allows the web app's origin to call the API with its cookie.
func (a *Service) cors(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Vary", "Origin")
	origin := r.Header.Get("Origin")
	if origin == "" || !config.OriginAllowed(a.AllowedOrigins, origin) {
		return
	}
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", origin)
	h.Set("Access-Control-Allow-Credentials", "true")
	h.Set("Access-Control-Allow-Methods", "GET, POST")
	h.Set("Access-Control-Allow-Headers", "Content-Type")
}

func (a *Service) allow(r *http.Request) bool {
	ip := clientip.Of(r, a.TrustForwardedFor)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.limits == nil {
		a.limits = map[string]*rate.Limiter{}
	}
	l, ok := a.limits[ip]
	if !ok {
		if len(a.limits) > 10_000 { // keep memory bounded
			a.limits = map[string]*rate.Limiter{}
		}
		l = rate.NewLimiter(rate.Limit(perMinute)/60, burst)
		a.limits[ip] = l
	}
	return l.Allow()
}

// checkName returns why a name cannot be used, or "".
func checkName(name string) string {
	n := utf8.RuneCountInString(name)
	switch {
	case n < 2 || n > 32:
		return "the name needs 2 to 32 characters"
	case strings.IndexFunc(name, unicode.IsControl) >= 0:
		return "the name contains invalid characters"
	case strings.EqualFold(name, "SuperUser"):
		return "this name is reserved"
	}
	return ""
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
