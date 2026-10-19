package mumbled

import (
	"net/netip"
	"sync/atomic"
	"testing"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

func (c *testClient) say(channel uint32, text string) {
	c.send(mumble.TypeTextMessage, &pb.TextMessage{ChannelId: []uint32{channel}, Message: proto.String(text)})
}

func (c *testClient) expectNoText() {
	c.t.Helper()
	if d := c.expectDenied(pb.PermissionDenied_Permission); d.GetPermission() != uint32(TextMessage) {
		c.t.Fatalf("denied %v", d)
	}
}

func TestGuestsReadOnly(t *testing.T) {
	db := testDB(t)
	setAdminPassword(db, "admin")
	changed := &atomic.Int32{}
	guestAccounts := map[string]bool{"gina": true, "gus": true}
	s, addr := startServerAndListen(t, "127.0.0.1", Config{
		DB:                      db,
		GuestAccount:            func(name string) bool { return guestAccounts[name] },
		OnGuestsReadOnlyChanged: func() { changed.Add(1) },
	}, nil)
	su := superUser(t, addr)
	alice, _ := registered(t, addr, "alice")
	guest := dial(t, addr, "guest")
	web := func(name string) *testClient {
		account := &WebAccount{Name: name, CertHash: name + "-hash"}
		return joined(t, webClient(t, s, Origin{Host: netip.MustParseAddr("203.0.113.5"), Account: account}, name))
	}
	gina, gus := web("gina"), web("gus")
	reg, _ := db.UserByHash("gus-hash")
	if err := s.SetModerator(su.session, reg.ID, true); err != nil { // staff are never guests
		t.Fatal(err)
	}

	// Off: guests write.
	guest.say(0, "hi")
	alice.expect(mumble.TypeTextMessage)

	if err := s.SetGuestsReadOnly(alice.session, true); err == nil {
		t.Fatal("a user changed the setting")
	}
	if err := s.SetGuestsReadOnly(su.session, true); err != nil {
		t.Fatal(err)
	}
	if !s.GuestsReadOnly() || changed.Load() != 1 {
		t.Fatalf("on %v, changed %d", s.GuestsReadOnly(), changed.Load())
	}
	for _, c := range []struct {
		name     string
		session  uint32
		readOnly bool
	}{{"guest", guest.session, true}, {"gina", gina.session, true}, {"gus", gus.session, false}, {"alice", alice.session, false}, {"su", su.session, false}} {
		if s.ReadOnly(c.session) != c.readOnly || s.MayWrite(c.session, 0) == c.readOnly {
			t.Errorf("%s: read only %v, may write %v", c.name, s.ReadOnly(c.session), s.MayWrite(c.session, 0))
		}
	}
	// Mumble guests and guest accounts get no texts out, to channels or
	// users; voice stays.
	guest.say(0, "spam")
	guest.expectNoText()
	gina.say(0, "spam")
	gina.expectNoText()
	guest.send(mumble.TypeTextMessage, &pb.TextMessage{Session: []uint32{alice.session}, Message: proto.String("spam")})
	guest.expectNoText()
	if !s.MaySpeak(guest.session, 0) {
		t.Fatal("a guest may not speak")
	}
	alice.say(0, "fine")
	guest.expect(mumble.TypeTextMessage)

	// A guest account with a password is a normal account.
	s.AccountPermanent("Gina")
	if s.ReadOnly(gina.session) {
		t.Fatal("still read only")
	}

	restarted, err := New(Config{DB: db})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if !restarted.GuestsReadOnly() {
		t.Fatal("the setting was not saved")
	}
	if err := s.SetGuestsReadOnly(su.session, false); err != nil || s.ReadOnly(guest.session) {
		t.Fatalf("off: %v", err)
	}
}

func TestCaptchaSetting(t *testing.T) {
	db := testDB(t)
	setAdminPassword(db, "admin")
	s, addr := startServerAndListen(t, "127.0.0.1", Config{DB: db}, nil)
	su := superUser(t, addr)
	alice, _ := registered(t, addr, "alice")
	if s.Captcha() != "off" {
		t.Fatalf("default %q", s.Captcha())
	}
	if err := s.SetCaptcha(alice.session, "hard"); err == nil || s.Captcha() != "off" {
		t.Fatal("a user changed the setting")
	}
	if err := s.SetCaptcha(su.session, "extreme"); err == nil {
		t.Fatal("an unknown level")
	}
	if err := s.SetCaptcha(su.session, "hard"); err != nil || s.Captcha() != "hard" {
		t.Fatalf("set: %v %q", err, s.Captcha())
	}
	restarted, err := New(Config{DB: db})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if restarted.Captcha() != "hard" {
		t.Fatal("the setting was not saved")
	}
}
