package mumbled

import (
	"cmp"
	"errors"
	"log/slog"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble/pb"
)

// Time-outs (Kallisti, not Mumble): a timed out user may not send texts
// and stays server-muted until the time ends. Registered users by user id
// (saved: they last over reconnects and restarts), guests by session
// (until they leave). Moderators and admins (MuteDeafen in root: a
// time-out is a mute everywhere) give them, as kicks: never to SuperUser,
// to staff only by admins.

const maxTimeout = 28 * 24 * time.Hour

type timeoutKey struct {
	guest bool
	id    uint32 // the user id, or a guest's session
}

func timeoutKeyOf(u *User) timeoutKey {
	if u.Registered {
		return timeoutKey{id: u.UserID}
	}
	return timeoutKey{guest: true, id: u.Session}
}

// keepMute: the user was server-muted before; the end does not unmute.
type timeout struct {
	until    time.Time
	timer    *time.Timer
	keepMute bool
}

// Timeout is a running time-out: of a registered user (UserID) or a guest
// (Session).
type Timeout struct {
	Registered bool
	UserID     uint32
	Session    uint32
	Until      time.Time
}

// loadTimeouts starts the saved time-outs that did not end yet.
func (s *Server) loadTimeouts() error {
	s.timeouts = map[timeoutKey]*timeout{}
	if s.cfg.DB == nil {
		return nil
	}
	saved, err := s.cfg.DB.Timeouts()
	if err != nil {
		return err
	}
	for id, until := range saved {
		if until.After(time.Now()) {
			s.startTimeoutLocked(timeoutKey{id: id}, until)
		} else if err := s.cfg.DB.SaveTimeout(id, time.Time{}); err != nil {
			return err
		}
	}
	return nil
}

// SetTimeout times a user out for d, or ends its time-out (d 0).
func (s *Server) SetTimeout(actor, session uint32, d time.Duration) error {
	if d < 0 || d > maxTimeout {
		return errors.New("invalid time-out")
	}
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	s.mu.Lock()
	a, u := s.state.Users[actor], s.state.Users[session]
	switch {
	case a == nil || u == nil:
		s.mu.Unlock()
		return errors.New("no such user")
	case a == u:
		s.mu.Unlock()
		return errors.New("you cannot time out yourself")
	case !s.mayActOn(a, u) || !s.state.Has(a, 0, MuteDeafen):
		s.mu.Unlock()
		return errors.New("you may not time out this user")
	}
	key := timeoutKeyOf(u)
	if _, ok := s.timedOutLocked(u); !ok && d == 0 {
		s.mu.Unlock()
		return errors.New("this user is not timed out")
	}
	var until time.Time
	if d > 0 {
		until = time.Now().Add(d)
	}
	s.setTimeoutLocked(key, until, actor)
	s.mu.Unlock()
	s.saveTimeout(key, until)
	s.timeoutsChanged()
	return nil
}

// expireTimeout ends a time-out when its time is up (if it was not
// changed meanwhile).
func (s *Server) expireTimeout(key timeoutKey, until time.Time) {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	s.mu.Lock()
	t := s.timeouts[key]
	if t == nil || !t.until.Equal(until) || s.closing.Load() {
		s.mu.Unlock()
		return
	}
	s.setTimeoutLocked(key, time.Time{}, 0)
	s.mu.Unlock()
	s.saveTimeout(key, time.Time{})
	s.timeoutsChanged()
}

// setTimeoutLocked replaces a time-out (zero: none) and mutes or unmutes
// the user if online. actor 0: the server.
func (s *Server) setTimeoutLocked(key timeoutKey, until time.Time, actor uint32) {
	old := s.timeouts[key]
	keep := old != nil && old.keepMute
	s.dropTimeoutLocked(key)
	on := !until.IsZero()
	var online *User
	for _, u := range s.state.Users {
		if timeoutKeyOf(u) == key {
			online = u
		}
	}
	if on {
		if old == nil && online != nil {
			keep = online.Mute
		}
		s.startTimeoutLocked(key, until)
		s.timeouts[key].keepMute = keep
	}
	if online == nil || online.Mute == on || (!on && keep) {
		return
	}
	u := online
	u.Mute, u.Deaf = serverMuteState(u.Mute, u.Deaf, &pb.UserState{Mute: proto.Bool(on)})
	out := &pb.UserState{Session: proto.Uint32(u.Session), Mute: proto.Bool(u.Mute), Deaf: proto.Bool(u.Deaf)}
	if actor != 0 {
		out.Actor = proto.Uint32(actor)
	}
	s.state.Changed()
	s.sendUserLocked(u, out)
}

func (s *Server) startTimeoutLocked(key timeoutKey, until time.Time) {
	s.timeouts[key] = &timeout{until: until, timer: time.AfterFunc(time.Until(until), func() { s.expireTimeout(key, until) })}
}

// dropTimeoutLocked forgets a time-out; it reports whether there was one.
func (s *Server) dropTimeoutLocked(key timeoutKey) bool {
	t := s.timeouts[key]
	if t != nil {
		t.timer.Stop()
		delete(s.timeouts, key)
	}
	return t != nil
}

// timedOutLocked: the user's time-out, if any.
func (s *Server) timedOutLocked(u *User) (time.Time, bool) {
	if t := s.timeouts[timeoutKeyOf(u)]; t != nil {
		return t.until, true
	}
	return time.Time{}, false
}

// TimedOut: the session's user is timed out, until when.
func (s *Server) TimedOut(session uint32) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u := s.state.Users[session]; u != nil {
		return s.timedOutLocked(u)
	}
	return time.Time{}, false
}

// Timeouts lists the running time-outs (registered users first).
func (s *Server) Timeouts() []Timeout {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Timeout{}
	for k, t := range s.timeouts {
		if k.guest {
			out = append(out, Timeout{Session: k.id, Until: t.until})
		} else {
			out = append(out, Timeout{Registered: true, UserID: k.id, Until: t.until})
		}
	}
	// Registered users have no session here: they come first.
	slices.SortFunc(out, func(a, b Timeout) int {
		return cmp.Or(cmp.Compare(a.Session, b.Session), cmp.Compare(a.UserID, b.UserID))
	})
	return out
}

// timedOutText is the refusal for a timed out user.
func timedOutText(who string, until time.Time) error {
	return deny(pb.PermissionDenied_Text, "%s timed out until %s UTC", who, until.UTC().Format("2006-01-02 15:04"))
}

// saveTimeout keeps a registered user's time-out (zero: none). Callers
// hold saveMu.
func (s *Server) saveTimeout(key timeoutKey, until time.Time) {
	if key.guest || s.cfg.DB == nil {
		return
	}
	if err := s.cfg.DB.SaveTimeout(key.id, until); err != nil {
		slog.Error("saving a time-out", "user", key.id, "err", err)
	}
}

// timeoutsChanged tells the Kallisti server (not under s.mu).
func (s *Server) timeoutsChanged() {
	if s.cfg.OnTimeoutsChanged != nil {
		s.cfg.OnTimeoutsChanged()
	}
}

// stopTimeouts stops the timers (the server closes).
func (s *Server) stopTimeouts() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.timeouts {
		t.timer.Stop()
	}
}
