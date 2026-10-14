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

	"kallisti.chat/server/internal/config"
	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/sidecar"
)

// MaxPayload is the Mumble frame limit. Larger messages close with 1009.
const MaxPayload = 8 * 1024 * 1024

// maxTextFrame limits sidecar requests (a 2000-character message fits).
const maxTextFrame = 16 * 1024

type Options struct {
	AllowedOrigins []string
	MaxConnections int
	// Dial opens the upstream connection (TLS to Mumble in production).
	Dial func(ctx context.Context) (net.Conn, error)
	// Sidecar serves text channels over text frames. Nil: text frames close
	// the connection with 1003.
	Sidecar *sidecar.Hub
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

	upstream, err := b.opts.Dial(b.ctx)
	if err != nil {
		slog.Warn("upstream connect failed", "err", err)
		ws.Close(websocket.StatusInternalError, "upstream unavailable")
		return
	}
	defer upstream.Close()

	// Not derived from b.ctx: cancelling a read drops the connection without
	// a close frame. Shutdown goes through the AfterFunc below instead.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := &conn{bridge: b, ws: ws, upstream: upstream, ctx: ctx, cancel: cancel}

	// Shutdown: close the WebSocket with 1001 and stop the upstream reader.
	stop := context.AfterFunc(b.ctx, func() {
		ws.Close(websocket.StatusGoingAway, "server shutting down")
		upstream.Close()
	})
	defer stop()

	var wg sync.WaitGroup
	if b.opts.Sidecar != nil {
		c.side = b.opts.Sidecar.Connect()
		wg.Go(c.writeSidecar)
	}
	wg.Go(c.upstreamToClient)
	c.clientToUpstream()
	// Stop everything else and wait for it, so no goroutine outlives the
	// connection count.
	cancel()
	upstream.Close()
	if c.side != nil {
		c.side.Close()
	}
	wg.Wait()
}

// conn is one browser connection with its upstream.
type conn struct {
	bridge   *Bridge
	ws       *websocket.Conn
	upstream net.Conn
	ctx      context.Context
	cancel   context.CancelFunc
	side     *sidecar.Conn // nil without sidecar
	sniffer  mumble.Sniffer
	sniffErr bool
}

// Blocking writes give natural backpressure: a slow side stops the reader
// of the other side.
func (c *conn) upstreamToClient() {
	defer c.cancel()
	buf := make([]byte, 32*1024)
	for {
		n, err := c.upstream.Read(buf)
		if n > 0 {
			if werr := c.ws.Write(c.ctx, websocket.MessageBinary, buf[:n]); werr != nil {
				return
			}
			c.sniff(buf[:n])
		}
		if err != nil {
			// Ended by us (client gone, shutdown): nothing to tell.
			if c.ctx.Err() != nil || c.bridge.ctx.Err() != nil {
				return
			}
			if errors.Is(err, io.EOF) {
				c.ws.Close(websocket.StatusNormalClosure, "")
			} else {
				c.ws.Close(websocket.StatusInternalError, "upstream error")
			}
			return
		}
	}
}

// sniff follows the stream (after forwarding it, so voice is not delayed)
// to learn the Mumble identity for the sidecar.
func (c *conn) sniff(chunk []byte) {
	if c.side == nil || c.sniffErr {
		return
	}
	id, changed, err := c.sniffer.Feed(chunk)
	if err != nil {
		slog.Warn("cannot follow the Mumble stream, text channels disabled for this connection", "err", err)
		c.sniffErr = true
		return
	}
	if changed {
		c.side.SetIdentity(id)
	}
}

func (c *conn) clientToUpstream() {
	for {
		typ, data, err := c.ws.Read(c.ctx)
		if err != nil {
			return
		}
		if typ == websocket.MessageText {
			if c.side == nil {
				c.ws.Close(websocket.StatusUnsupportedData, "binary frames only")
				return
			}
			if len(data) > maxTextFrame {
				c.ws.Close(websocket.StatusMessageTooBig, "request too large")
				return
			}
			c.side.Handle(data)
			continue
		}
		if _, err := c.upstream.Write(data); err != nil {
			if !errors.Is(err, net.ErrClosed) {
				c.ws.Close(websocket.StatusInternalError, "upstream write failed")
			}
			return
		}
	}
}

// writeSidecar sends the sidecar's messages as text frames. It ends when
// the sidecar connection closes its outbox.
func (c *conn) writeSidecar() {
	for msg := range c.side.Outbox() {
		if c.ws.Write(c.ctx, websocket.MessageText, msg) != nil {
			return
		}
	}
}
