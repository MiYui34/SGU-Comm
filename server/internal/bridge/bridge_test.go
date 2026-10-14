package bridge

import (
	"bufio"
	"context"
	"errors"
	"io"
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
