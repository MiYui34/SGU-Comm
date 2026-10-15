package sidecar

import (
	"log/slog"
	"regexp"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/names"
)

// Read state for registered users: the newest message read per channel,
// and on ready a summary (newest message, unread mentions) per channel, so
// unread marks and mention badges survive reloads and other devices.
// Guests keep theirs in the browser only.

// mentionPattern is the web client's rule (lib/mentions.ts) on plain text:
// "@" plus the whole name, any case, after a space (as JavaScript's \s) or
// an opening mark.
func mentionPattern(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)(?:^|[\s\v\p{Z}\x{FEFF}(\[{,:])@` + regexp.QuoteMeta(name) + `(?:$|[^\p{L}\p{N}_-])`)
}

func mentionsName(text, name string) bool { return mentionPattern(name).MatchString(text) }

type summaryEntry struct {
	Channel  int64  `json:"channel"`
	Voice    uint32 `json:"voice,omitempty"` // a voice channel's chat
	Latest   int64  `json:"latest"`
	Mentions int    `json:"mentions"`
}

// readState is what ready carries for a registered user: read marks and a
// summary of the text channels and the voice chats it may read.
func (h *Hub) readState(id mumble.Identity) (map[int64]int64, []summaryEntry) {
	user := names.Key(id.Name)
	marks, err := h.store.ReadMarks(user)
	if err != nil {
		slog.Error("read marks", "err", err)
		return nil, nil
	}
	var chats []summaryEntry
	h.mu.Lock()
	for _, c := range h.channels {
		chats = append(chats, summaryEntry{Channel: c.ID})
	}
	h.mu.Unlock()
	if h.voice != nil {
		voiceChats, err := h.store.VoiceChats()
		if err != nil {
			slog.Error("voice chats", "err", err)
		}
		for chat, voice := range voiceChats {
			if h.voice.MayRead(id.Session, voice) {
				chats = append(chats, summaryEntry{Channel: chat, Voice: voice})
			}
		}
	}
	mention := mentionPattern(id.Name)
	out := chats[:0]
	for _, e := range chats {
		latest, msgs, err := h.store.MessagesAfter(e.Channel, marks[e.Channel], "@")
		if err != nil {
			slog.Error("channel summary", "err", err)
			continue
		}
		e.Latest = latest
		for _, m := range msgs {
			repliesToMe := m.Reply != nil && m.Reply.Author != "" && names.Key(m.Reply.Author) == user
			if names.Key(m.Author) != user && (mention.MatchString(m.Text) || repliesToMe) {
				e.Mentions++
			}
		}
		if e.Latest > 0 {
			out = append(out, e)
		}
	}
	return marks, out
}

// markRead stores a registered user's read mark (guests: nothing).
func (c *Conn) markRead(id mumble.Identity, req request) {
	if !id.Registered {
		return
	}
	voice, isVoice, ok := c.hub.chatOf(req.Channel)
	if !ok || (isVoice && !c.hub.mayUse(id.Session, voice, false)) {
		return
	}
	if err := c.hub.store.SetReadMark(names.Key(id.Name), req.Channel, req.Message); err != nil {
		slog.Error("read mark", "err", err)
	}
}
