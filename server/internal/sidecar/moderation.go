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
