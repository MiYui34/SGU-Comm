package sidecar

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"kallisti.chat/server/internal/msgrules"
	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/store"
)

// threadHub: privateHub (general: mirror 100; secret: mirror 101, private)
// with staff (mod: session 1, user 10), bob (session 2, user 20, a member
// of secret) and carol (session 3, user 30, not one), all without a rate
// limit.
func threadHub(t *testing.T) (h *Hub, v *fakeVoice, staff, bob, carol *Conn) {
	t.Helper()
	h, v, _ = privateHub(t)
	v.members[2] = true
	v.moderates = map[uint32]uint32{}
	v.names = map[uint32]string{10: "mod", 20: "bob", 30: "carol", 40: "dave"}
	staff = threadUser(t, h, 1, "mod", 10)
	bob = threadUser(t, h, 2, "bob", 20)
	carol = threadUser(t, h, 3, "carol", 30)
	return h, v, staff, bob, carol
}

// threadUser connects a registered user without a rate limit.
func threadUser(t *testing.T, h *Hub, session uint32, name string, user uint32) *Conn {
	t.Helper()
	c := joinAs(t, h, mumble.Identity{Session: session, Name: name, Registered: true, UserID: user})
	c.limit = rate.NewLimiter(rate.Inf, 0)
	return c
}

// readyOf connects a user again and returns its ready event.
func readyOf(t *testing.T, h *Hub, session uint32, name string, user uint32) map[string]any {
	t.Helper()
	c := h.Connect()
	t.Cleanup(c.Close)
	c.SetIdentity(mumble.Identity{Session: session, Name: name, Registered: true, UserID: user})
	return next(t, c)
}

func do(c *Conn, format string, args ...any) { c.Handle(fmt.Appendf(nil, format, args...)) }

// postTo sends a text to a chat and returns the message id (everyone's
// copies and what came with them are dropped).
func postTo(t *testing.T, h *Hub, c *Conn, chat int64, text string) int64 {
	t.Helper()
	do(c, `{"type":"send","channel":%d,"text":%q}`, chat, text)
	m := nextOf(t, c, "message")
	drain(h.all()...)
	return int64(m["message"].(map[string]any)["id"].(float64))
}

// startThread makes a thread from a message and returns its id (the
// events are dropped).
func startThread(t *testing.T, h *Hub, c *Conn, message int64, name string) int64 {
	t.Helper()
	do(c, `{"type":"create-thread","message":%d,"name":%q}`, message, name)
	m := nextOf(t, c, "thread")
	drain(h.all()...)
	return int64(threadOf(m)["id"].(float64))
}

func refused(t *testing.T, c *Conn, format string, args ...any) map[string]any {
	t.Helper()
	do(c, format, args...)
	m := next(t, c)
	if m["type"] != "error" {
		t.Fatalf("%s: got %v", fmt.Sprintf(format, args...), m)
	}
	return m
}

func threadOf(m map[string]any) map[string]any { return m["thread"].(map[string]any) }

// ids of a list of threads (or of numbers).
func ids(list any) []int64 {
	out := []int64{}
	for _, v := range list.([]any) {
		if th, ok := v.(map[string]any); ok {
			v = th["id"]
		}
		out = append(out, int64(v.(float64)))
	}
	return out
}

