package mumbled

import (
	"html"
	"log/slog"
	"maps"
	"slices"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

// Text channel mirrors: the Kallisti server's text channels as Mumble
// channels under one parent, so Mumble clients can read and write them.
// Nobody speaks, edits or builds there; the Kallisti server owns them.

const (
	textParentName = "Text channels"
	// After the voice channels in clients' trees.
	textParentPosition = 1000
	mirrorDeny         = Speak | Write | MakeChannel | MakeTempChannel | LinkChannel // see mirrorACL
)

// TextMirror is one wanted mirror: its Mumble channel (0: make one) and
// the text channel's name.
type TextMirror struct {
	Channel uint32
	Name    string
	Topic   string // plain text; the mirror's description
}

// SyncTextChannels makes the mirrors match want, in order, and returns
// their Mumble channels. Missing ones (or a missing parent) are made anew.
func (s *Server) SyncTextChannels(want []TextMirror) ([]uint32, error) {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	saved, err := s.loadTextParent()
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	var save []*Channel
	var removed []uint32
	touched := map[uint32]bool{}
	parent := s.state.Channels[saved]
	if saved == 0 || parent == nil || parent.Parent != 0 {
		// One from before whose id was not saved, or a new one.
		parent = nil
		for _, ch := range s.state.Channels {
			if ch.Parent == 0 && ch.ID != 0 && ch.Name == textParentName {
				parent = ch
			}
		}
		if parent == nil {
			if parent, err = s.state.AddChannel(0, textParentName, false); err != nil {
				s.mu.Unlock()
				return nil, err
			}
			parent.Position = textParentPosition
		}
		touched[parent.ID] = true
	}
	if !slices.Equal(parent.ACLs, mirrorACL()) {
		parent.ACLs = mirrorACL()
		touched[parent.ID] = true
	}
	s.textParent = parent.ID
	keep := map[uint32]bool{}
	for _, w := range want {
		if ch := s.state.Channels[w.Channel]; w.Channel != 0 && ch != nil && ch.Parent == parent.ID {
			keep[w.Channel] = true
		}
	}
	// Unwanted mirrors go first, so their names are free.
	for _, ch := range s.state.Tree() {
		if ch.Parent == parent.ID && ch.ID != parent.ID && !keep[ch.ID] {
			removed = append(removed, s.removeChannelLocked(ch.ID, 0)...)
		}
	}
	ids := make([]uint32, len(want))
	for i, w := range want {
		name, pos := "#"+w.Name, int32(i)
		ch := s.state.Channels[w.Channel]
		if !keep[w.Channel] {
			if ch, err = s.state.AddChannel(parent.ID, name, false); err != nil {
				slog.Error("text channel mirror", "name", name, "err", err)
				continue
			}
		}
		desc := html.EscapeString(w.Topic)
		if ch.Name != name || ch.Position != pos || ch.Description != desc {
			if err := s.state.EditChannel(ch.ID, ChannelEdit{Name: &name, Position: &pos, Description: &desc}); err != nil {
				slog.Error("text channel mirror", "name", name, "err", err)
			}
			touched[ch.ID] = true
		}
		ids[i] = ch.ID
	}
	for _, id := range slices.Sorted(maps.Keys(touched)) {
		ch := s.state.Channels[id]
		save = append(save, cloneChannel(ch))
		for _, u := range s.state.Users {
			if c := s.clients[u.Session]; c != nil {
				c.sendMsg(mumble.TypeChannelState, s.state.channelStateFor(ch, u))
			}
		}
	}
	if touched[parent.ID] {
		s.permissionsChangedLocked(parent) // suppress states follow Speak
	}
	lastID := s.state.LastChannelID()
	s.mu.Unlock()
	s.save(save, removed, lastID)
	if parent.ID != saved {
		if err := s.storeTextParent(parent.ID); err != nil {
			return ids, err
		}
	}
	return ids, nil
}

// mirrorACL: in the parent and the mirrors nobody speaks, edits or builds;
// texts only in the mirrors.
func mirrorACL() []ACL {
	return []ACL{
		{ApplyHere: true, ApplySubs: true, Group: "all", Deny: mirrorDeny},
		{ApplyHere: true, ApplySubs: false, Group: "all", Deny: TextMessage},
	}
}

// isMirrorLocked: a text channel mirror or their parent.
func (s *Server) isMirrorLocked(id uint32) bool {
	ch := s.state.Channels[id]
	return s.textParent != 0 && ch != nil && (ch.ID == s.textParent || ch.Parent == s.textParent)
}

func (s *Server) loadTextParent() (uint32, error) {
	if s.cfg.DB == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.textParent, nil
	}
	return s.cfg.DB.TextParent()
}

func (s *Server) storeTextParent(id uint32) error {
	if s.cfg.DB == nil {
		return nil // s.textParent has it
	}
	return s.cfg.DB.SetTextParent(id)
}

// SendServerText sends HTML to one user, from the server (no actor).
func (s *Server) SendServerText(session uint32, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c := s.clients[session]; c != nil {
		c.sendMsg(mumble.TypeTextMessage, &pb.TextMessage{Session: []uint32{session}, Message: proto.String(text)})
	}
}

// The event queue: hooks run on one goroutine, in order, outside mu.
const eventQueue = 4096

func (s *Server) startEvents() {
	s.events, s.stopEvents = make(chan func(), eventQueue), make(chan struct{})
	go func() {
		for {
			select {
			case f := <-s.events:
				f()
			case <-s.stopEvents:
				return
			}
		}
	}()
}

// queue never blocks (it runs under mu): when full, the event is lost.
func (s *Server) queue(f func()) {
	select {
	case s.events <- f:
	default:
		slog.Warn("event queue full: an event was lost")
	}
}
