package emoji

import (
	"os"
	"regexp"
	"strconv"
	"testing"
)

func TestNormalIsTheFullyQualifiedForm(t *testing.T) {
	for in, want := range map[string]string{
		"❤": "❤️", "❤️": "❤️", "👍️": "👍", "✅️": "✅", "🕳": "🕳️",
		"✋️🏻": "✋🏻", "⛹🏽‍♂": "⛹🏽‍♂️", "😶‍🌫": "😶‍🌫️",
		"👁‍🗨": "👁️‍🗨️", "#⃣": "#️⃣", "ℹ︎": "ℹ️", "🎓️": "🎓",
	} {
		if got := Normal(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

func TestValid(t *testing.T) {
	for _, s := range []string{"❤️", "👍🏽", "👨‍👩‍👧", "🇩🇪", "🫩", "🫈", "ℹ️", "‼️", "↔️",
		"〰️", "🙂‍↔️", "#️⃣", "7️⃣", "✋🏻", "©️", "◽", "◾"} {
		if !Valid(Normal(s)) {
			t.Errorf("%q rejected", s)
		}
	}
	for _, s := range []string{"", "lol", "#", "7", "7️", "a⃣", "7⃣👍", "+", "�", "\xe2", "👍‍", "‍★", "★ПРИВЕТ"} {
		if Valid(Normal(s)) {
			t.Errorf("%q accepted", s)
		}
	}
}

// The web app normalizes with the same list.
func TestWebHasTheSameList(t *testing.T) {
	src, err := os.ReadFile("../../../apps/web/src/lib/emojiText.ts")
	if err != nil {
		t.Fatal(err)
	}
	pairs := regexp.MustCompile(`\[(0x[0-9A-F]+), (0x[0-9A-F]+)\]`).FindAllStringSubmatch(string(src), -1)
	if len(pairs) != len(textStyle) {
		t.Fatalf("%d ranges in the web app, %d here", len(pairs), len(textStyle))
	}
	for i, p := range pairs {
		a, _ := strconv.ParseInt(p[1], 0, 32)
		b, _ := strconv.ParseInt(p[2], 0, 32)
		if rune(a) != textStyle[i][0] || rune(b) != textStyle[i][1] {
			t.Errorf("range %d: %v, want %v", i, p[1:], textStyle[i])
		}
	}
}
