package whip

import (
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"

	"kallisti.chat/server/internal/stream"
)

// loopback: pion on one UDP port of 127.0.0.1.
func loopback(t *testing.T) webrtc.SettingEngine {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	mux := webrtc.NewICEUDPMux(nil, conn)
	t.Cleanup(func() { mux.Close() })
	s, err := Settings(mux, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.SetIncludeLoopbackCandidate(true)
	return s
}

// setup: alice (key "alice-key") is session 1, bob ("bob-key") session 3
// (no Speak), carol ("carol-key") is not connected; session 2 watches.
func setup(t *testing.T) (*Handler, *httptest.Server, *stream.Hub, *viewer) {
	hub, _, w := hubWithViewer()
	h := &Handler{
		Settings: loopback(t), Hub: hub, BytesPerSecond: 1 << 20, MaxSessions: 2,
		Account: func(key string) (string, bool) {
			name, ok := strings.CutSuffix(key, "-key")
			return name, ok && key != "-key"
		},
		Session: func(name string) (uint32, bool) {
			s, ok := map[string]uint32{"alice": 1, "bob": 3}[name]
			return s, ok
		},
	}
	srv := httptest.NewServer(h)
	t.Cleanup(func() {
		srv.Close()
		h.Close()
	})
	return h, srv, hub, w
}

func request(t *testing.T, method, url, key, ctype, body string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

func TestRefusals(t *testing.T) {
	h, srv, _, _ := setup(t)
	good := offerSDP(opusMedia, h264Media)
	for _, c := range []struct {
		key, ctype, body string
		status           int
		says             string
	}{
		{"", "application/sdp", good, 401, "stream key"},
		{"nobody", "application/sdp", good, 401, "stream key"},
		{"alice-key", "text/plain", good, 415, "SDP"},
		{"carol-key", "application/sdp", good, 403, "voice channel"},
		{"bob-key", "application/sdp", good, 403, "speak"},
		{"alice-key", "application/sdp", offerSDP(vp8Media), 400, "H.264"},
		{"alice-key", "application/sdp", offerSDP(aacMedia, h264Media), 400, "Opus"},
	} {
		resp, body := request(t, "POST", srv.URL+Path, c.key, c.ctype, c.body)
		if resp.StatusCode != c.status || !strings.Contains(body, c.says) {
			t.Errorf("key %q: got %d %q, want %d %q", c.key, resp.StatusCode, body, c.status, c.says)
		}
	}
	if resp, _ := request(t, "PATCH", srv.URL+Path+"/abc", "alice-key", "application/trickle-ice-sdpfrag", ""); resp.StatusCode != 405 {
		t.Errorf("PATCH: %d", resp.StatusCode)
	}
	if resp, _ := request(t, "DELETE", srv.URL+Path+"/abc", "", "", ""); resp.StatusCode != 404 {
		t.Errorf("DELETE of nothing: %d", resp.StatusCode)
	}
	if h.Sessions() != 0 {
		t.Fatal("sessions after refusals")
	}
}

// obs is a pion peer that streams like OBS: H.264 and Opus, sendonly.
type obs struct {
	pc           *webrtc.PeerConnection
	video, sound *webrtc.TrackLocalStaticSample
}

func newOBS(t *testing.T) *obs {
	t.Helper()
	m := &webrtc.MediaEngine{}
	m.RegisterCodec(webrtc.RTPCodecParameters{PayloadType: 102, RTPCodecCapability: webrtc.RTPCodecCapability{
		MimeType: webrtc.MimeTypeH264, ClockRate: 90000, SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
	}}, webrtc.RTPCodecTypeVideo)
	m.RegisterCodec(webrtc.RTPCodecParameters{PayloadType: 111, RTPCodecCapability: webrtc.RTPCodecCapability{
		MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2, SDPFmtpLine: "minptime=10;useinbandfec=1;stereo=1;sprop-stereo=1",
	}}, webrtc.RTPCodecTypeAudio)
	i := &interceptor.Registry{}
	webrtc.RegisterDefaultInterceptors(m, i)
	pc, err := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i), webrtc.WithSettingEngine(loopback(t))).NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pc.Close() })
	o := &obs{pc: pc}
	o.video, _ = webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000}, "video", "obs")
	o.sound, _ = webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2}, "audio", "obs")
	for _, tr := range []webrtc.TrackLocal{o.sound, o.video} {
		if _, err := pc.AddTransceiverFromTrack(tr, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendonly}); err != nil {
			t.Fatal(err)
		}
	}
	return o
}

