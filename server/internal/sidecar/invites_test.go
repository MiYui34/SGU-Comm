package sidecar

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"kallisti.chat/server/internal/mumble"
)

// Test users: session = user id / 10.
func (f *fakeVoice) AddMember(channel, user uint32) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.hidden[channel] {
		return false, errors.New("not a private channel")
	}
	if f.members[user/10] {
		return false, nil
	}
	f.members[user/10] = true
	return true, nil
}

func inviteHub(t *testing.T) (h *Hub, v *fakeVoice, secret int64, staff, carol *Conn) {
	h, v, secret = privateHub(t)
	staff = joinAs(t, h, mumble.Identity{Session: 1, Name: "mod", Registered: true, UserID: 10})
	carol = joinAs(t, h, mumble.Identity{Session: 3, Name: "carol", Registered: true, UserID: 30})
	staff.limit = rate.NewLimiter(rate.Inf, 0)
	carol.limit = rate.NewLimiter(rate.Inf, 0)
	return h, v, secret, staff, carol
}

// createInvite makes a link to the secret channel's mirror and returns its
// token and the channel's links.
func createInvite(t *testing.T, staff *Conn, seconds, uses int) (string, []any) {
	t.Helper()
	staff.Handle([]byte(`{"type":"create-invite","voice":101,"seconds":` + strconv.Itoa(seconds) + `,"uses":` + strconv.Itoa(uses) + `}`))
	m := next(t, staff)
	token, _ := m["token"].(string)
	if m["type"] != "invite-created" || m["voice"] != 101.0 || len(token) < 22 {
		t.Fatalf("staff got %v", m)
	}
	return token, invitesOf(t, staff)
}

func invitesOf(t *testing.T, c *Conn) []any {
	t.Helper()
	m := next(t, c)
	if m["type"] != "invites" || m["voice"] != 101.0 {
		t.Fatalf("got %v", m)
	}
	return m["invites"].([]any)
}

func redeem(c *Conn, token string) {
	c.Handle([]byte(`{"type":"redeem-invite","token":"` + token + `"}`))
}

// notValid: the one answer to a bad link, which tells nothing.
func notValid(t *testing.T, c *Conn) {
	t.Helper()
	if m := next(t, c); m["type"] != "error" || m["message"] != "This invite is not valid any more" || len(m) != 3 {
		t.Fatalf("got %v", m)
	}
}

func joined(t *testing.T, c *Conn, secret int64) {
	t.Helper()
	if m := next(t, c); m["type"] != "invite-joined" || m["voice"] != 101.0 || m["channel"] != float64(secret) {
		t.Fatalf("got %v", m)
	}
}

func TestOnlyStaffMakeAndSeeInvites(t *testing.T) {
	_, _, _, staff, carol := inviteHub(t)
	for _, req := range []string{
		`{"type":"create-invite","voice":101,"seconds":0,"uses":0}`,
		`{"type":"invites","voice":101}`,
	} {
		carol.Handle([]byte(req))
		if m := next(t, carol); m["type"] != "error" || m["about"] != "manage" {
			t.Fatalf("%s: carol got %v", req, m)
		}
	}
	// Only for private channels, with sane limits.
	for _, req := range []string{
		`{"type":"create-invite","voice":100,"seconds":0,"uses":0}`,
		`{"type":"create-invite","voice":101,"seconds":-1,"uses":0}`,
		`{"type":"create-invite","voice":101,"seconds":0,"uses":-1}`,
	} {
		staff.Handle([]byte(req))
		if m := next(t, staff); m["type"] != "error" {
			t.Fatalf("%s: staff got %v", req, m)
		}
	}
	_, list := createInvite(t, staff, 86400, 5)
	i := list[0].(map[string]any)
	if len(list) != 1 || i["creator"] != "mod" || i["uses"] != 0.0 || i["maxUses"] != 5.0 ||
		i["expires"] != float64(clock.Add(24*time.Hour).UnixMilli()) || i["token"] != nil {
		t.Fatalf("invites %v", list)
	}
	empty(t, carol) // nothing about the channel before redeeming
}

func TestRedeemAnInvite(t *testing.T) {
	h, v, secret, staff, carol := inviteHub(t)
	token, _ := createInvite(t, staff, 0, 1)
	for _, bad := range []string{"", "nope", strings.Repeat("A", 32)} {
		redeem(carol, bad)
		notValid(t, carol)
	}
	redeem(carol, token)
	joined(t, carol, secret)
	if !v.Sees(3, 101) {
		t.Fatal("carol is not a member")
	}
	// Used up.
	bob := joinAs(t, h, mumble.Identity{Session: 2, Name: "bob", Registered: true, UserID: 20})
	redeem(bob, token)
	notValid(t, bob)
	if v.Sees(2, 101) {
		t.Fatal("bob is a member")
	}
}

// Members (and staff) just open the channel: no use counted.
func TestRedeemAsAMember(t *testing.T) {
	_, _, secret, staff, carol := inviteHub(t)
	token, _ := createInvite(t, staff, 0, 2)
	redeem(staff, token)
	joined(t, staff, secret)
	redeem(carol, token)
	joined(t, carol, secret)
	redeem(carol, token)
	joined(t, carol, secret)
	staff.Handle([]byte(`{"type":"invites","voice":101}`))
	if list := invitesOf(t, staff); len(list) != 1 || list[0].(map[string]any)["uses"] != 1.0 {
		t.Fatalf("invites %v", list)
	}
}

func TestExpiredDeletedAndPublicInvitesStopWorking(t *testing.T) {
	h, v, _, staff, carol := inviteHub(t)
	now := clock
	h.now = func() time.Time { return now }
	day, _ := createInvite(t, staff, 86400, 0)
	gone, list := createInvite(t, staff, 0, 0)
	now = now.Add(24 * time.Hour)
	redeem(carol, day)
	notValid(t, carol)

	id := strconv.Itoa(int(list[1].(map[string]any)["id"].(float64)))
	carol.Handle([]byte(`{"type":"delete-invite","invite":` + id + `}`))
	if m := next(t, carol); m["type"] != "error" {
		t.Fatalf("carol got %v", m)
	}
	staff.Handle([]byte(`{"type":"delete-invite","invite":` + id + `}`))
	if list := invitesOf(t, staff); len(list) != 0 {
		t.Fatalf("invites %v", list)
	}
	redeem(carol, gone)
	notValid(t, carol)

	// The channel is public now: its links do not work, and go.
	public, _ := createInvite(t, staff, 0, 0)
	v.SetPrivate(1, 101, false, nil)
	redeem(carol, public)
	notValid(t, carol)
	h.PrivateChanged()
	if left, _ := h.store.Invites(101, now); len(left) != 0 {
		t.Fatalf("left %v", left)
	}
}
