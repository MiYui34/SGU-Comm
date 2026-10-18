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
		Words:      []string{" spam ", "SPAM", "", "buy   now", "Café", "cafe"},
		Domains:    []string{"https://Spam.Example/path?q", "*.spam.example", "spam.example.", "Bücher.example", ".XYZ", "  "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"buy now", "Café", "spam"}; !slices.Equal(got.Words, want) {
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
