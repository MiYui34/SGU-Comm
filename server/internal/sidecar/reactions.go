package sidecar

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/store"
)

type wireReaction struct {
	Emoji string   `json:"emoji"`
	Users []string `json:"users"`
}

// Never null in JSON: an event with no reactions left has [].
func toWireReactions(rs []store.Reaction) []wireReaction {
	out := make([]wireReaction, len(rs))
	for i, r := range rs {
		out[i] = wireReaction{Emoji: r.Emoji, Users: r.Users}
	}
	return out
}

const maxEmojiRunes = 8 // long enough for sequences (families, flags, skin tones)

// validEmoji: short, with a symbol, and nothing a person could read as
// text (no ASCII, spaces or invisible characters except the joiner).
func validEmoji(s string) bool {
	if n := utf8.RuneCountInString(s); n == 0 || n > maxEmojiRunes || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r == 0x200D { // zero-width joiner, in sequences
			continue
		}
		if r < 0x80 || unicode.IsSpace(r) || unicode.Is(unicode.Cf, r) || !unicode.IsPrint(r) {
			return false
		}
	}
	return strings.ContainsFunc(s, func(r rune) bool { return unicode.Is(unicode.So, r) })
}

// react adds or removes the user's reaction, as writing does: everyone in
// text channels, in voice chats those who may write. All readers get the
// message's reactions.
func (c *Conn) react(id mumble.Identity, req request) {
	if !validEmoji(req.Emoji) {
		c.sendError("not an emoji")
		return
	}
	m, err := c.hub.store.MessageByID(req.Message)
	if err != nil {
		c.sendError("unknown message")
		return
	}
	voice, isVoice, ok := c.hub.chatOf(m.ChannelID)
	if !ok || isVoice && !c.hub.voice.MayWrite(id.Session, voice) {
		c.sendError("you may not use this channel's chat")
		return
	}
	err = c.hub.store.SetReaction(m.ID, req.Emoji, id.Name, req.On, c.hub.now())
	if errors.Is(err, store.ErrTooManyReactions) {
		c.sendError("this message has too many different reactions")
		return
	}
	if err != nil {
		c.sendError("cannot store the reaction")
		return
	}
	if m, err = c.hub.store.MessageByID(m.ID); err != nil {
		return // deleted meanwhile
	}
	event := map[string]any{"type": "reactions", "channel": m.ChannelID, "message": m.ID, "reactions": toWireReactions(m.Reactions)}
	for _, r := range c.hub.readers(voice, isVoice) {
		r.send(event)
	}
}
