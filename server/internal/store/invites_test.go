package store

import (
	"errors"
	"testing"
	"time"
)

func TestInvites(t *testing.T) {
	s := open(t)
	t0 := time.UnixMilli(1_000_000).UTC()
	day := t0.Add(24 * time.Hour)
	once, err := s.AddInvite("h1", Invite{Channel: 7, CreatorID: 3, Creator: "mod", CreatedAt: t0, ExpiresAt: day, MaxUses: 1})
	if err != nil || once.ID == 0 {
		t.Fatalf("add: %+v %v", once, err)
	}
	forever, _ := s.AddInvite("h2", Invite{Channel: 7, CreatorID: 3, Creator: "mod", CreatedAt: t0})
	s.AddInvite("h3", Invite{Channel: 8, CreatorID: 3, Creator: "mod", CreatedAt: t0})
	if got, _ := s.Invites(7, t0); len(got) != 2 || got[0] != once || got[1].ID != forever.ID {
		t.Fatalf("invites %+v", got)
	}
	if got, err := s.InviteByHash("h1", t0); err != nil || got != once {
		t.Fatalf("by hash %+v %v", got, err)
	}
	if _, err := s.InviteByHash("nope", t0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown: %v", err)
	}

	// A use; used up, it is not valid any more.
	if err := s.UseInvite(once.ID, t0); err != nil {
		t.Fatal(err)
	}
	if err := s.UseInvite(once.ID, t0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("used twice: %v", err)
	}
	if _, err := s.InviteByHash("h1", t0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("used up: %v", err)
	}
	// Expired.
	exp, _ := s.AddInvite("h4", Invite{Channel: 7, CreatedAt: t0, ExpiresAt: day})
	if _, err := s.InviteByHash("h4", day); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired: %v", err)
	}
	if err := s.UseInvite(exp.ID, day); !errors.Is(err, ErrNotFound) {
		t.Fatalf("used when expired: %v", err)
	}
	if got, _ := s.Invites(7, day); len(got) != 1 || got[0].ID != forever.ID || got[0].Uses != 0 {
		t.Fatalf("invites %+v", got)
	}

	// Delete: it says whose it was.
	if ch, err := s.InviteChannel(forever.ID); err != nil || ch != 7 {
		t.Fatalf("channel: %v %v", ch, err)
	}
	if ch, err := s.DeleteInvite(forever.ID); err != nil || ch != 7 {
		t.Fatalf("delete: %v %v", ch, err)
	}
	if _, err := s.DeleteInvite(forever.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted twice: %v", err)
	}
	if _, err := s.InviteChannel(forever.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("channel after the delete: %v", err)
	}

	// Pruning: invalid ones and those of channels not private any more.
	s.AddInvite("h5", Invite{Channel: 9, CreatedAt: t0})
	if err := s.PruneInvites([]uint32{9}, t0); err != nil {
		t.Fatal(err)
	}
	var n int
	s.db.QueryRow(`SELECT count(*) FROM invites`).Scan(&n)
	if _, err := s.InviteByHash("h5", t0); err != nil || n != 1 {
		t.Fatalf("after pruning: %d left, %v", n, err)
	}
}
