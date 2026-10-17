package store

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestCustomEmoji(t *testing.T) {
	s := open(t)
	t0 := time.UnixMilli(1_000_000).UTC()
	png := []byte("\x89PNG\r\n\x1a\nimage")
	e, err := s.AddEmoji(Emoji{Name: "party", Type: "image/png", Data: png, Author: "alice"}, t0)
	if err != nil || e.ID == 0 || !e.CreatedAt.Equal(t0) {
		t.Fatalf("add: %+v %v", e, err)
	}
	if _, err := s.AddEmoji(Emoji{Name: "party", Type: "image/png", Data: png, Author: "bob"}, t0); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("same name: %v", err)
	}
	s.AddEmoji(Emoji{Name: "cat", Type: "image/gif", Data: []byte("GIF89a"), Author: "bob"}, t0)
	// The list (by name) has no images.
	list, err := s.Emojis()
	if err != nil || len(list) != 2 || list[0].Name != "cat" || list[1].Name != "party" || list[1].Data != nil || list[1].Author != "alice" {
		t.Fatalf("list: %+v %v", list, err)
	}
	got, err := s.EmojiByID(e.ID)
	if err != nil || got.Type != "image/png" || !bytes.Equal(got.Data, png) {
		t.Fatalf("by id: %+v %v", got, err)
	}
	if err := s.DeleteEmoji("party"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteEmoji("party"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted twice: %v", err)
	}
	if _, err := s.EmojiByID(e.ID); err == nil {
		t.Fatal("image of a deleted emoji")
	}
	// A new emoji never gets an old one's id (its URL is cached).
	again, _ := s.AddEmoji(Emoji{Name: "party", Type: "image/png", Data: png, Author: "alice"}, t0)
	if again.ID <= e.ID {
		t.Fatalf("id %d reused (was %d)", again.ID, e.ID)
	}
}

func TestCustomEmojiLimit(t *testing.T) {
	s := open(t)
	for i := range MaxEmoji {
		if _, err := s.AddEmoji(Emoji{Name: fmt.Sprintf("e%d", i), Type: "image/png", Data: []byte("x")}, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.AddEmoji(Emoji{Name: "more", Type: "image/png", Data: []byte("x")}, time.Now()); !errors.Is(err, ErrTooManyEmoji) {
		t.Fatalf("over the limit: %v", err)
	}
}
