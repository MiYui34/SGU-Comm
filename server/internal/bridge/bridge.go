// Package bridge moves raw bytes between browser WebSockets and the TLS
// control port of one Mumble server. Mumble framing is done by the browser,
// so chunk boundaries do not matter here.
package bridge

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"

	"github.com/coder/websocket"

	"github.com/mumble-discord/server/internal/config"
)

// MaxPayload is the Mumble frame limit. Larger messages close with 1009.
const MaxPayload = 8 * 1024 * 1024

type Options struct {
	AllowedOrigins []string
	MaxConnections int
	// Dial opens the upstream connection (TLS to Mumble in production).
	Dial func(ctx context.Context) (net.Conn, error)
}

type Bridge struct {
	opts   Options
	mu     sync.Mutex
	active int
	closed bool
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

func New(opts Options) *Bridge {
	ctx, cancel := context.WithCancel(context.Background())
	return &Bridge{opts: opts, ctx: ctx, cancel: cancel}
}

// Close refuses new connections, ends the open ones (close code 1001) and
// waits for them.
func (b *Bridge) Close() {
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()
	b.cancel()
	b.wg.Wait()
}

// admit counts a new connection. Under the lock, so wg.Add never runs
// concurrently with Close's wg.Wait.
func (b *Bridge) admit() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed || b.active >= b.opts.MaxConnections {
		return false
	}
	b.active++
	b.wg.Add(1)
	return true
}

func (b *Bridge) release() {
	b.mu.Lock()
	b.active--
	b.mu.Unlock()
	b.wg.Done()
}

func (b *Bridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !config.OriginAllowed(b.opts.AllowedOrigins, r.Header.Get("Origin")) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}
	if !b.admit() {
		http.Error(w, "too many connections", http.StatusServiceUnavailable)
		return
	}
	defer b.release()

	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// The origin is checked above, against our own list.
		InsecureSkipVerify: true,
		// Compression adds latency to voice.
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		return
	}
	defer ws.CloseNow()
	ws.SetReadLimit(MaxPayload)

	// Not derived from b.ctx: cancelling a read drops the connection without
	// a close frame. Shutdown goes through the AfterFunc below instead.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	upstream, err := b.opts.Dial(b.ctx)
	if err != nil {
		slog.Warn("upstream connect failed", "err", err)
		ws.Close(websocket.StatusInternalError, "upstream unavailable")
		return
	}
	defer upstream.Close()

	// Shutdown: close the WebSocket with 1001 and stop the upstream reader.
	stop := context.AfterFunc(b.ctx, func() {
		ws.Close(websocket.StatusGoingAway, "server shutting down")
		upstream.Close()
	})
	defer stop()

	done := make(chan struct{})
	go func() {
		defer close(done)
		b.upstreamToClient(ctx, cancel, upstream, ws)
	}()
	b.clientToUpstream(ctx, upstream, ws)
	// Stop the other direction and wait for it, so no goroutine outlives
	// the connection count.
	cancel()
	upstream.Close()
	<-done
}

// Blocking writes give natural backpressure: a slow side stops the reader
// of the other side.
func (b *Bridge) upstreamToClient(ctx context.Context, cancel context.CancelFunc, upstream net.Conn, ws *websocket.Conn) {
	defer cancel()
	buf := make([]byte, 32*1024)
	for {
		n, err := upstream.Read(buf)
		if n > 0 {
			if werr := ws.Write(ctx, websocket.MessageBinary, buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			// Ended by us (client gone, shutdown): nothing to tell.
			if ctx.Err() != nil || b.ctx.Err() != nil {
				return
			}
			if errors.Is(err, io.EOF) {
				ws.Close(websocket.StatusNormalClosure, "")
			} else {
				ws.Close(websocket.StatusInternalError, "upstream error")
			}
			return
		}
	}
}

func (b *Bridge) clientToUpstream(ctx context.Context, upstream net.Conn, ws *websocket.Conn) {
	for {
		typ, data, err := ws.Read(ctx)
		if err != nil {
			return
		}
		if typ != websocket.MessageBinary {
			ws.Close(websocket.StatusUnsupportedData, "binary frames only")
			return
		}
		if _, err := upstream.Write(data); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				ws.Close(websocket.StatusInternalError, "upstream write failed")
			}
			return
		}
	}
}
