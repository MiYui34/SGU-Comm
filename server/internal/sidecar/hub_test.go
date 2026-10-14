package sidecar

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/store"
)

var clock = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

func newHub(t *testing.T, channels ...string) *Hub {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.EnsureChannels(channels); err != nil {
		t.Fatal(err)
	}
	return NewHub(s, func() time.Time { return clock })
}

// next returns the next outgoing message of a connection, decoded.
func next(t *testing.T, c *Conn) map[string]any {
	t.Helper()
	select {
	case raw := <-c.Outbox():
		var v map[string]any
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		return v
	case <-time.After(time.Second):
		t.Fatal("no message")
		return nil
	}
}

func empty(t *testing.T, c *Conn) {
	t.Helper()
	select {
	case raw := <-c.Outbox():
		t.Fatalf("unexpected message %s", raw)
	default:
	}
}

func ready(t *testing.T, h *Hub, name string) *Conn {
	t.Helper()
	c := h.Connect()
	t.Cleanup(c.Close)
	c.SetIdentity(mumble.Identity{Session: 1, Name: name})
	if m := next(t, c); m["type"] != "ready" {
		t.Fatalf("got %v", m)
	}
	return c
}

func TestReadySendsTheChannelList(t *testing.T) {
	h := newHub(t, "general", "links")
	c := h.Connect()
	defer c.Close()
	empty(t, c)
	c.SetIdentity(mumble.Identity{Session: 4, Name: "alice"})
	m := next(t, c)
	chans := m["channels"].([]any)
	if m["type"] != "ready" || len(chans) != 2 || chans[0].(map[string]any)["name"] != "general" {
		t.Fatalf("got %v", m)
	}
	// A rename sends no second ready.
	c.SetIdentity(mumble.Identity{Session: 4, Name: "alicia"})
	empty(t, c)
}

func TestRequestsBeforeIdentityAreRefused(t *testing.T) {
	h := newHub(t, "general")
	c := h.Connect()
	defer c.Close()
	c.Handle([]byte(`{"type":"send","channel":1,"text":"hi"}`))
	if m := next(t, c); m["type"] != "error" {
		t.Fatalf("got %v", m)
	}
}

func TestSendBroadcastsToEveryoneAndStores(t *testing.T) {
	h := newHub(t, "general")
	alice, bob := ready(t, h, "alice"), ready(t, h, "bob")
	alice.Handle([]byte(`{"type":"send","channel":1,"text":"  hello  "}`))
	for _, c := range []*Conn{alice, bob} {
		m := next(t, c)
		msg := m["message"].(map[string]any)
		if m["type"] != "message" || msg["author"] != "alice" || msg["text"] != "hello" ||
			msg["channel"] != 1.0 || msg["at"] != float64(clock.UnixMilli()) {
			t.Fatalf("got %v", m)
		}
	}
	bob.Handle([]byte(`{"type":"history","channel":1}`))
	m := next(t, bob)
	msgs := m["messages"].([]any)
	if m["type"] != "history" || len(msgs) != 1 || m["more"] != false {
		t.Fatalf("got %v", m)
	}
}

func TestHistoryPages(t *testing.T) {
	h := newHub(t, "general")
	c := ready(t, h, "alice")
	for i := 0; i < pageSize+5; i++ {
		c.Handle([]byte(`{"type":"send","channel":1,"text":"x"}`))
		next(t, c)
	}
	c.Handle([]byte(`{"type":"history","channel":1}`))
	first := next(t, c)
	if len(first["messages"].([]any)) != pageSize || first["more"] != true {
		t.Fatalf("first page: %d more=%v", len(first["messages"].([]any)), first["more"])
	}
	oldest := first["messages"].([]any)[0].(map[string]any)["id"]
	c.Handle([]byte(`{"type":"history","channel":1,"before":` + jsonNum(oldest) + `}`))
	second := next(t, c)
	if len(second["messages"].([]any)) != 5 || second["more"] != false {
		t.Fatalf("second page: %v", second)
	}
}

func jsonNum(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestInvalidRequests(t *testing.T) {
	h := newHub(t, "general")
	c := ready(t, h, "alice")
	for _, req := range []string{
		`not json`,
		`{"type":"send","channel":99,"text":"x"}`,
		`{"type":"send","channel":1,"text":"   "}`,
		`{"type":"send","channel":1,"text":"` + strings.Repeat("a", MaxTextLength+1) + `"}`,
		`{"type":"history","channel":99}`,
		`{"type":"dance"}`,
	} {
		c.Handle([]byte(req))
		if m := next(t, c); m["type"] != "error" {
			t.Errorf("%.40s: got %v", req, m)
		}
	}
}

func TestClosedConnectionsGetNoBroadcasts(t *testing.T) {
	h := newHub(t, "general")
	alice, bob := ready(t, h, "alice"), ready(t, h, "bob")
	bob.Close()
	alice.Handle([]byte(`{"type":"send","channel":1,"text":"hi"}`))
	next(t, alice)
	if _, ok := <-bob.Outbox(); ok {
		t.Fatal("closed connection got a message")
	}
}

func TestSlowClientsDoNotBlockOthers(t *testing.T) {
	h := newHub(t, "general")
	alice, slow := ready(t, h, "alice"), ready(t, h, "slow")
	_ = slow // never reads
	done := make(chan struct{})
	go func() {
		for i := 0; i < outboxSize*3; i++ {
			alice.Handle([]byte(`{"type":"send","channel":1,"text":"x"}`))
			<-alice.Outbox()
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("a slow client blocked the hub")
	}
}
