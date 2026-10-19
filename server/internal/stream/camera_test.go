package stream

import (
	"context"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/coder/websocket"
)

func cameraFrame(key bool, data byte) []byte {
	f := frame(key, data)
	f[0] |= cameraFlag
	return f
}

var camera1 = ID(1, true)

func newTestServer(t *testing.T, hub *Hub, tokens *Tokens) *httptest.Server {
	srv := httptest.NewServer(&Handler{Hub: hub, Redeem: tokens.Redeem, AllowedOrigins: []string{"*"}, BytesPerSecond: 1 << 20})
	t.Cleanup(srv.Close)
	return srv
}

func TestAUserCanShareAScreenAndACamera(t *testing.T) {
	h, _, sinks := setup()
	if err := h.Start(1, Config{Codec: "vp8", Width: 1280, Height: 720}); err != nil {
		t.Fatal(err)
	}
	if err := h.Start(1, Config{Codec: "vp8", Width: 640, Height: 480, Camera: true}); err != nil {
		t.Fatal(err)
	}
	if got := h.Streams(); !slices.Equal(got, []Info{{Session: 1, Channel: 5}, {Session: camera1, Channel: 5}}) {
		t.Fatalf("streams %v", got)
	}
	if err := h.Watch(2, camera1); err != nil {
		t.Fatal(err)
	}
	if m := sinks[2].texts[0]; m["session"] != float64(camera1) || m["width"] != float64(640) {
		t.Fatalf("viewer texts %v", sinks[2].texts)
	}
	if m := sinks[1].texts[len(sinks[1].texts)-1]; m["type"] != "keyframe" || m["camera"] != true {
		t.Fatalf("streamer texts %v", sinks[1].texts)
	}
	// Only the camera's frames reach its viewer, with the camera's id in front.
	h.Frame(1, frame(true, 7))
	h.Frame(1, cameraFrame(true, 8))
	if len(sinks[2].frames) != 1 || sinks[2].frames[0][0] != 0x80 || sinks[2].frames[0][3] != 1 || sinks[2].frames[0][len(sinks[2].frames[0])-1] != 8 {
		t.Fatalf("viewer frames %v", sinks[2].frames)
	}
}

func TestCamerasHaveNoSound(t *testing.T) {
	h, _, sinks := setup()
	if err := h.Start(1, Config{Codec: "vp8", Width: 640, Height: 480, Camera: true, Audio: "opus"}); err == nil {
		t.Fatal("a camera with sound")
	}
	h.Start(1, Config{Codec: "vp8", Width: 640, Height: 480, Camera: true})
	h.Watch(2, camera1)
	f := cameraFrame(false, 1)
	f[0] |= audioFlag
	h.Frame(1, f)
	if len(sinks[2].frames) != 0 {
		t.Fatal("sound of a camera was forwarded")
	}
}

func TestStoppingTheCameraKeepsTheScreen(t *testing.T) {
	h, _, sinks := setup()
	h.Start(1, Config{Codec: "vp8", Width: 1280, Height: 720})
	h.Start(1, Config{Codec: "vp8", Width: 640, Height: 480, Camera: true})
	h.Watch(2, camera1)
	h.Stop(camera1)
	if got := h.Streams(); !slices.Equal(got, []Info{{Session: 1, Channel: 5}}) {
		t.Fatalf("streams %v", got)
	}
	if m := sinks[1].texts[len(sinks[1].texts)-1]; m["type"] != "stopped" || m["camera"] != true {
		t.Fatalf("streamer texts %v", sinks[1].texts)
	}
	if m := sinks[2].texts[len(sinks[2].texts)-1]; m["type"] != "ended" || m["session"] != float64(camera1) {
		t.Fatalf("viewer texts %v", sinks[2].texts)
	}
}

func TestBothStreamsEndWhenTheStreamerLeaves(t *testing.T) {
	h, _, sinks := setup()
	h.Start(1, Config{Codec: "vp8", Width: 1280, Height: 720})
	h.Start(1, Config{Codec: "vp8", Width: 640, Height: 480, Camera: true})
	h.Moved(1, 6)
	if got := h.Streams(); len(got) != 0 {
		t.Fatalf("streams %v", got)
	}
	h.Start(2, Config{Codec: "vp8", Width: 640, Height: 480, Camera: true})
	h.Disconnect(2, sinks[2])
	if got := h.Streams(); len(got) != 0 {
		t.Fatalf("streams %v", got)
	}
}

func TestNobodyWatchesTheirOwnCamera(t *testing.T) {
	h, _, _ := setup()
	h.Start(1, Config{Codec: "vp8", Width: 640, Height: 480, Camera: true})
	if err := h.Watch(1, camera1); err == nil {
		t.Fatal("watched the own camera")
	}
}

func TestHandlerStartsAndStopsACamera(t *testing.T) {
	hub := New(&fakeVoice{channels: map[uint32]uint32{1: 5}})
	tokens := NewTokens()
	srv := newTestServer(t, hub, tokens)
	ws := dialStream(t, srv, tokens.Issue(1))
	ready := readText(t, ws, "ready")
	if limits, _ := ready["limits"].(map[string]any); limits["bytesPerSecond"] != float64(1<<20) || limits["maxSide"] != float64(maxSide) {
		t.Fatalf("ready %v", ready)
	}
	ctx := context.Background()
	ws.Write(ctx, websocket.MessageText, []byte(`{"type":"start","codec":"vp8","width":640,"height":480,"camera":true}`))
	if m := readText(t, ws, "started"); m["camera"] != true {
		t.Fatalf("started %v", m)
	}
	ws.Write(ctx, websocket.MessageText, []byte(`{"type":"stop","camera":true}`))
	if m := readText(t, ws, "stopped"); m["camera"] != true {
		t.Fatalf("stopped %v", m)
	}
}
