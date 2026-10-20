package mumbled

import (
	"fmt"
	"testing"

	"golang.org/x/time/rate"
	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

// refused sends a channel message and expects a missing permission in a
// channel. A refusal is also a round trip: messages before it are done.
func (c *testClient) refused(typ uint16, m proto.Message, p Perm, channel uint32) {
	c.t.Helper()
	c.send(typ, m)
	if d := c.expectDenied(pb.PermissionDenied_Permission); Perm(d.GetPermission()) != p || d.GetChannelId() != channel {
		c.t.Fatalf("denied %v, want %#x in %d", d, p, channel)
	}
}

// unlimited lifts a client's message limit (the tests send many).
func (c clubServer) unlimited(u *testClient) {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	c.s.clients[u.session].limiter.SetLimit(rate.Inf)
}

func rename(id uint32, name string) *pb.ChannelState {
	return &pb.ChannelState{ChannelId: proto.Uint32(id), Name: proto.String(name)}
}

func reparent(id, parent uint32) *pb.ChannelState {
	return &pb.ChannelState{ChannelId: proto.Uint32(id), Parent: proto.Uint32(parent)}
}

func removal(id uint32) *pb.ChannelRemove { return &pb.ChannelRemove{ChannelId: proto.Uint32(id)} }

// channel copies a channel of the server's state (nil: gone).
func (c clubServer) channel(id uint32) *Channel {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	if ch := c.s.state.Channels[id]; ch != nil {
		return cloneChannel(ch)
	}
	return nil
}

func (c clubServer) perms(u *testClient, id uint32) Perm {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	return c.s.state.Permissions(c.s.state.Users[u.session], id)
}

func TestChannelModeratorsGetMakeChannelAndWriteBelowTheirChannel(t *testing.T) {
	c := newClubServer(t)
	deep := c.su.createChannel(c.sub, "Deep", false)
	for _, e := range []struct {
		channel     uint32
		make, write bool
	}{{c.club, true, false}, {c.sub, true, true}, {deep, true, true}, {c.other, false, false}, {0, false, false}} {
		p := c.perms(c.alice, e.channel)
		if p&MakeChannel != 0 != e.make || p&Write != 0 != e.write {
			t.Errorf("channel %d: %#x", e.channel, p)
		}
	}
	// Not the ACL editor of a subchannel: Write there is not from the ACLs.
	c.alice.refused(mumble.TypeACL, &pb.ACL{ChannelId: proto.Uint32(c.sub), Query: proto.Bool(true)}, Write, c.sub)
}

func TestChannelModeratorsMakeSubchannelsOnlyInTheirSubtree(t *testing.T) {
	c := newClubServer(t)
	lounge := c.alice.createChannel(c.club, "Lounge", false)
	corner := c.alice.createChannel(lounge, "Corner", false)
	c.alice.refused(mumble.TypeChannelState, &pb.ChannelState{Parent: proto.Uint32(c.other), Name: proto.String("No")}, MakeChannel, c.other)
	c.alice.refused(mumble.TypeChannelState, &pb.ChannelState{Parent: proto.Uint32(0), Name: proto.String("No")}, MakeChannel, 0)
	ch := c.channel(corner)
	if ch == nil || ch.Parent != lounge || ch.Temporary || len(ch.ACLs) > 0 || len(ch.Groups) > 0 {
		t.Fatalf("corner %+v", ch)
	}
	// They moderate them, by inheritance; and the channels are saved.
	if !c.s.ModeratesIn(c.alice.session, corner) || c.perms(c.alice, corner)&Write == 0 {
		t.Fatal("no moderator in a new subchannel")
	}
	restarted, err := New(Config{DB: c.db})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if got := restarted.state.Channels[corner]; got == nil || got.Parent != lounge || got.Name != "Corner" {
		t.Fatalf("after a restart: %+v", got)
	}
}

func TestChannelModeratorsEditAndDeleteOnlySubchannels(t *testing.T) {
	c := newClubServer(t)
	c.unlimited(c.alice)
	deep := c.su.createChannel(c.sub, "Deep", false)
	c.alice.send(mumble.TypeChannelState, rename(c.sub, "Den"))
	c.alice.send(mumble.TypeChannelState, &pb.ChannelState{ChannelId: proto.Uint32(deep), Position: proto.Int32(5), Description: proto.String("hi")})
	// The moderated channel itself, and channels elsewhere: no.
	for _, id := range []uint32{c.club, c.other, 0} {
		c.alice.refused(mumble.TypeChannelState, rename(id, "X"), Write, id)
		c.alice.refused(mumble.TypeChannelState, &pb.ChannelState{ChannelId: proto.Uint32(id), Position: proto.Int32(3)}, Write, id)
		c.alice.refused(mumble.TypeChannelRemove, removal(id), Write, id)
	}
	if sub, d := c.channel(c.sub), c.channel(deep); sub.Name != "Den" || d.Position != 5 || d.Description != "hi" {
		t.Fatalf("sub %+v, deep %+v", sub, d)
	}
	c.alice.send(mumble.TypeChannelRemove, removal(deep))
	c.alice.refused(mumble.TypeChannelRemove, removal(c.club), Write, c.club)
	if c.channel(deep) != nil || c.channel(c.club) == nil {
		t.Fatal("deep is still there, or club went")
	}
	restarted, err := New(Config{DB: c.db})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if got := restarted.state.Channels[c.sub]; got == nil || got.Name != "Den" || restarted.state.Channels[deep] != nil {
		t.Fatalf("after a restart: %+v", restarted.state.Channels)
	}
}

func TestChannelModeratorsMoveChannelsOnlyWithinTheirSubtree(t *testing.T) {
	c := newClubServer(t)
	c.unlimited(c.alice)
	deep := c.su.createChannel(c.sub, "Deep", false)
	c.alice.send(mumble.TypeChannelState, reparent(deep, c.club))
	// Out of it, from outside into it, the moderated channel itself.
	c.alice.refused(mumble.TypeChannelState, reparent(c.sub, 0), MakeChannel, 0)
	c.alice.refused(mumble.TypeChannelState, reparent(c.sub, c.other), MakeChannel, c.other)
	c.alice.refused(mumble.TypeChannelState, reparent(c.other, c.club), Write, c.other)
	c.alice.refused(mumble.TypeChannelState, reparent(c.club, c.other), Write, c.club)
	// Nor into a temporary channel of hers (she has Write there by its ACL),
	// nor that one into her subtree.
	mine := c.alice.createChannel(0, "Mine", true)
	c.alice.refused(mumble.TypeChannelState, reparent(c.sub, mine), MakeChannel, mine)
	c.alice.refused(mumble.TypeChannelState, reparent(mine, c.sub), MakeChannel, c.sub)
	if d := c.channel(deep); d.Parent != c.club || c.channel(c.sub).Parent != c.club || c.channel(mine).Parent != 0 {
		t.Fatalf("deep %+v", d)
	}
}

func TestChannelModeratorsLeaveHiddenPrivateSubchannelsAlone(t *testing.T) {
	c := newClubServer(t)
	hidden := c.su.createChannel(c.sub, "Hidden", false)
	c.s.SetPrivate(c.su.session, hidden, true, nil)
	if p := c.perms(c.alice, hidden); p&(Write|MakeChannel) != 0 {
		t.Fatalf("hidden: %#x", p)
	}
	c.alice.refused(mumble.TypeChannelState, rename(hidden, "X"), Write, hidden)
	// Nor does a delete of the channel above take it along.
	c.alice.refused(mumble.TypeChannelRemove, removal(c.sub), Write, c.sub)
	if c.channel(hidden) == nil || c.channel(c.sub) == nil {
		t.Fatal("removed")
	}
}

func TestChannelModeratorsMakePrivateSubchannels(t *testing.T) {
	c := newClubServer(t)
	s := c.s
	bob, _ := registered(t, c.addr, "bob")
	id, err := s.CreatePrivateChannel(c.alice.session, c.sub, "Secret")
	if err != nil {
		t.Fatal(err)
	}
	ch := c.channel(id)
	if ch.Parent != c.sub || !isPrivate(ch) || ch.Groups["admin"] != nil || !ch.Groups[membersGroup].Add[c.aliceID] {
		t.Fatalf("secret %+v", ch)
	}
	// She sees and moderates it (a member); bob does not see it.
	if !s.ModeratesIn(c.alice.session, id) || s.Sees(bob.session, id) || c.perms(c.alice, id)&Write == 0 {
		t.Fatal("wrong access to the private subchannel")
	}
	for _, e := range []struct {
		name          string
		actor, parent uint32
	}{
		{"elsewhere", c.alice.session, c.other},
		{"in root", c.alice.session, 0},
		{"a plain user", bob.session, c.club},
		{"an unknown channel", c.su.session, 999},
	} {
		if _, err := s.CreatePrivateChannel(e.actor, e.parent, "No"); err == nil {
			t.Errorf("%s: no error", e.name)
		}
	}
	// Staff make them anywhere (and are their creator's admin).
	if id, err := s.CreatePrivateChannel(c.su.session, c.other, "Staff"); err != nil || c.channel(id).Parent != c.other {
		t.Fatalf("staff: %v", err)
	}
}

func TestChannelModeratorsOfTextChannelsMakeNoSubchannels(t *testing.T) {
	c := newClubServer(t)
	ids, err := c.s.SyncTextChannels([]TextMirror{{Name: "general"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.s.SetChannelModerator(c.su.session, c.aliceID, ids[0], true); err != nil {
		t.Fatal(err)
	}
	if p := c.perms(c.alice, ids[0]); p&(Write|MakeChannel) != 0 {
		t.Fatalf("mirror: %#x", p)
	}
	if _, err := c.s.CreatePrivateChannel(c.alice.session, ids[0], "No"); err == nil {
		t.Fatal("a private channel below a mirror")
	}
}

func TestChannelModeratorsGetNoACLEditorRightsInSubchannels(t *testing.T) {
	c := newClubServer(t)
	if err := c.s.SetListenOnly(c.alice.session, c.sub, true, nil); err == nil {
		t.Error("a channel moderator made a subchannel listen-only")
	}
	// Mumble's user lookup is for ACL editors.
	c.alice.refused(mumble.TypeQueryUsers, &pb.QueryUsers{Ids: []uint32{c.aliceID}}, Write, 0)
}

func TestChannelModeratorsMoveNoHiddenPrivateSubchannels(t *testing.T) {
	c := newClubServer(t)
	lounge := c.alice.createChannel(c.club, "Lounge", false)
	hidden := c.su.createChannel(c.sub, "Hidden", false)
	c.s.SetPrivate(c.su.session, hidden, true, nil)
	c.alice.refused(mumble.TypeChannelState, reparent(c.sub, lounge), MakeChannel, lounge)
	if c.channel(c.sub).Parent != c.club {
		t.Fatal("moved")
	}
}

// fill adds channels below a channel in the server's state (fast, no
// clients) and returns the last one.
func (c clubServer) fill(parent uint32, n int, nested bool) uint32 {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	for i := range n {
		ch, err := c.s.state.AddChannel(parent, fmt.Sprintf("Fill %d", i), false)
		if err != nil {
			c.alice.t.Fatal(err)
		}
		if nested {
			parent = ch.ID
		}
	}
	return parent
}

func TestChannelModeratorsMakeAtMostFiftySubchannels(t *testing.T) {
	c := newClubServer(t)
	c.fill(c.club, maxModSubchannels-1, false) // and Sub
	c.alice.send(mumble.TypeChannelState, &pb.ChannelState{Parent: proto.Uint32(c.sub), Name: proto.String("No")})
	if d := c.alice.expectDenied(pb.PermissionDenied_ChannelCountLimit); d.GetReason() != "This channel has the most subchannels it may have (50)" {
		t.Fatalf("reason %q", d.GetReason())
	}
	if _, err := c.s.CreatePrivateChannel(c.alice.session, c.club, "No"); err == nil || err.Error() != "This channel has the most subchannels it may have (50)" {
		t.Fatalf("web: %v", err)
	}
	// Staff are not limited.
	c.su.createChannel(c.sub, "Staff", false)
	mod, modID := registered(t, c.addr, "mod")
	c.s.SetModerator(c.su.session, modID, true)
	if _, err := c.s.CreatePrivateChannel(mod.session, c.club, "Mods"); err != nil {
		t.Fatalf("a moderator: %v", err)
	}
}

func TestChannelModeratorsMoveNoChannelsIntoAFullModeratedChannel(t *testing.T) {
	c := newClubServer(t)
	c.unlimited(c.alice)
	if err := c.s.SetChannelModerator(c.su.session, c.aliceID, c.other, true); err != nil {
		t.Fatal(err)
	}
	a, b := c.su.createChannel(c.other, "A", false), c.su.createChannel(c.other, "B", false)
	c.fill(c.other, maxModSubchannels-3, false) // one place left
	x := c.alice.createChannel(c.club, "X", false)
	c.alice.createChannel(x, "Y", false) // two channels go
	c.alice.send(mumble.TypeChannelState, reparent(x, c.other))
	if d := c.alice.expectDenied(pb.PermissionDenied_ChannelCountLimit); d.GetReason() != "This channel has the most subchannels it may have (50)" {
		t.Fatalf("reason %q", d.GetReason())
	}
	if c.channel(x).Parent != c.club {
		t.Fatalf("x %+v", c.channel(x))
	}
	// Within the full one, moves go on.
	c.alice.send(mumble.TypeChannelState, reparent(a, b))
	c.alice.sync()
	if c.channel(a).Parent != b {
		t.Fatalf("a %+v", c.channel(a))
	}
}

func TestChannelModeratorsMakeSubchannelsAtMostFiveLevelsDeep(t *testing.T) {
	c := newClubServer(t)
	// Also a moderator of Sub: the levels count from Club, the channel
	// she may not edit.
	if err := c.s.SetChannelModerator(c.su.session, c.aliceID, c.sub, true); err != nil {
		t.Fatal(err)
	}
	fifth := c.alice.createChannel(c.fill(c.sub, maxModSubchannelDepth-2, true), "Fifth", false)
	c.alice.send(mumble.TypeChannelState, &pb.ChannelState{Parent: proto.Uint32(fifth), Name: proto.String("No")})
	if d := c.alice.expectDenied(pb.PermissionDenied_NestingLimit); d.GetReason() != "Subchannels may be at most 5 levels deep" {
		t.Fatalf("reason %q", d.GetReason())
	}
	if _, err := c.s.CreatePrivateChannel(c.alice.session, fifth, "No"); err == nil || err.Error() != "Subchannels may be at most 5 levels deep" {
		t.Fatalf("web: %v", err)
	}
	c.su.createChannel(fifth, "Staff", false)
}

func TestChannelModeratorsMoveChannelsAtMostFiveLevelsDeep(t *testing.T) {
	c := newClubServer(t)
	c.unlimited(c.alice)
	third := c.fill(c.sub, 2, true)
	second := c.channel(third).Parent
	z := c.alice.createChannel(c.club, "Z", false)
	c.fill(z, 2, true)                                         // two levels below Z
	c.alice.send(mumble.TypeChannelState, reparent(z, second)) // its deepest at level 5
	c.alice.send(mumble.TypeChannelState, reparent(z, third))
	if d := c.alice.expectDenied(pb.PermissionDenied_NestingLimit); d.GetReason() != "Subchannels may be at most 5 levels deep" {
		t.Fatalf("reason %q", d.GetReason())
	}
	if c.channel(z).Parent != second {
		t.Fatalf("z %+v", c.channel(z))
	}
	// Staff are not limited.
	c.su.send(mumble.TypeChannelState, reparent(z, third))
	c.su.createChannel(0, "Sync", false)
	if c.channel(z).Parent != third {
		t.Fatalf("staff: z %+v", c.channel(z))
	}
}
