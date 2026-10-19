package mumbled

import (
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
	"kallisti.chat/server/internal/password"
	"kallisti.chat/server/internal/recording"
)

var speech = []byte{1 << 3, 0xaa} // SILK 20 ms

// recordingServer: a server with SuperUser's password "admin" and a
// recordings library; changes counts the reported changes.
func recordingServer(t *testing.T, limits recording.Limits) (*Server, string, *recording.Library, *atomic.Int32) {
	t.Helper()
	db := testDB(t)
	hash, _ := password.Hash("admin")
	db.SetPassword(0, hash)
	if limits.MaxDuration == 0 {
		limits = recording.Limits{MaxDuration: time.Hour, MaxBytes: 1 << 30}
	}
	lib := &recording.Library{Dir: t.TempDir(), Limits: limits, Keep: time.Hour, Now: time.Now}
	changes := &atomic.Int32{}
	s, addr := startServerAndListen(t, "127.0.0.1", Config{DB: db, Recordings: lib, OnRecordingChanged: func() { changes.Add(1) }}, nil)
	return s, addr, lib, changes
}

func TestOnlyStaffRecordVoiceChannels(t *testing.T) {
	s, addr, lib, changes := recordingServer(t, recording.Limits{})
	su := superUser(t, addr)
	alice, aliceID := registered(t, addr, "alice")
	bob, _ := registered(t, addr, "bob")
	s.SetModerator(su.session, aliceID, true)
	lounge := su.createChannel(0, "Lounge", false)
	ids, _ := s.SyncTextChannels([]TextMirror{{Name: "general"}})

	if s.MayRecord(bob.session) || !s.MayRecord(alice.session) || !s.MayRecord(su.session) {
		t.Fatal("MayRecord")
	}
	for _, c := range []struct {
		actor, channel uint32
	}{{bob.session, lounge}, {alice.session, 0}, {alice.session, ids[0]}, {alice.session, 999}} {
		if err := s.StartRecording(c.actor, c.channel); err == nil {
			t.Fatalf("%+v recorded", c)
		}
	}
	if err := s.StartRecording(alice.session, lounge); err != nil {
		t.Fatal(err)
	}
	if err := s.StartRecording(su.session, lounge); err == nil {
		t.Fatal("two recordings of one channel")
	}
	if got := s.Recording(); !slices.Equal(got, []uint32{lounge}) || changes.Load() != 1 {
		t.Fatalf("recording %v, %d changes", got, changes.Load())
	}
	if err := s.StopRecording(bob.session, lounge); err == nil {
		t.Fatal("bob stopped it")
	}
	if err := s.StopRecording(su.session, lounge); err != nil {
		t.Fatal(err)
	}
	if len(s.Recording()) != 0 || changes.Load() != 2 {
		t.Fatalf("recording %v, %d changes", s.Recording(), changes.Load())
	}
	if got := lib.List(); len(got) != 1 || got[0].By != "alice" || got[0].ChannelName != "Lounge" || got[0].Reason != "stopped" {
		t.Fatalf("finished %+v", got)
	}
	if err := s.StopRecording(su.session, lounge); err == nil {
		t.Fatal("stopped twice")
	}
}

// Who speaks to the channel gets a track; whispers, muted users and other
// channels are not recorded. Mumble clients are told.
func TestSpeakersInARecordedChannelGetTracks(t *testing.T) {
	s, addr, lib, _ := recordingServer(t, recording.Limits{})
	su := superUser(t, addr)
	lounge := su.createChannel(0, "Lounge", false)
	alice, bob, carol, dave := dial(t, addr, "alice"), dial(t, addr, "bob"), dial(t, addr, "carol"), dial(t, addr, "dave")
	for _, c := range []*testClient{alice, bob, carol} {
		su.move(c.session, lounge)
		su.waitForMove(c.session, lounge)
	}
	if err := s.StartRecording(su.session, lounge); err != nil {
		t.Fatal(err)
	}
	var m pb.TextMessage
	proto.Unmarshal(alice.expect(mumble.TypeTextMessage), &m)
	if !strings.Contains(m.GetMessage(), "Recording started") || !slices.Equal(m.ChannelId, []uint32{lounge}) {
		t.Fatalf("text %v", &m)
	}
	carol.send(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(carol.session), SelfMute: proto.Bool(true)})
	su.waitForSelfMute(carol.session)
	for i := range 5 {
		alice.conn.Write(mumble.EncodeFrame(mumble.TypeUDPTunnel, audio(uint64(2*i), speech)))
		bob.conn.Write(mumble.EncodeFrame(mumble.TypeUDPTunnel, audio(uint64(2*i), speech)))
		bob.conn.Write(mumble.EncodeFrame(mumble.TypeUDPTunnel, audioTo(1, uint64(2*i), speech))) // no target: nobody
		carol.conn.Write(mumble.EncodeFrame(mumble.TypeUDPTunnel, audio(uint64(2*i), speech)))
		dave.conn.Write(mumble.EncodeFrame(mumble.TypeUDPTunnel, audio(uint64(2*i), speech)))
	}
	alice.expect(mumble.TypeUDPTunnel)
	bob.expect(mumble.TypeUDPTunnel)
	time.Sleep(100 * time.Millisecond) // the last packets reach the recorder
	if err := s.StopRecording(su.session, lounge); err != nil {
		t.Fatal(err)
	}
	proto.Unmarshal(alice.expect(mumble.TypeTextMessage), &m)
	if !strings.Contains(m.GetMessage(), "Recording stopped") {
		t.Fatalf("text %v", &m)
	}
	info := lib.List()[0]
	var names []string
	for _, tr := range info.Tracks {
		names = append(names, tr.Name)
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"alice", "bob"}) {
		t.Fatalf("tracks %v", names)
	}
}

