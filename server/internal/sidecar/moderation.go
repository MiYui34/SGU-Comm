package sidecar

import (
	"time"

	"kallisti.chat/server/internal/mumble"
)

// maxReasonLength: a kick or ban reason, in characters.
const maxReasonLength = 200

// ban bans a user for some seconds (0: forever) with the built-in server:
// Mumble's UserRemove has no duration. The voice server checks the
// rights; everyone sees the user leave.
func (c *Conn) ban(id mumble.Identity, req request) {
	if c.hub.voice == nil {
		c.manageError("timed bans need the built-in Mumble server")
		return
	}
	if req.Seconds < 0 {
		c.manageError("invalid ban duration")
		return
	}
	reason := shorten(req.Text, maxReasonLength)
	if err := c.hub.voice.Ban(id.Session, req.Session, reason, time.Duration(req.Seconds)*time.Second); err != nil {
		c.manageError(err.Error())
	}
}

// Timeout is a running time-out: of a registered user (UserID) or a guest
// (Session). Timed out users may not chat and are server-muted.
type Timeout struct {
	Registered bool
	UserID     uint32
	Session    uint32
	Until      time.Time
}

type wireTimeout struct {
	User    *uint32 `json:"user,omitempty"`
	Session uint32  `json:"session,omitempty"`
	Until   int64   `json:"until"` // Unix milliseconds
}

// timeOut times a user out for some seconds, or ends it (0). The voice
// server checks the rights and reports the change (TimeoutsChanged).
func (c *Conn) timeOut(id mumble.Identity, req request) {
	if c.hub.voice == nil {
		c.manageError("time-outs need the built-in Mumble server")
		return
	}
	if req.Seconds < 0 {
		c.manageError("invalid time-out")
		return
	}
	if err := c.hub.voice.SetTimeout(id.Session, req.Session, time.Duration(req.Seconds)*time.Second); err != nil {
		c.manageError(err.Error())
	}
}

// timedOut refuses chat requests of a timed out user (typing quietly).
func (c *Conn) timedOut(id mumble.Identity, req request) bool {
	if c.hub.voice == nil {
		return false
	}
	until, ok := c.hub.voice.TimedOut(id.Session)
	if ok && req.Type != "typing" {
		c.sendError("you are timed out until " + until.UTC().Format("2006-01-02 15:04") + " UTC")
	}
	return ok
}

// timeouts is a list, never null in JSON.
func (h *Hub) timeouts() []wireTimeout {
	out := []wireTimeout{}
	for _, t := range h.voice.Timeouts() {
		w := wireTimeout{Until: t.Until.UnixMilli()}
		if t.Registered {
			w.User = &t.UserID
		} else {
			w.Session = t.Session
		}
		out = append(out, w)
	}
	return out
}

// TimeoutsChanged sends everyone the running time-outs.
func (h *Hub) TimeoutsChanged() {
	if h.voice == nil {
		return
	}
	h.broadcast(map[string]any{"type": "timeouts", "timeouts": h.timeouts()})
}