// offer: the complete offer (no trickle ICE, as OBS).
func (o *obs) offer(t *testing.T) string {
	offer, err := o.pc.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	}
	gathered := webrtc.GatheringCompletePromise(o.pc)
	o.pc.SetLocalDescription(offer)
	<-gathered
	return o.pc.LocalDescription().SDP
}

// connect posts the offer and returns the session's URL.
func (o *obs) connect(t *testing.T, url, key string) string {
	t.Helper()
	resp, answer := request(t, "POST", url+Path, key, "application/sdp", o.offer(t))
	if resp.StatusCode != http.StatusCreated || resp.Header.Get("Content-Type") != "application/sdp" {
		t.Fatalf("POST: %d %s", resp.StatusCode, answer)
	}
	if err := o.pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: answer}); err != nil {
		t.Fatal(err)
	}
	return url + resp.Header.Get("Location")
}

// send streams the clip (looped) and sound until stop is closed.
func (o *obs) send(frames [][]byte, stop chan struct{}) {
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for i := 0; ; i++ {
		select {
		case <-stop:
			return
		case <-tick.C:
			o.video.WriteSample(media.Sample{Data: frames[i%len(frames)], Duration: time.Second / 30})
			o.sound.WriteSample(media.Sample{Data: []byte{0xfc, 0xff, 0xfe}, Duration: 20 * time.Millisecond})
		}
	}
}

func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for end := time.Now().Add(10 * time.Second); !ok(); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(end) {
			t.Fatal("timed out: " + what)
		}
	}
}

func TestStreamFromOBS(t *testing.T) {
	h, srv, hub, w := setup(t)
	o := newOBS(t)
	location := o.connect(t, srv.URL, "alice-key")
	frames := clipFrames(t)
	stop := make(chan struct{})
	defer close(stop)
	go o.send(frames, stop)

	eventually(t, "pictures and sound", func() bool { return len(w.pictures()) >= 4 && len(w.sounds()) > 0 })
	got := w.pictures()
	if !got[0].key || !bytes.Equal(got[0].data, frames[0]) && !bytes.Equal(got[0].data, frames[2]) {
		t.Fatalf("first picture: key %v, %d bytes", got[0].key, len(got[0].data))
	}
	for _, g := range got {
		if !bytes.Equal(g.data, frames[0]) && !bytes.Equal(g.data, frames[1]) && !bytes.Equal(g.data, frames[2]) || g.key == bytes.Equal(g.data, frames[1]) {
			t.Fatalf("a picture not from the clip: key %v, %d bytes", g.key, len(g.data))
		}
	}
	if s := w.sounds()[0]; !bytes.Equal(s.data, []byte{0xfc, 0xff, 0xfe}) {
		t.Fatalf("sound %x", s.data)
	}
	if streams := hub.Streams(); len(streams) != 1 || streams[0].Session != 1 || !streams[0].Audio {
		t.Fatalf("streams %+v", streams)
	}

	// DELETE ends it.
	if resp, _ := request(t, "DELETE", location, "", "", ""); resp.StatusCode != 200 {
		t.Fatalf("DELETE: %d", resp.StatusCode)
	}
	if len(hub.Streams()) != 0 || h.Sessions() != 0 {
		t.Fatalf("after DELETE: streams %v, sessions %d", hub.Streams(), h.Sessions())
	}
}

func TestStreamEndsWhenTheOwnerLeaves(t *testing.T) {
	h, srv, hub, _ := setup(t)
	o := newOBS(t)
	o.connect(t, srv.URL, "alice-key")
	stop := make(chan struct{})
	defer close(stop)
	go o.send(clipFrames(t), stop)
	eventually(t, "the stream", func() bool { return len(hub.Streams()) == 1 })
	hub.Moved(1, 6)
	eventually(t, "the session's end", func() bool { return h.Sessions() == 0 })
	eventually(t, "OBS's connection closed", func() bool {
		s := o.pc.ConnectionState()
		return s == webrtc.PeerConnectionStateFailed || s == webrtc.PeerConnectionStateDisconnected || s == webrtc.PeerConnectionStateClosed
	})
}

