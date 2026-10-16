package mumbled

import (
	"bytes"
	"net/netip"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

// A screen share's sound reaches Mumble clients as a hidden user's voice;
// web clients never see that user.
func TestStreamSoundForMumbleClients(t *testing.T) {
	s, addr := startServerAndListen(t, "127.0.0.1", Config{}, openACL)
	web := joined(t, webClient(t, s, Origin{Host: netip.MustParseAddr("203.0.113.5")}, "streamer"))
	native := dial(t, addr, "listener")
	session, ok := s.StartStreamSound(web.session)
	if !ok {
		t.Fatal("no stream user")
	}
	st := native.waitForState(session, func(m *pb.UserState) bool { return m.GetName() == "streamer (stream)" })
	if st.GetChannelId() != 0 {
		t.Fatalf("stream user %v", st)
	}
	s.StreamSound(web.session, []byte{0xfc, 1, 2}, 1_000_000)
	h := parseAudio(t, native.expect(mumble.TypeUDPTunnel))
	if h.sender != session || h.frame != 100 || !bytes.Equal(h.opus, []byte{0xfc, 1, 2}) {
		t.Fatalf("heard %+v", h)
	}
	web.expectNone(mumble.TypeUDPTunnel)
	// A Mumble client that joins later sees it too; the web client never.
	late, _ := tryDial(t, addr, "late")
	late.waitForState(session, func(m *pb.UserState) bool { return m.GetName() == "streamer (stream)" })
	s.EndStreamSound(web.session)
	var rm pb.UserRemove
	proto.Unmarshal(native.expect(mumble.TypeUserRemove), &rm)
	if rm.GetSession() != session {
		t.Fatalf("removed %v", &rm)
	}
	timeout := time.After(300 * time.Millisecond)
	for {
		select {
		case f := <-web.frames:
			var m pb.UserState
			if f.typ == mumble.TypeUserState && proto.Unmarshal(f.body, &m) == nil && m.GetSession() == session {
				t.Fatal("the web client saw the stream user")
			}
		case <-timeout:
			return
		}
	}
}