func TestThreadsAreChatsOfTheirOwn(t *testing.T) {
	h, v, staff, bob, carol := threadHub(t)
	general := h.channels[0].ID
	start := postTo(t, h, bob, general, "shall we play?")
	do(bob, `{"type":"create-thread","message":%d,"name":"  Games   tonight "}`, start)
	// The creator follows it.
	if m := next(t, bob); m["type"] != "follows" || len(m["threads"].([]any)) != 1 {
		t.Fatalf("bob got %v", m)
	}
	var id int64
	for _, c := range []*Conn{bob, staff, carol} {
		m := next(t, c)
		if m["type"] != "thread" {
			t.Fatalf("got %v", m)
		}
		th := threadOf(m)
		if th["name"] != "Games tonight" || th["channel"] != float64(general) || th["start"] != float64(start) ||
			th["author"] != "bob" || th["replies"] != 0.0 || th["quote"].(map[string]any)["text"] != "shall we play?" {
			t.Fatalf("got %v", th)
		}
		id = int64(th["id"].(float64))
	}
	do(carol, `{"type":"send","channel":%d,"text":"count me in"}`, id)
	if m := next(t, carol); m["type"] != "follows" || !slices.Equal(ids(m["threads"]), []int64{id}) {
		t.Fatalf("carol got %v", m)
	}
	for _, c := range []*Conn{bob, staff, carol} {
		if m := next(t, c); m["type"] != "message" || m["message"].(map[string]any)["channel"] != float64(id) {
			t.Fatalf("got %v", m)
		}
		if m := next(t, c); m["type"] != "thread" || threadOf(m)["replies"] != 1.0 || threadOf(m)["last"] != float64(clock.UnixMilli()) {
			t.Fatalf("got %v", m)
		}
	}
	do(staff, `{"type":"history","channel":%d}`, id)
	if m := next(t, staff); m["type"] != "history" || len(m["messages"].([]any)) != 1 {
		t.Fatalf("history %v", m)
	}
	// The start message shows its thread.
	do(staff, `{"type":"history","channel":%d}`, general)
	msgs := next(t, staff)["messages"].([]any)
	if th := msgs[0].(map[string]any)["thread"].(map[string]any); th["id"] != float64(id) || th["replies"] != 1.0 || th["quote"] != nil {
		t.Fatalf("start message %v", msgs[0])
	}
	// Reactions, typing and read marks as anywhere.
	reply := postTo(t, h, staff, id, "great")
	do(bob, `{"type":"react","message":%d,"emoji":"👍","on":true}`, reply)
	if m := next(t, carol); m["type"] != "reactions" || m["channel"] != float64(id) {
		t.Fatalf("carol got %v", m)
	}
	do(carol, `{"type":"mark-read","channel":%d,"message":%d}`, id, reply)
	if mark, ok, _ := h.store.ReadMark(30, id); !ok || mark != reply {
		t.Fatalf("read mark %d %v", mark, ok)
	}
	// Mumble clients see the start message only.
	v.mu.Lock()
	defer v.mu.Unlock()
	if texts := v.toChannel[100]; len(texts) != 1 {
		t.Fatalf("Mumble got %q", texts)
	}
}

func TestThreadRightsAreTheParentChannels(t *testing.T) {
	h, v, staff, bob, carol := threadHub(t)
	general, secret := h.channels[0].ID, h.channels[1].ID
	plan := postTo(t, h, bob, secret, "the plan")
	refused(t, carol, `{"type":"create-thread","message":%d,"name":"Plan"}`, plan)
	do(bob, `{"type":"create-thread","message":%d,"name":"Plan"}`, plan)
	nextOf(t, bob, "thread")
	nextOf(t, staff, "thread")
	empty(t, carol)
	id := ids(readyOf(t, h, 2, "bob", 20)["threads"])[0]
	postTo(t, h, bob, id, "step one")
	for _, req := range []string{
		`{"type":"history","channel":%d}`,
		`{"type":"pins","channel":%d}`,
		`{"type":"send","channel":%d,"text":"hi"}`,
		`{"type":"follow-thread","channel":%d,"on":true}`,
		`{"type":"rename-thread","channel":%d,"name":"x"}`,
		`{"type":"archive-thread","channel":%d,"on":true}`,
		`{"type":"delete-thread","channel":%d}`,
	} {
		refused(t, carol, req, id)
	}
	do(carol, `{"type":"search","text":"step"}`)
	if m := next(t, carol); len(m["messages"].([]any)) != 0 {
		t.Fatalf("carol found %v", m)
	}
	do(carol, `{"type":"search","text":"","channel":%d}`, id)
	if m := next(t, carol); len(m["messages"].([]any)) != 0 {
		t.Fatalf("carol found %v", m)
	}
	if r := readyOf(t, h, 3, "carol", 30); len(r["threads"].([]any)) != 0 {
		t.Fatalf("carol's ready %v", r["threads"])
	}
	// Staff find it, with the thread.
	do(staff, `{"type":"search","text":"step"}`)
	m := next(t, staff)
	if len(m["messages"].([]any)) != 1 || !slices.Equal(ids(m["threads"]), []int64{id}) {
		t.Fatalf("staff found %v", m)
	}
	// One level, in text channels only; a name is needed.
	inThread := postTo(t, h, bob, id, "deeper")
	refused(t, bob, `{"type":"create-thread","message":%d,"name":"Deeper"}`, inThread)
	voice, _ := h.store.VoiceChat(5)
	vm, _ := h.store.AddMessage(store.NewMessage{ChannelID: voice, Author: "bob", Text: "hi", At: clock})
	refused(t, bob, `{"type":"create-thread","message":%d,"name":"Voice"}`, vm.ID)
	start := postTo(t, h, bob, general, "hello")
	refused(t, bob, `{"type":"create-thread","message":%d,"name":"   "}`, start)
	refused(t, bob, `{"type":"create-thread","message":%d,"name":%q}`, start, strings.Repeat("x", 101))
	startThread(t, h, bob, start, "Hello")
	refused(t, bob, `{"type":"create-thread","message":%d,"name":"Again"}`, start)
	// Read-only guests neither start threads nor write in them.
	v.mu.Lock()
	v.guests, v.guestsReadOnly = map[uint32]bool{4: true}, true
	v.mu.Unlock()
	guest := threadUser(t, h, 4, "guest", 40)
	other := postTo(t, h, staff, general, "another")
	if m := refused(t, guest, `{"type":"create-thread","message":%d,"name":"Mine"}`, other); !strings.Contains(m["message"].(string), "guests") {
		t.Fatalf("guest got %v", m)
	}
	refused(t, guest, `{"type":"send","channel":%d,"text":"hi"}`, id)
}

