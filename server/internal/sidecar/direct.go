package sidecar

import (
	"html"
	"log/slog"
	"slices"
	"strings"
	"unicode/utf8"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/store"
)

// Direct messages between registered users (built-in server): a stored
// chat per pair, readable and writable by the two only.

type wirePeer struct {
	ID   uint32 `json:"id"`
	Name string `json:"name"`
}

type wireDirect struct {
	Channel int64    `json:"channel"`
	Peer    wirePeer `json:"peer"`
}

func directEvent(channel int64, peer uint32, name string) map[string]any {
	return map[string]any{"type": "dm", "channel": channel, "peer": wirePeer{peer, name}}
}

// direct answers the chat with a user (made if new).
func (c *Conn) direct(id mumble.Identity, user uint32) {
	h := c.hub
	switch {
	case h.voice == nil || !id.Registered:
		c.sendError("direct messages need an account")
		return
	case user == id.UserID:
		c.sendError("you cannot write to yourself")
		return
	}
	name := h.voice.UserNames([]uint32{user})[user]
	if name == "" {
		c.sendError("unknown user")
		return
	}
	channel, err := h.store.DirectChat(id.UserID, user)
	if err != nil {
		slog.Error("direct chat", "err", err)
		c.sendError("cannot open the conversation")
		return
	}
	c.send(directEvent(channel, user, name))
}

// announceDirect tells the two users' connections about the chat (each
// with the other user), before a message: the recipient may not know it.
func (h *Hub) announceDirect(ch chat) {
	names := h.voice.UserNames(ch.peers[:])
	for _, conn := range h.readers(ch) {
		conn.mu.Lock()
		me := conn.identity.UserID
		conn.mu.Unlock()
		other := ch.peers[0] + ch.peers[1] - me
		conn.send(directEvent(ch.id, other, names[other]))
	}
}

// MumblePrivateText stores a private message from a Mumble client to a
// registered user (mumbled.Config.OnPrivateText) and sends it to the two
// users' web connections.
func (h *Hub) MumblePrivateText(from mumble.Identity, to uint32, text string, images []Image) {
	if h.voice == nil || !from.Registered || from.UserID == to {
		return
	}
	if utf8.RuneCountInString(text) > MaxTextLength {
		text = string([]rune(text)[:MaxTextLength])
	}
	channel, err := h.store.DirectChat(from.UserID, to)
	if err != nil {
		slog.Error("direct chat", "err", err)
		return
	}
	ch := chat{id: channel, direct: true, peers: [2]uint32{min(from.UserID, to), max(from.UserID, to)}}
	h.announceDirect(ch)
	h.storeMumble(from, ch, text, images)
}

// relayDirect sends a stored direct message to the other user's Mumble
// clients; seen there, it counts as read.
func (h *Hub) relayDirect(from mumble.Identity, ch chat, m store.Message) {
	other := ch.peers[0] + ch.peers[1] - from.UserID
	if h.voice.SendPrivateText(from.Session, other, h.mumbleText(m)) {
		if err := h.store.SetReadMark(other, ch.id, m.ID); err != nil {
			slog.Error("read mark", "err", err)
		}
	}
}

// Messages per direct chat a joining Mumble client gets, at most.
const missedDirects = 20

// sendMissedDirects tells a registered user's joining Mumble client the
// direct messages it has not read, then marks them read.
func (h *Hub) sendMissedDirects(session, user uint32) {
	chats, err := h.store.DirectChats(user)
	if err != nil || len(chats) == 0 {
		return
	}
	marks, err := h.store.ReadMarks(user)
	if err != nil {
		slog.Error("read marks", "err", err)
		return
	}
	peers := make([]uint32, 0, len(chats))
	for _, p := range chats {
		peers = append(peers, p)
	}
	names := h.voice.UserNames(peers)
	for channel, peer := range chats {
		latest, msgs, err := h.store.MessagesAfter(channel, marks[channel], "")
		if err != nil {
			slog.Error("missed direct messages", "err", err)
			continue
		}
		msgs = slices.DeleteFunc(msgs, func(m store.Message) bool { return m.AuthorID != nil && *m.AuthorID == user })
		if len(msgs) == 0 {
			continue
		}
		slices.Reverse(msgs) // oldest first
		msgs = msgs[max(0, len(msgs)-missedDirects):]
		var b strings.Builder
		b.WriteString("<b>Direct messages from " + html.EscapeString(names[peer]) + " while you were away:</b>")
		for _, m := range msgs {
			at := m.CreatedAt.UTC().Format("Jan 2 15:04")
			b.WriteString("<br><i>" + at + " UTC</i>: " + strings.ReplaceAll(html.EscapeString(h.mumbleText(m)), "\n", "<br>"))
		}
		h.voice.SendServerText(session, b.String())
		if err := h.store.SetReadMark(user, channel, latest); err != nil {
			slog.Error("read mark", "err", err)
		}
	}
}

// directs are a user's conversations with messages, for ready.
func (h *Hub) directs(id mumble.Identity) []wireDirect {
	out := []wireDirect{}
	chats, err := h.store.DirectChats(id.UserID)
	if err != nil {
		slog.Error("direct chats", "err", err)
		return out
	}
	peers := make([]uint32, 0, len(chats))
	for _, p := range chats {
		peers = append(peers, p)
	}
	names := h.voice.UserNames(peers)
	for channel, p := range chats {
		out = append(out, wireDirect{channel, wirePeer{p, names[p]}})
	}
	return out
}
