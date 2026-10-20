package mumbled

import (
	"testing"
	"time"
)

// Push notifications ask about registered users who are not connected.
func TestOfflineAccess(t *testing.T) {
	s, addr := rolesServer(t)
	su := superUser(t, addr)
	open := su.createChannel(0, "Open", false)
	club := su.createChannel(0, "Club", false)
	mod, _, modID := webUser(t, s, "mod")
	s.SetModerator(su.session, modID, true)
	_, _, aliceID := webUser(t, s, "alice")
	bob, _, bobID := webUser(t, s, "bob")
	if err := s.SetPrivate(mod.session, club, true, []uint32{aliceID}); err != nil {
		t.Fatal(err)
	}
	bob.conn.Close() // gone: still known by id
	for _, c := range []struct {
		user, channel uint32
		want          bool
	}{
		{bobID, open, true}, {bobID, club, false}, {aliceID, club, true}, {modID, club, true},
		{bobID, 999, false}, {12345, club, false},
	} {
		if got := s.UserSees(c.user, c.channel); got != c.want {
			t.Errorf("UserSees(%d, %d) = %v", c.user, c.channel, got)
		}
		if got := s.UserMayRead(c.user, c.channel); got != c.want {
			t.Errorf("UserMayRead(%d, %d) = %v", c.user, c.channel, got)
		}
	}
}

// A banned user gets no pushes: by its certificate (an address ban cannot
// apply to a user who is not connected).
func TestUserBanned(t *testing.T) {
	s, _ := rolesServer(t)
	alice, _, aliceID := webUser(t, s, "alice")
	_, _, bobID := webUser(t, s, "bob")
	_, _, carolID := webUser(t, s, "carol")
	alice.conn.Close()
	old := time.Now().Add(-2 * time.Hour)
	s.bans.Store(&[]BanEntry{{Hash: "alice-hash"}, {Hash: "carol-hash", Start: old, Duration: time.Hour}})
	for id, want := range map[uint32]bool{aliceID: true, bobID: false, carolID: false, 12345: false} {
		if got := s.UserBanned(id); got != want {
			t.Errorf("UserBanned(%d) = %v", id, got)
		}
		if got := s.UserMayRead(id, 0); got == want && id != 12345 {
			t.Errorf("UserMayRead(%d, root) = %v", id, got)
		}
	}
}
