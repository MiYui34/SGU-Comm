package emoji

import (
	"strings"
	"testing"
)

func TestValidName(t *testing.T) {
	for _, s := range []string{"ok", "party_parrot", "a1", "__", strings.Repeat("x", 32)} {
		if !ValidName(s) {
			t.Errorf("%q rejected", s)
		}
	}
	for _, s := range []string{"", "a", "Party", "a-b", "a b", "ä", ":ok:", strings.Repeat("x", 33)} {
		if ValidName(s) {
			t.Errorf("%q accepted", s)
		}
	}
}

func TestCustomName(t *testing.T) {
	if name, ok := CustomName(":party1:"); !ok || name != "party1" {
		t.Errorf("got %q %v", name, ok)
	}
	for _, s := range []string{"party", ":party", ":Party:", ":a:", "::", ":ok: ", "👍"} {
		if _, ok := CustomName(s); ok {
			t.Errorf("%q accepted", s)
		}
	}
}

// Digits look like text emoji: a custom name keeps them as they are.
func TestNormalKeepsCustomNames(t *testing.T) {
	if got := Normal(":top10:"); got != ":top10:" {
		t.Errorf("got %q", got)
	}
}
