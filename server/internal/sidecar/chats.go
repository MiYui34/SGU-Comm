package sidecar

import "kallisti.chat/server/internal/mumble"

// chat is where stored messages live: a text channel or a voice channel's
// chat. Access and delivery go through it.
type chat struct {
	id      int64
	voice   uint32 // with isVoice: the Mumble channel
	isVoice bool
}

// chatOf finds a chat by its store channel.
func (h *Hub) chatOf(channel int64) (chat, bool) {
	if h.channelExists(channel) {
		return chat{id: channel}, true
	}
	if h.voice == nil {
		return chat{}, false
	}
	voice, isVoice, err := h.store.VoiceOf(channel)
	return chat{id: channel, voice: voice, isVoice: true}, err == nil && isVoice
}

// may: the user may read (or write) the chat. Text channels are for all.
func (h *Hub) may(id mumble.Identity, c chat, write bool) bool {
	if !c.isVoice {
		return true
	}
	if write {
		return h.voice.MayWrite(id.Session, c.voice)
	}
	return h.voice.MayRead(id.Session, c.voice)
}

// readers are the connections whose user may read the chat.
func (h *Hub) readers(c chat) []*Conn {
	out := h.all()[:0:0]
	for _, conn := range h.all() {
		conn.mu.Lock()
		id := conn.identity
		conn.mu.Unlock()
		if id != nil && h.may(*id, c, false) {
			out = append(out, conn)
		}
	}
	return out
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
