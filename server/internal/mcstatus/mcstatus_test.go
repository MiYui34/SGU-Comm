package mcstatus

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const sample = `{"version":{"name":"1.21.4","protocol":769},"players":{"max":50,"online":2,"sample":[{"name":"Kernel","id":"1"},{"name":"Ender","id":"2"}]},"description":{"text":"hi"}}`

// fakeServer answers the status ping like a Minecraft server does, and keeps
// the handshake it was sent.
type fakeServer struct {
	listener net.Listener
	status   string
	// delay is how long the answer takes: the round trip of the test.
	delay time.Duration

	mu    sync.Mutex
	dials int
	host  string
	port  int
}

func newFakeServer(t *testing.T) *fakeServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeServer{listener: listener, status: sample}
	t.Cleanup(func() { listener.Close() })
	go f.serve()
	return f
}

func (f *fakeServer) addr() string { return f.listener.Addr().String() }

func (f *fakeServer) counts() (dials int, host string, port int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.dials, f.host, f.port
}

func (f *fakeServer) serve() {
	for {
		conn, err := f.listener.Accept()
		if err != nil {
			return
		}
		f.mu.Lock()
		f.dials++
		f.mu.Unlock()
		go f.handle(conn)
	}
}

func (f *fakeServer) handle(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	for range 2 { // handshake, then the status request
		length, err := readVarint(r)
		if err != nil {
			return
		}
		body := make([]byte, length)
		if _, err := io.ReadFull(r, body); err != nil {
			return
		}
		f.record(body)
	}
	var packet, out bytes.Buffer
	writeVarint(&packet, 0x00)
	writeString(&packet, f.status)
	writePacket(&out, packet.Bytes())
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	if _, err := conn.Write(out.Bytes()); err != nil {
		return
	}
}

func (f *fakeServer) record(body []byte) {
	r := bytes.NewReader(body)
	if id, err := readVarint(r); err != nil || id != 0x00 || r.Len() == 0 {
		return // the status request itself
	}
	if _, err := readVarint(r); err != nil {
		return
	}
	host, err := readText(r)
	if err != nil {
		return
	}
	var number [2]byte
	if _, err := io.ReadFull(r, number[:]); err != nil {
		return
	}
	f.mu.Lock()
	f.host, f.port = host, int(binary.BigEndian.Uint16(number[:]))
	f.mu.Unlock()
}

func readText(r *bytes.Reader) (string, error) {
	size, err := readVarint(r)
	if err != nil {
		return "", err
	}
	if size < 0 || int(size) > r.Len() {
		return "", errors.New("bad string")
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func TestGet(t *testing.T) {
	f := newFakeServer(t)
	s := &Status{Addr: f.addr()}
	reply, err := s.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if reply.Online != 2 || reply.Slots != 50 || reply.Version != "1.21.4" {
		t.Errorf("reply %+v", reply)
	}
	if len(reply.Players) != 2 || reply.Players[0] != "Kernel" || reply.Players[1] != "Ender" {
		t.Errorf("players %v", reply.Players)
	}
	if reply.At == 0 {
		t.Error("no time")
	}
	_, host, port := f.counts()
	wantHost, wantPort, _ := net.SplitHostPort(f.addr())
	if host != wantHost {
		t.Errorf("handshake host %q, want %q", host, wantHost)
	}
	if p := port; p != portOf(wantPort) {
		t.Errorf("handshake port %d, want %s", p, wantPort)
	}
}

func portOf(text string) int {
	port, err := net.LookupPort("tcp", text)
	if err != nil {
		return 0
	}
	return port
}

func TestGetCaches(t *testing.T) {
	f := newFakeServer(t)
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	s := &Status{Addr: f.addr(), Now: func() time.Time { return now }}
	for range 3 {
		if _, err := s.Get(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if dials, _, _ := f.counts(); dials != 1 {
		t.Errorf("%d dials within the cache time", dials)
	}
	now = now.Add(6 * time.Second)
	if _, err := s.Get(context.Background()); err != nil {
		t.Fatal(err)
	}
	if dials, _, _ := f.counts(); dials != 2 {
		t.Errorf("%d dials after the cache time", dials)
	}
}

func TestGetReportsTheRoundTrip(t *testing.T) {
	f := newFakeServer(t)
	f.delay = 40 * time.Millisecond
	s := &Status{Addr: f.addr()}
	reply, err := s.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// A slow answer is slow to read: 40 ms of waiting, a few of scheduling.
	if reply.RTT < 35 {
		t.Errorf("round trip %d ms, want about 40", reply.RTT)
	}
	if reply.RTT > 500 {
		t.Errorf("round trip %d ms is not the answer's delay", reply.RTT)
	}
}

func TestGetFails(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close() // nothing listens there now
	s := &Status{Addr: addr, Timeout: time.Second}
	if _, err := s.Get(context.Background()); err == nil {
		t.Fatal("no error from a closed port")
	}
	// The failure is cached too: no second question of a dead server.
	if _, err := s.Get(context.Background()); err == nil {
		t.Fatal("no error from the cached failure")
	}
}

func TestHandler(t *testing.T) {
	t.Run("not configured", func(t *testing.T) {
		w := httptest.NewRecorder()
		(&Status{}).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/mc/status", nil))
		if body := decode(t, w); body["ok"] != false || body["configured"] != false {
			t.Errorf("body %v", body)
		}
	})

	t.Run("reachable", func(t *testing.T) {
		f := newFakeServer(t)
		w := httptest.NewRecorder()
		(&Status{Addr: f.addr()}).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/mc/status", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status %d", w.Code)
		}
		body := decode(t, w)
		if body["ok"] != true || body["online"] != float64(2) || body["slots"] != float64(50) {
			t.Errorf("body %v", body)
		}
		if players, _ := body["players"].([]any); len(players) != 2 {
			t.Errorf("players %v", body["players"])
		}
	})

	t.Run("keeps the address to itself", func(t *testing.T) {
		f := newFakeServer(t)
		w := httptest.NewRecorder()
		(&Status{Addr: f.addr()}).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/mc/status", nil))
		// The answer goes to the player's browser: it must not say where the
		// game server is, or the address is one fetch away from anyone.
		host, _, err := net.SplitHostPort(f.addr())
		if err != nil {
			t.Fatalf("split %q: %v", f.addr(), err)
		}
		if raw := w.Body.String(); strings.Contains(raw, host) {
			t.Errorf("the answer names the game server: %s", raw)
		}
	})

	t.Run("unreachable", func(t *testing.T) {
		w := httptest.NewRecorder()
		s := &Status{Addr: "127.0.0.1:1", Timeout: time.Second}
		s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/mc/status", nil))
		if body := decode(t, w); body["ok"] != false || body["configured"] != true {
			t.Errorf("body %v", body)
		}
	})

	t.Run("method", func(t *testing.T) {
		w := httptest.NewRecorder()
		(&Status{}).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/mc/status", nil))
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("status %d", w.Code)
		}
	})
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q: %v", w.Body.String(), err)
	}
	return body
}
