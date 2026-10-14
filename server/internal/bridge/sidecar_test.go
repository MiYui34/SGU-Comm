package bridge

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
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

func TestStreamErrorTellsTheClient(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		header := make([]byte, 6)
		binary.BigEndian.PutUint32(header[2:], 9<<20) // larger than any Mumble frame
		c.Write(header)
		io.Copy(io.Discard, c)
	}()
	c, _, err := connect(t, startBridge(t, Options{Dial: dialer(ln.Addr().String()), Sidecar: newSidecar(t)}), "")
	if err != nil {
		t.Fatal(err)
	}
	if m := readJSON(t, c); m["type"] != "error" {
		t.Fatalf("got %v, want an error event", m)
	}
}

func hello(name string) []byte {
	return append(
		mumbleFrame(mumble.TypeVersion, &pb.Version{Release: proto.String("Kallisti")}),
		mumbleFrame(mumble.TypeAuthenticate, &pb.Authenticate{Username: proto.String(name)})...,
	)
}

// readReject returns the Reject message if the next binary frame is one.
func readReject(t *testing.T, c *websocket.Conn) *pb.Reject {
	t.Helper()
	typ, data, err := c.Read(testCtx(t))
	if err != nil || typ != websocket.MessageBinary || len(data) < 6 || data[1] != mumble.TypeReject {
		return nil
	}
	var m pb.Reject
	if proto.Unmarshal(data[6:], &m) != nil {
		return nil
	}
	return &m
}

func TestDuplicateGuestNamesAreRefused(t *testing.T) {
	echo := startEcho(t)
	url := startBridge(t, Options{Dial: dialer(echo.addr), UniqueGuestNames: true})
	first, _, err := connect(t, url, "")
	if err != nil {
		t.Fatal(err)
	}
	first.Write(testCtx(t), websocket.MessageBinary, hello("alice"))
	eventually(t, func() bool { return echo.open.Load() == 1 })
	second, _, err := connect(t, url, "")
	if err != nil {
		t.Fatal(err)
	}
	second.Write(testCtx(t), websocket.MessageBinary, hello("Alice")) // names are case-insensitive
	if m := readReject(t, second); m == nil || m.GetType() != pb.Reject_UsernameInUse {
		t.Fatalf("got %v, want a UsernameInUse reject", m)
	}
	if echo.open.Load() != 1 {
		t.Fatal("the refused guest reached the upstream")
	}
	// The first guest is still connected.
	first.Write(testCtx(t), websocket.MessageBinary, []byte{9})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for {
		_, data, err := first.Read(ctx)
		if err != nil {
			t.Fatalf("first guest lost: %v", err)
		}
		if bytes.HasSuffix(data, []byte{9}) {
			break
		}
	}
}

func TestNameIsFreeAgainAfterLeaving(t *testing.T) {
	echo := startEcho(t)
	url := startBridge(t, Options{Dial: dialer(echo.addr), UniqueGuestNames: true})
	first, _, _ := connect(t, url, "")
	first.Write(testCtx(t), websocket.MessageBinary, hello("bob"))
	eventually(t, func() bool { return echo.open.Load() == 1 })
	first.Close(websocket.StatusNormalClosure, "")
	eventually(t, func() bool { return echo.open.Load() == 0 })
	second, _, _ := connect(t, url, "")
	second.Write(testCtx(t), websocket.MessageBinary, hello("bob"))
	eventually(t, func() bool { return echo.open.Load() == 1 })
}

// accounts: requests with a "user" query value belong to that account.
func accounts(names ...string) (func(*http.Request) (string, *tls.Certificate, bool), func(string) bool) {
	cert := &tls.Certificate{}
	lookup := func(r *http.Request) (string, *tls.Certificate, bool) {
		u := r.URL.Query().Get("user")
		return u, cert, u != ""
	}
	isAccount := func(name string) bool {
		for _, n := range names {
			if strings.EqualFold(n, name) {
				return true
			}
		}
		return false
	}
	return lookup, isAccount
}

func TestAccountsDialWithTheirCertificate(t *testing.T) {
	echo := startEcho(t)
	var got atomic.Pointer[tls.Certificate]
	dial := func(ctx context.Context, cert *tls.Certificate) (net.Conn, error) {
		got.Store(cert)
		return dialer(echo.addr)(ctx, cert)
	}
	lookup, isAccount := accounts("carol")
	url := startBridge(t, Options{Dial: dial, UniqueGuestNames: true, Account: lookup, IsAccountName: isAccount})
	c, _, _ := connect(t, url+"?user=carol", "")
	c.Write(testCtx(t), websocket.MessageBinary, hello("carol"))
	eventually(t, func() bool { return echo.open.Load() == 1 })
	if got.Load() == nil {
		t.Fatal("dialed without the account certificate")
	}
}

func TestAccountsMustUseTheirName(t *testing.T) {
	echo := startEcho(t)
	lookup, isAccount := accounts("carol")
	url := startBridge(t, Options{Dial: dialer(echo.addr), UniqueGuestNames: true, Account: lookup, IsAccountName: isAccount})
	c, _, _ := connect(t, url+"?user=carol", "")
	c.Write(testCtx(t), websocket.MessageBinary, hello("somebody-else"))
	if m := readReject(t, c); m == nil {
		t.Fatal("an account used a foreign name")
	}
}

func TestGuestsCannotUseAnAccountName(t *testing.T) {
	echo := startEcho(t)
	lookup, isAccount := accounts("carol")
	url := startBridge(t, Options{Dial: dialer(echo.addr), UniqueGuestNames: true, Account: lookup, IsAccountName: isAccount})
	c, _, _ := connect(t, url, "")
	c.Write(testCtx(t), websocket.MessageBinary, hello("Carol"))
	m := readReject(t, c)
	if m == nil || !strings.Contains(m.GetReason(), "account") {
		t.Fatalf("got %v", m)
	}
	if echo.open.Load() != 0 {
		t.Fatal("the guest reached the upstream")
	}
}
