package sidecar

import (
	"strings"
	"unicode/utf8"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/names"
	"kallisti.chat/server/internal/store"
)

// Editing and deleting messages. Registered users own the messages with
// their user id (messages from before ids were kept: their verified name);
// guests (whose names are free) only those they wrote in their current
// connection. Moderators and admins delete any message.

// Guest messages remembered, at most (oldest forgotten first is not
// needed: a full map starts over).
const maxGuestMessages = 10_000

func (h *Hub) rememberGuest(message int64, session uint32) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.guestMessages == nil || len(h.guestMessages) >= maxGuestMessages {
		h.guestMessages = map[int64]uint32{}
	}
	h.guestMessages[message] = session
}

func (h *Hub) wrote(id mumble.Identity, m store.Message) bool {
	if m.Verified {
		return byUser(id, m)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	session, ok := h.guestMessages[m.ID]
	return ok && !id.Registered && session == id.Session
}

// byUser: a registered user's own message (see wrote).
func byUser(id mumble.Identity, m store.Message) bool {
	if !id.Registered || !m.Verified {
		return false
	}
	if m.AuthorID != nil {
		return *m.AuthorID == id.UserID
	}
	return names.Key(id.Name) == names.Key(m.Author)
}

func (c *Conn) changeMessage(id mumble.Identity, req request) {
	h := c.hub
	m, err := h.store.MessageByID(req.Message)
	if err != nil {
		c.sendError("unknown message")
		return
	}
	ch, ok := h.chatOf(m.ChannelID)
	if !ok || !h.may(id, ch, req.Type == "edit-message") {
		c.sendError("unknown message")
		return
	}
	send := func(v any) { h.sendTo(ch, v) }
	own := h.wrote(id, m)
	if req.Type == "edit-message" {
		text, valid := validText(req.Text, m.Upload != nil)
		switch {
		case !own:
			c.sendError("you may only edit your own messages")
		case !valid:
			c.sendError(textError)
		default:
			if m, err = h.store.EditMessage(m.ID, text, h.now()); err != nil {
				c.sendError("cannot edit the message")
				return
			}
			send(map[string]any{"type": "message-edited", "message": toWire(m)})
		}
		return
	}
	if !own && (h.voice == nil || !h.voice.MayManage(id.Session)) {
		c.sendError("you may only delete your own messages")
		return
	}
	if m, err = h.store.DeleteMessage(m.ID); err != nil {
		c.sendError("cannot delete the message")
		return
	}
	if m.Upload != nil && h.OnUploadDeleted != nil {
		h.OnUploadDeleted(m.Upload.Key)
	}
	send(map[string]any{"type": "message-deleted", "channel": m.ChannelID, "id": m.ID})
}

// A message has 1 to MaxTextLength characters; with an attachment it may
// have none.
const textError = "message must be 1 to 2000 characters"

func validText(raw string, hasUpload bool) (string, bool) {
	text := strings.TrimSpace(raw)
	return text, (text != "" || hasUpload) && utf8.RuneCountInString(text) <= MaxTextLength
}
