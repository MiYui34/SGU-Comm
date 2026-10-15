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
	"kallisti.chat/server/internal/names"
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

// serverMuteState applies a moderator's mute and deafen, with Murmur's
// rules: deafened means muted, and unmuting undeafens.
func serverMuteState(mute, deaf bool, m *pb.UserState) (bool, bool) {
	if m.Deaf != nil {
		deaf = m.GetDeaf()
		if deaf {
			mute = true
		}
	}
	if m.Mute != nil {
		mute = m.GetMute()
		if !mute {
			deaf = false
		}
	}
	return mute, deaf
}

// moderatePermission checks server mute, deafen, suppress and priority
// speaker as Murmur: MuteDeafen in the target's channel (for temporary
// channels: in the first channel above that is not temporary); clients
// can only lift a suppress.
func (s *Server) moderatePermission(actor, u *User, m *pb.UserState) (Perm, uint32, error) {
	if !s.state.Has(actor, u.Channel, MuteDeafen) || m.GetSuppress() {
		return MuteDeafen, u.Channel, nil
	}
	if m.Mute != nil || m.Deaf != nil || m.Suppress != nil {
		ch := s.state.Channels[u.Channel]
		for ch != nil && ch.Temporary {
			ch = s.state.Channels[ch.Parent]
		}
		if ch == nil || !s.state.Has(actor, ch.ID, MuteDeafen) {
			return 0, 0, deny(pb.PermissionDenied_TemporaryChannel, "not in temporary channels")
		}
	}
	return 0, 0, nil
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
	// A replaced or kicked session may still have messages on the way.
	s.mu.Lock()
	current := s.clients[c.session] == c
	s.mu.Unlock()
	if !current {
		return
	}
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
			if m.UserId != nil {
				s.registerMsg(c, &m)
			}
			s.userStateMsg(c, &m)
		}
	case mumble.TypeACL:
		var m pb.ACL
		if proto.Unmarshal(body, &m) == nil && c.limiter.Allow() {
			s.aclMsg(c, &m)
		}
	case mumble.TypeUserRemove:
		var m pb.UserRemove
		if proto.Unmarshal(body, &m) == nil && c.limiter.Allow() {
			s.userRemoveMsg(c, &m)
		}
	case mumble.TypeBanList:
		var m pb.BanList
		if proto.Unmarshal(body, &m) == nil && c.limiter.Allow() {
			s.banListMsg(c, &m)
		}
	case mumble.TypeVoiceTarget:
		var m pb.VoiceTarget
		if proto.Unmarshal(body, &m) == nil {
			s.voiceTargetMsg(c, &m)
		}
	case mumble.TypeRequestBlob:
		var m pb.RequestBlob
		if proto.Unmarshal(body, &m) == nil && c.limiter.Allow() {
			s.requestBlobMsg(c, &m)
		}
	case mumble.TypeUserList:
		var m pb.UserList
		if proto.Unmarshal(body, &m) == nil && c.limiter.Allow() {
			s.userListMsg(c, &m)
		}
	case mumble.TypeQueryUsers:
		var m pb.QueryUsers
		if proto.Unmarshal(body, &m) == nil && c.limiter.Allow() {
			s.queryUsersMsg(c, &m)
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
		s.mu.Lock()
		var perms Perm
		if u := s.state.Users[c.session]; u != nil {
			perms = s.state.Permissions(u, m.GetChannelId())
		}
		s.mu.Unlock()
		c.sendMsg(mumble.TypePermissionQuery, &pb.PermissionQuery{
			ChannelId: proto.Uint32(m.GetChannelId()), Permissions: proto.Uint32(uint32(perms)),
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
	if m.Comment != nil || m.Texture != nil {
		s.saveMu.Lock() // the save below keeps the order of the changes
		defer s.saveMu.Unlock()
	}
	s.mu.Lock()
	after := s.userStateLocked(c, m)
	s.mu.Unlock()
	if after != nil {
		after()
	}
}

// userStateLocked applies a UserState; it returns work for after the lock
// (saving), or nil.
func (s *Server) userStateLocked(c *client, m *pb.UserState) (after func()) {
	actor := s.state.Users[c.session]
	target := c.session
	if m.Session != nil {
		target = m.GetSession()
	}
	u := s.state.Users[target]
	if u == nil || actor == nil {
		return
	}
	// Checks first, as Murmur: nothing changes if one fails.
	if isSuperUser(u) && u != actor {
		c.deny(deny(pb.PermissionDenied_SuperUser, "SuperUser cannot be changed"))
		return
	}
	if m.Name != nil {
		c.deny(deny(pb.PermissionDenied_UserName, "names cannot be changed"))
		return
	}
	if u != actor && (m.SelfMute != nil || m.SelfDeaf != nil || m.Recording != nil || m.PluginContext != nil ||
		len(m.ListeningChannelAdd)+len(m.ListeningChannelRemove) > 0) {
		return // only users themselves set these
	}
	content := m.Comment != nil || m.Texture != nil
	if content {
		if missing, err := s.contentChange(actor, u, m); err != nil {
			c.deny(err)
			return
		} else if missing != 0 {
			c.denyPerm(missing, 0)
			return
		}
	}
	moderate := m.Mute != nil || m.Deaf != nil || m.Suppress != nil || m.PrioritySpeaker != nil
	if moderate && isSuperUser(u) {
		c.deny(deny(pb.PermissionDenied_SuperUser, "SuperUser cannot be muted"))
		return
	}
	if moderate {
		if missing, where, err := s.moderatePermission(actor, u, m); err != nil {
			c.deny(err)
			return
		} else if missing != 0 {
			c.denyPerm(missing, where)
			return
		}
	}
	dest := s.state.Channels[m.GetChannelId()]
	move := m.ChannelId != nil && dest != nil && dest.ID != u.Channel
	if move {
		if u != actor && !s.state.Has(actor, u.Channel, Move) {
			c.denyPerm(Move, u.Channel)
			return
		}
		if !s.state.Has(actor, dest.ID, Move) && !s.state.Has(u, dest.ID, Enter) {
			c.denyPerm(Enter, dest.ID)
			return
		}
	}
	out := &pb.UserState{Session: proto.Uint32(target), Actor: proto.Uint32(c.session)}
	changed := false
	if m.PluginContext != nil {
		u.PluginContext = m.PluginContext // private: not sent to others, as Murmur
	}
	routing := false // a change of who hears whom
	if m.SelfMute != nil || m.SelfDeaf != nil {
		routing = routing || m.SelfDeaf != nil
		u.SelfMute, u.SelfDeaf = selfState(u.SelfMute, u.SelfDeaf, m)
		out.SelfMute, out.SelfDeaf = proto.Bool(u.SelfMute), proto.Bool(u.SelfDeaf)
		changed = true
	}
	if m.Recording != nil && m.GetRecording() != u.Recording {
		u.Recording = m.GetRecording()
		out.Recording = proto.Bool(u.Recording)
		changed = true
	}
	if moderate {
		routing = true
		u.Mute, u.Deaf = serverMuteState(u.Mute, u.Deaf, m)
		out.Mute, out.Deaf = proto.Bool(u.Mute), proto.Bool(u.Deaf)
		if m.Suppress != nil {
			u.Suppress = m.GetSuppress() // only false gets here
			out.Suppress = proto.Bool(u.Suppress)
		}
		if m.PrioritySpeaker != nil {
			u.PrioritySpeaker = m.GetPrioritySpeaker()
			out.PrioritySpeaker = proto.Bool(u.PrioritySpeaker)
		}
		changed = true
	}
	for _, ch := range m.ListeningChannelAdd {
		if s.state.Channels[ch] != nil && !s.state.Has(u, ch, Listen) {
			c.denyPerm(Listen, ch)
		} else if !u.Listening[ch] && len(u.Listening) >= maxListening {
			c.deny(deny(pb.PermissionDenied_UserListenerLimit, "you listen to too many channels"))
		} else if s.state.Listen(u.Session, ch, true) {
			out.ListeningChannelAdd = append(out.ListeningChannelAdd, ch)
			changed = true
		}
	}
	for _, ch := range m.ListeningChannelRemove {
		if s.state.Listen(u.Session, ch, false) {
			out.ListeningChannelRemove = append(out.ListeningChannelRemove, ch)
			changed = true
		}
	}
	if content {
		applyContent(u, m, out)
		changed = true
		saved := contentOf(u)
		after = func() { s.saveContent(saved) }
	}
	if routing {
		s.state.Changed()
	}
	if changed {
		s.broadcastLocked(mumble.MessageFrame(mumble.TypeUserState, out), nil)
	}
	if move {
		s.moveLocked(u, dest.ID, c.session)
	}
	return after
}

// cloneChannel copies a channel for saving outside the lock.
func cloneChannel(ch *Channel) *Channel {
	cp := *ch
	cp.Links = maps.Clone(ch.Links)
	cp.ACLs = slices.Clone(ch.ACLs)
	cp.Groups = make(map[string]*Group, len(ch.Groups))
	for name, g := range ch.Groups {
		gc := *g
		gc.Add, gc.Remove = maps.Clone(g.Add), maps.Clone(g.Remove)
		cp.Groups[name] = &gc
	}
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
	actor := s.state.Users[c.session]
	need := MakeChannel
	if m.GetTemporary() {
		need = MakeTempChannel
	}
	if actor == nil || s.state.Channels[m.GetParent()] == nil {
		s.mu.Unlock()
		return
	}
	if !s.state.Has(actor, m.GetParent(), need) {
		s.mu.Unlock()
		c.denyPerm(need, m.GetParent())
		return
	}
	if !actor.Registered && actor.Hash == "" {
		s.mu.Unlock()
		c.deny(deny(pb.PermissionDenied_MissingCertificate, "making channels needs a client certificate"))
		return
	}
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
	s.state.grantCreator(ch, actor)
	for _, u := range s.state.Users {
		if o := s.clients[u.Session]; o != nil {
			o.sendMsg(mumble.TypeChannelState, s.state.channelStateFor(ch, u))
		}
	}
	var save []*Channel
	if ch.Temporary {
		// The creator goes into a temporary channel, as in Murmur.
		s.moveLocked(actor, ch.ID, c.session)
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
	actor := s.state.Users[c.session]
	if actor == nil {
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
	if missing, where := s.editPermission(actor, ch, m); missing != 0 {
		s.mu.Unlock()
		c.denyPerm(missing, where)
		return
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
		out.Description, out.DescriptionHash = commentFields(ch.Description)
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
	if m.Parent != nil {
		s.permissionsChangedLocked(ch) // inherited ACLs changed
	}
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
	actor := s.state.Users[c.session]
	if ch := s.state.Channels[m.GetChannelId()]; ch == nil || actor == nil {
		s.mu.Unlock()
		return
	} else if !s.state.Has(actor, ch.ID, Write) {
		s.mu.Unlock()
		c.denyPerm(Write, ch.ID)
		return
	}
	moved, removed, err := s.state.RemoveChannel(m.GetChannelId())
	if err != nil {
		s.mu.Unlock()
		c.deny(err)
		return
	}
	for _, session := range slices.Sorted(maps.Keys(moved)) {
		u := s.state.Users[session]
		s.refreshLocked(u)
		s.broadcastLocked(mumble.MessageFrame(mumble.TypeUserState, &pb.UserState{
			Session: proto.Uint32(session), Actor: proto.Uint32(c.session), ChannelId: proto.Uint32(moved[session]),
			Suppress: proto.Bool(u.Suppress),
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
	// Runs after the unlock below (defers run last-in, first-out).
	var hooks []func()
	defer func() {
		for _, h := range hooks {
			h()
		}
	}()
	s.mu.Lock()
	defer s.mu.Unlock()
	actor := s.state.Users[c.session]
	if actor == nil {
		return
	}
	// TextMessage in every target channel, tree and user's channel, or
	// nothing is sent (as Murmur). Inside a tree, channels without it are
	// skipped.
	check := map[uint32]bool{}
	for _, id := range slices.Concat(m.ChannelId, m.TreeId) {
		check[id] = true
	}
	for _, session := range m.Session {
		if u := s.state.Users[session]; u != nil {
			check[u.Channel] = true
		}
	}
	for _, id := range slices.Sorted(maps.Keys(check)) {
		if s.state.Channels[id] != nil && !s.state.Has(actor, id, TextMessage) {
			c.denyPerm(TextMessage, id)
			return
		}
	}
	channels := map[uint32]bool{}
	for _, id := range m.ChannelId {
		channels[id] = true
	}
	for _, id := range m.TreeId {
		if s.state.Channels[id] != nil {
			for _, sub := range s.state.Subtree(id) {
				if sub == id || s.state.Has(actor, sub, TextMessage) {
					channels[sub] = true
				}
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
	if hook := s.cfg.OnChannelText; hook != nil {
		for _, id := range m.ChannelId {
			if s.state.Channels[id] != nil {
				from, name, registered := c.session, actor.Name, actor.Registered
				hooks = append(hooks, func() { hook(from, name, registered, id, text) })
			}
		}
	}
}

// registerMsg registers a user with its certificate: itself (SelfRegister)
// or another user (Register).
func (s *Server) registerMsg(c *client, m *pb.UserState) {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	s.mu.Lock()
	target := c.session
	if m.Session != nil {
		target = m.GetSession()
	}
	actor, u := s.state.Users[c.session], s.state.Users[target]
	if actor == nil || u == nil {
		s.mu.Unlock()
		return
	}
	need := SelfRegister
	if u != actor {
		need = Register
	}
	var err error
	switch {
	case u.Registered || !s.state.Has(actor, 0, need):
		s.mu.Unlock()
		c.denyPerm(need, 0)
		return
	case u.Hash == "":
		err = deny(pb.PermissionDenied_MissingCertificate, "registration needs a client certificate")
	case s.cfg.DB == nil:
		err = deny(pb.PermissionDenied_Text, "this server does not register users")
	}
	if err == nil && s.cfg.ReservedName != nil && s.cfg.ReservedName(names.Key(u.Name)) {
		err = deny(pb.PermissionDenied_UserName, "this name belongs to a Kallisti account")
	}
	name, hash := u.Name, u.Hash
	s.mu.Unlock()
	if err != nil {
		c.deny(err)
		return
	}
	id, err := s.cfg.DB.RegisterUser(name, hash)
	if err != nil {
		slog.Warn("registering a user", "err", err) // for example a name taken just now
		c.deny(deny(pb.PermissionDenied_Text, "registration failed"))
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if u := s.state.Users[target]; u != nil {
		u.Registered, u.UserID = true, id
		s.broadcastLocked(mumble.MessageFrame(mumble.TypeUserState, &pb.UserState{
			Session: proto.Uint32(target), Actor: proto.Uint32(c.session), UserId: proto.Uint32(id),
		}), nil)
		s.permissionsChangedLocked(nil) // @auth and user entries apply now
		if u.Comment != "" || len(u.Texture) > 0 {
			saved := contentOf(u) // what was set before registering stays
			s.mu.Unlock()
			s.saveContent(saved)
			s.mu.Lock()
		}
	}
}

// queryUsersMsg maps registered user ids to names and back.
func (s *Server) queryUsersMsg(c *client, m *pb.QueryUsers) {
	db := s.cfg.DB
	if db == nil || len(m.Ids)+len(m.Names) > 100 {
		return
	}
	// As Murmur: for ACL editors, who have Write somewhere.
	s.mu.Lock()
	u := s.state.Users[c.session]
	editor := u != nil && slices.ContainsFunc(slices.Collect(maps.Keys(s.state.Channels)), func(id uint32) bool {
		return s.state.Has(u, id, Write)
	})
	s.mu.Unlock()
	if !editor {
		c.denyPerm(Write, 0)
		return
	}
	out := &pb.QueryUsers{}
	byID, err := db.UserNames(m.Ids)
	if err != nil {
		slog.Error("query users", "err", err)
		return
	}
	for _, id := range slices.Sorted(maps.Keys(byID)) {
		out.Ids, out.Names = append(out.Ids, id), append(out.Names, byID[id])
	}
	for _, name := range m.Names {
		if r, err := db.UserByName(name); err == nil && r != nil {
			out.Ids, out.Names = append(out.Ids, r.ID), append(out.Names, r.Name)
		}
	}
	c.sendMsg(mumble.TypeQueryUsers, out)
}

// editPermission checks a channel edit as Murmur does: Write for name,
// description and position; Write and MakeChannel in the new parent for a
// move; LinkChannel in both channels for links. It returns the missing
// permission and where, or 0.
func (s *Server) editPermission(u *User, ch *Channel, m *pb.ChannelState) (Perm, uint32) {
	if (m.Name != nil || m.Description != nil || m.Position != nil || m.Parent != nil) && !s.state.Has(u, ch.ID, Write) {
		return Write, ch.ID
	}
	if m.Parent != nil && !s.state.Has(u, m.GetParent(), MakeChannel) {
		return MakeChannel, m.GetParent()
	}
	for _, l := range slices.Concat(m.LinksAdd, m.LinksRemove) {
		for _, id := range []uint32{ch.ID, l} {
			if !s.state.Has(u, id, LinkChannel) {
				return LinkChannel, id
			}
		}
	}
	return 0, 0
}

// voiceTargetMsg stores a whisper or shout target.
func (s *Server) voiceTargetMsg(c *client, m *pb.VoiceTarget) {
	var vt VoiceTarget
	for _, t := range m.Targets {
		vt.Sessions = append(vt.Sessions, t.Session...)
		if t.ChannelId != nil {
			vt.Channels = append(vt.Channels, TargetChannel{ID: t.GetChannelId(), Children: t.GetChildren(), Links: t.GetLinks(), Group: t.GetGroup()})
		}
		if len(vt.Sessions) > maxTargetSessions*2 || len(vt.Channels) > maxTargetChannels*2 {
			break
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.SetTarget(c.session, m.GetId(), vt)
}