func (c *testClient) waitForSelfMute(session uint32) {
	c.t.Helper()
	for {
		var m pb.UserState
		proto.Unmarshal(c.expect(mumble.TypeUserState), &m)
		if m.GetSession() == session && m.GetSelfMute() {
			return
		}
	}
}

func TestMumbleClientsJoiningARecordedChannelAreTold(t *testing.T) {
	s, addr, _, _ := recordingServer(t, recording.Limits{})
	su := superUser(t, addr)
	lounge := su.createChannel(0, "Lounge", false)
	s.StartRecording(su.session, lounge)
	alice := dial(t, addr, "alice")
	alice.expectNone(mumble.TypeTextMessage)
	alice.move(alice.session, lounge)
	var m pb.TextMessage
	proto.Unmarshal(alice.expect(mumble.TypeTextMessage), &m)
	if !strings.Contains(m.GetMessage(), "being recorded") {
		t.Fatalf("text %v", &m)
	}
}

func TestDeletingAChannelStopsItsRecording(t *testing.T) {
	s, addr, lib, changes := recordingServer(t, recording.Limits{})
	su := superUser(t, addr)
	lounge := su.createChannel(0, "Lounge", false)
	s.StartRecording(su.session, lounge)
	su.send(mumble.TypeChannelRemove, &pb.ChannelRemove{ChannelId: proto.Uint32(lounge)})
	eventually(t, func() bool { return len(lib.List()) == 1 && changes.Load() == 2 })
	if len(s.Recording()) != 0 || lib.List()[0].Reason != "channel deleted" {
		t.Fatalf("recording %v, %+v", s.Recording(), lib.List())
	}
}

func TestARecordingAtItsLimitEnds(t *testing.T) {
	s, addr, lib, changes := recordingServer(t, recording.Limits{MaxDuration: 50 * time.Millisecond, MaxBytes: 1 << 20})
	su := superUser(t, addr)
	lounge := su.createChannel(0, "Lounge", false)
	su.move(su.session, lounge)
	s.StartRecording(su.session, lounge)
	eventually(t, func() bool { return len(s.Recording()) == 0 && changes.Load() == 2 })
	if got := lib.List(); len(got) != 1 || got[0].Reason != "time limit" {
		t.Fatalf("finished %+v", got)
	}
	var m pb.TextMessage
	proto.Unmarshal(su.expect(mumble.TypeTextMessage), &m) // started
	proto.Unmarshal(su.expect(mumble.TypeTextMessage), &m)
	if !strings.Contains(m.GetMessage(), "time limit") {
		t.Fatalf("text %v", &m)
	}
}

// Also those of channels removed just before.
func TestClosingTheServerFinishesRecordings(t *testing.T) {
	s, addr, lib, _ := recordingServer(t, recording.Limits{})
	su := superUser(t, addr)
	lounge, other := su.createChannel(0, "Lounge", false), su.createChannel(0, "Other", false)
	s.StartRecording(su.session, lounge)
	s.StartRecording(su.session, other)
	s.mu.Lock()
	s.stopRecordingsLocked([]uint32{other}, "channel deleted")
	s.mu.Unlock()
	s.Close()
	reasons := []string{}
	for _, info := range lib.List() {
		reasons = append(reasons, info.Reason)
	}
	slices.Sort(reasons)
	if !slices.Equal(reasons, []string{"channel deleted", "server shutdown"}) {
		t.Fatalf("finished %v", reasons)
	}
}
