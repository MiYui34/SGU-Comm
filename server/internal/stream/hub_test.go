package stream

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"
)

// fakeVoice: users' channels; session 9 may not speak anywhere.
type fakeVoice struct{ channels map[uint32]uint32 }

func (f *fakeVoice) Channel(session uint32) (uint32, bool) {
	c, ok := f.channels[session]
	return c, ok
}
func (f *fakeVoice) MaySpeak(session, _ uint32) bool { return session != 9 }

// sink collects what a session's connection gets.
type sink struct {
	frames [][]byte
	texts  []map[string]any
	room   int // frames it takes before it is full (-1: any)
	closed bool
}

func (s *sink) Close() { s.closed = true }

func (s *sink) SendFrame(b []byte) bool {
	if s.room == 0 {
		return false
	}
	s.room--
	s.frames = append(s.frames, b)
	return true
}
func (s *sink) SendText(v any) {
	var m map[string]any
	b, _ := json.Marshal(v)
	json.Unmarshal(b, &m)
	s.texts = append(s.texts, m)
}
func (s *sink) types() []string {
	var out []string
	for _, m := range s.texts {
		out = append(out, m["type"].(string))
	}
	return out
}

func setup() (*Hub, *fakeVoice, map[uint32]*sink) {
	v := &fakeVoice{channels: map[uint32]uint32{1: 5, 2: 5, 3: 6, 9: 5}}
	h := New(v)
	sinks := map[uint32]*sink{}
	for _, s := range []uint32{1, 2, 3, 9} {
		sinks[s] = &sink{room: -1}
		h.Connect(s, sinks[s])
	}
	return h, v, sinks
}

func frame(key bool, data byte) []byte {
	f := byte(0)
	if key {
		f = 1
	}
	return []byte{f, 0, 0, 0, 0, 0, 0, 0, 1, data}
}

func TestStartWatchAndFrames(t *testing.T) {
	h, _, sinks := setup()
	var changes [][]Info
	h.OnChange = func(i []Info) { changes = append(changes, i) }
	if err := h.Start(1, Config{Codec: "avc1.42E01F", Width: 1280, Height: 720}); err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || !slices.Equal(changes[0], []Info{{Session: 1, Channel: 5}}) {
		t.Fatalf("changes %v", changes)
	}
	if err := h.Watch(3, 1); err == nil {
		t.Fatal("a viewer in another channel watches")
	}
	if err := h.Watch(2, 1); err != nil {
		t.Fatal(err)
	}
	// The viewer gets the stream's config, the streamer is asked for a keyframe.
	if got := sinks[2].types(); !slices.Equal(got, []string{"stream"}) || sinks[2].texts[0]["codec"] != "avc1.42E01F" {
		t.Fatalf("viewer texts %v", sinks[2].texts)
	}
	if got := sinks[1].types(); !slices.Contains(got, "keyframe") {
		t.Fatalf("streamer texts %v", got)
	}
	// Frames before the first keyframe are not sent; then all are, with the
	// streamer's session in front.
	h.Frame(1, frame(false, 7))
	h.Frame(1, frame(true, 8))
	h.Frame(1, frame(false, 9))
	if len(sinks[2].frames) != 2 || sinks[2].frames[0][3] != 1 || sinks[2].frames[0][len(sinks[2].frames[0])-1] != 8 {
		t.Fatalf("viewer frames %v", sinks[2].frames)
	}
	if len(sinks[3].frames) != 0 || len(sinks[9].frames) != 0 {
		t.Fatal("frames to people not watching")
	}
}

func TestStartNeedsSpeakAndVoice(t *testing.T) {
	h, v, _ := setup()
	if err := h.Start(9, Config{Codec: "vp8", Width: 640, Height: 360}); err == nil {
		t.Fatal("started without Speak")
	}
	delete(v.channels, 1)
	if err := h.Start(1, Config{Codec: "vp8", Width: 640, Height: 360}); err == nil {
		t.Fatal("started outside voice")
	}
}

func TestSlowViewersDropFramesUntilAKeyframe(t *testing.T) {
	h, _, sinks := setup()
	h.Start(1, Config{Codec: "vp8", Width: 640, Height: 360})
	h.Watch(2, 1)
	h.Frame(1, frame(true, 1))
	sinks[2].room = 0 // full
	h.Frame(1, frame(false, 2))
	sinks[2].room = -1
	h.Frame(1, frame(false, 3)) // dropped: waits for a keyframe
	h.Frame(1, frame(true, 4))
	var got []byte
	for _, f := range sinks[2].frames {
		got = append(got, f[len(f)-1])
	}
	if !slices.Equal(got, []byte{1, 4}) {
		t.Fatalf("viewer got %v", got)
	}
}

func TestStreamsEndWhenTheStreamerLeaves(t *testing.T) {
	h, _, sinks := setup()
	var last []Info
	h.OnChange = func(i []Info) { last = i }
	h.Start(1, Config{Codec: "vp8", Width: 640, Height: 360})
	h.Watch(2, 1)
	h.Moved(1, 6)
	if len(last) != 0 || !slices.Contains(sinks[2].types(), "ended") {
		t.Fatalf("streams %v, viewer %v", last, sinks[2].types())
	}
	h.Frame(1, frame(true, 1))
	if len(sinks[2].frames) != 0 {
		t.Fatal("frames after the end")
	}
}

