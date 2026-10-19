// Package stream forwards screen shares and cameras from a streamer in a
// voice channel to the people there who watch: encoded frames (WebCodecs),
// never decoded or re-encoded here. A user has at most one screen share
// and one camera.
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
	Audio  string `json:"audio,omitempty"`  // "opus" (48 kHz mono, 20 ms frames) or none
	Camera bool   `json:"camera,omitempty"` // a camera (no sound), else a screen share
}

// CameraBit marks a camera's stream id; a screen share's id is its
// streamer's session.
const CameraBit = 1 << 31

// ID is the id of a session's screen share or camera.
func ID(session uint32, camera bool) uint32 {
	if camera {
		return session | CameraBit
	}
	return session
}

func owner(id uint32) uint32 { return id &^ CameraBit }

// Info is a live stream, for everyone's channel list. Session: its id (see
// ID). Preview: its latest thumbnail (a JPEG data URL), if any.
type Info struct {
	Session uint32 `json:"session"`
	Channel uint32 `json:"channel"`
	Preview string `json:"preview,omitempty"`
	Audio   bool   `json:"-"` // it has sound
}

const (
	previewPrefix = "data:image/jpeg;base64,"
	maxPreview    = 64 << 10
)

// A frame from a streamer: flags (bit 0: keyframe, bit 1: sound, bit 2:
// the camera's), an 8-byte timestamp (microseconds; picture and sound on
// one clock), the data. Viewers get it with the stream's id (4 bytes) in
// front.
const (
	headerSize    = 9
	keyFlag       = 1
	audioFlag     = 2
	cameraFlag    = 4
	maxFrame      = 1 << 20
	keyframeEvery = time.Second // keyframe requests to a streamer, at most
	maxSound      = 1500        // bytes of one Opus packet (20 ms), at most
	speakEvery    = 2 * time.Second
	previewEvery  = 5 * time.Second // thumbnails, at most
	maxSide       = 4096
)

var codecName = regexp.MustCompile(`^[A-Za-z0-9.]{1,64}$`)

func (c Config) valid() bool {
	return codecName.MatchString(c.Codec) && (c.Audio == "" || c.Audio == "opus" && !c.Camera) && c.Width > 0 && c.Width <= maxSide && c.Height > 0 && c.Height <= maxSide
}

type stream struct {
	channel uint32
	preview string
	// When the streamer's Speak was checked last, and the last thumbnail
	// came.
	speakAt, previewAt time.Time
	config             Config
	viewers            map[uint32]*viewer
	askedKey           time.Time
}

// viewer: waiting for a keyframe (new, or frames were dropped).
type viewer struct{ needKey bool }

type Hub struct {
	voice Voice
	// OnChange gets the live streams after each change (outside the lock).
	OnChange func([]Info)
	// OnPreview gets a stream's new thumbnail (outside the lock).
	OnPreview func(Info)
	// OnSound gets a stream's sound frames (Opus; timestamp in
	// microseconds), outside the lock: for Mumble clients.
	OnSound func(streamer uint32, opus []byte, timestamp uint64)
	now     func() time.Time

	// changes keeps OnChange calls in order (taken before mu).
	changes sync.Mutex
	mu      sync.Mutex
	sinks   map[uint32]Sink
	streams map[uint32]*stream // by id (see ID)
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
	ended = h.endLocked(ID(session, true)) || ended
	h.mu.Unlock()
	if ended {
		h.changed()
	}
}

// Start begins a stream in the streamer's voice channel (needs Speak), or
// changes a running one's config (viewers get the new one).
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
	id := ID(session, c.Camera)
	if st := h.streams[id]; st != nil { // a new config: viewers start over
		soundChanged := st.config.Audio != c.Audio
		st.config = c
		for v, vw := range st.viewers {
			vw.needKey = true
			h.text(v, streamText(id, c))
		}
		h.mu.Unlock()
		if soundChanged {
			h.changed()
		}
		return nil
	}
	h.streams[id] = &stream{channel: channel, config: c, viewers: map[uint32]*viewer{}, speakAt: h.now()}
	h.mu.Unlock()
	h.changed()
	return nil
}

