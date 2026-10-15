package mumbled

import (
	"html"
	"strings"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

// MayRead: a user may see a channel's chat if it may enter the channel (or
// is in it). For the Kallisti server's stored voice channel chats.
func (s *Server) MayRead(session, channel uint32) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.state.Users[session]
	return u != nil && s.state.Channels[channel] != nil && (u.Channel == channel || s.state.Has(u, channel, Enter))
}

// MayWrite: a user may write to a channel's chat.
func (s *Server) MayWrite(session, channel uint32) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.state.Users[session]
	return u != nil && s.state.Channels[channel] != nil && s.state.Has(u, channel, TextMessage)
}

// SendChannelText delivers a plain text from a user to the others in a
// channel, as if the user had sent it (for messages written in the web
// client). Mumble clients read HTML, so it goes out escaped.
func (s *Server) SendChannelText(from, channel uint32, text string) {
	text = html.EscapeString(strings.TrimSpace(text))
	if text == "" {
		return
	}
	frame := mumble.MessageFrame(mumble.TypeTextMessage, &pb.TextMessage{
		Actor: proto.Uint32(from), ChannelId: []uint32{channel}, Message: proto.String(text),
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.state.Users {
		if u.Channel == channel && u.Session != from {
			if c := s.clients[u.Session]; c != nil {
				c.send(frame, false)
			}
		}
	}
}

// MayManage: a user may manage the Kallisti server's text channels (Write
// in root: admins).
func (s *Server) MayManage(session uint32) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.state.Users[session]
	return u != nil && s.state.Has(u, 0, MakeChannel) // admins and moderators
}
