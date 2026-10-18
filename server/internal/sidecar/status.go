package sidecar

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"kallisti.chat/server/internal/emoji"
	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/store"
)

// A custom status text has at most this many characters.
const maxStatusText = 128

// statusView is a status as clients see it: "online", "idle" or "dnd", and
// a custom status (Until: Unix milliseconds; 0: no end).
type statusView struct {
	Status string `json:"status"`
	Emoji  string `json:"emoji,omitempty"`
	Text   string `json:"text,omitempty"`
	Until  int64  `json:"until,omitempty"`
}

// sessionStatus is the status others see of a web client's session.
type sessionStatus struct {
	Type    string `json:"type,omitempty"` // "status" as an event of its own
	Session uint32 `json:"session"`
	statusView
}

// view is the chosen status; a custom status after its end is gone.
func view(st store.UserStatus, now time.Time) statusView {
	v := statusView{Status: st.Status, Emoji: st.Emoji, Text: st.Text, Until: st.Until}
	if v.Status == "" {
		v.Status = "online"
	}
	if v.Until != 0 && v.Until <= now.UnixMilli() {
		v.Emoji, v.Text, v.Until = "", "", 0
	}
	return v
}

// effective is the status others see: away (no input for a while) makes
// online idle.
func effective(st store.UserStatus, away bool, now time.Time) statusView {
	v := view(st, now)
	if away && v.Status == "online" {
		v.Status = "idle"
	}
	return v
}

func (c *Conn) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (v statusView) plain() bool { return v == statusView{Status: "online"} }

// shown is the connection's status as others see it.
func (c *Conn) shown() statusView {
	c.mu.Lock()
	st, away := c.status, c.away
	c.mu.Unlock()
	return effective(st, away, c.hub.now())
}

// statuses of the web clients that are not plainly online (a list, never
// null in JSON).
func (h *Hub) statuses() []sessionStatus {
	out := []sessionStatus{}
	for _, c := range h.all() {
		c.mu.Lock()
		id := c.identity
		c.mu.Unlock()
		if v := c.shown(); id != nil && !v.plain() {
			out = append(out, sessionStatus{Session: id.Session, statusView: v})
		}
	}
	return out
}

// statusChanged tells everyone a session's status.
func (h *Hub) statusChanged(session uint32, v statusView) {
	h.broadcast(sessionStatus{Type: "status", Session: session, statusView: v})
}

// sendReady sends ready with the statuses (and our own, kept for
// registered users), and tells the others ours. Under statusMu: no status
// event slips in before ready, and a connection that closed meanwhile
// announces nothing.
func (c *Conn) sendReady(id mumble.Identity, ready map[string]any) {
	c.hub.statusMu.Lock()
	defer c.hub.statusMu.Unlock()
	var st store.UserStatus
	if id.Registered {
		var err error
		if st, err = c.hub.store.UserStatus(id.UserID); err != nil {
			slog.Error("load a status", "err", err)
		}
	}
	c.mu.Lock()
	c.status = st
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return
	}
	ready["statuses"], ready["ownStatus"] = c.hub.statuses(), view(st, c.hub.now())
	if c.hub.voice != nil {
		ready["online"] = c.hub.syncOnline(c)
	}
	c.send(ready)
	if v := c.shown(); !v.plain() {
		for _, other := range c.hub.all() {
			if other != c {
				other.send(sessionStatus{Type: "status", Session: id.Session, statusView: v})
			}
		}
	}
}

// statusGone: a closed connection's status leaves with it.
func (c *Conn) statusGone() {
	c.hub.statusMu.Lock()
	defer c.hub.statusMu.Unlock()
	c.mu.Lock()
	id, ready := c.identity, c.ready
	c.mu.Unlock()
	if ready && !c.shown().plain() {
		c.hub.statusChanged(id.Session, statusView{Status: "online"})
	}
}

// parseStatus checks a set-status request; hasEmoji: the server's custom
// emoji (":name:"), as in reactions.
func parseStatus(req request, now time.Time, hasEmoji func(name string) bool) (store.UserStatus, string) {
	st := store.UserStatus{Status: req.Status, Text: strings.Join(strings.Fields(req.Text), " "), Until: req.Until}
	switch st.Status {
	case "online":
		st.Status = ""
	case "idle", "dnd":
	default:
		return st, "unknown status"
	}
	if req.Emoji != "" {
		st.Emoji = emoji.Normal(req.Emoji)
		if name, custom := emoji.CustomName(st.Emoji); custom && !hasEmoji(name) {
			return st, "no such emoji on this server"
		} else if !custom && !emoji.Valid(st.Emoji) {
			return st, "not an emoji"
		}
	}
	if utf8.RuneCountInString(st.Text) > maxStatusText {
		return st, fmt.Sprintf("a custom status has at most %d characters", maxStatusText)
	}
	if st.Emoji == "" && st.Text == "" || st.Until < 0 || st.Until != 0 && st.Until <= now.UnixMilli() {
		st.Emoji, st.Text, st.Until = "", "", 0
	}
	return st, ""
}

// setStatus sets the user's chosen status and custom status: of a
// registered user on all its devices (and kept), of a guest only here.
func (c *Conn) setStatus(id mumble.Identity, req request) {
	st, problem := parseStatus(req, c.hub.now(), c.hub.hasEmoji)
	if problem != "" {
		c.sendError(problem)
		return
	}
	c.hub.statusMu.Lock()
	defer c.hub.statusMu.Unlock()
	conns := []*Conn{c}
	if id.Registered {
		if err := c.hub.store.SetUserStatus(id.UserID, st); err != nil {
			c.sendError("cannot store the status")
			return
		}
		conns = conns[:0]
		for _, other := range c.hub.all() {
			other.mu.Lock()
			oid := other.identity
			other.mu.Unlock()
			if oid != nil && oid.Registered && oid.UserID == id.UserID && !other.isClosed() {
				conns = append(conns, other)
			}
		}
	}
	// Each device hears it first, then everyone the new statuses.
	own := view(st, c.hub.now())
	var changed []sessionStatus
	for _, conn := range conns {
		before := conn.shown()
		conn.mu.Lock()
		conn.status = st
		session := conn.identity.Session
		conn.mu.Unlock()
		conn.send(struct {
			Type string `json:"type"`
			statusView
		}{"own-status", own})
		if after := conn.shown(); after != before {
			changed = append(changed, sessionStatus{Session: session, statusView: after})
		}
	}
	for _, s := range changed {
		c.hub.statusChanged(s.Session, s.statusView)
	}
}

// setAway: the client had no input for a while (or again). Not kept.
func (c *Conn) setAway(id mumble.Identity, on bool) {
	c.hub.statusMu.Lock()
	defer c.hub.statusMu.Unlock()
	if c.isClosed() {
		return
	}
	before := c.shown()
	c.mu.Lock()
	c.away = on
	c.mu.Unlock()
	if after := c.shown(); after != before {
		c.hub.statusChanged(id.Session, after)
	}
}
