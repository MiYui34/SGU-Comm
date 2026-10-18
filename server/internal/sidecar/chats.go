package sidecar

import "kallisti.chat/server/internal/mumble"

// chat is where stored messages live: a text channel, a voice channel's
// chat, or a direct chat of two users. Access and delivery go through it.
type chat struct {
	id      int64
	voice   uint32 // with isVoice: the Mumble channel
	isVoice bool
	direct  bool
	peers   [2]uint32 // with direct: the two users' ids
}

// chatOf finds a chat by its store channel.
func (h *Hub) chatOf(channel int64) (chat, bool) {
	if h.channelExists(channel) {
		return chat{id: channel}, true
	}
	if h.voice == nil {
		return chat{}, false
	}
	if voice, isVoice, err := h.store.VoiceOf(channel); err != nil || isVoice {
		return chat{id: channel, voice: voice, isVoice: true}, err == nil
	}
	a, b, direct, err := h.store.DirectPeers(channel)
	return chat{id: channel, direct: true, peers: [2]uint32{a, b}}, err == nil && direct
}

// may: the user may read (or write) the chat. Text channels are for all
// who see their mirror (private ones for members and staff).
func (h *Hub) may(id mumble.Identity, c chat, write bool) bool {
	if c.direct {
		return id.Registered && (id.UserID == c.peers[0] || id.UserID == c.peers[1])
	}
	if !c.isVoice {
		mirror := h.mirrorOf(c.id)
		return h.voice == nil || mirror == 0 || h.voice.Sees(id.Session, mirror)
	}
	if write {
		return h.voice.MayWrite(id.Session, c.voice)
	}
	return h.voice.MayRead(id.Session, c.voice)
}

// readers are the connections whose user may read the chat.
func (h *Hub) readers(c chat) []*Conn {
	var out []*Conn
	h.eachReady(func(conn *Conn, id mumble.Identity) {
		if h.may(id, c, false) {
			out = append(out, conn)
		}
	})
	return out
}

// eachReady calls f for each ready connection, with its user.
func (h *Hub) eachReady(f func(*Conn, mumble.Identity)) {
	for _, conn := range h.all() {
		conn.mu.Lock()
		id := conn.identity
		conn.mu.Unlock()
		if id != nil {
			f(conn, *id)
		}
	}
}

// sendTo sends to the chat's readers.
func (h *Hub) sendTo(c chat, v any) {
	for _, conn := range h.readers(c) {
		conn.send(v)
	}
}

func (h *Hub) all() []*Conn {
	h.mu.Lock()
	defer h.mu.Unlock()
	conns := make([]*Conn, 0, len(h.conns))
	for c := range h.conns {
		conns = append(conns, c)
	}
	return conns
}