func TestANewKeyEndsTheStream(t *testing.T) {
	h, srv, hub, _ := setup(t)
	o := newOBS(t)
	o.connect(t, srv.URL, "alice-key")
	stop := make(chan struct{})
	defer close(stop)
	go o.send(clipFrames(t), stop)
	eventually(t, "the stream", func() bool { return len(hub.Streams()) == 1 })
	h.EndAccount("alice")
	if len(hub.Streams()) != 0 || h.Sessions() != 0 {
		t.Fatal("the stream goes on")
	}
}

// A key revoked while its request's offer comes in (slowly) starts nothing.
func TestAKeyRevokedDuringTheOffer(t *testing.T) {
	h, srv, _, _ := setup(t)
	var mu sync.Mutex
	revoked, checked := false, make(chan struct{}, 4)
	h.Account = func(key string) (string, bool) {
		mu.Lock()
		defer mu.Unlock()
		checked <- struct{}{}
		return "alice", key == "alice-key" && !revoked
	}
	body, pw := io.Pipe()
	done := make(chan *http.Response)
	go func() {
		req, _ := http.NewRequest("POST", srv.URL+Path, body)
		req.Header.Set("Authorization", "Bearer alice-key")
		req.Header.Set("Content-Type", "application/sdp")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Error(err)
		}
		done <- resp
	}()
	offer := newOBS(t).offer(t)
	io.WriteString(pw, offer[:10])
	<-checked // authenticated, the offer still coming
	mu.Lock()
	revoked = true
	mu.Unlock()
	h.EndAccount("alice")
	io.WriteString(pw, offer[10:])
	pw.Close()
	resp := <-done
	if resp == nil {
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized || h.Sessions() != 0 {
		t.Fatalf("%d, %d sessions", resp.StatusCode, h.Sessions())
	}
}

func TestTimeoutsAndLimits(t *testing.T) {
	h, srv, _, _ := setup(t)
	h.ConnectTimeout = 300 * time.Millisecond
	// Offers that are never followed by a connection.
	for range 2 {
		resp, body := request(t, "POST", srv.URL+Path, "alice-key", "application/sdp", newOBS(t).offer(t))
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("POST: %d %s", resp.StatusCode, body)
		}
	}
	if h.Sessions() != 1 {
		t.Fatalf("%d sessions: a new one of the account replaces the old", h.Sessions())
	}
	h.Account = func(key string) (string, bool) { return key, true } // everyone is alice's voice session
	h.Session = func(string) (uint32, bool) { return 1, true }
	request(t, "POST", srv.URL+Path, "x", "application/sdp", newOBS(t).offer(t))
	if resp, body := request(t, "POST", srv.URL+Path, "y", "application/sdp", newOBS(t).offer(t)); resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("over the limit: %d %s", resp.StatusCode, body)
	}
	eventually(t, "the sessions' timeout", func() bool { return h.Sessions() == 0 })
}

// Pictures that stop after the start (OBS still connected) end the stream.
func TestStreamEndsWithoutPictures(t *testing.T) {
	h, srv, hub, _ := setup(t)
	h.MediaTimeout = 500 * time.Millisecond
	o := newOBS(t)
	o.connect(t, srv.URL, "alice-key")
	stop := make(chan struct{})
	go o.send(clipFrames(t), stop)
	eventually(t, "the stream", func() bool { return len(hub.Streams()) == 1 })
	close(stop)
	eventually(t, "the session's end", func() bool { return h.Sessions() == 0 && len(hub.Streams()) == 0 })
}

type failingReader struct{}

func (failingReader) ReadRTP() (*rtp.Packet, interceptor.Attributes, error) {
	return nil, nil, errors.New("unknown payload type")
}

// A track that cannot be read ends the session (it would freeze).
func TestAReadErrorEndsTheSession(t *testing.T) {
	for _, read := range []func(*session, rtpReader){(*session).readVideo, (*session).readSound} {
		s := newSession(&Handler{}, "id", "alice")
		read(s, failingReader{})
		select {
		case <-s.done:
		default:
			t.Fatal("the session goes on")
		}
	}
}
