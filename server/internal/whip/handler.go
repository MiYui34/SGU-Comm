// Package whip takes streams from OBS (WHIP, RFC 9725): an encoder posts
// an SDP offer with its owner's stream key and sends H.264 and Opus over
// WebRTC. The stream shows as a screen share of the owner's voice session
// (see stream.Ingest): pictures and sound are forwarded, never decoded.
package whip

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v4"

	"kallisti.chat/server/internal/stream"
)

const Path = "/api/whip"

// Handler serves Path: POST with the stream key as a Bearer token and an
// SDP offer gives the answer and the session's URL (Location); DELETE on
// that URL ends the stream.
type Handler struct {
	Settings webrtc.SettingEngine // see Settings
	Hub      *stream.Hub
	// Account: a stream key's account name. Session: the account's voice
	// session (registered and connected).
	Account func(key string) (name string, ok bool)
	Session func(name string) (uint32, bool)
	// A stream's pictures, at most (above it they are dropped), and the
	// sessions at the same time.
	BytesPerSecond int
	MaxSessions    int
	// Until the peer connects, and then until its first keyframe (0: the
	// defaults).
	ConnectTimeout, KeyTimeout time.Duration
	Closer                     io.Closer // the UDP port (see Listen), closed by Close; nil: none

	mu       sync.Mutex
	sessions map[string]*session
	closed   bool
}

const (
	maxOffer       = 64 << 10
	connectTimeout = 15 * time.Second
	keyTimeout     = 15 * time.Second
	gatherTimeout  = 5 * time.Second
)

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch id, sub := strings.CutPrefix(r.URL.Path, Path+"/"); {
	case r.URL.Path == Path && r.Method == http.MethodPost:
		h.start(w, r)
	case r.URL.Path == Path && r.Method == http.MethodOptions:
		w.Header().Set("Accept-Post", "application/sdp")
		w.WriteHeader(http.StatusNoContent)
	case sub && r.Method == http.MethodDelete:
		if s := h.take(id); s != nil {
			s.close()
			w.WriteHeader(http.StatusOK)
		} else {
			http.Error(w, "no such stream", http.StatusNotFound)
		}
	case sub || r.URL.Path == Path: // PATCH (trickle ICE) too: not supported
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	key, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	name, ok := h.Account(strings.TrimSpace(key))
	if !ok {
		w.Header().Set("WWW-Authenticate", "Bearer")
		http.Error(w, "unknown stream key (see Settings in Kallisti)", http.StatusUnauthorized)
		return
	}
	if ctype, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); ctype != "application/sdp" {
		http.Error(w, "send an SDP offer (application/sdp)", http.StatusUnsupportedMediaType)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxOffer))
	if err != nil {
		http.Error(w, "the offer is too large", http.StatusRequestEntityTooLarge)
		return
	}
	voice, ok := h.Session(name)
	if !ok {
		http.Error(w, "connect to Kallisti and join a voice channel first", http.StatusForbidden)
		return
	}
	if err := h.Hub.MayStream(voice); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	o, err := checkOffer(string(body))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s, old, err := h.reserve(name)
	for _, o := range old {
		o.close()
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	answer, err := s.open(o, string(body), voice)
	if err != nil {
		slog.Warn("stream from OBS: no WebRTC session", "account", name, "err", err)
		s.close()
		http.Error(w, "cannot set up the WebRTC session", http.StatusBadRequest)
		return
	}
	slog.Info("stream from OBS", "account", name)
	go s.run()
	w.Header().Set("Content-Type", "application/sdp")
	w.Header().Set("Location", Path+"/"+s.id)
	// Not chunked: FFmpeg's WHIP client reads the end of a chunked body as an error.
	w.Header().Set("Content-Length", strconv.Itoa(len(answer)))
	w.WriteHeader(http.StatusCreated)
	io.WriteString(w, answer)
}

// reserve makes a session (not yet open) if there is room. It replaces
// the account's old ones (OBS reconnecting), returned to be closed.
func (h *Handler) reserve(account string) (*session, []*session, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	old := h.takeAccountLocked(account)
	if h.sessions == nil {
		h.sessions = map[string]*session{}
	}
	if h.closed || len(h.sessions) >= h.MaxSessions {
		return nil, old, errors.New("too many streams from OBS at once, try again later")
	}
	b := make([]byte, 16)
	rand.Read(b)
	s := newSession(h, hex.EncodeToString(b), account)
	h.sessions[s.id] = s
	return s, old, nil
}

// take removes a session from the list (to close it).
func (h *Handler) take(id string) *session {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.sessions[id]
	delete(h.sessions, id)
	return s
}

func (h *Handler) takeAccount(name string) []*session {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.takeAccountLocked(name)
}

func (h *Handler) takeAccountLocked(name string) []*session {
	var out []*session
	for id, s := range h.sessions {
		if s.account == name {
			out = append(out, s)
			delete(h.sessions, id)
		}
	}
	return out
}

// EndAccount ends an account's stream (its key changed).
func (h *Handler) EndAccount(name string) {
	for _, s := range h.takeAccount(name) {
		s.close()
	}
}

// Close ends every stream.
func (h *Handler) Close() {
	h.mu.Lock()
	all := h.sessions
	h.sessions, h.closed = nil, true
	h.mu.Unlock()
	for _, s := range all {
		s.close()
	}
	if h.Closer != nil {
		h.Closer.Close()
	}
}

// Sessions: how many streams are open (or opening).
func (h *Handler) Sessions() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.sessions)
}

// api is pion for one offer: its own codecs only, as offered.
func (h *Handler) api(o offer) (*webrtc.API, error) {
	m := &webrtc.MediaEngine{}
	if err := m.RegisterCodec(o.video, webrtc.RTPCodecTypeVideo); err != nil {
		return nil, err
	}
	if o.audio != nil {
		if err := m.RegisterCodec(*o.audio, webrtc.RTPCodecTypeAudio); err != nil {
			return nil, err
		}
	}
	i := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		return nil, err
	}
	return webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i), webrtc.WithSettingEngine(h.Settings)), nil
}
