package bridge

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
	"kallisti.chat/server/internal/sidecar"
	"kallisti.chat/server/internal/store"
)

func mumbleFrame(typ uint16, msg proto.Message) []byte {
	body, _ := proto.Marshal(msg)
	out := make([]byte, 6, 6+len(body))
	binary.BigEndian.PutUint16(out, typ)
	binary.BigEndian.PutUint32(out[2:], uint32(len(body)))
	return append(out, body...)
}

// fakeMumble greets every connection as a new user, like Murmur's handshake.
func fakeMumble(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var session atomic.Uint32
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			s := session.Add(1)
			name := []string{"", "alice", "bob"}[min(int(s), 2)]
			c.Write(mumbleFrame(mumble.TypeUserState, &pb.UserState{Session: proto.Uint32(s), Name: proto.String(name)}))
			c.Write(mumbleFrame(mumble.TypeServerSync, &pb.ServerSync{Session: proto.Uint32(s)}))
			go func() { io.Copy(io.Discard, c); c.Close() }()
		}
	}()
	t.Cleanup(func() { ln.Close() })
	return ln.Addr().String()
}

func newSidecar(t *testing.T) *sidecar.Hub {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	s.EnsureChannels([]string{"general"})
	h, err := sidecar.NewHub(s, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// readJSON skips the binary Mumble frames and returns the next text message.
func readJSON(t *testing.T, c *websocket.Conn) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for {
		typ, data, err := c.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if typ == websocket.MessageText {
			var v map[string]any
			if err := json.Unmarshal(data, &v); err != nil {
				t.Fatal(err)
			}
			return v
		}
	}
}

func TestSidecarOverTheBridge(t *testing.T) {
	url := startBridge(t, Options{Dial: dialer(fakeMumble(t)), Sidecar: newSidecar(t)})
	alice, _, err := connect(t, url, "")
	if err != nil {
		t.Fatal(err)
	}
	if m := readJSON(t, alice); m["type"] != "ready" {
		t.Fatalf("alice: %v", m)
	}
	bob, _, err := connect(t, url, "")
	if err != nil {
		t.Fatal(err)
	}
	if m := readJSON(t, bob); m["type"] != "ready" {
		t.Fatalf("bob: %v", m)
	}
	alice.Write(testCtx(t), websocket.MessageText, []byte(`{"type":"send","channel":1,"text":"hi bob"}`))
	for _, c := range []*websocket.Conn{alice, bob} {
		m := readJSON(t, c)
		msg, _ := m["message"].(map[string]any)
		if m["type"] != "message" || msg["author"] != "alice" || msg["text"] != "hi bob" {
			t.Fatalf("got %v", m)
		}
	}
}

func TestTextFramesWithoutSidecarStillClose1003(t *testing.T) {
	c, _, err := connect(t, startBridge(t, Options{Dial: dialer(fakeMumble(t))}), "")
	if err != nil {
		t.Fatal(err)
	}
	c.Write(testCtx(t), websocket.MessageText, []byte(`{}`))
	if got := closeStatus(t, c); got != websocket.StatusUnsupportedData {
		t.Errorf("close status %v, want 1003", got)
	}
}