func TestThreadModeration(t *testing.T) {
	h, v, staff, bob, carol := threadHub(t)
	v.setModerates(3, 100) // carol moderates general
	dave := threadUser(t, h, 4, "dave", 40)
	general := h.channels[0].ID
	id := startThread(t, h, bob, postTo(t, h, bob, general, "topic"), "Topic")
	for _, req := range []string{
		`{"type":"rename-thread","channel":%d,"name":"Mine"}`,
		`{"type":"archive-thread","channel":%d,"on":true}`,
		`{"type":"delete-thread","channel":%d}`,
	} {
		refused(t, dave, req, id)
	}
	// Its maker renames and archives it, but does not delete it.
	do(bob, `{"type":"rename-thread","channel":%d,"name":"Better topic"}`, id)
	if m := next(t, dave); m["type"] != "thread" || threadOf(m)["name"] != "Better topic" {
		t.Fatalf("dave got %v", m)
	}
	do(bob, `{"type":"archive-thread","channel":%d,"on":true}`, id)
	if m := next(t, dave); m["type"] != "thread" || threadOf(m)["archived"] != true {
		t.Fatalf("dave got %v", m)
	}
	drain(h.all()...)
	refused(t, bob, `{"type":"delete-thread","channel":%d}`, id)
	// Not while timed out.
	v.mu.Lock()
	v.timeouts = map[uint32]time.Time{2: clock.Add(time.Hour)}
	v.mu.Unlock()
	for _, req := range []string{`{"type":"rename-thread","channel":%d,"name":"x"}`, `{"type":"archive-thread","channel":%d,"on":false}`} {
		if m := refused(t, bob, req, id); !strings.Contains(m["message"].(string), "timed out") {
			t.Fatalf("bob got %v", m)
		}
	}
	v.mu.Lock()
	v.timeouts = nil
	v.mu.Unlock()
	// The channel's moderators delete and pin messages there, and delete it.
	spam := postTo(t, h, dave, id, "spam")
	do(carol, `{"type":"pin","message":%d,"on":true}`, spam)
	if m := next(t, carol); m["type"] != "message-pinned" {
		t.Fatalf("carol got %v", m)
	}
	do(carol, `{"type":"delete-message","message":%d}`, spam)
	if m := next(t, carol); m["type"] != "message-deleted" {
		t.Fatalf("carol got %v", m)
	}
	if m := next(t, carol); m["type"] != "thread" || threadOf(m)["replies"] != 0.0 {
		t.Fatalf("carol got %v", m)
	}
	drain(h.all()...)
	do(carol, `{"type":"delete-thread","channel":%d}`, id)
	for _, c := range []*Conn{carol, bob, staff} {
		if m := next(t, c); m["type"] != "thread-deleted" || m["id"] != float64(id) || m["channel"] != float64(general) {
			t.Fatalf("got %v", m)
		}
	}
	if m := next(t, bob); m["type"] != "follows" || len(m["threads"].([]any)) != 0 {
		t.Fatalf("bob got %v", m)
	}
	refused(t, bob, `{"type":"history","channel":%d}`, id)
}

