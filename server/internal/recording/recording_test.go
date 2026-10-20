package recording

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 19, 15, 30, 0, 0, time.UTC)

// clock is a settable Now.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) set(t time.Time) {
	c.mu.Lock()
	c.now = t
	c.mu.Unlock()
}

func testLibrary(t *testing.T, limits Limits) (*Library, *clock) {
	t.Helper()
	c := &clock{now: t0}
	if limits.MaxDuration == 0 {
		limits.MaxDuration = time.Hour
	}
	if limits.MaxBytes == 0 {
		limits.MaxBytes = 1 << 30
	}
	return &Library{Dir: t.TempDir(), Limits: limits, Keep: 7 * 24 * time.Hour, Now: c.Now}, c
}

func talk(r *Recording, speaker string, from time.Duration, packets int, opus []byte) {
	for i := range packets {
		r.Feed(Packet{Speaker: speaker, Seq: uint64(i * 2), Opus: opus, Terminator: i == packets-1, At: t0.Add(from + time.Duration(i)*20*time.Millisecond)})
	}
}

func TestOneAlignedTrackPerSpeaker(t *testing.T) {
	lib, c := testLibrary(t, Limits{})
	r, err := lib.Start(5, "Lounge", "mod", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Feed(Packet{Speaker: "nobody", Opus: []byte{31<<3 | 3}, At: t0}) // not Opus: no track
	talk(r, "alice", 0, 10, speech20)
	talk(r, "a/b", time.Second, 5, stereo20)
	talk(r, "a_b", 2*time.Second, 5, speech20)
	c.set(t0.Add(3 * time.Second))
	info := r.Stop("stopped")
	if info.Reason != "stopped" || info.Channel != 5 || info.ChannelName != "Lounge" || info.By != "mod" || !info.Started.Equal(t0) || !info.Stopped.Equal(c.Now()) {
		t.Fatalf("info %+v", info)
	}
	names := []string{}
	files := map[string]bool{}
	for _, tr := range info.Tracks {
		names = append(names, tr.Name)
		files[tr.File] = true
	}
	if !slices.Equal(names, []string{"alice", "a/b", "a_b"}) || len(files) != 3 {
		t.Fatalf("tracks %+v", info.Tracks)
	}
	want := map[string]struct {
		start    int64
		channels byte
	}{"alice": {0, 1}, "a/b": {1000 * ms, 2}, "a_b": {2000 * ms, 1}}
	for _, tr := range info.Tracks {
		p := parseTrack(t, filepath.Join(lib.Dir, info.ID, tr.File))
		w := want[tr.Name]
		first := slices.IndexFunc(p.packets, func(b []byte) bool { return !isSilence(b) })
		if p.starts[first] != w.start || p.channels != w.channels || p.end != 3000*ms {
			t.Fatalf("%s: speech at %d, %d channels, end %d", tr.Name, p.starts[first], p.channels, p.end)
		}
		if tr.Start != w.start/ms || int(w.channels) != tr.Channels {
			t.Fatalf("track info %+v", tr)
		}
	}
	if got := lib.List(); len(got) != 1 || got[0].ID != info.ID || got[0].Size == 0 || !got[0].Expires.Equal(c.Now().Add(lib.Keep)) {
		t.Fatalf("list %+v", got)
	}
	// Stopping again, or feeding after the end, does nothing.
	r.Stop("again")
	talk(r, "alice", 0, 300, speech20)
}

func TestTheSizeLimitStopsARecording(t *testing.T) {
	lib, _ := testLibrary(t, Limits{MaxBytes: 2000})
	ended := make(chan Info, 1)
	r, _ := lib.Start(5, "Lounge", "mod", func(i Info) { ended <- i })
	for i := range 2000 {
		r.Feed(Packet{Speaker: "alice", Seq: uint64(i * 2), Opus: speech20, At: t0.Add(time.Duration(i) * 20 * time.Millisecond)})
		if i%50 == 0 {
			time.Sleep(time.Millisecond) // let the recorder keep up
		}
	}
	select {
	case info := <-ended:
		if info.Reason != "size limit" || info.Size > 2000+64<<10 {
			t.Fatalf("info %+v", info)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("still recording")
	}
	if got := r.Stop("stopped"); got.Reason != "size limit" {
		t.Fatalf("stop after the end: %+v", got)
	}
}

func TestTheTimeLimitStopsARecording(t *testing.T) {
	lib, _ := testLibrary(t, Limits{MaxDuration: 50 * time.Millisecond})
	ended := make(chan Info, 1)
	lib.Start(5, "Lounge", "mod", func(i Info) { ended <- i })
	select {
	case info := <-ended:
		if info.Reason != "time limit" || len(info.Tracks) != 0 {
			t.Fatalf("info %+v", info)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("still recording")
	}
}

func TestAFullQueueDropsPackets(t *testing.T) {
	r := &Recording{in: make(chan Packet, 1), quit: make(chan struct{})}
	r.Feed(Packet{Opus: speech20})
	r.Feed(Packet{Opus: speech20})
	if r.dropped.Load() != 1 {
		t.Fatalf("dropped %d", r.dropped.Load())
	}
}

func TestRecordingsAreRemovedAfterTheDownloadPeriod(t *testing.T) {
	lib, c := testLibrary(t, Limits{})
	r, _ := lib.Start(5, "Lounge", "mod", nil)
	talk(r, "alice", 0, 3, speech20)
	info := r.Stop("stopped")
	// A crashed recording without info: removed once it cannot run any
	// more.
	stale := filepath.Join(lib.Dir, "0123456789abcdef")
	os.Mkdir(stale, 0o700)
	os.Chtimes(stale, t0, t0)
	running, _ := lib.Start(6, "Other", "mod", nil)
	defer running.Stop("stopped")

	c.set(t0.Add(lib.Keep - time.Minute))
	lib.Clean()
	if len(lib.List()) != 1 {
		t.Fatal("removed too early")
	}
	c.set(t0.Add(lib.Keep + time.Minute))
	if len(lib.List()) != 0 {
		t.Fatal("listed after the download period")
	}
	c.set(t0.Add(lib.Keep + lib.Limits.MaxDuration + time.Minute))
	lib.Clean()
	entries, _ := os.ReadDir(lib.Dir)
	if len(lib.List()) != 0 || len(entries) != 1 || entries[0].Name() != running.ID() {
		t.Fatalf("left %v (recording %s)", entries, info.ID)
	}
}

func TestTheZipHasTheTracksAndTheInfo(t *testing.T) {
	lib, _ := testLibrary(t, Limits{})
	r, _ := lib.Start(5, "Lounge", "mod", nil)
	talk(r, "alice", 0, 3, speech20)
	talk(r, "bob", 0, 3, speech20)
	info := r.Stop("stopped")
	var buf bytes.Buffer
	if err := lib.WriteZip(&buf, info.ID); err != nil {
		t.Fatal(err)
	}
	if name := zipName(info); name != "Lounge 2026-10-19 1530.zip" {
		t.Fatalf("zip name %q", name)
	}
	z, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range z.File {
		names = append(names, f.Name)
		if f.Name == "recording.json" {
			rc, _ := f.Open()
			var got Info
			json.NewDecoder(rc).Decode(&got)
			if got.ID != info.ID || len(got.Tracks) != 2 {
				t.Fatalf("recording.json %+v", got)
			}
		}
	}
	if !slices.Equal(names, []string{"2026-10-19 1530 alice.opus", "2026-10-19 1530 bob.opus", "recording.json"}) {
		t.Fatalf("zip entries %v", names)
	}
	if err := lib.WriteZip(io.Discard, "../etc"); err == nil {
		t.Fatal("a bad id was served")
	}
}

func TestDownloadsNeedATokenOfStaff(t *testing.T) {
	lib, _ := testLibrary(t, Limits{})
	r, _ := lib.Start(5, "Lounge", "mod", nil)
	talk(r, "alice", 0, 3, speech20)
	info := r.Stop("stopped")
	h := &Handler{
		Library: lib,
		Redeem: func(token string) (uint32, bool) {
			return map[string]uint32{"staff": 1, "user": 2}[token], token == "staff" || token == "user"
		},
		MayDownload: func(session uint32) bool { return session == 1 },
	}
	for _, c := range []struct {
		url  string
		code int
	}{
		{"/api/recordings/" + info.ID, http.StatusUnauthorized},
		{"/api/recordings/" + info.ID + "?token=bad", http.StatusUnauthorized},
		{"/api/recordings/" + info.ID + "?token=user", http.StatusForbidden},
		{"/api/recordings/ffffffffffffffff?token=staff", http.StatusNotFound},
		{"/api/recordings/" + info.ID + "?token=staff", http.StatusOK},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", c.url, nil))
		if w.Code != c.code {
			t.Errorf("%s: %d, want %d", c.url, w.Code, c.code)
		}
		if w.Code == http.StatusOK && (!strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment") || w.Header().Get("Content-Type") != "application/zip") {
			t.Errorf("headers %v", w.Header())
		}
	}
}

// Speakers beyond the track limit are not recorded (each track is an open
// file until the end); the info names them.
func TestSpeakersBeyondTheTrackLimit(t *testing.T) {
	lib, _ := testLibrary(t, Limits{})
	r, _ := lib.Start(5, "Lounge", "mod", nil)
	for i := range maxTracks + 10 {
		talk(r, "guest"+strconv.Itoa(i), 0, 2, speech20)
		time.Sleep(time.Millisecond) // let the recorder keep up
	}
	talk(r, "guest0", time.Second, 2, speech20) // has a track
	info := r.Stop("stopped")
	entries, _ := os.ReadDir(filepath.Join(lib.Dir, info.ID))
	if len(info.Tracks) != maxTracks || len(entries) != maxTracks+1 || len(info.Unrecorded) != 10 || info.Unrecorded[0] != "guest"+strconv.Itoa(maxTracks) {
		t.Fatalf("%d tracks, %d files, unrecorded %v", len(info.Tracks), len(entries), info.Unrecorded)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

// handRecording: a recording without its goroutine, with a track for
// alice that cannot be written.
func handRecording(t *testing.T) *Recording {
	lib, _ := testLibrary(t, Limits{})
	tr, err := createTrack(filepath.Join(lib.Dir, "alice.opus"), "alice", 1)
	if err != nil {
		t.Fatal(err)
	}
	tr.ogg.w = failingWriter{}
	return &Recording{lib: lib, info: Info{Started: t0}, tracks: map[string]*track{"alice": tr}, order: []*track{tr}}
}

// A failed write stops the recording (at once, or when it is finished).
func TestAWriteErrorStopsTheRecording(t *testing.T) {
	r := handRecording(t)
	reason := ""
	for i := 0; i < 200 && reason == ""; i++ {
		reason = r.add(Packet{Speaker: "alice", Seq: uint64(2 * i), Opus: speech20, At: t0.Add(time.Duration(i) * 20 * time.Millisecond)})
	}
	if reason != "write error" {
		t.Fatalf("reason %q", reason)
	}
	r = handRecording(t)
	r.add(Packet{Speaker: "alice", Opus: speech20, At: t0}) // buffered
	if r.finish("stopped"); r.info.Reason != "write error" {
		t.Fatalf("reason %q", r.info.Reason)
	}
}

// The silence that ends the tracks counts too: a long quiet recording
// ends early instead of going over the size limit; the tracks stay valid
// and end together.
func TestFinalSilenceStaysWithinTheSizeLimit(t *testing.T) {
	lib, c := testLibrary(t, Limits{MaxBytes: 20_000})
	r, _ := lib.Start(5, "Lounge", "mod", nil)
	talk(r, "alice", 0, 1, speech20)
	talk(r, "bob", 0, 1, speech20)
	time.Sleep(10 * time.Millisecond)
	c.set(t0.Add(time.Hour))
	info := r.Stop("stopped")
	if info.Size > lib.Limits.MaxBytes || info.Reason != "size limit" {
		t.Fatalf("%d bytes over %d, reason %q", info.Size, lib.Limits.MaxBytes, info.Reason)
	}
	var ends []int64
	for _, tr := range info.Tracks {
		ends = append(ends, parseTrack(t, filepath.Join(lib.Dir, info.ID, tr.File)).end)
	}
	if len(ends) != 2 || ends[0] != ends[1] || ends[0] < 10*sampleRate {
		t.Fatalf("the tracks end at %v", ends)
	}
}