func TestViewersWhoLeaveStopWatching(t *testing.T) {
	h, _, sinks := setup()
	h.Start(1, Config{Codec: "vp8", Width: 640, Height: 360})
	h.Watch(2, 1)
	h.Moved(2, 6)
	h.Frame(1, frame(true, 1))
	if len(sinks[2].frames) != 0 {
		t.Fatal("a viewer who left still gets frames")
	}
	h.Disconnect(1, &sink{}) // an old connection: nothing happens
	if len(h.Streams()) != 1 {
		t.Fatal("an old connection ended the stream")
	}
	h.Disconnect(1, sinks[1])
	if len(h.Streams()) != 0 {
		t.Fatal("a stream outlives its connection")
	}
}

func TestStartChecksTheConfig(t *testing.T) {
	h, _, _ := setup()
	for _, c := range []Config{{Codec: strings.Repeat("v", 100), Width: 640, Height: 360}, {Codec: "vp8", Width: 0, Height: 360}, {Codec: "vp8 <x>", Width: 640, Height: 360}, {Codec: "vp8", Width: 99999, Height: 360}} {
		if err := h.Start(1, c); err == nil {
			t.Fatalf("accepted %+v", c)
		}
	}
}

// A new connection of a session closes the old one (one upload budget per
// session).
func TestANewConnectionReplacesTheOld(t *testing.T) {
	h, _, _ := setup()
	old := &sink{room: -1}
	h.Connect(4, old)
	if !old.closed {
		h.Connect(4, &sink{room: -1})
	}
	if !old.closed {
		t.Fatal("the old connection is still open")
	}
}

// Frames the server dropped (rate limit): viewers wait for a keyframe.
func TestDroppedFramesMakeViewersWaitForAKeyframe(t *testing.T) {
	h, _, sinks := setup()
	h.Start(1, Config{Codec: "vp8", Width: 640, Height: 360})
	h.Watch(2, 1)
	h.Frame(1, frame(true, 1))
	h.Dropped(1)
	h.Frame(1, frame(false, 2))
	h.Frame(1, frame(true, 3))
	if len(sinks[2].frames) != 2 || sinks[2].frames[1][len(sinks[2].frames[1])-1] != 3 {
		t.Fatalf("viewer frames %v", sinks[2].frames)
	}
}

// A viewer asks for a keyframe (its decoder is new); only viewers may.
func TestViewersAskForKeyframes(t *testing.T) {
	h, _, sinks := setup()
	h.now = func() time.Time { return time.Unix(100, 0) }
	h.Start(1, Config{Codec: "vp8", Width: 640, Height: 360})
	h.Watch(2, 1)
	asked := func() int { return strings.Count(strings.Join(sinks[1].types(), ","), "keyframe") }
	before := asked()
	h.now = func() time.Time { return time.Unix(102, 0) }
	h.RequestKey(3, 1) // not a viewer
	if asked() != before {
		t.Fatal("a non-viewer asked for a keyframe")
	}
	h.RequestKey(2, 1)
	if asked() != before+1 {
		t.Fatal("the viewer's request did not reach the streamer")
	}
}

func lastViewers(s *sink) []any {
	for i := len(s.texts) - 1; i >= 0; i-- {
		if s.texts[i]["type"] == "viewers" {
			return s.texts[i]["sessions"].([]any)
		}
	}
	return nil
}

func TestTheStreamerSeesItsViewers(t *testing.T) {
	h, v, sinks := setup()
	v.channels[3] = 5
	h.Start(1, Config{Codec: "vp8", Width: 640, Height: 360})
	h.Watch(2, 1)
	h.Watch(3, 1)
	if got := lastViewers(sinks[1]); len(got) != 2 || got[0] != float64(2) || got[1] != float64(3) {
		t.Fatalf("viewers %v", got)
	}
	h.Unwatch(2, 1)
	h.Moved(3, 6)
	if got := lastViewers(sinks[1]); len(got) != 0 {
		t.Fatalf("viewers after leaving %v", got)
	}
}

func TestPreviews(t *testing.T) {
	h, _, _ := setup()
	var got []Info
	h.OnPreview = func(i Info) { got = append(got, i) }
	img := "data:image/jpeg;base64,/9j/4AAQ"
	if err := h.SetPreview(1, img); err == nil {
		t.Fatal("a preview without a stream")
	}
	h.Start(1, Config{Codec: "vp8", Width: 640, Height: 360})
	for _, bad := range []string{"data:image/svg+xml;base64,PHN2Zz4=", "javascript:alert(1)", "data:image/jpeg;base64," + strings.Repeat("A", 70000)} {
		if err := h.SetPreview(1, bad); err == nil {
			t.Fatalf("accepted %.40q", bad)
		}
	}
	if err := h.SetPreview(1, img); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Preview != img || got[0].Channel != 5 {
		t.Fatalf("previews %v", got)
	}
	if s := h.Streams(); s[0].Preview != img {
		t.Fatalf("streams %v", s)
	}
}

func audio(data byte) []byte { return []byte{2, 0, 0, 0, 0, 0, 0, 0, 1, data} }

// Sound goes to every viewer, also one that waits for a video keyframe.
func TestAudioFramesReachAllViewers(t *testing.T) {
	h, _, sinks := setup()
	h.Start(1, Config{Codec: "vp8", Width: 640, Height: 360, Audio: "opus"})
	h.Watch(2, 1)
	h.Frame(1, audio(7))
	h.Frame(1, frame(false, 8)) // video before a keyframe: not sent
	if len(sinks[2].frames) != 1 || sinks[2].frames[0][len(sinks[2].frames[0])-1] != 7 {
		t.Fatalf("viewer frames %v", sinks[2].frames)
	}
	if sinks[2].texts[0]["audio"] != "opus" {
		t.Fatalf("stream text %v", sinks[2].texts[0])
	}
	if err := h.Start(1, Config{Codec: "vp8", Width: 640, Height: 360, Audio: "mp3"}); err == nil {
		t.Fatal("accepted another sound codec")
	}
}
