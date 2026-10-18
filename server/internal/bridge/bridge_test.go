package bridge

import (
	"bufio"
	"context"
	"errors"
	"io"
	"kallisti.chat/server/internal/mumble"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// echoServer accepts TCP connections and echoes their bytes back.
type echoServer struct {
	addr  string
	open  atomic.Int64
	conns chan net.Conn
}

func startEcho(t *testing.T) *echoServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	e := &echoServer{addr: ln.Addr().String(), conns: make(chan net.Conn, 10)}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			e.open.Add(1)
			e.conns <- c
			go func() {
				io.Copy(c, c)
				c.Close()
				e.open.Add(-1)
			}()
		}
	}()
	t.Cleanup(func() { ln.Close() })
	return e
}

func dialer(addr string) func(context.Context, Upstream) (net.Conn, error) {
	return func(ctx context.Context, _ Upstream) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", addr)
	}
}

func startBridge(t *testing.T, opts Options) string {
	_, url := startBridgeWith(t, opts)
	return url
}

func startBridgeWith(t *testing.T, opts Options) (*Bridge, string) {
	t.Helper()
	if opts.AllowedOrigins == nil {
		opts.AllowedOrigins = []string{"*"}
	}
	if opts.MaxConnections == 0 {
		opts.MaxConnections = 10
	}
	if opts.MaxPerIP == 0 {
		opts.MaxPerIP = 10
	}
	b := New(opts)
	srv := httptest.NewServer(b)
	t.Cleanup(func() {
		b.Close()
		srv.Close()
	})
	return b, "ws" + strings.TrimPrefix(srv.URL, "http")
}

func connect(t *testing.T, url, origin string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	h := http.Header{}
	if origin != "" {
		h.Set("Origin", origin)
	}
	c, resp, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: h})
	if c != nil {
		c.SetReadLimit(-1)
		t.Cleanup(func() { c.CloseNow() })
	}
	return c, resp, err
}

func testCtx(t *testing.T) context.Context {
	c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	return c
}

// closeStatus reads until the connection closes and returns the close code.
func closeStatus(t *testing.T, c *websocket.Conn) websocket.StatusCode {
	t.Helper()
	for {
		if _, _, err := c.Read(testCtx(t)); err != nil {
			return websocket.CloseStatus(err)
		}
	}
}

func eventually(t *testing.T, cond func() bool) {
	t.Helper()
	for i := 0; i < 100; i++ {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition not met in time")
}

func TestForwardsBinaryBothWays(t *testing.T) {
	echo := startEcho(t)
	c, _, err := connect(t, startBridge(t, Options{Dial: dialer(echo.addr)}), "")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Write(testCtx(t), websocket.MessageBinary, []byte{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	typ, data, err := c.Read(testCtx(t))
	if err != nil || typ != websocket.MessageBinary || string(data) != "\x01\x02\x03" {
		t.Fatalf("got %v %v %v", typ, data, err)
	}
}

func TestClosesWebSocketWhenUpstreamCloses(t *testing.T) {
	echo := startEcho(t)
	c, _, err := connect(t, startBridge(t, Options{Dial: dialer(echo.addr)}), "")
	if err != nil {
		t.Fatal(err)
	}
	(<-echo.conns).Close()
	if got := closeStatus(t, c); got != websocket.StatusNormalClosure {
		t.Errorf("close status %v, want normal closure", got)
	}
}

func TestClosesUpstreamWhenWebSocketCloses(t *testing.T) {
	echo := startEcho(t)
	c, _, err := connect(t, startBridge(t, Options{Dial: dialer(echo.addr)}), "")
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return echo.open.Load() == 1 })
	c.Close(websocket.StatusNormalClosure, "")
	eventually(t, func() bool { return echo.open.Load() == 0 })
}

func TestUpstreamDialErrorCloses1011(t *testing.T) {
	fail := func(context.Context, Upstream) (net.Conn, error) { return nil, errors.New("no route") }
	c, _, err := connect(t, startBridge(t, Options{Dial: fail}), "")
	if err != nil {
		t.Fatal(err)
	}
	if got := closeStatus(t, c); got != websocket.StatusInternalError {
		t.Errorf("close status %v, want 1011", got)
	}
}

func TestRejectsOriginsThatAreNotAllowed(t *testing.T) {
	echo := startEcho(t)
	url := startBridge(t, Options{Dial: dialer(echo.addr), AllowedOrigins: []string{"https://good.example"}})
	_, resp, err := connect(t, url, "https://evil.example")
	if err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("got %v %v, want 403", resp, err)
	}
	if echo.open.Load() != 0 {
		t.Error("upstream was dialed")
	}
}

func TestClosesOnTextFrames(t *testing.T) {
	echo := startEcho(t)
	c, _, err := connect(t, startBridge(t, Options{Dial: dialer(echo.addr)}), "")
	if err != nil {
		t.Fatal(err)
	}
	c.Write(testCtx(t), websocket.MessageText, []byte("hello"))
	if got := closeStatus(t, c); got != websocket.StatusUnsupportedData {
		t.Errorf("close status %v, want 1003", got)
	}
}

