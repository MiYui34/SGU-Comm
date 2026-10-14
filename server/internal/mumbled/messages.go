package mumbled

import (
	"errors"
	"log/slog"
	"maps"
	"regexp"
	"slices"
	"strings"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

// selfState applies a client's own mute and deafen change, with Mumble's
// rules: deafened means muted, and unmuting undeafens.
func selfState(mute, deaf bool, m *pb.UserState) (bool, bool) {
	if m.SelfMute != nil {
		mute = m.GetSelfMute()
		if !mute {
			deaf = false
		}
	}
	if m.SelfDeaf != nil {
		deaf = m.GetSelfDeaf()
		if deaf {
			mute = true
		}
	}
	return mute, deaf
}

// Tags start with a letter, "/" or "!"; "a < b" is not a tag. An unclosed
// tag at the end is removed too.
var htmlTag = regexp.MustCompile(`<[a-zA-Z/!][^>]*(>|$)`)

// plainText removes HTML tags: the server does not allow HTML (as Murmur
// with allowhtml off), and Mumble clients would render it.
func plainText(s string) string {
	return strings.TrimSpace(htmlTag.ReplaceAllString(s, ""))
}

func optionalPlainText(s *string) *string {
	if s == nil {
		return nil
	}
	return proto.String(plainText(*s))
}

func (c *client) deny(err error) {
	var d *DenyError
	if !errors.As(err, &d) {
		d = &DenyError{Type: pb.PermissionDenied_Text, Reason: err.Error()}
	}
	c.sendMsg(mumble.TypePermissionDenied, &pb.PermissionDenied{Type: d.Type.Enum(), Reason: proto.String(d.Reason)})
}

func (s *Server) message(c *client, typ uint16, body []byte) {
	switch typ {
	case mumble.TypePing:
		var ping pb.Ping
		proto.Unmarshal(body, &ping)
		stats := c.stats()
		c.sendMsg(mumble.TypePing, &pb.Ping{Timestamp: ping.Timestamp, Good: stats.Good, Late: stats.Late, Lost: stats.Lost})
	case mumble.TypeUDPTunnel:
		s.mu.Lock()
		c.udpVoice = false
		s.mu.Unlock()
		s.voice(c, body)
	case mumble.TypeUserState:
		var m pb.UserState
		if proto.Unmarshal(body, &m) == nil && c.limiter.Allow() {
			s.userStateMsg(c, &m)
		}
	case mumble.TypeChannelState:
		var m pb.ChannelState
		if proto.Unmarshal(body, &m) == nil && c.limiter.Allow() {
			s.channelStateMsg(c, &m)
		}
	case mumble.TypeChannelRemove:
		var m pb.ChannelRemove
		if proto.Unmarshal(body, &m) == nil && c.limiter.Allow() {
			s.channelRemoveMsg(c, &m)
		}
	case mumble.TypeTextMessage:
		var m pb.TextMessage
		if proto.Unmarshal(body, &m) == nil && len(m.GetMessage()) <= maxTextLength && c.limiter.Allow() {
			s.textMsg(c, &m)
		}
	case mumble.TypeCryptSetup:
		var m pb.CryptSetup
		proto.Unmarshal(body, &m)
		c.cryptMu.Lock()
		if len(m.ClientNonce) == 16 {
			c.crypt.DecryptIV = [16]byte(m.ClientNonce)
		}
		iv := c.crypt.EncryptIV
		c.cryptMu.Unlock()
		if len(m.ClientNonce) != 16 {
			c.sendMsg(mumble.TypeCryptSetup, &pb.CryptSetup{ServerNonce: iv[:]})
		}
	case mumble.TypePermissionQuery:
		var m pb.PermissionQuery
		proto.Unmarshal(body, &m)
		c.sendMsg(mumble.TypePermissionQuery, &pb.PermissionQuery{
			ChannelId: proto.Uint32(m.GetChannelId()), Permissions: proto.Uint32(permissions),
		})
	case mumble.TypeUserStats:
		var m pb.UserStats
		proto.Unmarshal(body, &m)
		s.mu.Lock()
		target := s.clients[m.GetSession()]
		s.mu.Unlock()
		if target != nil {
			c.sendMsg(mumble.TypeUserStats, &pb.UserStats{
				Session: proto.Uint32(target.session), StatsOnly: proto.Bool(true), FromClient: target.stats(),
			})
		}
	}
}

func (s *Server) userStateMsg(c *client, m *pb.UserState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	target := c.session
	if m.Session != nil {
		target = m.GetSession()
	}
	u := s.state.Users[target]
	if u == nil {
		return
	}
	out := &pb.UserState{Session: proto.Uint32(target), Actor: proto.Uint32(c.session)}
	changed := false
	if target == c.session && (m.SelfMute != nil || m.SelfDeaf != nil) {
		u.SelfMute, u.SelfDeaf = selfState(u.SelfMute, u.SelfDeaf, m)
		out.SelfMute, out.SelfDeaf = proto.Bool(u.SelfMute), proto.Bool(u.SelfDeaf)
		changed = true
	}
	var removed []uint32
	if m.ChannelId != nil && s.state.Channels[m.GetChannelId()] != nil && u.Channel != m.GetChannelId() {
		removed = s.state.MoveUser(target, m.GetChannelId())
		out.ChannelId = proto.Uint32(u.Channel)
		changed = true
	}
	if changed {
		s.broadcastLocked(mumble.MessageFrame(mumble.TypeUserState, out), nil)
	}
	s.broadcastRemovedLocked(removed)
}

func cloneChannel(ch *Channel) *Channel {
	cp := *ch
	cp.Links = maps.Clone(ch.Links)
	return &cp
}

// save writes channel changes after the state changed. Callers hold
// saveMu (not mu), so saves keep the order of the changes.
func (s *Server) save(channels []*Channel, removed []uint32, lastID uint32) {
	db := s.cfg.DB
	if db == nil {
		return
	}
	for _, ch := range channels {
		if err := db.SaveChannel(ch); err != nil {
			slog.Error("saving channel", "channel", ch.ID, "err", err)
		}
	}
	if len(removed) > 0 {
		if err := db.DeleteChannels(removed); err != nil {
			slog.Error("removing channels", "channels", removed, "err", err)
		}
	}
	if lastID > 0 {
		if err := db.SaveNextChannel(lastID); err != nil {
			slog.Error("saving channel id", "err", err)
		}
	}
}

func (s *Server) channelStateMsg(c *client, m *pb.ChannelState) {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	if m.ChannelId == nil {
		s.createChannel(c, m)
	} else {
		s.editChannel(c, m)
	}
}

func (s *Server) createChannel(c *client, m *pb.ChannelState) {
	if m.Parent == nil || m.Name == nil {
		return // Murmur needs both
	}
	s.mu.Lock()
	ch, err := s.state.AddChannel(m.GetParent(), m.GetName(), m.GetTemporary())
	if err == nil {
		err = s.state.EditChannel(ch.ID, ChannelEdit{Description: optionalPlainText(m.Description), Position: m.Position})
		if err != nil {
			s.state.RemoveChannel(ch.ID)
		}
	}
	if err != nil {
		s.mu.Unlock()
		c.deny(err)
		return
	}
	s.broadcastLocked(mumble.MessageFrame(mumble.TypeChannelState, channelState(ch)), nil)
	var save []*Channel
	if ch.Temporary {
		// The creator goes into a temporary channel, as in Murmur.
		removed := s.state.MoveUser(c.session, ch.ID)
		s.broadcastLocked(mumble.MessageFrame(mumble.TypeUserState, &pb.UserState{
			Session: proto.Uint32(c.session), Actor: proto.Uint32(c.session), ChannelId: proto.Uint32(ch.ID),
		}), nil)
		s.broadcastRemovedLocked(removed)
	} else {
		save = []*Channel{cloneChannel(ch)}
	}
	lastID := s.state.LastChannelID()
	s.mu.Unlock()
	s.save(save, nil, lastID)
}

func (s *Server) editChannel(c *client, m *pb.ChannelState) {
	s.mu.Lock()
	id := m.GetChannelId()
	ch := s.state.Channels[id]
	if ch == nil {
		s.mu.Unlock()
		return
	}
	// As Murmur: unknown link targets refuse the whole message.
	for _, l := range slices.Concat(m.LinksAdd, m.LinksRemove) {
		if s.state.Channels[l] == nil {
			s.mu.Unlock()
			return
		}
	}
	edit := ChannelEdit{Name: m.Name, Description: optionalPlainText(m.Description), Position: m.Position, Parent: m.Parent}
	if err := s.state.EditChannel(id, edit); err != nil {
		s.mu.Unlock()
		c.deny(err)
		return
	}
	out := &pb.ChannelState{ChannelId: proto.Uint32(id), Position: m.Position, Parent: m.Parent}
	if m.Name != nil {
		out.Name = proto.String(ch.Name)
	}
	if m.Description != nil {
		out.Description = proto.String(ch.Description)
	}
	// Only links that really change.
	save := []*Channel{}
	for _, l := range m.LinksAdd {
		if l != id && !ch.Links[l] {
			s.state.Link(id, l)
			out.LinksAdd = append(out.LinksAdd, l)
		}
	}
	for _, l := range m.LinksRemove {
		if ch.Links[l] {
			s.state.Unlink(id, l)
			out.LinksRemove = append(out.LinksRemove, l)
		}
	}
	s.broadcastLocked(mumble.MessageFrame(mumble.TypeChannelState, out), nil)
	// Linked channels are saved too: a link is stored with both.
	for _, other := range slices.Concat([]uint32{id}, out.LinksAdd, out.LinksRemove) {
		if o := s.state.Channels[other]; o != nil && !o.Temporary {
			save = append(save, cloneChannel(o))
		}
	}
	s.mu.Unlock()
	s.save(save, nil, 0)
}

func (s *Server) channelRemoveMsg(c *client, m *pb.ChannelRemove) {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	s.mu.Lock()
	moved, removed, err := s.state.RemoveChannel(m.GetChannelId())
	if err != nil {
		s.mu.Unlock()
		c.deny(err)
		return
	}
	for _, session := range slices.Sorted(maps.Keys(moved)) {
		s.broadcastLocked(mumble.MessageFrame(mumble.TypeUserState, &pb.UserState{
			Session: proto.Uint32(session), Actor: proto.Uint32(c.session), ChannelId: proto.Uint32(moved[session]),
		}), nil)
	}
	s.broadcastRemovedLocked(removed)
	s.mu.Unlock()
	s.save(nil, removed, 0)
}

func (s *Server) textMsg(c *client, m *pb.TextMessage) {
	text := plainText(m.GetMessage())
	if text == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	channels := map[uint32]bool{}
	for _, id := range m.ChannelId {
		channels[id] = true
	}
	for _, id := range m.TreeId {
		if s.state.Channels[id] != nil {
			for _, sub := range s.state.Subtree(id) {
				channels[sub] = true
			}
		}
	}
	to := map[uint32]bool{}
	for _, u := range s.state.Users {
		if channels[u.Channel] {
			to[u.Session] = true
		}
	}
	for _, session := range m.Session {
		to[session] = true
	}
	delete(to, c.session)
	frame := mumble.MessageFrame(mumble.TypeTextMessage, &pb.TextMessage{
		Actor: proto.Uint32(c.session), Session: m.Session, ChannelId: m.ChannelId, TreeId: m.TreeId, Message: proto.String(text),
	})
	for session := range to {
		if o := s.clients[session]; o != nil {
			o.send(frame, false)
		}
	}
}
