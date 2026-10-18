// Package msgrules has the server's message rules: blocked words and link
// domains (for everyone but staff), and limits for new accounts and guests.
// Pure functions: the built-in server keeps the rules, the registration
// times and the last message times.
package msgrules

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/idna"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// Rules are saved by the server and changed by admins.
type Rules struct {
	// Accounts younger than this (and guests) are limited; 0: off.
	NewMinutes int      `json:"newMinutes"`
	Words      []string `json:"words"`   // words and phrases
	Domains    []string `json:"domains"` // with their subdomains
}

const (
	MaxNewMinutes = 7 * 24 * 60
	maxEntries    = 500
	maxEntry      = 100 // characters
	// Limited authors send one message per Interval.
	Interval = 5 * time.Second
)

// Author is who writes.
type Author struct {
	Staff bool
	Guest bool // no account, or a guest account
	// The registration; older than NewMinutes (or zero: from before the
	// rules) is not new.
	Since time.Time
}

// Message is what an author wants to post.
type Message struct {
	Text       string
	Upload     bool // a file or picture
	MentionAll bool // @everyone or @here
	Post       bool // a new message (edits and uploads have no rate)
}

var (
	ErrWord   = errors.New("This message contains a blocked word")
	ErrDomain = errors.New("This message links to a blocked site")
)

// Limited: the author is new or a guest while the limits are on.
func Limited(r Rules, a Author, now time.Time) bool {
	return r.NewMinutes > 0 && !a.Staff && (a.Guest || now.Sub(a.Since) < time.Duration(r.NewMinutes)*time.Minute)
}

// Check says why a message is refused (nil: it is not). last: the author's
// last message (zero: none), for the rate of limited authors.
func Check(r Rules, a Author, m Message, now, last time.Time) error {
	if a.Staff {
		return nil
	}
	if blockedWord(r.Words, m.Text) {
		return ErrWord
	}
	if blockedDomain(r.Domains, m.Text) {
		return ErrDomain
	}
	if !Limited(r, a, now) {
		return nil
	}
	refuse := func(what string) error {
		if a.Guest {
			return fmt.Errorf("Guests cannot %s: log in or make an account", what)
		}
		return fmt.Errorf("New accounts can %s after %d minutes", what, r.NewMinutes)
	}
	switch {
	case m.Upload:
		return refuse("upload files")
	case linkPattern.MatchString(clean(m.Text)):
		return refuse("post links")
	case m.MentionAll:
		return refuse("mention @everyone or @here")
	case m.Post && now.Sub(last) < Interval:
		who := "New accounts"
		if a.Guest {
			who = "Guests"
		}
		return fmt.Errorf("%s can send one message every %d seconds", who, int(Interval/time.Second))
	}
	return nil
}

// Normalize checks rules from an admin: entries trimmed, without
// duplicates (as matched), sorted; domains without scheme or path. Lists
// are never nil (JSON lists).
func Normalize(r Rules) (Rules, error) {
	if r.NewMinutes < 0 || r.NewMinutes > MaxNewMinutes {
		return Rules{}, fmt.Errorf("new accounts: 0 to %d minutes", MaxNewMinutes)
	}
	if len(r.Words) > maxEntries || len(r.Domains) > maxEntries {
		return Rules{}, fmt.Errorf("at most %d words and %d domains", maxEntries, maxEntries)
	}
	out := Rules{NewMinutes: r.NewMinutes, Words: []string{}, Domains: []string{}}
	seen := map[string]bool{}
	for _, w := range r.Words {
		w = strings.Join(strings.Fields(w), " ")
		key := fold(w)
		switch {
		case w == "" || seen[key]:
			continue
		case utf8.RuneCountInString(w) > maxEntry:
			return Rules{}, fmt.Errorf("a blocked word has at most %d characters", maxEntry)
		case !wordEntry.MatchString(w):
			return Rules{}, fmt.Errorf("%q: use words of letters and digits (with ' - . _ between them)", w)
		}
		seen[key] = true
		out.Words = append(out.Words, w)
	}
	for _, d := range r.Domains {
		if strings.TrimSpace(d) == "" {
			continue
		}
		ascii, ok := domain(d)
		if !ok {
			return Rules{}, fmt.Errorf("%q is not a domain", strings.TrimSpace(d))
		}
		if !seen["\x00"+ascii] {
			seen["\x00"+ascii] = true
			show, _ := idna.ToUnicode(ascii)
			out.Domains = append(out.Domains, show)
		}
	}
	slices.SortFunc(out.Words, func(a, b string) int { return strings.Compare(fold(a), fold(b)) })
	slices.Sort(out.Domains)
	return out, nil
}

