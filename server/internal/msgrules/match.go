package msgrules

import (
	"cmp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// Matching of blocked words and replaced phrases. Each character (with its
// combining marks) is folded on its own, so that a match maps back to the
// original text: compatibility forms (full-width, mathematical letters),
// without marks on Latin, Greek and Cyrillic letters and on what is no
// letter or digit (other scripts' marks, as Japanese dakuten, make other
// letters), case-folded, common lookalikes (Cyrillic а, Greek ο, small
// capitals) as Latin letters. Invisible format characters and fillers fold
// to nothing.

// lookalikes: pairs of a lower-case letter and the Latin letter it looks
// like (Cyrillic в н к м т as small capitals: their capitals look Latin).
var lookalikes = func() map[rune]rune {
	pairs := []rune("аaвbеeкkмmнhоoрpсcтtуyхxѕsіiјjԁdһhԛqԝwӏlүy" + "αaβbεeηnιiκkνvοoρpτtυuχx" +
		"ıiɑaɡgᴀaʙbᴄcᴅdᴇeɢgʜhɪiᴊjᴋkʟlᴍmɴnᴏoᴘpʀrꜱsᴛtᴜuᴠvᴡwʏyᴢz")
	m := map[rune]rune{}
	for i := 0; i < len(pairs); i += 2 {
		m[pairs[i]] = pairs[i+1]
	}
	return m
}()

func foldChar(c string) string {
	if r, _ := utf8.DecodeRuneInString(c); unicode.Is(unicode.Cf, r) || strings.ContainsRune(fillers, r) {
		return ""
	}
	var b strings.Builder
	marks := false // marks count: after a letter or digit of another script
	for _, r := range norm.NFD.String(norm.NFKC.String(c)) {
		if !unicode.Is(unicode.M, r) {
			marks = (unicode.IsLetter(r) || unicode.IsNumber(r)) && !unicode.In(r, unicode.Latin, unicode.Greek, unicode.Cyrillic)
		} else if !marks {
			continue
		}
		b.WriteRune(r)
	}
	return norm.NFC.String(strings.Map(func(r rune) rune {
		if l, ok := lookalikes[r]; ok {
			return l
		}
		return r
	}, cases.Fold().String(b.String())))
}

// folded is text as it is matched; from and to give, for each byte of s,
// the bytes of its character in the original.
type folded struct {
	s        string
	from, to []int
}

func fold(text string) folded {
	var f folded
	var b strings.Builder
	for i := 0; i < len(text); {
		// A character and its combining marks.
		_, end := utf8.DecodeRuneInString(text[i:])
		for end += i; end < len(text); {
			r, n := utf8.DecodeRuneInString(text[end:])
			if !unicode.Is(unicode.M, r) {
				break
			}
			end += n
		}
		c := foldChar(text[i:end])
		b.WriteString(c)
		for range len(c) {
			f.from, f.to = append(f.from, i), append(f.to, end)
		}
		i = end
	}
	f.s = b.String()
	return f
}

// word: a folded word, its bytes in the original and in the folded text.
type word struct {
	key        string
	start, end int
	i, j       int
}

func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.Is(unicode.M, r)
}

// isSeparator: between single letters, these join them to a word ("a s s",
// "a.s.s", "a-s-s", "a_s_s").
func isSeparator(r rune) bool { return unicode.IsSpace(r) || strings.ContainsRune(".-_", r) }

