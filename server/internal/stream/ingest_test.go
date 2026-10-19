package stream

import (
	"slices"
	"testing"
	"time"
)

type hooks struct{ keyframes, ended int }

func (k *hooks) hooks() IngestHooks {
	return IngestHooks{Keyframe: func() { k.keyframes++ }, Ended: func() { k.ended++ }}
}

var obs = Config{Codec: "avc1.64001f", Width: 1920, Height: 1080, Audio: "opus"}

func TestIngestNeedsVoiceAndSpeak(t *testing.T) {
	h, v, _ := setup()
	var k hooks
	if err := h.MayStream(9); err == nil {
		t.Fatal("may stream without Speak")
	}
	if _, err := h.StartIngest(9, obs, k.hooks()); err == nil {
		t.Fatal("ingest without Speak")
	}
	delete(v.channels, 1)
	if err := h.MayStream(1); err == nil {
		t.Fatal("may stream outside voice")
	}
	if _, err := h.StartIngest(1, obs, k.hooks()); err == nil {
		t.Fatal("ingest outside voice")
	}
	if _, err := h.StartIngest(2, Config{Codec: "avc1.64001f", Width: 5000, Height: 1080}, k.hooks()); err == nil {
		t.Fatal("ingest above the size limit")
	}
}

func TestIngestFramesAndKeyframes(t *testing.T) {
	h, _, sinks := setup()
	var changes [][]Info
	h.OnChange = func(i []Info) { changes = append(changes, i) }
	var k hooks
	in, err := h.StartIngest(1, obs, k.hooks())
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || !slices.Equal(changes[0], []Info{{Session: 1, Channel: 5, Audio: true}}) {
		t.Fatalf("changes %v", changes)
	}
	if err := h.Watch(2, 1); err != nil {
		t.Fatal(err)
	}
	if sinks[2].texts[0]["codec"] != "avc1.64001f" || sinks[2].texts[0]["width"] != 1920.0 {
		t.Fatalf("viewer texts %v", sinks[2].texts)
	}
	// The keyframe request goes to the hook, not to the streamer's connection.
	if k.keyframes != 1 || slices.Contains(sinks[1].types(), "keyframe") {
		t.Fatalf("keyframes %d, streamer texts %v", k.keyframes, sinks[1].types())
	}
	in.Frame(false, 10, []byte{7})
	in.Frame(true, 20, []byte{0, 0, 0, 1, 0x65})
	in.Frame(false, 30, []byte{9})
	want := [][]byte{
		{0, 0, 0, 1, 1, 0, 0, 0, 0, 0, 0, 0, 20, 0, 0, 0, 1, 0x65},
		{0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 30, 9},
	}
	if !slices.EqualFunc(sinks[2].frames, want, slices.Equal) {
		t.Fatalf("viewer frames %v", sinks[2].frames)
	}
	in.Dropped() // a frame lost on the way: viewers wait for the next keyframe
	in.Frame(false, 40, []byte{9})
	if len(sinks[2].frames) != 2 {
		t.Fatal("a frame after a loss reached the viewer")
	}
	in.Frame(true, 50, []byte{1})
	in.Frame(true, 60, make([]byte, maxFrame)) // too large: dropped, as a loss
	in.Frame(false, 70, []byte{9})
	if len(sinks[2].frames) != 3 {
		t.Fatalf("%d frames: a too large one or a frame after it reached the viewer", len(sinks[2].frames))
	}
}

func TestIngestSound(t *testing.T) {
	h, _, sinks := setup()
	var got []uint64
	h.OnSound = func(streamer uint32, opus []byte, ts uint64) {
		if streamer == 1 && slices.Equal(opus, []byte{0xfc, 1}) {
			got = append(got, ts)
		}
	}
	var k hooks
	in, _ := h.StartIngest(1, obs, k.hooks())
	h.Watch(2, 1)
	in.Sound(20_000, []byte{0xfc, 1})
	if !slices.Equal(got, []uint64{20_000}) || len(sinks[2].frames) != 1 || sinks[2].frames[0][4] != audioFlag {
		t.Fatalf("sound %v, viewer frames %v", got, sinks[2].frames)
	}
	silent, _ := h.StartIngest(3, Config{Codec: "avc1.64001f", Width: 640, Height: 360}, k.hooks())
	silent.Sound(1, []byte{1})
	if len(got) != 1 {
		t.Fatal("sound from a stream without sound")
	}
}

