package stream

import (
	"encoding/binary"
	"errors"
)

// Ingest is a stream that the server feeds itself (WHIP from OBS) for a
// streamer's voice session. It looks like a screen share to everyone; the
// streamer's own stream connection neither feeds nor ends it.
type Ingest struct {
	h       *Hub
	session uint32
	hooks   IngestHooks
	stopped bool // by Stop (no Ended hook)
}

// IngestHooks are called under the hub's lock: they must not block or call
// the hub.
type IngestHooks struct {
	Keyframe func() // viewers need a keyframe (at most one call a second)
	Ended    func() // the hub ended the stream (a move, a leave, Speak gone)
}

// MayStream: the session is in a voice channel and may speak there.
func (h *Hub) MayStream(session uint32) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.mayStreamLocked(session)
	return err
}

// StartIngest begins an ingest stream in the streamer's voice channel
// (needs Speak, and no screen share of the session).
func (h *Hub) StartIngest(session uint32, c Config, hooks IngestHooks) (*Ingest, error) {
	if !c.valid() {
		return nil, errors.New("bad stream settings")
	}
	h.mu.Lock()
	channel, err := h.mayStreamLocked(session)
	if err == nil && h.streams[session] != nil {
		err = errors.New("you are sharing your screen already")
	}
	if err != nil {
		h.mu.Unlock()
		return nil, err
	}
	in := &Ingest{h: h, session: session, hooks: hooks}
	h.streams[session] = &stream{channel: channel, config: c, viewers: map[uint32]*viewer{}, speakAt: h.now(), ingest: in}
	h.mu.Unlock()
	h.changed()
	return in, nil
}

// streamLocked is the ingest's stream (nil: it ended).
func (in *Ingest) streamLocked() *stream {
	if st := in.h.streams[in.session]; st != nil && st.ingest == in {
		return st
	}
	return nil
}

// Configure changes the stream's settings (a new size): viewers start over.
func (in *Ingest) Configure(c Config) error {
	if !c.valid() {
		return errors.New("bad stream settings")
	}
	h := in.h
	h.mu.Lock()
	st := in.streamLocked()
	if st == nil {
		h.mu.Unlock()
		return errors.New("the stream ended")
	}
	soundChanged := st.config.Audio != c.Audio
	st.config = c
	for v, vw := range st.viewers {
		vw.needKey = true
		h.text(v, streamText(in.session, c))
	}
	h.mu.Unlock()
	if soundChanged {
		h.changed()
	}
	return nil
}

// Frame forwards an encoded picture (timestamp in microseconds, on the
// sound's clock).
func (in *Ingest) Frame(key bool, timestamp uint64, data []byte) {
	flags := byte(0)
	if key {
		flags = keyFlag
	}
	in.h.forward(in.session, ingestFrame(flags, timestamp, data), in)
}

// Sound forwards an Opus packet.
func (in *Ingest) Sound(timestamp uint64, opus []byte) {
	in.h.forward(in.session, ingestFrame(audioFlag, timestamp, opus), in)
}

func ingestFrame(flags byte, timestamp uint64, data []byte) []byte {
	f := binary.BigEndian.AppendUint64(append(make([]byte, 0, headerSize+len(data)), flags), timestamp)
	return append(f, data...)
}

// Dropped: a picture was lost (on the way, or over the rate limit), so
// viewers wait for a keyframe.
func (in *Ingest) Dropped() {
	h := in.h
	h.mu.Lock()
	defer h.mu.Unlock()
	if st := in.streamLocked(); st != nil {
		h.droppedLocked(in.session, st)
	}
}

// Stop ends the stream (if it is still this ingest's).
func (in *Ingest) Stop() {
	h := in.h
	h.mu.Lock()
	ended := false
	if in.streamLocked() != nil {
		in.stopped = true
		ended = h.endLocked(in.session)
	}
	h.mu.Unlock()
	if ended {
		h.changed()
	}
}
