package mumbled

import "testing"

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