// words are the runs of letters, digits and marks (everything else
// separates), single letters with only separators between them joined.
func (f folded) words() []word {
	var ws []word
	single := func(key string) bool {
		r, n := utf8.DecodeRuneInString(key)
		return n == len(key) && unicode.IsLetter(r)
	}
	joining := false // the last word is single letters
	gap := 0         // where the last word ended in s
	for i := 0; i < len(f.s); {
		r, n := utf8.DecodeRuneInString(f.s[i:])
		if !isWordChar(r) {
			i += n
			continue
		}
		j := i + n
		for j < len(f.s) {
			r, m := utf8.DecodeRuneInString(f.s[j:])
			if !isWordChar(r) {
				break
			}
			j += m
		}
		w := word{f.s[i:j], f.from[i], f.to[j-1], i, j}
		separated := !strings.ContainsFunc(f.s[gap:i], func(r rune) bool { return !isSeparator(r) })
		if k := len(ws) - 1; joining && single(w.key) && separated {
			ws[k].key += w.key
			ws[k].end, ws[k].j = w.end, w.j
		} else {
			ws = append(ws, w)
			joining = single(w.key)
		}
		gap, i = j, j
	}
	return ws
}

// unspaced: scripts written without spaces between words, where an entry
// matches anywhere.
func unspaced(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool {
		return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Thai, unicode.Lao, unicode.Khmer, unicode.Myanmar)
	})
}

// entry is a blocked word or a replaced phrase as matched: whole words,
// or in unspaced scripts a run anywhere.
type entry struct {
	words []string
	run   string
}

func entryOf(s string) entry {
	f := fold(s)
	var keys []string
	for _, w := range f.words() {
		keys = append(keys, w.key)
	}
	if unspaced(f.s) {
		return entry{run: strings.Join(keys, "")}
	}
	return entry{words: keys}
}

// key: entries with the same key match the same.
func key(s string) string {
	e := entryOf(s)
	return e.run + strings.Join(e.words, " ")
}

// sameWord: a word of the text is the entry's word, also with a letter
// three times or more where the entry has it fewer times ("asss" for
// "ass"; "aas" stays another word, as "too" is not "to").
func sameWord(have, want string) bool {
	// Runs in have are never shorter than in want.
	if len(have) <= len(want) || have[0] != want[0] {
		return have == want
	}
	a, b := []rune(have), []rune(want)
	runLen := func(s []rune) int {
		n := 1
		for n < len(s) && s[n] == s[0] {
			n++
		}
		return n
	}
	for len(a) > 0 && len(b) > 0 {
		if a[0] != b[0] {
			return false
		}
		na, nb := runLen(a), runLen(b)
		if na != nb && (na < 3 || nb > na || !unicode.IsLetter(a[0])) {
			return false
		}
		a, b = a[na:], b[nb:]
	}
	return len(a) == 0 && len(b) == 0
}

// match: an entry at the bytes start to end of the original text.
type match struct{ start, end, entry int }

// find gives the entries in a text, in order and not overlapping; of
// entries at one place, the longest. tight: only separators between the
// words of a phrase (else anything that is no letter or digit).
func find(entries []entry, text string, tight bool) []match {
	f := fold(text)
	ws := f.words()
	matchesAt := func(i int, e entry) bool {
		for k, want := range e.words {
			if !sameWord(ws[i+k].key, want) || tight && k > 0 && strings.ContainsFunc(f.s[ws[i+k-1].j:ws[i+k].i], func(r rune) bool { return !isSeparator(r) }) {
				return false
			}
		}
		return true
	}
	var ms []match
	for i := 0; i < len(ws); {
		best, n := 0, 0
		for k, e := range entries {
			if l := len(e.words); l > n && i+l <= len(ws) && matchesAt(i, e) {
				best, n = k, l
			}
		}
		if n == 0 {
			i++
			continue
		}
		ms = append(ms, match{ws[i].start, ws[i+n-1].end, best})
		i += n
	}
	for k, e := range entries {
		for at := 0; e.run != ""; {
			j := strings.Index(f.s[at:], e.run)
			if j < 0 {
				break
			}
			at += j + len(e.run)
			ms = append(ms, match{f.from[at-len(e.run)], f.to[at-1], k})
		}
	}
	slices.SortFunc(ms, func(a, b match) int { return cmp.Or(a.start-b.start, b.end-a.end) })
	out := ms[:0]
	for _, m := range ms {
		if len(out) == 0 || m.start >= out[len(out)-1].end {
			out = append(out, m)
		}
	}
	return out
}