func TestSlowModeAndMessageRulesHoldInThreads(t *testing.T) {
	h, v, staff, bob, _ := threadHub(t)
	general := h.channels[0].ID
	id := startThread(t, h, bob, postTo(t, h, bob, general, "topic"), "Topic")
	h.store.SetSlowMode(general, 30)
	h.reloadChannels()
	postTo(t, h, bob, id, "first")
	// Starting a thread takes a turn in the channel.
	other := postTo(t, h, staff, general, "staff are exempt")
	if m := refused(t, bob, `{"type":"create-thread","message":%d,"name":"Too soon"}`, other); m["about"] != "slow-mode" {
		t.Fatalf("bob got %v", m)
	}
	if m := refused(t, bob, `{"type":"send","channel":%d,"text":"second"}`, id); m["about"] != "slow-mode" || m["channel"] != float64(id) {
		t.Fatalf("bob got %v", m)
	}
	v.mu.Lock()
	v.rules = msgrules.Rules{Words: []string{"spam"}}
	v.mu.Unlock()
	if m := refused(t, bob, `{"type":"send","channel":%d,"text":"spam"}`, id); m["about"] != "refused" {
		t.Fatalf("bob got %v", m)
	}
	if m := refused(t, bob, `{"type":"rename-thread","channel":%d,"name":"spam"}`, id); m["about"] != nil {
		t.Fatalf("bob got %v", m)
	}
}

func TestArchivingThreads(t *testing.T) {
	h, _, staff, bob, carol := threadHub(t)
	now := clock
	h.now = func() time.Time { return now }
	general := h.channels[0].ID
	id := startThread(t, h, bob, postTo(t, h, bob, general, "topic"), "Topic")
	now = now.Add(24 * time.Hour)
	h.ArchiveIdleThreads()
	empty(t, carol)
	// After 3 days without messages.
	now = now.Add(48*time.Hour + time.Minute)
	h.ArchiveIdleThreads()
	for _, c := range []*Conn{staff, bob, carol} {
		if m := next(t, c); m["type"] != "thread" || threadOf(m)["archived"] != true {
			t.Fatalf("got %v", m)
		}
	}
	if r := readyOf(t, h, 3, "carol", 30); len(r["threads"].([]any)) != 0 {
		t.Fatalf("active threads %v", r["threads"])
	}
	do(carol, `{"type":"threads","channel":%d}`, general)
	if m := next(t, carol); m["type"] != "archived-threads" || m["channel"] != float64(general) || !slices.Equal(ids(m["threads"]), []int64{id}) || m["more"] != false {
		t.Fatalf("carol got %v", m)
	}
	// A new message opens it again.
	do(carol, `{"type":"send","channel":%d,"text":"back"}`, id)
	nextOf(t, carol, "message")
	if m := next(t, carol); m["type"] != "thread" || threadOf(m)["archived"] != nil {
		t.Fatalf("carol got %v", m)
	}
	// Staff set the time.
	drain(h.all()...)
	if m := refused(t, bob, `{"type":"set-thread-archive","seconds":3600}`); m["about"] != "manage" {
		t.Fatalf("bob got %v", m)
	}
	refused(t, staff, `{"type":"set-thread-archive","seconds":7}`)
	do(staff, `{"type":"set-thread-archive","seconds":3600}`)
	for _, c := range []*Conn{staff, bob} {
		if m := nextOf(t, c, "thread-archive"); m["seconds"] != 3600.0 {
			t.Fatalf("got %v", m)
		}
	}
	if r := readyOf(t, h, 3, "carol", 30); r["threadArchive"] != 3600.0 {
		t.Fatalf("ready %v", r["threadArchive"])
	}
	now = now.Add(time.Hour + time.Minute)
	h.ArchiveIdleThreads()
	if m := nextOf(t, bob, "thread"); threadOf(m)["archived"] != true {
		t.Fatalf("bob got %v", m)
	}
}

