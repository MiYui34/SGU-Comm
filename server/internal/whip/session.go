package whip

import (
	"cmp"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"

	"kallisti.chat/server/internal/stream"
)

// session is one stream from OBS: a WebRTC connection that feeds a stream
// ingest. It ends on DELETE, when the connection fails, when the hub ends
// the stream (the owner left the channel, or Speak), or on a timeout.
type session struct {
	h           *Handler
	id, account string
	pc          *webrtc.PeerConnection
	feed        *feed
	begun       time.Time
	videoSSRC   atomic.Uint32
	pli         chan struct{} // a keyframe is wanted
	connected   chan struct{}
	connectOnce sync.Once
	done        chan struct{} // closed by end
	endOnce     sync.Once
	closeOnce   sync.Once
	opened      chan struct{} // open returned (close waits for it)
}

const pliEvery = time.Second // keyframe requests while none came

func newSession(h *Handler, id, account string) *session {
	return &session{h: h, id: id, account: account, begun: time.Now(),
		pli: make(chan struct{}, 1), connected: make(chan struct{}), done: make(chan struct{}), opened: make(chan struct{})}
}

// open answers the offer.
func (s *session) open(o offer, sdp string, voice uint32) (string, error) {
	defer close(s.opened)
	s.feed = newFeed(s.h.Hub, voice, o.audio != nil, s.h.BytesPerSecond, stream.IngestHooks{
		Keyframe: func() {
			select {
			case s.pli <- struct{}{}:
			default:
			}
		},
		Ended: s.end,
	})
	api, err := s.h.api(o)
	if err != nil {
		return "", err
	}
	if s.pc, err = api.NewPeerConnection(webrtc.Configuration{}); err != nil {
		return "", err
	}
	s.pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		switch state {
		case webrtc.PeerConnectionStateConnected:
			s.connectOnce.Do(func() { close(s.connected) })
		case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
			s.end()
		}
	})
	s.pc.OnTrack(func(t *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		switch strings.ToLower(t.Codec().MimeType) {
		case "video/h264":
			s.videoSSRC.Store(uint32(t.SSRC()))
			go s.readVideo(t)
		case "audio/opus":
			go s.readSound(t)
		}
	})
	if err := s.pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: sdp}); err != nil {
		return "", err
	}
	answer, err := s.pc.CreateAnswer(nil)
	if err != nil {
		return "", err
	}
	gathered := webrtc.GatheringCompletePromise(s.pc)
	if err := s.pc.SetLocalDescription(answer); err != nil {
		return "", err
	}
	select { // host candidates only: at once
	case <-gathered:
	case <-time.After(gatherTimeout):
		return "", errors.New("ICE gathering timed out")
	}
	return s.pc.LocalDescription().SDP, nil
}

// end asks the session to close (never blocks: hooks call it).
func (s *session) end() { s.endOnce.Do(func() { close(s.done) }) }

func (s *session) close() {
	s.closeOnce.Do(func() {
		s.end()
		<-s.opened
		if s.feed != nil {
			s.feed.stop()
		}
		if s.pc != nil {
			s.pc.Close()
		}
		s.h.take(s.id)
	})
}

func (s *session) run() {
	defer s.close()
	select {
	case <-s.connected:
	case <-s.done:
		return
	case <-time.After(cmp.Or(s.h.ConnectTimeout, connectTimeout)):
		slog.Info("stream from OBS: no connection", "account", s.account)
		return
	}
	noKeyBy := time.Now().Add(cmp.Or(s.h.KeyTimeout, keyTimeout))
	tick := time.NewTicker(pliEvery)
	defer tick.Stop()
	s.keyframe()
	for {
		select {
		case <-s.done:
			slog.Info("stream from OBS ended", "account", s.account)
			return
		case <-s.pli:
			s.keyframe()
		case now := <-tick.C:
			if s.feed.started() {
				continue
			}
			if now.After(noKeyBy) {
				slog.Info("stream from OBS: no keyframe", "account", s.account)
				return
			}
			s.keyframe()
		}
	}
}

// keyframe asks the encoder for one (PLI).
func (s *session) keyframe() {
	if ssrc := s.videoSSRC.Load(); ssrc != 0 {
		s.pc.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: ssrc}})
	}
}

// arrival: microseconds since the session began.
func (s *session) arrival() int64 { return time.Since(s.begun).Microseconds() }

func (s *session) readVideo(t *webrtc.TrackRemote) {
	v := newVideoIn()
	for {
		p, _, err := t.ReadRTP()
		if err != nil {
			return
		}
		for _, pic := range v.push(p, s.arrival()) {
			if err := s.feed.picture(pic); err != nil {
				slog.Info("stream from OBS refused", "account", s.account, "err", err)
				s.end()
				return
			}
		}
	}
}

func (s *session) readSound(t *webrtc.TrackRemote) {
	for {
		p, _, err := t.ReadRTP()
		if err != nil {
			return
		}
		s.feed.sound(p, s.arrival())
	}
}