func TestClosesMessagesAboveTheMumbleFrameLimit(t *testing.T) {
	echo := startEcho(t)
	c, _, err := connect(t, startBridge(t, Options{Dial: dialer(echo.addr)}), "")
	if err != nil {
		t.Fatal(err)
	}
	c.Write(testCtx(t), websocket.MessageBinary, make([]byte, 9*1024*1024))
	if got := closeStatus(t, c); got != websocket.StatusMessageTooBig {
		t.Errorf("close status %v, want 1009", got)
	}
}

func TestRejectsConnectionsAboveTheLimit(t *testing.T) {
	echo := startEcho(t)
	url := startBridge(t, Options{Dial: dialer(echo.addr), MaxConnections: 1})
	if _, _, err := connect(t, url, ""); err != nil {
		t.Fatal(err)
	}
	_, resp, err := connect(t, url, "")
	if err == nil || resp == nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("got %v %v, want 503", resp, err)
	}
}

func TestSurvivesAMalformedFrame(t *testing.T) {
	echo := startEcho(t)
	url := startBridge(t, Options{Dial: dialer(echo.addr)})
	raw, err := net.Dial("tcp", strings.TrimPrefix(url, "ws://"))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	io.WriteString(raw, "GET / HTTP/1.1\r\nHost: x\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n"+
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\n\r\n")
	resp, err := http.ReadResponse(bufio.NewReader(raw), nil)
	if err != nil || resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("handshake: %v %v", resp, err)
	}
	raw.Write([]byte{0x82, 0x01, 0x00}) // unmasked client frame: protocol error
	raw.SetReadDeadline(time.Now().Add(2 * time.Second))
	io.Copy(io.Discard, raw) // the server closes the connection
	if _, _, err := connect(t, url, ""); err != nil {
		t.Fatalf("bridge stopped accepting: %v", err)
	}
}

func TestCloseEndsAllConnections(t *testing.T) {
	echo := startEcho(t)
	b, url := startBridgeWith(t, Options{Dial: dialer(echo.addr)})
	c, _, err := connect(t, url, "")
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return echo.open.Load() == 1 })
	// Close waits for the client's close frame: read at the same time.
	go b.Close()
	if got := closeStatus(t, c); got != websocket.StatusGoingAway {
		t.Errorf("close status %v, want 1001", got)
	}
	eventually(t, func() bool { return echo.open.Load() == 0 })
	if _, resp, err := connect(t, url, ""); err == nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("after Close: got %v %v, want 503", resp, err)
	}
}

func TestRejectsConnectionsAboveThePerIPLimit(t *testing.T) {
	echo := startEcho(t)
	url := startBridge(t, Options{Dial: dialer(echo.addr), MaxPerIP: 2})
	for i := 0; i < 2; i++ {
		if _, _, err := connect(t, url, ""); err != nil {
			t.Fatal(err)
		}
	}
	_, resp, err := connect(t, url, "")
	if err == nil || resp == nil || resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("got %v %v, want 429", resp, err)
	}
}

func TestPerIPLimitCountsIPv6Slash64s(t *testing.T) {
	// Not closed: Close would wait for the connections counted here.
	b := New(Options{MaxConnections: 10, MaxPerIP: 1})
	admit := func(ip string, want int) {
		t.Helper()
		if s := b.admit(ip); s != want {
			t.Errorf("%s: %d, want %d", ip, s, want)
		}
	}
	admit("2001:db8:1:2::1", 0)
	admit("2001:db8:1:2::2", http.StatusTooManyRequests) // the same /64
	admit("2001:db8:1:3::1", 0)
	admit("192.0.2.1", 0)
	admit("192.0.2.2", 0)
}

func TestKeepsSilentClientsAlive(t *testing.T) {
	// The page's timers may sleep in a background tab: the bridge pings
	// upstream itself (the echo sends the ping back).
	echo := startEcho(t)
	c, _, err := connect(t, startBridge(t, Options{Dial: dialer(echo.addr), KeepAlive: 50 * time.Millisecond}), "")
	if err != nil {
		t.Fatal(err)
	}
	typ, data, err := c.Read(testCtx(t))
	if err != nil || typ != websocket.MessageBinary || len(data) < 6 || data[0] != 0 || data[1] != byte(mumble.TypePing) {
		t.Fatalf("got %v %x %v", typ, data, err)
	}
}

func TestClosesBrowsersThatDoNotAnswerPings(t *testing.T) {
	echo := startEcho(t)
	b, url := startBridgeWith(t, Options{Dial: dialer(echo.addr), KeepAlive: 30 * time.Millisecond})
	c, _, err := connect(t, url, "")
	if err != nil {
		t.Fatal(err)
	}
	// Not reading: the client answers no WebSocket ping.
	_ = c
	eventually(t, func() bool { return b.count() == 0 })
}

// The desktop app sends its session token as a WebSocket subprotocol next
// to "kallisti"; browsers close the connection unless one is chosen.
func TestDesktopSubprotocol(t *testing.T) {
	url := startBridge(t, Options{Dial: dialer(startEcho(t).addr)})
	ctx := testCtx(t)
	c, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		HTTPHeader:   http.Header{"Origin": {"tauri://localhost"}},
		Subprotocols: []string{"kallisti", "kallisti.session.abc"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	if c.Subprotocol() != "kallisti" {
		t.Fatalf("subprotocol %q", c.Subprotocol())
	}
}
