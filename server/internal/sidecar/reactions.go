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

const maxEmojiRunes = 16 // enough for sequences (couples with skin tones, flags)

// validEmoji (on a normalEmoji): symbols, with only what emoji sequences
// add to them: skin tones, the emoji variation selector, tags (flags of
// regions), and joiners between parts. No letters, marks or invisible text.
func validEmoji(s string) bool {
	rs := []rune(s)
	if len(rs) == 0 || len(rs) > maxEmojiRunes || !utf8.ValidString(s) {
		return false
	}
	symbol := false
	for i, r := range rs {
		switch {
		case r == 0x200D: // zero-width joiner
			if i == 0 || i == len(rs)-1 || rs[i-1] == 0x200D {
				return false
			}
		case r == 0xFE0F, r >= 0x1F3FB && r <= 0x1F3FF, r >= 0xE0020 && r <= 0xE007F:
			if i == 0 {
				return false
			}
		case r >= 0x80 && unicode.Is(unicode.So, r):
			symbol = true
		default:
			return false
		}
	}
	return symbol
}

// normalEmoji: one form per emoji (with or without the variation selector
// they look the same): the selector after each symbol of the older,
// shorter range, and nowhere else.
func normalEmoji(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == 0xFE0F || r == 0xFE0E {
			continue
		}
		b.WriteRune(r)
		if r >= 0x80 && r < 0x1F000 && unicode.Is(unicode.So, r) {
			b.WriteRune(0xFE0F)
		}
	}
	return b.String()
}

// react adds or removes the user's reaction, as writing does: everyone in
// text channels, in voice chats those who may write. All readers get the
// message's reactions.
func (c *Conn) react(id mumble.Identity, req request) {
	emoji := normalEmoji(req.Emoji)
	if !validEmoji(emoji) {
		c.sendError("not an emoji")
		return
	}
	m, err := c.hub.store.MessageByID(req.Message)
	if err != nil {
		c.sendError("unknown message")
		return
	}
	voice, isVoice, ok := c.hub.chatOf(m.ChannelID)
	if !ok || isVoice && !c.hub.mayUse(id.Session, voice, true) {
		c.sendError("unknown message") // as for messages that do not exist
		return
	}
	changed, err := c.hub.store.SetReaction(m.ID, emoji, id.Name, req.On, c.hub.now())
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
	c.hub.sendToChat(voice, isVoice)(map[string]any{"type": "reactions", "channel": m.ChannelID, "message": m.ID, "reactions": toWireReactions(m.Reactions)})
}
