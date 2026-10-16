package stream

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"golang.org/x/time/rate"

	"kallisti.chat/server/internal/config"
)

// Handler serves /api/stream: a WebSocket per voice session, separate from
// the voice connection so video never delays voice. The first message is
// {"type":"auth","token":...} with a token from the sidecar; then text
// commands (start, stop, watch, unwatch) and, from a streamer, binary
// frames (see Hub.Frame).
type Handler struct {
	Hub            *Hub
	Redeem         func(token string) (session uint32, ok bool)
	AllowedOrigins []string
	// A streamer's upload, at most (frames above it are dropped).
	BytesPerSecond int
}

const (
	authTimeout = 10 * time.Second
	outFrames   = 32 // frames queued per connection before it counts as full
	outTexts    = 32
)

type command struct {
	Type    string `json:"type"`
	Token   string `json:"token"`
	Session uint32 `json:"session"` // watch, unwatch: the streamer
	Config
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !config.OriginAllowed(h.AllowedOrigins, r.Header.Get("Origin")) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // the origin is checked above
		CompressionMode:    websocket.CompressionDisabled,
	})
	if err != nil {
		return
	}
	defer ws.CloseNow()
	ws.SetReadLimit(maxFrame + 1024)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	authCtx, authCancel := context.WithTimeout(ctx, authTimeout)
	_, data, err := ws.Read(authCtx)
	authCancel()
	var auth command
	if err != nil || json.Unmarshal(data, &auth) != nil || auth.Type != "auth" {
		ws.Close(websocket.StatusPolicyViolation, "no authentication")
		return
	}
	session, ok := h.Redeem(auth.Token)
	if !ok {
		ws.Close(websocket.StatusPolicyViolation, "bad token")
		return
	}

	c := &conn{frames: make(chan []byte, outFrames), texts: make(chan []byte, outTexts)}
	h.Hub.Connect(session, c)
	defer h.Hub.Disconnect(session, c)
	go c.write(ctx, ws, cancel)
	limit := rate.NewLimiter(rate.Limit(h.BytesPerSecond), 2*h.BytesPerSecond)
	for {
		typ, data, err := ws.Read(ctx)
		if err != nil {
			return
		}
		if typ == websocket.MessageBinary {
			if limit.AllowN(time.Now(), len(data)) {
				h.Hub.Frame(session, data)
			}
			continue
		}
		var cmd command
		if json.Unmarshal(data, &cmd) != nil {
			continue
		}
		switch cmd.Type {
		case "start":
			if err := h.Hub.Start(session, cmd.Config); err != nil {
				c.SendText(map[string]any{"type": "error", "message": err.Error()})
			} else {
				c.SendText(map[string]any{"type": "started"})
			}
		case "stop":
			h.Hub.Stop(session)
		case "watch":
			if err := h.Hub.Watch(session, cmd.Session); err != nil {
				c.SendText(map[string]any{"type": "error", "message": err.Error(), "session": cmd.Session})
			}
		case "unwatch":
			h.Hub.Unwatch(session, cmd.Session)
		}
	}
}

// conn queues what goes out; it never blocks the hub.
type conn struct {
	frames chan []byte
	texts  chan []byte
}

func (c *conn) SendFrame(b []byte) bool {
	select {
	case c.frames <- b:
		return true
	default:
		return false
	}
}

func (c *conn) SendText(v any) {
	b, _ := json.Marshal(v) // our own messages always marshal
	select {
	case c.texts <- b:
	default:
		slog.Warn("stream connection full, a message was dropped")
	}
}

func (c *conn) write(ctx context.Context, ws *websocket.Conn, cancel func()) {
	defer cancel()
	for {
		var typ websocket.MessageType
		var b []byte
		select { // control messages first
		case b = <-c.texts:
			typ = websocket.MessageText
		default:
			select {
			case b = <-c.texts:
				typ = websocket.MessageText
			case b = <-c.frames:
				typ = websocket.MessageBinary
			case <-ctx.Done():
				return
			}
		}
		if ws.Write(ctx, typ, b) != nil {
			return
		}
	}
}

// Tokens are single-use, short-lived stream tokens for voice sessions,
// issued over the authenticated voice connection (the sidecar).
type Tokens struct {
	mu     sync.Mutex
	tokens map[string]token
	now    func() time.Time
}

type token struct {
	session uint32
	expires time.Time
}

const tokenLifetime = time.Minute

func NewTokens() *Tokens { return &Tokens{tokens: map[string]token{}, now: time.Now} }

func (t *Tokens) Issue(session uint32) string {
	b := make([]byte, 24)
	rand.Read(b)
	s := hex.EncodeToString(b)
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	for k, v := range t.tokens {
		if now.After(v.expires) {
			delete(t.tokens, k)
		}
	}
	t.tokens[s] = token{session: session, expires: now.Add(tokenLifetime)}
	return s
}

func (t *Tokens) Redeem(s string) (uint32, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	v, ok := t.tokens[s]
	delete(t.tokens, s)
	if !ok || t.now().After(v.expires) {
		return 0, false
	}
	return v.session, true
}