// Stop ends a stream (by id).
func (h *Hub) Stop(id uint32) {
	h.mu.Lock()
	ended := h.endLocked(id)
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
	case viewerSession == owner(streamer):
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
func (h *Hub) viewersLocked(id uint32, st *stream) {
	h.toStreamer(id, map[string]any{"type": "viewers", "sessions": append([]uint32{}, slices.Sorted(maps.Keys(st.viewers))...)})
}

// SetPreview keeps a stream's thumbnail (a small JPEG data URL, at most
// one every few seconds) and passes it on; "" clears it (previews turned
// off). session: the stream's id.
func (h *Hub) SetPreview(session uint32, image string) error {
	if image != "" && (!strings.HasPrefix(image, previewPrefix) || len(image) > maxPreview) {
		return errors.New("bad preview")
	}
	h.mu.Lock()
	st := h.streams[session]
	if st == nil {
		h.mu.Unlock()
		return errors.New("no stream")
	}
	now := h.now()
	if image == st.preview || image != "" && now.Sub(st.previewAt) < previewEvery {
		h.mu.Unlock()
		return nil
	}
	st.preview, st.previewAt = image, now
	info := Info{Session: session, Channel: st.channel, Preview: image}
	h.mu.Unlock()
	if h.OnPreview != nil {
		h.OnPreview(info)
	}
	return nil
}

// Frame forwards a streamer's frame to the viewers of its screen share or
// camera. A viewer that is full drops frames until the next keyframe (then
// the picture is whole again).
func (h *Hub) Frame(session uint32, f []byte) {
	if len(f) < headerSize || len(f) > maxFrame {
		return
	}
	key, sound := f[0]&keyFlag != 0, f[0]&audioFlag != 0
	streamer := FrameID(session, f)
	h.mu.Lock()
	st := h.streams[streamer]
	if st == nil || sound && (st.config.Audio == "" || len(f)-headerSize > maxSound) {
		h.mu.Unlock()
		return
	}
	// Speak can go (an admin's mute, an ACL change): then so does the stream.
	if now := h.now(); now.Sub(st.speakAt) >= speakEvery {
		st.speakAt = now
		if !h.voice.MaySpeak(session, st.channel) {
			h.endLocked(streamer)
			h.mu.Unlock()
			h.changed()
			return
		}
	}
	defer func() {
		h.mu.Unlock()
		if sound && h.OnSound != nil {
			h.OnSound(session, f[headerSize:], binary.BigEndian.Uint64(f[1:headerSize]))
		}
	}()
	out := binary.BigEndian.AppendUint32(make([]byte, 0, 4+len(f)), streamer)
	out = append(out, f...)
	for v, vw := range st.viewers {
		sink := h.sinks[v]
		if sink == nil {
			continue
		}
		if sound { // no keyframes: a full viewer loses this bit of sound
			sink.SendFrame(out)
			continue
		}
		if vw.needKey && !key {
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

// FrameID is the id of the stream a session's frame belongs to.
func FrameID(session uint32, f []byte) uint32 {
	return ID(session, len(f) > 0 && f[0]&cameraFlag != 0)
}

// Dropped: the server dropped a frame of a stream (by id; its rate limit),
// so viewers wait for a keyframe.
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
// a streamer's streams end; a viewer stops watching streams elsewhere.
func (h *Hub) Moved(session, to uint32) {
	h.mu.Lock()
	ended := false
	for _, id := range []uint32{ID(session, false), ID(session, true)} {
		if st := h.streams[id]; st != nil && st.channel != to {
			ended = h.endLocked(id) || ended
		}
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

// Streams are the live streams, by id.
func (h *Hub) Streams() []Info {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := []Info{}
	for _, s := range slices.Sorted(maps.Keys(h.streams)) {
		st := h.streams[s]
		out = append(out, Info{Session: s, Channel: st.channel, Preview: st.preview, Audio: st.config.Audio != ""})
	}
	return out
}

func (h *Hub) endLocked(id uint32) bool {
	st := h.streams[id]
	if st == nil {
		return false
	}
	delete(h.streams, id)
	for v := range st.viewers {
		h.text(v, map[string]any{"type": "ended", "session": id})
	}
	h.toStreamer(id, map[string]any{"type": "stopped"})
	return true
}

// streamText tells a viewer how to decode a stream.
func streamText(session uint32, c Config) map[string]any {
	return map[string]any{"type": "stream", "session": session, "codec": c.Codec, "width": c.Width, "height": c.Height, "audio": c.Audio}
}

func (h *Hub) askKeyLocked(id uint32, st *stream) {
	if now := h.now(); now.Sub(st.askedKey) >= keyframeEvery {
		st.askedKey = now
		h.toStreamer(id, map[string]any{"type": "keyframe"})
	}
}

// toStreamer sends a text about a stream to its streamer, marked when it
// is about the camera.
func (h *Hub) toStreamer(id uint32, m map[string]any) {
	if id&CameraBit != 0 {
		m["camera"] = true
	}
	h.text(owner(id), m)
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
