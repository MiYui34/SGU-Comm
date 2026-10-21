// Package emoji checks reactions and gives each emoji one form.
package emoji

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxRunes = 16 // enough for sequences (couples with skin tones, flags)

const selector = 0xFE0F // the emoji variation selector

func isTextStyle(r rune) bool {
	i := sort.Search(len(textStyle), func(i int) bool { return textStyle[i][1] >= r })
	return i < len(textStyle) && textStyle[i][0] <= r
}

func skinTone(r rune) bool { return r >= 0x1F3FB && r <= 0x1F3FF }

// symbol: a rune an emoji is made of. The emoji block counts as a whole:
// Go's Unicode tables do not know the newest emoji yet. ◽ and ◾ are the
// only other emoji that are neither So nor text style.
func symbol(r rune) bool {
	return r >= 0x1F000 && r <= 0x1FAFF || r == 0x25FD || r == 0x25FE ||
		r >= 0x80 && r != utf8.RuneError && (isTextStyle(r) || unicode.Is(unicode.So, r))
}

// Normal is the fully qualified form (as in Unicode's emoji-test.txt),
// so the same emoji is one reaction: the variation selector after symbols
// that look like text without it, except before a skin tone. Custom emoji
// (":name:", "<:name:id>") stay as they are.
func Normal(s string) string {
	if _, ok := CustomName(s); ok {
		return s
	}
	if _, _, ok := ParseRef(s); ok {
		return s
	}
	var rs []rune
	for _, r := range s {
		if r != selector && r != 0xFE0E {
			rs = append(rs, r)
		}
	}
	var b strings.Builder
	for i, r := range rs {
		b.WriteRune(r)
		if isTextStyle(r) && (i+1 == len(rs) || !skinTone(rs[i+1])) {
			b.WriteRune(selector)
		}
	}
	return b.String()
}

// keycap: "#", "*" or a digit, the variation selector, the keycap mark.
func keycap(rs []rune) bool {
	return len(rs) == 3 && strings.ContainsRune("#*0123456789", rs[0]) && rs[1] == selector && rs[2] == 0x20E3
}

// Valid (on a Normal form): symbols, with only what emoji sequences add to
// them: skin tones, the variation selector, tags (flags of regions), and
// joiners between parts; or a keycap. No letters, marks or invisible text.
func Valid(s string) bool {
	rs := []rune(s)
	if len(rs) == 0 || len(rs) > maxRunes || !utf8.ValidString(s) {
		return false
	}
	if keycap(rs) {
		return true
	}
	found := false
	for i, r := range rs {
		switch {
		case r == 0x200D: // zero-width joiner
			if i == 0 || i == len(rs)-1 || rs[i-1] == 0x200D {
				return false
			}
		case r == selector, skinTone(r), r >= 0xE0020 && r <= 0xE007F:
			if i == 0 {
				return false
			}
		case symbol(r):
			found = true
		default:
			return false
		}
	}
	return found
}