// clean takes out what hides text without changing it: invisible format
// characters (zero-width spaces) and fillers (Hangul fillers, the Arabic
// tatweel), compatibility forms (full-width letters), ideographic full
// stops (dots in domains, as for browsers).
func clean(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) || strings.ContainsRune(fillers, r) {
			return -1
		}
		return r
	}, s)
	return strings.ReplaceAll(norm.NFKC.String(s), "。", ".")
}

// fold is text as words are compared: clean, case-folded, without the
// accents of Latin, Greek and Cyrillic letters (other scripts' marks, as
// Japanese dakuten, make other letters).
func fold(s string) string {
	var b strings.Builder
	accented := false
	for _, r := range norm.NFD.String(clean(s)) {
		if unicode.Is(unicode.Mn, r) && accented {
			continue
		}
		accented = unicode.In(r, unicode.Latin, unicode.Greek, unicode.Cyrillic)
		b.WriteRune(r)
	}
	return norm.NFC.String(cases.Fold().String(b.String()))
}

// words splits folded text at everything but letters, digits and marks.
func words(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.Is(unicode.M, r)
	})
}

// unspaced: scripts written without spaces between words, where a blocked
// word matches anywhere.
func unspaced(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool {
		return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Thai, unicode.Lao, unicode.Khmer, unicode.Myanmar)
	})
}

// blockedWord: the text has a blocked word or phrase as whole words.
func blockedWord(blocked []string, text string) bool {
	if len(blocked) == 0 {
		return false
	}
	folded := fold(text)
	have := words(folded)
	for _, b := range blocked {
		key := fold(b)
		if unspaced(key) {
			if strings.Contains(folded, strings.Join(words(key), "")) {
				return true
			}
			continue
		}
		want := words(key)
		for i := 0; len(want) > 0 && i+len(want) <= len(have); i++ {
			if slices.Equal(have[i:i+len(want)], want) {
				return true
			}
		}
	}
	return false
}

const fillers = "\u0640\u115f\u1160\u3164\uffa0"

var (
	// Blocked words: words of letters, digits and marks, with spaces or
	// one of ' - . _ between them (other symbols would split a word, so
	// "c++" would block every "c").
	wordEntry = regexp.MustCompile(`^[\p{L}\p{N}\p{M}]+(?:(?: |['’._-])[\p{L}\p{N}\p{M}]+)*$`)
	// Percent-encoded bytes (browsers decode them in hosts).
	percent = regexp.MustCompile(`%[0-9a-fA-F]{2}`)
	// Hosts in text: labels with dots between them.
	hostPattern = regexp.MustCompile(`[\p{L}\p{N}\p{M}-]+(?:\.[\p{L}\p{N}\p{M}-]+)+`)
	asciiHost   = regexp.MustCompile(`^[a-z0-9-]+(?:\.[a-z0-9-]+)*$`)
	// What clients make links of: a scheme, or www.
	linkPattern = regexp.MustCompile(`(?i)[a-z][a-z0-9+.-]*://|(?:^|[^\p{L}\p{N}.-])www\.`)
)

// domain is a host (or URL) in its ASCII form, lower case.
func domain(s string) (string, bool) {
	s = strings.TrimSpace(clean(s))
	if _, rest, ok := strings.Cut(s, "://"); ok {
		s = rest
	}
	s, _, _ = strings.Cut(s, "/")
	s, _, _ = strings.Cut(s, "?")
	s, _, _ = strings.Cut(s, "#")
	if i := strings.LastIndex(s, "@"); i >= 0 {
		s = s[i+1:]
	}
	s, _, _ = strings.Cut(s, ":")
	s = strings.Trim(strings.TrimPrefix(s, "*."), ".")
	ascii, err := idna.Lookup.ToASCII(s)
	if err != nil {
		ascii = strings.ToLower(s)
	}
	return ascii, asciiHost.MatchString(ascii)
}

// blockedDomain: a host in the text is a blocked domain or below one
// (also percent-encoded, or not valid: browsers may still open it).
func blockedDomain(blocked []string, text string) bool {
	if len(blocked) == 0 {
		return false
	}
	decoded := percent.ReplaceAllStringFunc(text, func(p string) string {
		b, _ := strconv.ParseUint(p[1:], 16, 8)
		return string([]byte{byte(b)}) // bytes in a row make UTF-8 again
	})
	for _, h := range hostPattern.FindAllString(clean(text+" "+decoded), -1) {
		host, _ := domain(h)
		for _, b := range blocked {
			if d, ok := domain(b); ok && (host == d || strings.HasSuffix(host, "."+d)) {
				return true
			}
		}
	}
	return false
}
