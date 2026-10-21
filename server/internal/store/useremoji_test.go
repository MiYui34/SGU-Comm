package store

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"
)

func TestPersonalEmojiShareImages(t *testing.T) {
	s := open(t)
	t0 := time.UnixMilli(1_000_000).UTC()
	png := []byte("\x89PNG\r\n\x1a\nparty")
	a, err := s.AddUserEmoji(1, "party", "image/png", png, t0)
	if err != nil || a.ImageID == 0 || a.Name != "party" {
		t.Fatalf("add: %+v %v", a, err)
	}
	// The same image from another user is the same image.
	b, err := s.AddUserEmoji(2, "yay", "image/png", png, t0)
	if err != nil || b.ImageID != a.ImageID {
		t.Fatalf("second user: %+v %v", b, err)
	}
	if _, err := s.AddUserEmoji(1, "party", "image/gif", []byte("GIF89a"), t0); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("same name: %v", err)
	}
	if _, err := s.AddUserEmoji(1, "again", "image/png", png, t0); !errors.Is(err, ErrEmojiOwned) {
		t.Fatalf("same image twice: %v", err)
	}
	img, err := s.EmojiImage(a.ImageID)
	if err != nil || img.Type != "image/png" || !bytes.Equal(img.Data, png) || img.Name != "party" {
		t.Fatalf("image: %+v %v", img, err)
	}
	// From an image that is there (an emoji in a message).
	c, err := s.AddUserEmojiImage(3, "copy", a.ImageID, t0)
	if err != nil || c.ImageID != a.ImageID {
		t.Fatalf("by image: %+v %v", c, err)
	}
	if _, err := s.AddUserEmojiImage(3, "none", 999, t0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown image: %v", err)
	}
	list, err := s.UserEmojis(1)
	if err != nil || len(list) != 1 || list[0] != a {
		t.Fatalf("list: %+v %v", list, err)
	}
	if empty, err := s.UserEmojis(9); err != nil || empty != nil {
		t.Fatalf("nobody's: %+v %v", empty, err)
	}
}

func TestPersonalEmojiRenameAndDelete(t *testing.T) {
	s := open(t)
	t0 := time.UnixMilli(1_000_000).UTC()
	a, _ := s.AddUserEmoji(1, "party", "image/png", []byte("a"), t0)
	s.AddUserEmoji(1, "cat", "image/png", []byte("b"), t0)
	if err := s.RenameUserEmoji(1, "party", "cat"); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("rename to a taken name: %v", err)
	}
	if err := s.RenameUserEmoji(1, "nope", "dog"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("rename unknown: %v", err)
	}
	if err := s.RenameUserEmoji(1, "party", "fiesta"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteUserEmoji(1, "cat"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteUserEmoji(1, "cat"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted twice: %v", err)
	}
	list, _ := s.UserEmojis(1)
	if len(list) != 1 || list[0].Name != "fiesta" || list[0].ImageID != a.ImageID {
		t.Fatalf("list: %+v", list)
	}
	// Images stay for the messages that show them; the first name stays
	// the image's name.
	if img, err := s.EmojiImage(a.ImageID); err != nil || img.Name != "party" {
		t.Fatalf("image after rename: %+v %v", img, err)
	}
}

func TestPersonalEmojiLimits(t *testing.T) {
	s := open(t)
	t0 := time.UnixMilli(1_000_000).UTC()
	for i := range MaxUserEmoji {
		if _, err := s.AddUserEmoji(1, fmt.Sprintf("e%d", i), "image/png", []byte{byte(i)}, t0); err != nil {
			t.Fatal(i, err)
		}
	}
	if _, err := s.AddUserEmoji(1, "more", "image/png", []byte("more"), t0); !errors.Is(err, ErrTooManyEmoji) {
		t.Fatalf("over the limit: %v", err)
	}
	other, _ := s.AddUserEmoji(2, "other", "image/png", []byte("other"), t0)
	if _, err := s.AddUserEmojiImage(1, "other", other.ImageID, t0); !errors.Is(err, ErrTooManyEmoji) {
		t.Fatalf("over the limit by image: %v", err)
	}
	// Deleting makes room, but not for new images the same day: the
	// server is no free image store.
	s.DeleteUserEmoji(1, "e0")
	if _, err := s.AddUserEmoji(1, "new", "image/png", []byte("new"), t0.Add(time.Hour)); !errors.Is(err, ErrTooManyNewEmoji) {
		t.Fatalf("new images in a day: %v", err)
	}
	// Images that are there already are no new ones.
	if _, err := s.AddUserEmojiImage(1, "other", other.ImageID, t0); err != nil {
		t.Fatalf("known image: %v", err)
	}
	s.DeleteUserEmoji(1, "other")
	if _, err := s.AddUserEmoji(1, "new", "image/png", []byte("new"), t0.Add(25*time.Hour)); err != nil {
		t.Fatalf("next day: %v", err)
	}
}

func TestRemoveEmojiImageEverywhere(t *testing.T) {
	s := open(t)
	t0 := time.UnixMilli(1_000_000).UTC()
	bad := []byte("bad image")
	a, _ := s.AddUserEmoji(1, "bad", "image/png", bad, t0)
	s.AddUserEmoji(2, "worse", "image/png", bad, t0)
	s.AddUserEmoji(2, "fine", "image/png", []byte("fine"), t0)
	owners, err := s.RemoveEmojiImage(a.ImageID)
	slices.Sort(owners)
	if err != nil || fmt.Sprint(owners) != "[1 2]" {
		t.Fatalf("remove: %v %v", owners, err)
	}
	if _, err := s.RemoveEmojiImage(a.ImageID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("removed twice: %v", err)
	}
	if _, err := s.EmojiImage(a.ImageID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("image after removal: %v", err)
	}
	if list, _ := s.UserEmojis(2); len(list) != 1 || list[0].Name != "fine" {
		t.Fatalf("list after removal: %+v", list)
	}
	// The image is blocked: nobody adds it again.
	if _, err := s.AddUserEmoji(3, "bad", "image/png", bad, t0); !errors.Is(err, ErrEmojiBlocked) {
		t.Fatalf("blocked image: %v", err)
	}
	if _, err := s.AddUserEmojiImage(3, "bad", a.ImageID, t0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("removed image by id: %v", err)
	}
	removed, err := s.RemovedEmojiImages()
	if err != nil || fmt.Sprint(removed) != fmt.Sprint([]int64{a.ImageID}) {
		t.Fatalf("removed: %v %v", removed, err)
	}
	// Its name stays known (reactions with it).
	if name, removed, err := s.EmojiImageName(a.ImageID); err != nil || name != "bad" || !removed {
		t.Fatalf("name: %q %v %v", name, removed, err)
	}
	if name, removed, err := s.EmojiImageName(imageOf2(t, s)); err != nil || name != "fine" || removed {
		t.Fatalf("name of one that is there: %q %v %v", name, removed, err)
	}
	if _, _, err := s.EmojiImageName(999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown name: %v", err)
	}
}

func imageOf2(t *testing.T, s *Store) int64 {
	t.Helper()
	list, err := s.UserEmojis(2)
	if err != nil || len(list) == 0 {
		t.Fatal(list, err)
	}
	return list[0].ImageID
}
