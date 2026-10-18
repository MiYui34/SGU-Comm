package sidecar

import (
	"slices"

	"kallisti.chat/server/internal/mumble"
)

// Private channels (built-in server): text and voice channels that only
// their members, admins and moderators see. The voice server keeps who
// may (see Voice.Sees); a private text channel is its mirror's access.
// Clients get the text channels and the private channels they see, again
// when access changes.

// PrivateChannel is a private Mumble channel (a voice channel or a text
// channel's mirror) and its members.
type PrivateChannel struct {
	Channel uint32   `json:"channel"`
	Members []Member `json:"members,omitempty"` // for staff only
}

// Member is a registered user in a private channel.
type Member = Speaker

// channelsFor: the text channels the user may read.
func (h *Hub) channelsFor(id mumble.Identity) []wireChannel {
	h.mu.Lock()
	all := slices.Clone(h.channels)
	h.mu.Unlock()
	return slices.DeleteFunc(all, func(c wireChannel) bool { return !h.may(id, chat{id: c.ID}, false) })
}

// privateFor: the private channels the user sees, with members for staff.
func (h *Hub) privateFor(id mumble.Identity) []PrivateChannel {
	staff := h.voice.MayManage(id.Session)
	out := []PrivateChannel{}
	for _, p := range h.voice.Private() {
		if !h.voice.Sees(id.Session, p.Channel) {
			continue
		}
		if !staff {
			p.Members = nil
		}
		out = append(out, p)
	}
	return out
}

// sendAccess sends a client the text channels and private channels it
// sees now.
func (c *Conn) sendAccess(id mumble.Identity) {
	c.send(map[string]any{"type": "channels", "channels": c.hub.channelsFor(id)})
	if c.hub.voice != nil {
		c.send(map[string]any{"type": "private", "channels": c.hub.privateFor(id)})
	}
}

func (h *Hub) sendAccessAll() {
	h.eachReady(func(c *Conn, id mumble.Identity) { c.sendAccess(id) })
}

// PrivateChanged runs after private channels or their members changed
// (see mumbled.Config.OnPrivateChanged).
func (h *Hub) PrivateChanged() {
	if h.voice != nil {
		h.sendAccessAll()
	}
}

// setPrivate makes a Mumble channel (a voice channel or a text channel's
// mirror) private with these members, or public. The voice server checks
// the right and reports the change (PrivateChanged).
func (c *Conn) setPrivate(id mumble.Identity, req request) {
	if c.hub.voice == nil {
		c.manageError("private channels need the built-in Mumble server")
	} else if err := c.hub.voice.SetPrivate(id.Session, req.Voice, req.On, req.Users); err != nil {
		c.manageError(err.Error())
	}
}

// createPrivateVoice makes a private voice channel (only staff see it,
// until they add members).
func (c *Conn) createPrivateVoice(id mumble.Identity, name string) {
	if c.hub.voice == nil {
		c.manageError("private channels need the built-in Mumble server")
	} else if _, err := c.hub.voice.CreatePrivateChannel(id.Session, name); err != nil {
		c.manageError(err.Error())
	}
}
