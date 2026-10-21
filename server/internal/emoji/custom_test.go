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

func TestPersonalRef(t *testing.T) {
	if got := Ref("party", 12); got != "<:party:12>" {
		t.Errorf("ref %q", got)
	}
	if name, id, ok := ParseRef("<:party:12>"); !ok || name != "party" || id != 12 {
		t.Errorf("got %q %d %v", name, id, ok)
	}
	for _, s := range []string{"<:party:>", "<:party:0>", "<:party:012>", "<:Party:1>", "<:a:1>", ":party:", "<:party:12> ",
		"<:party:1234567890123456789>", "<:party:-1>"} {
		if _, _, ok := ParseRef(s); ok {
			t.Errorf("%q accepted", s)
		}
	}
	if got := Normal("<:top10:3>"); got != "<:top10:3>" {
		t.Errorf("normal %q", got)
	}
}

func TestRefsAsNames(t *testing.T) {
	got := RefsAsNames("hi <:party:12> and <:cat:3>, <b> <:x:1> <:ok:0>")
	if want := "hi :party: and :cat:, <b> <:x:1> <:ok:0>"; got != want {
		t.Errorf("got %q", got)
	}
}
