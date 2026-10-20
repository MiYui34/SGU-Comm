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

// account is the connection and the other open ones of its registered
// user (devices and tabs).
func (h *Hub) account(c *Conn) []*Conn {
	c.mu.Lock()
	id := c.identity
	c.mu.Unlock()
	out := []*Conn{c}
	if id == nil || !id.Registered {
		return out
	}
	h.eachReady(func(o *Conn, oid mumble.Identity) {
		if o != c && oid.Registered && oid.UserID == id.UserID && !o.isClosed() {
			out = append(out, o)
		}
	})
	return out
}

// shown is the connection's status as others see it: away only while all
// sessions of the account are, as on Discord (each session shows the
// account's status: clients look it up by the session they see).
func (c *Conn) shown() statusView {
	away := true
	for _, o := range c.hub.account(c) {
		o.mu.Lock()
		away = away && o.away
		o.mu.Unlock()
	}
	c.mu.Lock()
	st := c.status
	c.mu.Unlock()
	return effective(st, away, c.hub.now())
}

// syncStatuses tells all but skip the statuses of c's account's sessions
// that changed since they were told. Under statusMu.
func (h *Hub) syncStatuses(c *Conn, skip *Conn) {
	for _, o := range h.account(c) {
		if o.isClosed() {
			continue
		}
		v := o.shown()
		o.mu.Lock()
		changed, session := o.sent != v, o.identity.Session
		o.sent = v
		o.mu.Unlock()
		if changed {
			for _, other := range h.all() {
				if other != skip {
					other.send(sessionStatus{Type: "status", Session: session, statusView: v})
				}
			}
		}
	}
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
	c.status, c.sent = st, statusView{Status: "online"} // what others assume
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
	c.hub.syncStatuses(c, c) // also the account's other sessions: no longer all away
}

// statusGone: a closed connection's status leaves with it.
func (c *Conn) statusGone() {
	c.hub.statusMu.Lock()
	defer c.hub.statusMu.Unlock()
	c.mu.Lock()
	id, ready, sent := c.identity, c.ready, c.sent
	c.mu.Unlock()
	if ready && !sent.plain() {
		c.hub.statusChanged(id.Session, statusView{Status: "online"})
	}
	if ready {
		c.hub.syncStatuses(c, nil) // the others may all be away now
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
	if id.Registered {
		if err := c.hub.store.SetUserStatus(id.UserID, st); err != nil {
			c.sendError("cannot store the status")
			return
		}
	}
	// Each device hears it first, then everyone the new statuses.
	own := view(st, c.hub.now())
	for _, conn := range c.hub.account(c) {
		conn.mu.Lock()
		conn.status = st
		conn.mu.Unlock()
		conn.send(struct {
			Type string `json:"type"`
			statusView
		}{"own-status", own})
	}
	c.hub.syncStatuses(c, nil)
}

// setAway: the client had no input for a while (or again). Not kept.
func (c *Conn) setAway(id mumble.Identity, on bool) {
	c.hub.statusMu.Lock()
	defer c.hub.statusMu.Unlock()
	if c.isClosed() {
		return
	}
	c.mu.Lock()
	c.away = on
	c.mu.Unlock()
	c.hub.syncStatuses(c, nil)
}
