package sidecar

import (
	"errors"
	"slices"
	"sync"
	"testing"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/recording"
)

// fakeRecorder: session 1 is staff.
type fakeRecorder struct {
	mu        sync.Mutex
	recording []uint32
}

func (f *fakeRecorder) MayRecord(session uint32) bool { return session == 1 }

func (f *fakeRecorder) StartRecording(session, channel uint32) error {
	if session != 1 {
		return errors.New("only admins and moderators record channels")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recording = append(f.recording, channel)
	return nil
}

func (f *fakeRecorder) StopRecording(session, channel uint32) error {
	if session != 1 {
		return errors.New("only admins and moderators record channels")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recording = slices.DeleteFunc(f.recording, func(c uint32) bool { return c == channel })
	return nil
}

func (f *fakeRecorder) Recording() []uint32 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.recording)
}

func (f *fakeRecorder) Finished() []recording.Info {
	return []recording.Info{{ID: "0123456789abcdef", Channel: 2, ChannelName: "Lounge", Tracks: []recording.TrackInfo{{Name: "alice"}}}}
}

func (f *fakeRecorder) DownloadToken(session uint32) string { return "token" }

func channels(v any) []float64 {
	var out []float64
	for _, c := range v.([]any) {
		out = append(out, c.(float64))
	}
	return out
}

// The indicator reaches everyone who sees the channel, also those who
// connect later (channel 7 is hidden from session 2 in the fake).
func TestRecordingIndicatorReachesThoseWhoSeeTheChannel(t *testing.T) {
	h := newHub(t, "general")
	v := &fakeVoice{hidden: map[uint32]bool{7: true}}
	h.SetVoice(v, "")
	rec := &fakeRecorder{}
	h.Recorder = rec
	staff := readyAs(t, h, 1, "mod")
	user := readyAs(t, h, 2, "bob")

	staff.Handle([]byte(`{"type":"start-recording","voice":2}`))
	staff.Handle([]byte(`{"type":"start-recording","voice":7}`))
	h.RecordingChanged() // as the voice server reports it
	if m := next(t, staff); m["type"] != "recording" || !slices.Equal(channels(m["channels"]), []float64{2, 7}) {
		t.Fatalf("staff got %v", m)
	}
	if m := next(t, user); m["type"] != "recording" || !slices.Equal(channels(m["channels"]), []float64{2}) {
		t.Fatalf("user got %v", m)
	}
	late := h.Connect()
	t.Cleanup(late.Close)
	late.SetIdentity(mumble.Identity{Session: 3, Name: "carol"})
	if m := next(t, late); !slices.Equal(channels(m["recording"]), []float64{2}) {
		t.Fatalf("ready %v", m["recording"])
	}

	user.Handle([]byte(`{"type":"stop-recording","voice":2}`))
	if m := next(t, user); m["type"] != "error" || len(rec.Recording()) != 2 {
		t.Fatalf("user stopped it: %v", m)
	}

	// Let in to the hidden channel: the indicator comes too.
	v.mu.Lock()
	v.hidden = nil
	v.mu.Unlock()
	h.PrivateChanged()
	for {
		if m := next(t, user); m["type"] == "recording" {
			if !slices.Equal(channels(m["channels"]), []float64{2, 7}) {
				t.Fatalf("after access %v", m)
			}
			break
		}
	}
}

func TestOnlyStaffListAndDownloadRecordings(t *testing.T) {
	h := newHub(t, "general")
	h.SetVoice(&fakeVoice{}, "")
	h.Recorder = &fakeRecorder{}
	staff := readyAs(t, h, 1, "mod")
	user := readyAs(t, h, 2, "bob")

	staff.Handle([]byte(`{"type":"recordings"}`))
	m := next(t, staff)
	if list := m["recordings"].([]any); m["type"] != "recordings" || len(list) != 1 || list[0].(map[string]any)["channelName"] != "Lounge" {
		t.Fatalf("list %v", m)
	}
	staff.Handle([]byte(`{"type":"recording-token"}`))
	if m := next(t, staff); m["type"] != "recording-token" || m["token"] != "token" {
		t.Fatalf("token %v", m)
	}
	for _, req := range []string{`{"type":"recordings"}`, `{"type":"recording-token"}`} {
		user.Handle([]byte(req))
		if m := next(t, user); m["type"] != "error" {
			t.Fatalf("%s for a user: %v", req, m)
		}
	}
}
