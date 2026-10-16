// Package stream forwards screen shares from a streamer in a voice channel
// to the people there who watch: encoded frames (WebCodecs), never decoded
// or re-encoded here.
package stream

import (
	"encoding/binary"
	"errors"
	"maps"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// Voice is the built-in Mumble server: where users are and who may speak.
type Voice interface {
	Channel(session uint32) (uint32, bool)
	MaySpeak(session, channel uint32) bool
}

// Sink is one session's stream connection. Sends never block: SendFrame
// reports false when the connection is full (the frame is dropped). Close
// ends a connection that a newer one of the session replaced.
type Sink interface {
	SendFrame(b []byte) bool
	SendText(v any)
	Close()
}

// Config describes a stream for its viewers' decoders.
type Config struct {
	Codec  string `json:"codec"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Info is a live stream, for everyone's channel list. Preview: its latest
// thumbnail (a JPEG data URL), if any.
type Info struct {
	Session uint32 `json:"session"`
	Channel uint32 `json:"channel"`
	Preview string `json:"preview,omitempty"`
}

const (
	previewPrefix = "data:image/jpeg;base64,"
	maxPreview    = 64 << 10
)

// A frame from a streamer: flags (bit 0: keyframe), an 8-byte timestamp
// (microseconds), the data. Viewers get it with the streamer's session (4
// bytes) in front.
const (
	headerSize    = 9
	keyFlag       = 1
	maxFrame      = 1 << 20
	keyframeEvery = time.Second // keyframe requests to a streamer, at most
	maxSide       = 4096
)

var codecName = regexp.MustCompile(`^[A-Za-z0-9.]{1,64}$`)

func (c Config) valid() bool {
	return codecName.MatchString(c.Codec) && c.Width > 0 && c.Width <= maxSide && c.Height > 0 && c.Height <= maxSide
}

type stream struct {
	channel  uint32
	preview  string
	config   Config
	viewers  map[uint32]*viewer
	askedKey time.Time
}

// viewer: waiting for a keyframe (new, or frames were dropped).
type viewer struct{ needKey bool }

type Hub struct {
	voice Voice
	// OnChange gets the live streams after each change (outside the lock).
	OnChange func([]Info)
	// OnPreview gets a stream's new thumbnail (outside the lock).
	OnPreview func(Info)
	now       func() time.Time

	// changes keeps OnChange calls in order (taken before mu).
	changes sync.Mutex
	mu      sync.Mutex
	sinks   map[uint32]Sink
	streams map[uint32]*stream // by streamer session
}

func New(v Voice) *Hub {
	return &Hub{voice: v, now: time.Now, sinks: map[uint32]Sink{}, streams: map[uint32]*stream{}}
}

// Connect makes s the session's connection; an older one is closed.
func (h *Hub) Connect(session uint32, s Sink) {
	h.mu.Lock()
	old := h.sinks[session]
	h.sinks[session] = s
	h.mu.Unlock()
	if old != nil && old != s {
		old.Close()
	}
}

// Disconnect ends the session's stream and what it watched, if s is still
// its connection (a newer one replaced it otherwise).
func (h *Hub) Disconnect(session uint32, s Sink) {
	h.mu.Lock()
	if h.sinks[session] != s {
		h.mu.Unlock()
		return
	}
	delete(h.sinks, session)
	for s, st := range h.streams {
		h.dropViewerLocked(s, st, session)
	}
	ended := h.endLocked(session)
	h.mu.Unlock()
	if ended {
		h.changed()
	}
}

// Start begins a stream in the streamer's voice channel (needs Speak).
func (h *Hub) Start(session uint32, c Config) error {
	if !c.valid() {
		return errors.New("bad stream settings")
	}
	h.mu.Lock() // the channel is looked up under it: a move waits (see Moved)
	channel, ok := h.voice.Channel(session)
	switch {
	case !ok:
		h.mu.Unlock()
		return errors.New("join a voice channel to share your screen")
	case !h.voice.MaySpeak(session, channel):
		h.mu.Unlock()
		return errors.New("you may not speak (or share your screen) in this channel")
	}
	if st := h.streams[session]; st != nil { // a new config: viewers start over
		st.config = c
		for v, vw := range st.viewers {
			vw.needKey = true
			h.text(v, streamText(session, c))
		}
		h.mu.Unlock()
		return nil
	}
	h.streams[session] = &stream{channel: channel, config: c, viewers: map[uint32]*viewer{}}
	h.mu.Unlock()
	h.changed()
	return nil
}

func (h *Hub) Stop(session uint32) {
	h.mu.Lock()
	ended := h.endLocked(session)
	h.mu.Unlock()
	if ended {
		h.changed()
	}
}

// Watch subscribes a viewer in the stream's channel.
func (h *Hub) Watch(viewerSession, streamer uint32) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	channel, ok := h.voice.Channel(viewerSession)
	st := h.streams[streamer]
	switch {
	case st == nil:
		return errors.New("no such stream")
	case !ok || channel != st.channel:
		return errors.New("join the stream's voice channel to watch")
	case viewerSession == streamer:
		return errors.New("that is your own stream")
	}
	st.viewers[viewerSession] = &viewer{needKey: true}
	h.viewersLocked(streamer, st)
	h.text(viewerSession, streamText(streamer, st.config))
	h.askKeyLocked(streamer, st)
	return nil
}

func (h *Hub) Unwatch(viewerSession, streamer uint32) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if st := h.streams[streamer]; st != nil {
		h.dropViewerLocked(streamer, st, viewerSession)
	}
}

func (h *Hub) dropViewerLocked(streamer uint32, st *stream, viewer uint32) {
	if st.viewers[viewer] != nil {
		delete(st.viewers, viewer)
		h.viewersLocked(streamer, st)
	}
}

// viewersLocked tells the streamer who watches.
func (h *Hub) viewersLocked(streamer uint32, st *stream) {
	h.text(streamer, map[string]any{"type": "viewers", "sessions": append([]uint32{}, slices.Sorted(maps.Keys(st.viewers))...)})
}

// SetPreview keeps a stream's thumbnail (a small JPEG data URL) and passes
// it on.
func (h *Hub) SetPreview(session uint32, image string) error {
	if !strings.HasPrefix(image, previewPrefix) || len(image) > maxPreview {
		return errors.New("bad preview")
	}
	h.mu.Lock()
	st := h.streams[session]
	if st == nil {
		h.mu.Unlock()
		return errors.New("no stream")
	}
	st.preview = image
	info := Info{Session: session, Channel: st.channel, Preview: image}
	h.mu.Unlock()
	if h.OnPreview != nil {
		h.OnPreview(info)
	}
	return nil
}

// Frame forwards a streamer's frame to its viewers. A viewer that is full
// drops frames until the next keyframe (then the picture is whole again).
func (h *Hub) Frame(streamer uint32, f []byte) {
	if len(f) < headerSize || len(f) > maxFrame {
		return
	}
	key := f[0]&keyFlag != 0
	h.mu.Lock()
	defer h.mu.Unlock()
	st := h.streams[streamer]
	if st == nil {
		return
	}
	out := binary.BigEndian.AppendUint32(make([]byte, 0, 4+len(f)), streamer)
	out = append(out, f...)
	for v, vw := range st.viewers {
		if vw.needKey && !key {
			continue
		}
		sink := h.sinks[v]
		if sink == nil {
			continue
		}
		if sink.SendFrame(out) {
			vw.needKey = false
		} else if !vw.needKey { // asked once; a viewer that stays full waits for the periodic keyframe
			vw.needKey = true
			h.askKeyLocked(streamer, st)
		}
	}
}

// Dropped: the server dropped a streamer's frame (its rate limit), so
// viewers wait for a keyframe.
func (h *Hub) Dropped(streamer uint32) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if st := h.streams[streamer]; st != nil {
		for _, vw := range st.viewers {
			vw.needKey = true
		}
		h.askKeyLocked(streamer, st)
	}
}

// RequestKey asks the streamer for a keyframe for a viewer (its decoder is
// new or failed).
func (h *Hub) RequestKey(viewerSession, streamer uint32) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if st := h.streams[streamer]; st != nil && st.viewers[viewerSession] != nil {
		h.askKeyLocked(streamer, st)
	}
}

// Moved follows a session to another channel (mumbled.Config.OnUserMoved):
// a streamer's stream ends; a viewer stops watching streams elsewhere.
func (h *Hub) Moved(session, to uint32) {
	h.mu.Lock()
	ended := false
	if st := h.streams[session]; st != nil && st.channel != to {
		ended = h.endLocked(session)
	}
	for s, st := range h.streams {
		if st.channel != to {
			h.dropViewerLocked(s, st, session)
		}
	}
	h.mu.Unlock()
	if ended {
		h.changed()
	}
}

// Streams are the live streams, by streamer session.
func (h *Hub) Streams() []Info {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := []Info{}
	for _, s := range slices.Sorted(maps.Keys(h.streams)) {
		out = append(out, Info{Session: s, Channel: h.streams[s].channel, Preview: h.streams[s].preview})
	}
	return out
}

func (h *Hub) endLocked(session uint32) bool {
	st := h.streams[session]
	if st == nil {
		return false
	}
	delete(h.streams, session)
	for v := range st.viewers {
		h.text(v, map[string]any{"type": "ended", "session": session})
	}
	h.text(session, map[string]any{"type": "stopped"})
	return true
}

// streamText tells a viewer how to decode a stream.
func streamText(session uint32, c Config) map[string]any {
	return map[string]any{"type": "stream", "session": session, "codec": c.Codec, "width": c.Width, "height": c.Height}
}

func (h *Hub) askKeyLocked(streamer uint32, st *stream) {
	if now := h.now(); now.Sub(st.askedKey) >= keyframeEvery {
		st.askedKey = now
		h.text(streamer, map[string]any{"type": "keyframe"})
	}
}

func (h *Hub) text(session uint32, v any) {
	if s := h.sinks[session]; s != nil {
		s.SendText(v)
	}
}

func (h *Hub) changed() {
	h.changes.Lock()
	defer h.changes.Unlock()
	if h.OnChange != nil {
		h.OnChange(h.Streams())
	}
}
