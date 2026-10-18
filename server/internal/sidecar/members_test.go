package sidecar

import (
	"maps"
	"testing"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/store"
)

func (f *fakeVoice) Users() map[uint32]string { return maps.Clone(f.names) }
func (f *fakeVoice) Bio(user uint32) string   { return f.bios[user] }

// membersHub has registered users alice (10), bob (20) and a guest
// account (30).
func membersHub(t *testing.T) *Hub {
	t.Helper()
	h := newHub(t, "general")
	if _, err := h.store.CreateAccount(store.Account{Name: "Guest-1", Temporary: true, CertPEM: []byte("c"), KeyPEM: []byte("k")}, clock); err != nil {
		t.Fatal(err)
	}
	h.SetVoice(&fakeVoice{names: map[uint32]string{20: "bob", 10: "alice", 30: "guest-1"}, bios: map[uint32]string{20: "<b>hi</b>"}}, "")
	return h
}

func TestReadyListsRegisteredUsersButGuestAccounts(t *testing.T) {
	h := membersHub(t)
	c := h.Connect()
	t.Cleanup(c.Close)
	c.SetIdentity(mumble.Identity{Session: 2, Name: "someone"}) // guests see it too
	m := next(t, c)
	members := m["members"].([]any)
	if len(members) != 2 {
		t.Fatalf("members %v", members)
	}
	for i, want := range []float64{10, 20} {
		if id, _ := peerOf(map[string]any{"peer": members[i]}); id != want {
			t.Fatalf("members %v", members)
		}
	}
}

func TestUserChangesReachEveryone(t *testing.T) {
	h := membersHub(t)
	c := ready(t, h, "someone")
	h.UserChanged(40, "carol")
	if m := next(t, c); m["type"] != "member" || m["id"] != float64(40) || m["name"] != "carol" {
		t.Fatalf("new user %v", m)
	}
	h.UserChanged(30, "Guest-1") // guest accounts are not listed
	empty(t, c)
	h.UserChanged(40, "")
	if m := next(t, c); m["type"] != "member" || m["id"] != float64(40) || m["gone"] != true {
		t.Fatalf("removed user %v", m)
	}
}

func TestAGuestAccountMadePermanentIsListed(t *testing.T) {
	h := membersHub(t)
	c := ready(t, h, "someone")
	acc, err := h.store.AccountByName("Guest-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.store.MakePermanent(acc.ID, "hash"); err != nil {
		t.Fatal(err)
	}
	h.AccountPermanent("Guest-1")
	if m := next(t, c); m["type"] != "member" || m["id"] != float64(30) || m["name"] != "guest-1" {
		t.Fatalf("member %v", m)
	}
	h.AccountPermanent("nobody") // no registration (yet): its first visit tells
	empty(t, c)
}

func TestBioOfARegisteredUser(t *testing.T) {
	h := membersHub(t)
	c := ready(t, h, "someone")
	c.Handle([]byte(`{"type":"bio","user":20}`))
	if m := next(t, c); m["type"] != "bio" || m["user"] != float64(20) || m["text"] != "<b>hi</b>" {
		t.Fatalf("bio %v", m)
	}
}
