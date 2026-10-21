package msgrules

import (
	"slices"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

func TestBlockedWords(t *testing.T) {
	r := Rules{Words: []string{"spam", "buy now", "Straße", "café", "広告"}}
	old := Author{}
	for _, c := range []struct {
		text    string
		blocked bool
	}{
		{"this is spam", true},
		{"SPAM!", true},
		{"Spam, spam and eggs", true},
		{"spammer", false},      // whole words only
		{"antispam", false},     // whole words only
		{"spam_bot here", true}, // _ separates words (markdown)
		{"**spam**", true},      // markdown
		{"s\u200bpam", true},    // zero-width characters are left out
		{"sp\u3164am", true},    // so are fillers that count as letters
		{"x sp\u0640am", true},  // and the Arabic tatweel
		{"ｓｐａｍ", true},          // full-width letters
		{"ＳＰＡＭ", true},          // full-width capitals
		{"buy now!", true},      // a phrase
		{"BUY   now", true},     // any space between its words
		{"buy\nnow", true},      // a line break too
		{"buy something now", false},
		{"now buy now", true}, // a repeated first word
		{"strasse", true},     // case folding: ß is ss
		{"STRASSE", true},
		{"cafe", true}, // accents of Latin letters do not count
		{"CAFÉ", true},
		{"cafe\u0301", true}, // combining accent
		{"これは広告です", true},    // scripts without spaces: any place
		{"s p a m", true},    // spaced letters
		{"s.p.a.m!", true},
		{"s-p-a-m", true},
		{"s_p_a_m", true},
		{"S . P . A . M", true},
		{"ѕраm", true},   // Cyrillic lookalikes
		{"ЅΡΑМ", true},   // Cyrillic and Greek capitals
		{"𝐬𝐩𝐚𝐦", true},   // mathematical letters
		{"spaaam", true}, // a letter three times or more
		{"SPAMMMM", true},
		{"spaam", false}, // twice is another word
		{"sp am", false}, // only single letters are joined
		{"a spam", true}, // a single letter before a word stays a word
		{"広い", false},
		{"", false},
	} {
		err := Check(r, old, Message{Text: c.text}, now, time.Time{})
		if (err != nil) != c.blocked {
			t.Errorf("%q: %v, want blocked %v", c.text, err, c.blocked)
		}
		if err != nil && err != ErrWord {
			t.Errorf("%q: %v", c.text, err)
		}
	}
}

// Lookalikes match in either case.
func TestLookalikeCase(t *testing.T) {
	for _, c := range [][2]string{{"нах", "НАХ"}, {"νυ", "ΝΥ"}, {"bot", "ВОТ"}, {"bot", "вот"}} {
		if Check(Rules{Words: []string{c[0]}}, Author{}, Message{Text: c[1]}, now, time.Time{}) == nil {
			t.Errorf("%q does not block %q", c[0], c[1])
		}
	}
}

func TestNoFalseHits(t *testing.T) {
	r := Rules{Words: []string{"ass", "nice ass"}}
	for _, text := range []string{"class", "assistant", "passage", "bass", "Assam", "as", "a ss", "as s", "glass. a sandwich", "нас"} {
		if err := Check(r, Author{}, Message{Text: text}, now, time.Time{}); err != nil {
			t.Errorf("%q: %v", text, err)
		}
	}
	// "-" separates words: "ass-istant" has the word "ass".
	if Check(r, Author{}, Message{Text: "classic ass-istant"}, now, time.Time{}) == nil {
		t.Error("ass-istant")
	}
}

func TestBlockedDomains(t *testing.T) {
	r := Rules{Domains: []string{"spam.example", "bücher.example", "xyz"}}
	for _, c := range []struct {
		text    string
		blocked bool
	}{
		{"see https://spam.example/x", true},
		{"spam.example", true},
		{"SPAM.EXAMPLE", true},
		{"https://www.spam.example", true}, // subdomains
		{"https://a.b.spam.example:8080/", true},
		{"notspam.example", false},                // not a subdomain
		{"https://spam.example.org", false},       // another domain
		{"[click](https://spam.example/x)", true}, // markdown link
		{"<https://spam.example>", true},
		{"https://good.example@spam.example/", true},
		{"spam。example", true}, // ideographic full stop
		{"spam\u200b.example", true},
		{"ｓｐａｍ．ｅｘａｍｐｌｅ", true},
		{"https://BÜCHER.example/", true},        // internationalized
		{"https://xn--bcher-kva.example/", true}, // its ASCII form
		{"https://buy.cheap.xyz", true},          // a top-level domain
		{"https://spam%2Eexample/", true},        // percent-encoded
		{"https://\u05d0a.spam.example/", true},  // not a valid IDNA host
		{"xyz", false},                           // a word, not a host
		{"spam example", false},
	} {
		err := Check(r, Author{}, Message{Text: c.text}, now, time.Time{})
		if (err != nil) != c.blocked {
			t.Errorf("%q: %v, want blocked %v", c.text, err, c.blocked)
		}
		if err != nil && err != ErrDomain {
			t.Errorf("%q: %v", c.text, err)
		}
	}
}

func TestStaffAreExempt(t *testing.T) {
	r := Rules{NewMinutes: 10, Words: []string{"spam"}, Domains: []string{"spam.example"}}
	staff := Author{Staff: true, Since: now}
	for _, m := range []Message{{Text: "spam"}, {Text: "spam.example"}, {Text: "https://x.example"}, {Upload: true}, {MentionAll: true}, {Text: "hi", Post: true}} {
		if err := Check(r, staff, m, now, now); err != nil {
			t.Errorf("%+v: %v", m, err)
		}
	}
}

func TestNewAccountLimits(t *testing.T) {
	r := Rules{NewMinutes: 10}
	newAcc := Author{Since: now.Add(-9 * time.Minute)}
	oldAcc := Author{Since: now.Add(-10 * time.Minute)}
	guest := Author{Guest: true}
	beforeRules := Author{} // registered before registration times were kept
	cases := []struct {
		name string
		m    Message
		last time.Time
		want string // part of the error, "" for none
	}{
		{"link", Message{Text: "see https://x.example"}, time.Time{}, "post links after 10 minutes"},
		{"www", Message{Text: "see www.x.example"}, time.Time{}, "post links"},
		{"markdown link", Message{Text: "[x](http://x.example)"}, time.Time{}, "post links"},
		{"bare domain", Message{Text: "x.example"}, time.Time{}, ""},
		{"upload", Message{Upload: true}, time.Time{}, "upload files after 10 minutes"},
		{"everyone", Message{Text: "@everyone", MentionAll: true}, time.Time{}, "@everyone"},
		{"rate", Message{Text: "hi", Post: true}, now.Add(-4 * time.Second), "one message every 5 seconds"},
		{"rate ok", Message{Text: "hi", Post: true}, now.Add(-5 * time.Second), ""},
		{"edits have no rate", Message{Text: "hi"}, now, ""},
		{"plain", Message{Text: "hello", Post: true}, time.Time{}, ""},
	}
	for _, c := range cases {
		err := Check(r, newAcc, c.m, now, c.last)
		if c.want == "" && err != nil || c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)) {
			t.Errorf("new, %s: %v, want %q", c.name, err, c.want)
		}
		if c.want != "" && !Limited(r, newAcc, now) {
			t.Errorf("new account not limited")
		}
		err = Check(r, guest, c.m, now, c.last)
		if (err != nil) != (c.want != "") || err != nil && !strings.HasPrefix(err.Error(), "Guests") {
			t.Errorf("guest, %s: %v", c.name, err)
		}
		for _, a := range []Author{oldAcc, beforeRules} {
			if err := Check(r, a, c.m, now, c.last); err != nil {
				t.Errorf("old account, %s: %v", c.name, err)
			}
		}
		if err := Check(Rules{}, guest, c.m, now, c.last); err != nil {
			t.Errorf("off, %s: %v", c.name, err)
		}
	}
	if Limited(r, oldAcc, now) || Limited(r, beforeRules, now) || !Limited(r, guest, now) || Limited(Rules{}, guest, now) {
		t.Fatal("Limited")
	}
}

