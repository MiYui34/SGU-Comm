// Package sidecar serves the Kallisti features that Mumble does not have
// (text channels with history) over JSON text frames on the bridge
// WebSocket.
package sidecar

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/time/rate"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/store"
)

const (
	MaxTextLength = 2000 // characters, like Discord
	// Small pages: a page goes out as one frame on the voice connection.
	pageSize   = 30
	outboxSize = 64
	// Requests per connection: a steady rate with a short burst.
	perSecond = 5
	burst     = 10
	// An upload token is valid this long, for one upload.
	tokenLifetime = 5 * time.Minute
)

type Hub struct {
	store *store.Store
	now   func() time.Time
	// Channels are fixed at startup.
	channels []wireChannel
	mu       sync.Mutex
	conns    map[*Conn]struct{}
	tokens   map[string]uploadToken
}

type uploadToken struct {
	author  string
	expires time.Time
}

func NewHub(s *store.Store, now func() time.Time) (*Hub, error) {
	channels, err := s.Channels()
	if err != nil {
		return nil, err
	}
	wire := make([]wireChannel, len(channels))
	for i, ch := range channels {
		wire[i] = wireChannel{ID: ch.ID, Name: ch.Name}
	}
	return &Hub{store: s, now: now, channels: wire, conns: map[*Conn]struct{}{}, tokens: map[string]uploadToken{}}, nil
}

func (h *Hub) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.conns)
}

// Conn is the sidecar side of one bridge connection. Outgoing messages go
// to a bounded outbox; the bridge writes them to the WebSocket.
type Conn struct {
	hub      *Hub
	out      chan []byte
	limit    *rate.Limiter
	mu       sync.Mutex
	identity *mumble.Identity
	closed   bool
}

func (h *Hub) Connect() *Conn {
	return &Conn{hub: h, out: make(chan []byte, outboxSize), limit: rate.NewLimiter(perSecond, burst)}
}

// Outbox is closed when the connection closes.
func (c *Conn) Outbox() <-chan []byte { return c.out }

func (c *Conn) Close() {
	c.hub.mu.Lock()
	delete(c.hub.conns, c)
	c.hub.mu.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.out)
	}
}

// SetIdentity is called when the Mumble identity is known or changes. The
// first call makes the connection ready.
func (c *Conn) SetIdentity(id mumble.Identity) {
	// Same lock order as Close (hub, then conn): a connection that closed
	// meanwhile is never registered.
	c.hub.mu.Lock()
	c.mu.Lock()
	first := c.identity == nil && !c.closed
	c.identity = &id
	if first {
		c.hub.conns[c] = struct{}{}
	}
	c.mu.Unlock()
	c.hub.mu.Unlock()
	if first {
		c.send(map[string]any{"type": "ready", "channels": c.hub.channels})
	}
}

type wireChannel struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type wireMessage struct {
	ID      int64       `json:"id"`
	Channel int64       `json:"channel"`
	Author  string      `json:"author"`
	Text    string      `json:"text"`
	At      int64       `json:"at"` // Unix milliseconds
	Upload  *wireUpload `json:"upload,omitempty"`
}

type wireUpload struct {
	URL  string `json:"url"`
	Name string `json:"name"`
	Type string `json:"type"`
	Size int64  `json:"size"`
}

// FileURL is where the server serves an upload (relative to the server).
func FileURL(u store.Upload) string {
	return "/files/" + u.Key + "/" + url.PathEscape(u.Name)
}

func toWire(m store.Message) wireMessage {
	w := wireMessage{ID: m.ID, Channel: m.ChannelID, Author: m.Author, Text: m.Text, At: m.CreatedAt.UnixMilli()}
	if u := m.Upload; u != nil {
		w.Upload = &wireUpload{URL: FileURL(*u), Name: u.Name, Type: u.Type, Size: u.Size}
	}
	return w
}

type request struct {
	Type    string `json:"type"`
	Channel int64  `json:"channel"`
	Before  int64  `json:"before"`
	Text    string `json:"text"`
	Upload  string `json:"upload"` // key of an upload to attach
}

