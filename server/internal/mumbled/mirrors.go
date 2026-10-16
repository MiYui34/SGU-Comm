package mumbled

import (
	"html"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"time"

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
// MumbleUser is who a Mumble client is (see MumbleClient).
type MumbleUser struct {
	Registered bool
	UserID     uint32 // with Registered
}

// MumbleClient: the session is a Mumble client (not a web client through
// the bridge), and who it is.
func (s *Server) MumbleClient(session uint32) (MumbleUser, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, u := s.clients[session], s.state.Users[session]
	if c == nil || c.web || u == nil {
		return MumbleUser{}, false
	}
	return MumbleUser{Registered: u.Registered, UserID: u.UserID}, true
}

func (s *Server) SendServerText(session uint32, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c := s.clients[session]; c != nil {
		c.sendMsg(mumble.TypeTextMessage, &pb.TextMessage{Session: []uint32{session}, Message: proto.String(text)})
	}
}

// eventQueue runs hooks on one goroutine, in order, outside mu. It has no
// bound: joins, moves and leaves must not get lost (they come from
// clients' messages, which are rate-limited).
type eventQueue struct {
	mu      sync.Mutex
	pending []func()
	closed  bool
	stopped bool // the last hooks were taken: later ones are lost
	wake    chan struct{}
	done    chan struct{} // the goroutine ended
}

// A queue this long means hooks are slow or stuck: a warning.
const longEventQueue = 10000

func newEventQueue() *eventQueue {
	q := &eventQueue{wake: make(chan struct{}, 1), done: make(chan struct{})}
	go q.run()
	return q
}

// add never blocks (it runs under mu).
func (q *eventQueue) add(f func()) {
	q.mu.Lock()
	stopped := q.stopped
	if !stopped {
		q.pending = append(q.pending, f)
	}
	n := len(q.pending)
	q.mu.Unlock()
	switch {
	case stopped:
		slog.Warn("an event came after shutdown: it was lost")
	case n == longEventQueue:
		slog.Warn("the event queue is long: hooks are slow", "events", n)
	}
	q.signal()
}

func (q *eventQueue) signal() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

func (q *eventQueue) run() {
	defer close(q.done)
	for range q.wake {
		q.mu.Lock()
		batch, closed := q.pending, q.closed
		q.pending, q.stopped = nil, closed
		q.mu.Unlock()
		for _, f := range batch {
			f()
		}
		if closed {
			return
		}
	}
}

// close runs the hooks queued so far and stops; it waits at most d.
func (q *eventQueue) close(d time.Duration) {
	q.mu.Lock()
	q.closed = true
	q.mu.Unlock()
	q.signal()
	select {
	case <-q.done:
	case <-time.After(d):
	}
}
