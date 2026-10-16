package mumbled

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

// Screen share sound for Mumble clients: a hidden user ("name (stream)")
// in the streamer's channel speaks the stream's Opus packets, so Mumble
// clients hear it with a volume of its own. Web clients never see it (they
// get the sound with the picture). It is not one of the server's users:
// nobody can message, move or kick it.

type streamUser struct {
	session, channel uint32
	name             string
}

func (su *streamUser) state() []byte {
	return mumble.MessageFrame(mumble.TypeUserState, &pb.UserState{
		Session: proto.Uint32(su.session), Name: proto.String(su.name), ChannelId: proto.Uint32(su.channel),
		Comment: proto.String("The sound of " + su.name[:len(su.name)-len(" (stream)")] + "'s screen share. The picture is in Kallisti's web app."),
	})
}

// StartStreamSound makes the stream user of a streamer (false: no such
// session).
func (s *Server) StartStreamSound(streamer uint32) (uint32, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.state.Users[streamer]
	if u == nil {
		return 0, false
	}
	if su := s.streams[streamer]; su != nil {
		return su.session, true
	}
	s.next++
	su := &streamUser{session: s.next, channel: u.Channel, name: fmt.Sprintf("%s (stream)", u.Name)}
	s.streams[streamer] = su
	s.broadcastNativeLocked(su.state())
	return su.session, true
}

// EndStreamSound removes a streamer's stream user.
func (s *Server) EndStreamSound(streamer uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.endStreamSoundLocked(streamer)
}

func (s *Server) endStreamSoundLocked(streamer uint32) {
	su := s.streams[streamer]
	if su == nil {
		return
	}
	delete(s.streams, streamer)
	s.broadcastNativeLocked(mumble.MessageFrame(mumble.TypeUserRemove, &pb.UserRemove{Session: proto.Uint32(su.session)}))
}

// StreamSound speaks an Opus packet of a stream (timestamp: microseconds)
// to the Mumble clients in its channel and to those listening there.
func (s *Server) StreamSound(streamer uint32, opus []byte, timestamp uint64) {
	s.mu.Lock()
	su := s.streams[streamer]
	if su == nil {
		s.mu.Unlock()
		return
	}
	var receivers []receiver
	for _, u := range s.state.Users {
		c := s.clients[u.Session]
		if c == nil || c.web || u.Deaf || u.SelfDeaf {
			continue
		}
		ctx, ok := ContextNormal, u.Channel == su.channel
		if !ok && u.Listening[su.channel] {
			ctx, ok = ContextListen, true
		}
		if ok {
			r := receiver{c: c, ctx: ctx}
			if c.udpVoice && s.udp != nil {
				r.addr = c.udpAddr
			}
			receivers = append(receivers, r)
		}
	}
	udp := s.udp
	s.mu.Unlock()
	// Frame numbers count 10 ms, as Mumble clients do.
	deliver(audioPacket{frame: timestamp / 10_000, opus: opus}, su.session, receivers, udp)
}

func (s *Server) broadcastNativeLocked(frame []byte) {
	for _, c := range s.clients {
		if !c.web {
			c.send(frame, false)
		}
	}
}
