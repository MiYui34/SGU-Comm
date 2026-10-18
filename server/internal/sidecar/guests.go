package sidecar

import "kallisti.chat/server/internal/mumble"

// Guests read only: a server setting of the built-in server (see
// Voice.SetGuestsReadOnly), which also refuses their Mumble texts. Here
// read-only guests may not send, edit, react, pin, upload or open direct
// chats.
var writes = map[string]bool{"send": true, "edit-message": true, "react": true, "pin": true, "typing": true, "upload-token": true, "dm": true}

// readOnly refuses a writing request of a read-only guest (typing quietly).
func (c *Conn) readOnly(id mumble.Identity, req request) bool {
	if !writes[req.Type] || c.hub.voice == nil || !c.hub.voice.ReadOnly(id.Session) {
		return false
	}
	if req.Type != "typing" {
		c.sendError("guests may only read here: log in or make an account to write")
	}
	return true
}

// GuestsReadOnlyChanged sends everyone the setting.
func (h *Hub) GuestsReadOnlyChanged() {
	if h.voice != nil {
		h.broadcast(map[string]any{"type": "guests-read-only", "on": h.voice.GuestsReadOnly()})
	}
}
