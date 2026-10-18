package mumbled

import (
	"database/sql"
	"net/netip"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kallisti.chat/server/internal/msgrules"
	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
	"kallisti.chat/server/internal/password"
)

func TestRegistrationTimes(t *testing.T) {
	sqlDB, _ := sql.Open("sqlite", ":memory:")
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	// A database from before: its users count as old.
	sqlDB.Exec(`CREATE TABLE mumble_users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, name_key TEXT NOT NULL UNIQUE,
		cert_hash TEXT UNIQUE, password_hash TEXT NOT NULL DEFAULT '', last_channel INTEGER NOT NULL DEFAULT 0)`)
	sqlDB.Exec(`INSERT INTO mumble_users (id, name, name_key) VALUES (1, 'Old', 'old')`)
	db, err := OpenDB(sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	old, _ := db.UserByName("old")
	if old == nil || !old.Since.IsZero() {
		t.Fatalf("old %+v", old)
	}
	before := time.Now().Add(-time.Second)
	db.RegisterUser("New", "nnn")
	if n, _ := db.UserByHash("nnn"); n == nil || n.Since.Before(before) || n.Since.After(time.Now()) {
		t.Fatalf("new %+v", n)
	}
}

func TestMessageRulesAreForAdmins(t *testing.T) {
	db := testDB(t)
	hash, _ := password.Hash("admin")
	db.SetPassword(0, hash)
	s, addr := startServerAndListen(t, "127.0.0.1", Config{DB: db}, nil)
	su := superUser(t, addr)
	alice, _ := registered(t, addr, "alice")
	if _, err := s.SetMessageRules(alice.session, msgrules.Rules{Words: []string{"x"}}); err == nil {
		t.Fatal("a user changed the rules")
	}
	if _, err := s.MessageRules(alice.session); err == nil {
		t.Fatal("a user read the rules")
	}
	if _, err := s.SetMessageRules(su.session, msgrules.Rules{Domains: []string{"no domain"}}); err == nil {
		t.Fatal("saved a bad domain")
	}
	got, err := s.SetMessageRules(su.session, msgrules.Rules{NewMinutes: 10, Words: []string{" Spam "}, Domains: []string{"https://Junk.example/"}})
	if err != nil || got.NewMinutes != 10 || !slices.Equal(got.Words, []string{"Spam"}) || !slices.Equal(got.Domains, []string{"junk.example"}) {
		t.Fatalf("%+v %v", got, err)
	}
	restarted, err := New(Config{DB: db})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if r := restarted.rules; r.NewMinutes != 10 || !slices.Equal(r.Words, []string{"Spam"}) {
		t.Fatalf("not saved: %+v", r)
	}
}

func TestMessageRules(t *testing.T) {
	db := testDB(t)
	hash, _ := password.Hash("admin")
	db.SetPassword(0, hash)
	var later atomic.Int64 // the clock runs ahead by this
	s, addr := startServerAndListen(t, "127.0.0.1", Config{DB: db, GuestAccount: func(name string) bool { return name == "gina" }},
		func(s *Server) { s.now = func() time.Time { return time.Now().Add(time.Duration(later.Load())) } })
	su := superUser(t, addr)
	alice, aliceID := registered(t, addr, "alice")
	bob, _ := registered(t, addr, "bob")
	carol, _ := registered(t, addr, "carol")
	guest := dial(t, addr, "guest")
	gina := joined(t, webClient(t, s, Origin{Host: netip.MustParseAddr("203.0.113.5"), Account: &WebAccount{Name: "gina", CertHash: "gina-hash"}}, "gina"))
	if err := s.SetModerator(su.session, aliceID, true); err != nil { // staff are exempt
		t.Fatal(err)
	}
	if _, err := s.SetMessageRules(su.session, msgrules.Rules{NewMinutes: 10, Words: []string{"spam"}, Domains: []string{"junk.example"}}); err != nil {
		t.Fatal(err)
	}
	refused := func(c *testClient, text, want string) {
		t.Helper()
		c.say(0, text)
		if d := c.expectDenied(pb.PermissionDenied_Text); !strings.Contains(d.GetReason(), want) {
			t.Fatalf("%q: %q, want %q", text, d.GetReason(), want)
		}
	}
	passes := func(c *testClient, text string) {
		t.Helper()
		c.say(0, text)
		su.expect(mumble.TypeTextMessage)
	}

	// Bob and carol registered just now: new accounts; the guest and the
	// guest account are limited too. (Few texts each: Mumble's own rate.)
	refused(bob, "buy SPAM", "blocked word")
	refused(carol, "see https://junk.example", "blocked site")
	refused(bob, "see https://ok.example", "New accounts can post links after 10 minutes")
	refused(carol, `<a href="https://x.example">click</a>`, "post links")
	refused(guest, "https://ok.example", "Guests cannot post links")
	passes(guest, "hi")
	refused(guest, "hi again", "one message every 5 seconds")
	passes(gina, "hello")
	refused(gina, "hello again", "one message every 5 seconds")
	passes(alice, "spam https://junk.example") // a moderator
	passes(alice, "and again")
	if err := s.CheckMessage(bob.session, msgrules.Message{Upload: true}); err == nil {
		t.Fatal("a new account may upload")
	}
	if err := s.CheckMessage(alice.session, msgrules.Message{Upload: true, Text: "spam"}); err != nil {
		t.Fatal(err)
	}

	// Ten minutes later bob is not new; guests stay limited.
	later.Store(int64(10 * time.Minute))
	passes(bob, "see https://ok.example")
	passes(bob, "and again")
	refused(carol, `<a href="https://junk.example">click</a>`, "blocked site") // a link's target counts
	refused(guest, "https://ok.example", "Guests cannot post links")
}
