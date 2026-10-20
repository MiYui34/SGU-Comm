package store

import (
	"errors"
	"slices"
	"testing"
)

func categoryNames(cs []Category) []string {
	out := []string{}
	for _, c := range cs {
		out = append(out, c.Name)
	}
	return out
}

func TestManagingCategories(t *testing.T) {
	s := open(t)
	games, err := s.CreateCategory("Games")
	if err != nil || games.Name != "Games" {
		t.Fatalf("create %+v %v", games, err)
	}
	if _, err := s.CreateCategory("games"); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("duplicate (by key): %v", err)
	}
	music, _ := s.CreateCategory("Music")
	if err := s.RenameCategory(music.ID, "Tunes"); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameCategory(music.ID, "GAMES"); !errors.Is(err, ErrNameTaken) {
		t.Fatalf("rename to a taken name: %v", err)
	}
	if err := s.RenameCategory(99, "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("rename unknown: %v", err)
	}
	if err := s.SetCategoryPositions([]int64{music.ID, games.ID}); err != nil {
		t.Fatal(err)
	}
	cats, _ := s.Categories()
	if got := categoryNames(cats); !slices.Equal(got, []string{"Tunes", "Games"}) {
		t.Fatalf("after order %v", got)
	}
	if err := s.SetCategoryMumble(games.ID, 42); err != nil {
		t.Fatal(err)
	}
	if cats, _ := s.Categories(); cats[1].Mumble != 42 {
		t.Fatalf("mumble %+v", cats[1])
	}
}

func TestChannelsInCategories(t *testing.T) {
	s := open(t)
	s.EnsureChannels([]string{"general"})
	games, _ := s.CreateCategory("Games")
	mc, err := s.CreateChannel("minecraft", games.ID)
	if err != nil || mc.Category != games.ID {
		t.Fatalf("create in a category: %+v %v", mc, err)
	}
	if _, err := s.CreateChannel("nope", 99); !errors.Is(err, ErrNotFound) {
		t.Fatalf("create in an unknown category: %v", err)
	}
	chans, _ := s.Channels()
	general := chans[0]
	if general.Category != 0 || chans[1].Category != games.ID {
		t.Fatalf("channels %+v", chans)
	}
	// A move puts the channel last in its new category.
	if err := s.SetChannelCategory(general.ID, games.ID); err != nil {
		t.Fatal(err)
	}
	chans, _ = s.Channels()
	if !slices.Equal(channelNames(chans), []string{"minecraft", "general"}) || chans[1].Category != games.ID {
		t.Fatalf("after move %+v", chans)
	}
	if err := s.SetChannelCategory(general.ID, 99); !errors.Is(err, ErrNotFound) {
		t.Fatalf("move to an unknown category: %v", err)
	}
	// Deleting a category leaves its channels uncategorized (also hidden
	// ones, which may come back).
	s.HideChannel(mc.ID)
	if err := s.DeleteCategory(games.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteCategory(games.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
	back, _ := s.CreateChannel("minecraft", 0)
	chans, _ = s.Channels()
	if cats, _ := s.Categories(); len(cats) != 0 || back.ID != mc.ID || chans[0].Category != 0 || chans[1].Category != 0 {
		t.Fatalf("after delete: %+v %+v", cats, chans)
	}
}

// Databases from before categories: every channel is uncategorized.
func TestChannelsWithoutCategoryColumn(t *testing.T) {
	path := t.TempDir() + "/old.db"
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.EnsureChannels([]string{"general"})
	for _, q := range []string{`ALTER TABLE channels DROP COLUMN category_id`, `DROP TABLE categories`} {
		if _, err := s.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	s.Close()
	if s, err = Open(path); err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if chans, err := s.Channels(); err != nil || len(chans) != 1 || chans[0].Category != 0 {
		t.Fatalf("channels %+v %v", chans, err)
	}
	if _, err := s.CreateCategory("Games"); err != nil {
		t.Fatal(err)
	}
}