// A thread by id (a link to an archived one): for its channel's readers;
// others learn no more than of a thread that does not exist.
func TestGetThread(t *testing.T) {
	h, _, staff, bob, carol := threadHub(t)
	secret := h.channels[1].ID
	id := startThread(t, h, bob, postTo(t, h, bob, secret, "the plan"), "Plan")
	do(bob, `{"type":"archive-thread","channel":%d,"on":true}`, id)
	drain(h.all()...)
	for _, c := range []*Conn{bob, staff} {
		do(c, `{"type":"get-thread","channel":%d}`, id)
		if m := next(t, c); m["type"] != "thread" || threadOf(m)["id"] != float64(id) || threadOf(m)["archived"] != true || threadOf(m)["name"] != "Plan" {
			t.Fatalf("got %v", m)
		}
	}
	empty(t, carol)
	// The same answer as for no thread (the client tells the link is not
	// here); not for other chats either.
	for _, c := range []struct {
		c  *Conn
		id int64
	}{{carol, id}, {carol, id + 100}, {bob, secret}} {
		do(c.c, `{"type":"get-thread","channel":%d}`, c.id)
		if m := next(t, c.c); m["type"] != "no-thread" || m["id"] != float64(c.id) || len(m) != 2 {
			t.Fatalf("%d: got %v", c.id, m)
		}
	}
}

func TestFollowingThreads(t *testing.T) {
	h, _, staff, bob, carol := threadHub(t)
	general := h.channels[0].ID
	id := startThread(t, h, bob, postTo(t, h, bob, general, "topic"), "Topic")
	// A mention makes carol follow it, before the message comes.
	do(staff, `{"type":"send","channel":%d,"text":"@carol look"}`, id)
	if m := next(t, carol); m["type"] != "follows" || !slices.Equal(ids(m["threads"]), []int64{id}) {
		t.Fatalf("carol got %v", m)
	}
	if m := next(t, carol); m["type"] != "message" {
		t.Fatalf("carol got %v", m)
	}
	drain(h.all()...)
	// Followers get the thread's summary (unread, mentions); others not.
	summaryOf := func(r map[string]any) map[string]any {
		for _, e := range r["summary"].([]any) {
			if e := e.(map[string]any); e["channel"] == float64(id) {
				return e
			}
		}
		return nil
	}
	r := readyOf(t, h, 3, "carol", 30)
	if e := summaryOf(r); !slices.Equal(ids(r["follows"]), []int64{id}) || e == nil || e["mentions"] != 1.0 || e["parent"] != float64(general) {
		t.Fatalf("carol's ready %v %v", r["follows"], r["summary"])
	}
	if e := summaryOf(readyOf(t, h, 4, "dave", 40)); e != nil {
		t.Fatalf("dave's summary %v", e)
	}
	do(carol, `{"type":"follow-thread","channel":%d,"on":false}`, id)
	if m := next(t, carol); m["type"] != "follows" || len(m["threads"].([]any)) != 0 {
		t.Fatalf("carol got %v", m)
	}
	if e := summaryOf(readyOf(t, h, 3, "carol", 30)); e != nil {
		t.Fatalf("carol's summary after unfollowing %v", e)
	}
	// Guests (no user id) follow nothing.
	guest := joinAs(t, h, mumble.Identity{Session: 5, Name: "guest"})
	refused(t, guest, `{"type":"follow-thread","channel":%d,"on":true}`, id)
	if got := h.where(chat{id: id, parent: general}); got != "#general › Topic" {
		t.Fatalf("where %q", got)
	}
}

func TestDeletingAroundThreads(t *testing.T) {
	h, _, staff, bob, carol := threadHub(t)
	general := h.channels[0].ID
	start := postTo(t, h, bob, general, "topic")
	id := startThread(t, h, bob, start, "Topic")
	postTo(t, h, carol, id, "reply")
	// The start message goes, the thread stays.
	do(bob, `{"type":"delete-message","message":%d}`, start)
	if m := next(t, carol); m["type"] != "message-deleted" {
		t.Fatalf("carol got %v", m)
	}
	if m := next(t, carol); m["type"] != "thread" || threadOf(m)["quote"].(map[string]any)["author"] != "" || threadOf(m)["replies"] != 1.0 {
		t.Fatalf("carol got %v", m)
	}
	do(carol, `{"type":"history","channel":%d}`, id)
	if m := next(t, carol); m["type"] != "history" || len(m["messages"].([]any)) != 1 {
		t.Fatalf("carol got %v", m)
	}
	// The channel goes: its threads with it.
	do(staff, `{"type":"delete-channel","channel":%d}`, general)
	if m := next(t, carol); m["type"] != "channels" || len(m["threads"].([]any)) != 0 {
		t.Fatalf("carol got %v", m)
	}
	refused(t, carol, `{"type":"history","channel":%d}`, id)
}