// The streamer's own stream connection neither feeds nor ends an ingest.
func TestIngestIsNotTheConnections(t *testing.T) {
	h, _, sinks := setup()
	var k hooks
	in, _ := h.StartIngest(1, obs, k.hooks())
	h.Watch(2, 1)
	in.Frame(true, 1, []byte{1})
	h.Frame(1, frame(true, 8))
	if len(sinks[2].frames) != 1 {
		t.Fatalf("frames from the connection reached the viewer: %v", sinks[2].frames)
	}
	if err := h.Start(1, Config{Codec: "vp8", Width: 640, Height: 360}); err == nil {
		t.Fatal("a screen share replaced the ingest")
	}
	// A camera from the browser goes with it; the connection's end ends
	// the camera only.
	if err := h.Start(1, Config{Codec: "vp8", Width: 640, Height: 360, Camera: true}); err != nil {
		t.Fatal(err)
	}
	h.Stop(1)
	h.Disconnect(1, sinks[1])
	if s := h.Streams(); len(s) != 1 || s[0].Session != 1 || k.ended != 0 {
		t.Fatalf("streams %v, ended %d: the connection ended the ingest, or not its camera", s, k.ended)
	}
	// And a screen share is not replaced by an ingest.
	h.Start(3, Config{Codec: "vp8", Width: 640, Height: 360})
	if err := h.MayStream(3); err == nil {
		t.Fatal("may stream from OBS while sharing the screen")
	}
	if _, err := h.StartIngest(3, obs, k.hooks()); err == nil {
		t.Fatal("an ingest replaced a screen share")
	}
	if err := h.MayStream(1); err != nil { // its ingest: a new one replaces it
		t.Fatal(err)
	}
}

func TestIngestEnds(t *testing.T) {
	h, v, sinks := setup()
	var k hooks
	in, _ := h.StartIngest(1, obs, k.hooks())
	h.Watch(2, 1)
	h.Moved(1, 6)
	if k.ended != 1 || len(h.Streams()) != 0 || !slices.Contains(sinks[2].types(), "ended") {
		t.Fatalf("ended %d, streams %v", k.ended, h.Streams())
	}
	in.Stop() // too late: nothing to do
	if k.ended != 1 {
		t.Fatal("ended twice")
	}

	// Speak lost: the next frame ends it.
	now := time.Now()
	h.now = func() time.Time { return now }
	var k2 hooks
	in2, _ := h.StartIngest(2, obs, k2.hooks())
	v.mute = map[uint32]bool{2: true}
	now = now.Add(speakEvery)
	in2.Frame(true, 1, []byte{1})
	if k2.ended != 1 || len(h.Streams()) != 0 {
		t.Fatal("an ingest without Speak goes on")
	}

	// Stop by the ingest itself: no Ended hook; a stale one stops nothing.
	var k3 hooks
	in3, _ := h.StartIngest(3, obs, k3.hooks())
	in3.Stop()
	in4, _ := h.StartIngest(3, obs, k3.hooks())
	in3.Stop()
	if k3.ended != 0 || len(h.Streams()) != 1 {
		t.Fatalf("ended %d, streams %v", k3.ended, h.Streams())
	}
	in4.Stop()
}

func TestIngestConfigure(t *testing.T) {
	h, _, sinks := setup()
	var k hooks
	in, _ := h.StartIngest(1, obs, k.hooks())
	h.Watch(2, 1)
	in.Frame(true, 1, []byte{1})
	if err := in.Configure(Config{Codec: "avc1.64001f", Width: 1280, Height: 720, Audio: "opus"}); err != nil {
		t.Fatal(err)
	}
	last := sinks[2].texts[len(sinks[2].texts)-1]
	if last["type"] != "stream" || last["width"] != 1280.0 {
		t.Fatalf("viewer texts %v", sinks[2].texts)
	}
	in.Frame(false, 2, []byte{2})
	if len(sinks[2].frames) != 1 {
		t.Fatal("a delta frame after a new config reached the viewer")
	}
	if err := in.Configure(Config{Codec: "avc1.64001f", Width: 9000, Height: 720}); err == nil {
		t.Fatal("a config above the limits")
	}
}