// Handle answers one request from the browser.
func (c *Conn) Handle(raw []byte) {
	var req request
	if err := json.Unmarshal(raw, &req); err != nil {
		c.sendError("invalid request")
		return
	}
	c.mu.Lock()
	id := c.identity
	c.mu.Unlock()
	if id == nil {
		c.sendError("not ready")
		return
	}
	if !c.limit.Allow() {
		c.sendError("too many requests, slow down")
		return
	}
	if req.Type == "upload-token" {
		c.send(map[string]any{"type": "upload-token", "token": c.hub.issueUploadToken(id.Name)})
		return
	}
	if req.Type != "send" && req.Type != "history" {
		c.sendError("unknown request type")
		return
	}
	if !c.hub.channelExists(req.Channel) {
		c.sendError("unknown channel")
		return
	}
	switch req.Type {
	case "history":
		c.history(req)
	case "send":
		c.post(*id, req)
	}
}

func (h *Hub) channelExists(id int64) bool {
	for _, ch := range h.channels {
		if ch.ID == id {
			return true
		}
	}
	return false
}

func (c *Conn) history(req request) {
	// One extra row tells whether there are older messages.
	msgs, err := c.hub.store.History(req.Channel, req.Before, pageSize+1)
	if err != nil {
		c.sendError("cannot load history")
		return
	}
	more := len(msgs) > pageSize
	if more {
		msgs = msgs[1:]
	}
	wire := make([]wireMessage, len(msgs))
	for i, m := range msgs {
		wire[i] = toWire(m)
	}
	c.send(map[string]any{"type": "history", "channel": req.Channel, "messages": wire, "more": more})
}

func (c *Conn) post(id mumble.Identity, req request) {
	text := strings.TrimSpace(req.Text)
	// A message with an attachment may have no text.
	if (text == "" && req.Upload == "") || utf8.RuneCountInString(text) > MaxTextLength {
		c.sendError("message must be 1 to 2000 characters")
		return
	}
	var m store.Message
	var err error
	if req.Upload == "" {
		m, err = c.hub.store.AddMessage(req.Channel, id.Name, text, c.hub.now())
	} else {
		u, uerr := c.hub.store.UploadByKey(req.Upload)
		if uerr != nil || u.Author != id.Name {
			c.sendError("unknown upload")
			return
		}
		m, err = c.hub.store.AddMessageWithUpload(req.Channel, id.Name, text, c.hub.now(), u.ID)
	}
	if err != nil {
		c.sendError("cannot store message")
		return
	}
	c.hub.broadcast(map[string]any{"type": "message", "message": toWire(m)})
}

func (h *Hub) broadcast(v any) {
	h.mu.Lock()
	conns := make([]*Conn, 0, len(h.conns))
	for c := range h.conns {
		conns = append(conns, c)
	}
	h.mu.Unlock()
	for _, c := range conns {
		c.send(v)
	}
}

func (c *Conn) sendError(msg string) {
	c.send(map[string]any{"type": "error", "message": msg})
}

// send never blocks: a client that does not read its outbox loses messages
// (it can reload the history) instead of holding up everyone else.
func (c *Conn) send(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		slog.Error("encode sidecar message", "err", err)
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	select {
	case c.out <- b:
	default:
		slog.Warn("sidecar outbox full, message dropped")
	}
}

func (h *Hub) issueUploadToken(author string) string {
	b := make([]byte, 24)
	rand.Read(b)
	token := hex.EncodeToString(b)
	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.now()
	for t, v := range h.tokens { // drop expired tokens
		if now.After(v.expires) {
			delete(h.tokens, t)
		}
	}
	h.tokens[token] = uploadToken{author: author, expires: now.Add(tokenLifetime)}
	return token
}

// RedeemUploadToken returns the author a token was issued to. A token works
// once and only before it expires.
func (h *Hub) RedeemUploadToken(token string) (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	t, ok := h.tokens[token]
	delete(h.tokens, token)
	if !ok || h.now().After(t.expires) {
		return "", false
	}
	return t.author, true
}
