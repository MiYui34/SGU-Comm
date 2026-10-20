package sidecar

import (
	"time"

	"kallisti.chat/server/internal/mumble"
)

// Channel moderators (built-in server): the voice server keeps them, as
// a group in a Mumble channel (a voice channel or a text channel's
// mirror), and says who moderates where (Voice.ModeratesIn). There they
// delete and pin messages, set a text channel's topic and slow mode (and
// are exempt from it), manage a private channel's members and invite
// links, and give channel time-outs.

// ChannelModerators are a Mumble channel's own moderators (registered
// user ids).
type ChannelModerators struct {
	Channel uint32   `json:"channel"`
	Users   []uint32 `json:"users"`
}

// mumbleOf is the Mumble channel of a chat: a voice channel, or a text
// channel's mirror (0: none, or a direct chat).
func (h *Hub) mumbleOf(c chat) uint32 {
	switch {
	case c.direct:
		return 0
	case c.isVoice:
		return c.voice
	case c.parent != 0:
		return h.mirrorOf(c.parent)
	}
	return h.mirrorOf(c.id)
}

// mayModerate: the user moderates the chat (staff everywhere, a
// channel's moderators there).
func (h *Hub) mayModerate(id mumble.Identity, c chat) bool {
	if h.voice == nil {
		return false
	}
	if c.direct {
		return h.voice.MayManage(id.Session)
	}
	return h.voice.ModeratesIn(id.Session, h.mumbleOf(c))
}

// setChannelModerator: the voice server checks the right and reports the
// change (RolesChanged).
func (c *Conn) setChannelModerator(id mumble.Identity, req request) {
	if c.hub.voice == nil {
		c.manageError("channel moderators need the built-in Mumble server")
	} else if err := c.hub.voice.SetChannelModerator(id.Session, req.User, req.Voice, req.On); err != nil {
		c.manageError(err.Error())
	}
}

// channelTimeOut times a user out in a Mumble channel for some seconds, or
// ends it (0). The voice server checks the rights and reports the change
// (TimeoutsChanged).
func (c *Conn) channelTimeOut(id mumble.Identity, req request) {
	switch {
	case c.hub.voice == nil:
		c.manageError("time-outs need the built-in Mumble server")
	case req.Seconds < 0 || req.Seconds > maxSeconds:
		c.manageError("invalid time-out")
	default:
		if err := c.hub.voice.SetChannelTimeout(id.Session, req.Session, req.Voice, time.Duration(req.Seconds)*time.Second); err != nil {
			c.manageError(err.Error())
		}
	}
}

// channelTimedOut refuses chat requests in a chat where the user has a
// channel time-out (typing quietly). The chat is the request's channel,
// or its message's.
func (c *Conn) channelTimedOut(id mumble.Identity, req request) bool {
	h := c.hub
	channel := req.Channel
	if req.Message != 0 {
		m, err := h.store.MessageByID(req.Message)
		if err != nil {
			return false // unknown: refused later
		}
		channel = m.ChannelID
	}
	ch, ok := h.chatOf(channel)
	if !ok || ch.direct {
		return false
	}
	until, out := h.voice.ChannelTimedOut(id.Session, h.mumbleOf(ch))
	if out && req.Type != "typing" {
		c.sendError("you are timed out in this channel until " + until.UTC().Format("2006-01-02 15:04") + " UTC")
	}
	return out
}

type wireChannelTimeout struct {
	wireTimeout
	Channel uint32 `json:"channel"`
}

// channelTimeoutsFor: the channel time-outs in channels the user sees (a
// list, never null in JSON).
func (h *Hub) channelTimeoutsFor(id mumble.Identity) []wireChannelTimeout {
	out := []wireChannelTimeout{}
	for _, t := range h.voice.ChannelTimeouts() {
		if h.voice.Sees(id.Session, t.Channel) {
			out = append(out, wireChannelTimeout{toWireTimeout(t), t.Channel})
		}
	}
	return out
}
