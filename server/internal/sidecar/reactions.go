package sidecar

import (
	"errors"

	"kallisti.chat/server/internal/emoji"
	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/store"
)

type wireReaction struct {
	Emoji string   `json:"emoji"`
	Users []string `json:"users"`
	IDs   []uint32 `json:"ids"` // Mumble user ids, in the order of Users
}

// Never null in JSON: an event with no reactions left has [].
func toWireReactions(rs []store.Reaction) []wireReaction {
	out := make([]wireReaction, len(rs))
	for i, r := range rs {
		out[i] = wireReaction{Emoji: r.Emoji, Users: r.Users, IDs: r.IDs}
	}
	return out
}

// react adds or removes the user's reaction, as writing does: everyone in
// text channels, in voice chats those who may write. Reactions belong to
// user ids, so only registered users react (web guests have guest
// accounts). All readers get the message's reactions.
func (c *Conn) react(id mumble.Identity, req request) {
	if !id.Registered {
		c.sendError("join with an account (or a guest account) to react")
		return
	}
	normal := emoji.Normal(req.Emoji)
	if !emoji.Valid(normal) {
		c.sendError("not an emoji")
		return
	}
	m, err := c.hub.store.MessageByID(req.Message)
	if err != nil {
		c.sendError("unknown message")
		return
	}
	ch, ok := c.hub.chatOf(m.ChannelID)
	if !ok || !c.hub.may(id, ch, true) {
		c.sendError("unknown message") // as for messages that do not exist
		return
	}
	changed, err := c.hub.store.SetReaction(m.ID, normal, id.UserID, id.Name, req.On, c.hub.now())
	if errors.Is(err, store.ErrTooManyReactions) {
		c.sendError("this message has too many different reactions")
		return
	}
	if err != nil {
		c.sendError("cannot store the reaction")
		return
	}
	if !changed {
		return
	}
	if m, err = c.hub.store.MessageByID(m.ID); err != nil {
		return // deleted meanwhile
	}
	c.hub.sendTo(ch, map[string]any{"type": "reactions", "channel": m.ChannelID, "message": m.ID, "reactions": toWireReactions(m.Reactions)})
}
