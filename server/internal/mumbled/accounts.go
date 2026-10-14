package mumbled

import (
	"net/netip"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"kallisti.chat/server/internal/names"
)

// Registered is a registered user. ID 0 is SuperUser.
type Registered struct {
	ID           uint32
	Name         string
	CertHash     string // SHA-1 of the client certificate, hex
	PasswordHash string // argon2id, see internal/password
	LastChannel  uint32
}

// authenticate decides who connects, as Murmur does: a registered name
// needs its password, or else its certificate; a known certificate logs in
// as its user (with the registered name). It returns nil for a guest, and
// ok false if the connection must be refused.
func authenticate(byName, byHash *Registered, password, hash string, check func(encoded, password string) bool) (*Registered, bool) {
	if byName != nil {
		switch {
		case byName.PasswordHash != "":
			return byName, check(byName.PasswordHash, password)
		case password != "":
			return nil, false // a password for a user who has none
		case byName.ID == 0:
			return nil, false // SuperUser always needs its password
		}
	}
	if hash != "" && byHash != nil {
		return byHash, true
	}
	return nil, byName == nil
}

// Presence is what decides about name conflicts for one connected user.
type Presence struct {
	Session    uint32
	Name       string
	Registered bool
	UserID     uint32 // only if Registered
	Host       netip.Addr
	Hash       string
}

// findGhost finds an old session that the new connection replaces: the
// same registered user, or (as Murmur) a guest with the same name from the
// same IP or with the same certificate. inUse: the name belongs to someone
// else who is online.
func findGhost(online []Presence, p Presence) (ghost uint32, inUse bool) {
	key := names.Key(p.Name)
	for _, o := range online {
		sameUser := p.Registered && o.Registered && o.UserID == p.UserID
		if !sameUser && names.Key(o.Name) != key {
			continue
		}
		if p.Registered || o.Host == p.Host || (p.Hash != "" && p.Hash == o.Hash) {
			return o.Session, false
		}
		return 0, true
	}
	return 0, false
}

// Failed logins per IP, instead of Murmur's autoban: after
// maxLoginFailures within loginFailureWindow, logins from that IP are
// refused until the count goes down again.
const (
	maxLoginFailures   = 10
	loginFailureWindow = 2 * time.Minute
	maxTrackedIPs      = 10000
)

type logins struct {
	mu       sync.Mutex
	failures map[netip.Addr]*rate.Limiter
}

func (l *logins) blocked(ip netip.Addr) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	f := l.failures[ip]
	return f != nil && f.Tokens() < 1
}

func (l *logins) failed(ip netip.Addr) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.failures == nil {
		l.failures = map[netip.Addr]*rate.Limiter{}
	}
	if len(l.failures) >= maxTrackedIPs {
		for k, f := range l.failures {
			if f.Tokens() >= maxLoginFailures {
				delete(l.failures, k) // recovered
			}
		}
	}
	f := l.failures[ip]
	if f == nil {
		f = rate.NewLimiter(rate.Every(loginFailureWindow/maxLoginFailures), maxLoginFailures)
		l.failures[ip] = f
	}
	f.Allow()
}
