// Package bridge moves raw bytes between browser WebSockets and the TLS
// control port of one Mumble server. Mumble framing is done by the browser,
// so chunk boundaries do not matter here.
package bridge

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"kallisti.chat/server/internal/clientip"
	"kallisti.chat/server/internal/config"
	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
	"kallisti.chat/server/internal/sidecar"
)

// MaxPayload is the Mumble frame limit. Larger messages close with 1009.
const MaxPayload = 8 * 1024 * 1024

// maxTextFrame limits sidecar requests (a 2000-character message fits).
const maxTextFrame = 16 * 1024

type Options struct {
	AllowedOrigins    []string
	MaxConnections    int
	MaxPerIP          int
	TrustForwardedFor bool
	// Dial opens the upstream connection (TLS to Mumble in production),
	// with the account's client certificate, or nil for guests.
	Dial func(ctx context.Context, cert *tls.Certificate) (net.Conn, error)
	// Account returns the account behind a request (session cookie): its
	// name and client certificate. Nil: everyone is a guest.
	Account func(r *http.Request) (name string, cert *tls.Certificate, ok bool)
	// IsAccountName: guests may not use the names of accounts.
	IsAccountName func(name string) bool
	// Sidecar serves text channels over text frames. Nil: text frames close
	// the connection with 1003.
	Sidecar *sidecar.Hub
	// Refuse a guest whose name another web guest already uses. All web
	// users share our IP, and Murmur kicks the older session of a name
	// from the same IP.
	UniqueGuestNames bool
}

type Bridge struct {
	opts   Options
	mu     sync.Mutex
	active int
	perIP  map[string]int
	names  map[string]bool // lower-case names of connected guests
	closed bool
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
}

func New(opts Options) *Bridge {
	ctx, cancel := context.WithCancel(context.Background())
	return &Bridge{opts: opts, ctx: ctx, cancel: cancel, perIP: map[string]int{}, names: map[string]bool{}}
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

// admit counts a new connection, or returns the HTTP status that refuses
// it. Under the lock, so wg.Add never runs concurrently with Close's wg.Wait.
func (b *Bridge) admit(ip string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch {
	case b.closed || b.active >= b.opts.MaxConnections:
		return http.StatusServiceUnavailable
	case b.perIP[ip] >= b.opts.MaxPerIP:
		return http.StatusTooManyRequests
	}
	b.active++
	b.perIP[ip]++
	b.wg.Add(1)
	return 0
}

func (b *Bridge) release(ip string) {
	b.mu.Lock()
	b.active--
	if b.perIP[ip]--; b.perIP[ip] <= 0 {
		delete(b.perIP, ip)
	}
	b.mu.Unlock()
	b.wg.Done()
}

func (b *Bridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !config.OriginAllowed(b.opts.AllowedOrigins, r.Header.Get("Origin")) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}
	ip := clientip.Of(r, b.opts.TrustForwardedFor)
	if status := b.admit(ip); status != 0 {
		http.Error(w, "too many connections", status)
		return
	}
	defer b.release(ip)

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

	var accountName string
	var cert *tls.Certificate
	isAccount := false
	if b.opts.Account != nil {
		accountName, cert, isAccount = b.opts.Account(r)
	}

	// Read up to Authenticate first. Accounts must use their own name (their
	// certificate registers it on Murmur). Guests may not use an account's
	// name, nor a name another web guest has.
	var hello []byte
	if b.opts.UniqueGuestNames {
		name, early, ok := b.readHello(ws)
		if !ok {
			return
		}
		reject := func(typ pb.Reject_RejectType, reason string) {
			ws.Write(b.ctx, websocket.MessageBinary, mumble.RejectFrame(typ, reason))
			ws.Close(websocket.StatusNormalClosure, "rejected")
		}
		switch {
		case isAccount && !strings.EqualFold(name, accountName):
			reject(pb.Reject_InvalidUsername, "Use your account name, "+accountName+".")
			return
		case !isAccount && b.opts.IsAccountName != nil && b.opts.IsAccountName(name):
			reject(pb.Reject_UsernameInUse, "This name belongs to an account. Log in to use it.")
			return
		case !isAccount:
			if !b.reserveName(name) {
				reject(pb.Reject_UsernameInUse, "This name is already in use.")
				return
			}
			defer b.releaseName(name)
		}
		hello = early
	}

	upstream, err := b.opts.Dial(b.ctx, cert)
	if err != nil {
		slog.Warn("upstream connect failed", "err", err)
		ws.Close(websocket.StatusInternalError, "upstream unavailable")
		return
	}
	defer upstream.Close()
	if _, err := upstream.Write(hello); err != nil {
		ws.Close(websocket.StatusInternalError, "upstream write failed")
		return
	}

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
		c.side.Fail("text channels are not available on this connection")
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

// helloTimeout: a client that does not authenticate in time is dropped.
const helloTimeout = 10 * time.Second

// readHello reads the client's first binary frames up to Authenticate. It
// returns the requested name and all bytes read (to forward unchanged).
func (b *Bridge) readHello(ws *websocket.Conn) (string, []byte, bool) {
	ctx, cancel := context.WithTimeout(b.ctx, helloTimeout)
	defer cancel()
	var auth mumble.AuthReader
	var early []byte
	for {
		typ, data, err := ws.Read(ctx)
		if err != nil {
			ws.Close(websocket.StatusPolicyViolation, "no authentication")
			return "", nil, false
		}
		if typ != websocket.MessageBinary {
			continue // sidecar requests before the login: nothing to answer yet
		}
		early = append(early, data...)
		name, done, err := auth.Feed(data)
		if err != nil {
			ws.Close(websocket.StatusProtocolError, "invalid Mumble stream")
			return "", nil, false
		}
		if done {
			return name, early, true
		}
	}
}

func (b *Bridge) reserveName(name string) bool {
	key := strings.ToLower(name)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.names[key] {
		return false
	}
	b.names[key] = true
	return true
}

func (b *Bridge) releaseName(name string) {
	b.mu.Lock()
	delete(b.names, strings.ToLower(name))
	b.mu.Unlock()
}