func TestNormalize(t *testing.T) {
	got, err := Normalize(Rules{
		NewMinutes: 15,
		Words:      []string{" spam ", "SPAM", "", "buy   now", "Café", "cafe", "don't", "e-mail"},
		Domains:    []string{"https://Spam.Example/path?q", "*.spam.example", "spam.example.", "Bücher.example", ".XYZ", "  "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"buy now", "Café", "don't", "e-mail", "spam"}; !slices.Equal(got.Words, want) {
		t.Errorf("words %q, want %q", got.Words, want)
	}
	if want := []string{"bücher.example", "spam.example", "xyz"}; !slices.Equal(got.Domains, want) {
		t.Errorf("domains %q, want %q", got.Domains, want)
	}
	if got.NewMinutes != 15 {
		t.Errorf("minutes %d", got.NewMinutes)
	}
	empty, err := Normalize(Rules{})
	if err != nil || empty.Words == nil || empty.Domains == nil {
		t.Errorf("empty: %+v %v (lists, never null in JSON)", empty, err)
	}
	for _, bad := range []Rules{
		{NewMinutes: -1},
		{NewMinutes: MaxNewMinutes + 1},
		{Words: []string{"!!!"}},
		{Words: []string{"c++"}}, // would block every "c"
		{Words: []string{"f*ck"}},
		{Words: []string{strings.Repeat("a", maxEntry+1)}},
		{Words: make([]string, maxEntries+1)},
		{Domains: []string{"not a domain"}},
		{Domains: []string{"https://"}},
	} {
		if _, err := Normalize(bad); err == nil {
			t.Errorf("%+v: no error", bad)
		}
	}
}

// picks returns the given indexes in turn (a seeded random source).
func picks(i ...int) func(int) int {
	return func(n int) int {
		v := i[0] % n
		i = append(i[1:], i[0])
		return v
	}
}

func TestReplace(t *testing.T) {
	r := Rules{Replacements: []Replacement{
		{Phrase: "nice ass", With: []string{"beautiful behind", "lovely posterior"}},
		{Phrase: "ass", With: []string{"behind"}},
		{Phrase: "広告", With: []string{"宣伝"}},
		{Phrase: "12", With: []string{"twelve"}},
	}}
	for _, c := range []struct{ text, want string }{
		{"what a nice ass", "what a beautiful behind"},
		{"Nice ass!", "Beautiful behind!"}, // a capital stays
		{"NICE ASS", "BEAUTIFUL BEHIND"},   // so do capitals
		{"nice   ass and ass", "beautiful behind and behind"},
		{"ass ass", "behind behind"},
		{"a s s", "behind"},
		{"a.s.s.", "behind."},
		{"a-s-s", "behind"},
		{"a_s_s", "behind"},
		{"аss", "behind"},       // Cyrillic а
		{"ａｓｓ", "behind"},       // full-width
		{"𝐚𝐬𝐬", "behind"},       // mathematical
		{"àss", "behind"},       // accents
		{"a\u200bss", "behind"}, // invisible characters
		{"asss", "behind"},      // a letter three times
		{"kick-ass", "kick-behind"},
		{"||nice ass||", "||beautiful behind||"},
		{"**ass**", "**behind**"},
		{"nice\na s s", "beautiful behind"},
		{"これは広告です", "これは宣伝です"},
		{"class assistant passage bass glass", "class assistant passage bass glass"},
		// Not in links, code, mentions, channels and emoji.
		{"https://x.example/nice-ass ass", "https://x.example/nice-ass behind"},
		{"`nice ass` ```\nass\n``` :nice_ass: @ass #ass", "`nice ass` ```\nass\n``` :nice_ass: @ass #ass"},
		{"<:nice_ass:12> 12", "<:nice_ass:12> twelve"}, // a personal emoji
		// Only spaces, dots and dashes between the words of a phrase:
		// markup stays.
		{"**nice** ass", "**nice** behind"},
		{"||nice|| ass ||secret||", "||nice|| behind ||secret||"},
		{"nice - ass", "beautiful behind"},
		{"", ""},
	} {
		if got := Replace(r, c.text, picks(0)); got != c.want {
			t.Errorf("%q: %q, want %q", c.text, got, c.want)
		}
	}
	// One of the replacements at random, for each match.
	if got := Replace(r, "nice ass, nice ass", picks(1, 0)); got != "lovely posterior, beautiful behind" {
		t.Errorf("random: %q", got)
	}
	if got := Replace(Rules{}, "nice ass", picks(0)); got != "nice ass" {
		t.Errorf("no rules: %q", got)
	}
}

func TestNormalizeReplacements(t *testing.T) {
	got, err := Normalize(Rules{Replacements: []Replacement{
		{Phrase: " nice   ass ", With: []string{" beautiful  behind ", "", "lovely posterior", "beautiful behind"}},
		{Phrase: "Nice a s s", With: []string{"twice"}}, // the same phrase
		{Phrase: "butt", With: []string{"🍑", "it's a bottom, really!"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := []Replacement{
		{Phrase: "butt", With: []string{"🍑", "it's a bottom, really!"}},
		{Phrase: "nice ass", With: []string{"beautiful behind", "lovely posterior"}},
	}
	if !slices.EqualFunc(got.Replacements, want, func(a, b Replacement) bool { return a.Phrase == b.Phrase && slices.Equal(a.With, b.With) }) {
		t.Errorf("%q, want %q", got.Replacements, want)
	}
	if empty, _ := Normalize(Rules{}); empty.Replacements == nil {
		t.Error("nil list")
	}
	// Plain text only: nothing a message would format, link or mention.
	for _, with := range []string{"", "**bold**", "_x_", "~~x~~", "`x`", "||x||", "@everyone", "#general", ":smile:", "<b>", "a > b", "https://x.example", "a | b", strings.Repeat("a", maxEntry+1)} {
		if _, err := Normalize(Rules{Replacements: []Replacement{{Phrase: "x", With: []string{with}}}}); err == nil {
			t.Errorf("%q: no error", with)
		}
	}
	for _, bad := range []Replacement{{Phrase: "x"}, {Phrase: "c++", With: []string{"y"}}, {Phrase: "", With: []string{"y"}}} {
		if _, err := Normalize(Rules{Replacements: []Replacement{bad}}); err == nil {
			t.Errorf("%+v: no error", bad)
		}
	}
}
