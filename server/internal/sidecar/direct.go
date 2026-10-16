package sidecar

import (
	"log/slog"

	"kallisti.chat/server/internal/mumble"
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
