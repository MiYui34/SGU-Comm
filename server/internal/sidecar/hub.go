// Package sidecar serves the Kallisti features that Mumble does not have
// (text channels with history) over JSON text frames on the bridge
// WebSocket.
package sidecar

import (
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/store"
)

const (
	MaxTextLength = 2000 // characters, like Discord
	pageSize      = 50
	outboxSize    = 64
)

type Hub struct {
	store *store.Store
	now   func() time.Time
	mu    sync.Mutex
	conns map[*Conn]struct{}
}

func NewHub(s *store.Store, now func() time.Time) *Hub {
	return &Hub{store: s, now: now, conns: map[*Conn]struct{}{}}
}

// Conn is the sidecar side of one bridge connection. Outgoing messages go
// to a bounded outbox; the bridge writes them to the WebSocket.
type Conn struct {
	hub      *Hub
	out      chan []byte
	mu       sync.Mutex
	identity *mumble.Identity
	closed   bool
}

func (h *Hub) Connect() *Conn {
	return &Conn{hub: h, out: make(chan []byte, outboxSize)}
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
	c.mu.Lock()
	first := c.identity == nil
	c.identity = &id
	c.mu.Unlock()
	if !first {
		return
	}
	channels, err := c.hub.store.Channels()
	if err != nil {
		slog.Error("load channels", "err", err)
		c.sendError("text channels are not available")
		return
	}
	wire := make([]wireChannel, len(channels))
	for i, ch := range channels {
		wire[i] = wireChannel{ID: ch.ID, Name: ch.Name}
	}
	c.send(map[string]any{"type": "ready", "channels": wire})
	c.hub.mu.Lock()
	c.hub.conns[c] = struct{}{}
	c.hub.mu.Unlock()
}

type wireChannel struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type wireMessage struct {
	ID      int64  `json:"id"`
	Channel int64  `json:"channel"`
	Author  string `json:"author"`
	Text    string `json:"text"`
	At      int64  `json:"at"` // Unix milliseconds
}

func toWire(m store.Message) wireMessage {
	return wireMessage{ID: m.ID, Channel: m.ChannelID, Author: m.Author, Text: m.Text, At: m.CreatedAt.UnixMilli()}
}

type request struct {
	Type    string `json:"type"`
	Channel int64  `json:"channel"`
	Before  int64  `json:"before"`
	Text    string `json:"text"`
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
	channels, err := h.store.Channels()
	if err != nil {
		return false
	}
	for _, ch := range channels {
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
	if text == "" || utf8.RuneCountInString(text) > MaxTextLength {
		c.sendError("message must be 1 to 2000 characters")
		return
	}
	m, err := c.hub.store.AddMessage(req.Channel, id.Name, text, c.hub.now())
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
