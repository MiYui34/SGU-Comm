// Package sidecar serves the Kallisti features that Mumble does not have
// (text channels with history) over JSON text frames on the bridge
// WebSocket.
package sidecar

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"regexp"
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

// Voice is the built-in Mumble server, for voice channel chats: its
// permissions, and delivery to Mumble clients.
type Voice interface {
	MayRead(session, channel uint32) bool
	MayWrite(session, channel uint32) bool
	SendChannelText(from, channel uint32, text string)
	// MayManage: the user may create, rename, order and delete text
	// channels.
	MayManage(session uint32) bool
	// SetModerator makes a registered user a moderator, or not (admins).
	SetModerator(session, user uint32, on bool) error
	// Roles: registered user ids of admins and moderators.
	Roles() (admins, moderators []uint32)
}

type Hub struct {
	store *store.Store
	now   func() time.Time
	// Nil: no voice channel chats (a Murmur server).
	voice Voice
	// The server's public address, for upload links sent to Mumble clients.
	publicBase string
	// Text channels; changed by admins (with the built-in server). Under mu.
	channels []wireChannel
	mu       sync.Mutex
	// One channel change at a time: store, reload and broadcast in order.
	manageMu sync.Mutex
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

// SetVoice turns on stored voice channel chats. Call it before clients
// connect.
func (h *Hub) SetVoice(v Voice, publicBase string) {
	h.voice, h.publicBase = v, strings.TrimSuffix(publicBase, "/")
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
	ready    bool
	closed   bool
	// Messages were dropped (full outbox): the client must reload.
	lagged bool
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
	// meanwhile is never registered. No name yet: not ready.
	c.hub.mu.Lock()
	c.mu.Lock()
	becomesReady := !c.ready && !c.closed && id.Name != ""
	c.identity = &id
	if becomesReady {
		c.ready = true
		c.hub.conns[c] = struct{}{}
	}
	channels := c.hub.channels
	c.mu.Unlock()
	c.hub.mu.Unlock()
	if becomesReady {
		ready := map[string]any{"type": "ready", "channels": channels, "voiceChats": c.hub.voice != nil}
		if c.hub.voice != nil {
			ready["roles"] = c.hub.roles()
		}
		c.send(ready)
	}
}

// Fail tells the client that text channels do not work on this connection.
func (c *Conn) Fail(reason string) { c.sendError(reason) }

type wireChannel struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type wireMessage struct {
	ID       int64       `json:"id"`
	Channel  int64       `json:"channel"`
	Author   string      `json:"author"`
	Text     string      `json:"text"`
	At       int64       `json:"at"` // Unix milliseconds
	Verified bool        `json:"verified"`
	Upload   *wireUpload `json:"upload,omitempty"`
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
	w := wireMessage{ID: m.ID, Channel: m.ChannelID, Author: m.Author, Text: m.Text, At: m.CreatedAt.UnixMilli(), Verified: m.Verified}
	if u := m.Upload; u != nil {
		w.Upload = &wireUpload{URL: FileURL(*u), Name: u.Name, Type: u.Type, Size: u.Size}
	}
	return w
}

type request struct {
	Type      string  `json:"type"`
	Name      string  `json:"name"`     // create-channel, rename-channel
	Channels  []int64 `json:"channels"` // order-channels
	Voice     uint32  `json:"voice"`    // voice-chat: the Mumble channel
	Channel   int64   `json:"channel"`
	Before    int64   `json:"before"`
	Text      string  `json:"text"`
	Upload    string  `json:"upload"`    // key of an upload to attach
	User      uint32  `json:"user"`      // set-moderator: registered user id
	Moderator bool    `json:"moderator"` // set-moderator
}

// Handle answers one request from the browser.
func (c *Conn) Handle(raw []byte) {
	var req request
	if err := json.Unmarshal(raw, &req); err != nil {
		c.sendError("invalid request")
		return
	}
	c.mu.Lock()
	id, ready := c.identity, c.ready
	c.mu.Unlock()
	if !ready {
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
	if req.Type == "voice-chat" {
		c.voiceChat(*id, req.Voice)
		return
	}
	switch req.Type {
	case "create-channel", "rename-channel", "delete-channel", "order-channels":
		c.manage(*id, req)
		return
	case "set-moderator":
		// The voice server reports the change (RolesChanged).
		if c.hub.voice == nil {
			c.manageError("roles need the built-in Mumble server")
		} else if err := c.hub.voice.SetModerator(id.Session, req.User, req.Moderator); err != nil {
			c.manageError(err.Error())
		}
		return
	}
	if req.Type != "send" && req.Type != "history" {
		c.sendError("unknown request type")
		return
	}
	voice, isVoice, ok := c.hub.chatOf(req.Channel)
	if !ok {
		c.sendError("unknown channel")
		return
	}
	if isVoice && !c.hub.mayUse(id.Session, voice, req.Type == "send") {
		c.sendError("you may not use this channel's chat")
		return
	}
	switch req.Type {
	case "history":
		c.history(req)
	case "send":
		c.post(*id, req, voice, isVoice)
	}
}

// chatOf finds a channel: a text channel, or a voice channel's chat.
func (h *Hub) chatOf(channel int64) (voice uint32, isVoice, ok bool) {
	if h.channelExists(channel) {
		return 0, false, true
	}
	if h.voice == nil {
		return 0, false, false
	}
	voice, isVoice, err := h.store.VoiceOf(channel)
	return voice, isVoice, err == nil && isVoice
}

func (h *Hub) mayUse(session, voice uint32, write bool) bool {
	if write {
		return h.voice.MayWrite(session, voice)
	}
	return h.voice.MayRead(session, voice)
}

// voiceChat tells the client which channel holds a voice channel's chat.
func (c *Conn) voiceChat(id mumble.Identity, voice uint32) {
	if c.hub.voice == nil {
		c.sendError("voice channel chats need the built-in Mumble server")
		return
	}
	if !c.hub.voice.MayRead(id.Session, voice) {
		c.sendError("you may not use this channel's chat")
		return
	}
	channel, err := c.hub.store.VoiceChat(voice)
	if err != nil {
		c.sendError("cannot open the chat")
		return
	}
	c.send(map[string]any{"type": "voice-chat", "voice": voice, "channel": channel})
}

// MumbleText stores a text a Mumble client sent to a channel, and shows it
// to the web clients that may read it (see mumbled.Config.OnChannelText).
func (h *Hub) MumbleText(from uint32, name string, registered bool, voice uint32, text string) {
	channel, err := h.store.VoiceChat(voice)
	if err != nil {
		slog.Error("voice channel chat", "channel", voice, "err", err)
		return
	}
	if utf8.RuneCountInString(text) > MaxTextLength {
		text = string([]rune(text)[:MaxTextLength])
	}
	m, err := h.store.AddMessage(store.NewMessage{ChannelID: channel, Author: name, Verified: registered, Text: text, At: h.now()})
	if err != nil {
		slog.Error("store a voice channel message", "err", err)
		return
	}
	h.broadcastVoice(voice, map[string]any{"type": "message", "message": toWire(m)})
}

func (h *Hub) channelExists(id int64) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
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

func (c *Conn) post(id mumble.Identity, req request, voice uint32, isVoice bool) {
	text := strings.TrimSpace(req.Text)
	// A message with an attachment may have no text.
	if (text == "" && req.Upload == "") || utf8.RuneCountInString(text) > MaxTextLength {
		c.sendError("message must be 1 to 2000 characters")
		return
	}
	n := store.NewMessage{ChannelID: req.Channel, Author: id.Name, Verified: id.Registered, Text: text, At: c.hub.now()}
	if req.Upload != "" {
		u, err := c.hub.store.UploadByKey(req.Upload)
		if err != nil || u.Author != id.Name {
			c.sendError("unknown upload")
			return
		}
		n.UploadID = u.ID
	}
	m, err := c.hub.store.AddMessage(n)
	if err != nil {
		c.sendError("cannot store message")
		return
	}
	event := map[string]any{"type": "message", "message": toWire(m)}
	if !isVoice {
		c.hub.broadcast(event)
		return
	}
	c.hub.broadcastVoice(voice, event)
	c.hub.voice.SendChannelText(id.Session, voice, c.hub.mumbleText(m))
}

// mumbleText is a stored message as Mumble clients see it: an upload as a
// link (or its name, without a public address).
func (h *Hub) mumbleText(m store.Message) string {
	if m.Upload == nil {
		return m.Text
	}
	file := "[" + m.Upload.Name + "]"
	if h.publicBase != "" {
		file = h.publicBase + FileURL(*m.Upload)
	}
	return strings.TrimSpace(m.Text + " " + file)
}

// broadcastVoice sends to the connections whose user may read the chat.
func (h *Hub) broadcastVoice(voice uint32, v any) {
	h.mu.Lock()
	conns := make([]*Conn, 0, len(h.conns))
	for c := range h.conns {
		conns = append(conns, c)
	}
	h.mu.Unlock()
	for _, c := range conns {
		c.mu.Lock()
		id := c.identity
		c.mu.Unlock()
		if id != nil && h.voice.MayRead(id.Session, voice) {
			c.send(v)
		}
	}
}

// roles are the registered user ids of admins and moderators.
type roles struct {
	Type       string   `json:"type,omitempty"` // "roles" as an event of its own
	Admins     []uint32 `json:"admins"`
	Moderators []uint32 `json:"moderators"`
}

func (h *Hub) roles() roles {
	admins, moderators := h.voice.Roles()
	// Lists, never null in JSON.
	return roles{Admins: append([]uint32{}, admins...), Moderators: append([]uint32{}, moderators...)}
}

// RolesChanged sends everyone the new admins and moderators.
func (h *Hub) RolesChanged() {
	if h.voice == nil {
		return
	}
	r := h.roles()
	r.Type = "roles"
	h.broadcast(r)
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

// manageError answers a management request (channels, roles): the client
// shows it apart from the chat.
func (c *Conn) manageError(msg string) {
	c.send(map[string]any{"type": "error", "message": msg, "about": "manage"})
}

var resync = []byte(`{"type":"resync"}`)

// send never blocks: a client that does not read its outbox loses messages
// instead of holding up everyone else. It gets a "resync" (reload what you
// show) as soon as there is room again.
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
	if c.lagged {
		if len(c.out) > cap(c.out)-2 { // room for the resync and this message
			return
		}
		c.out <- resync
		c.lagged = false
	}
	select {
	case c.out <- b:
	default:
		c.lagged = true
		slog.Warn("sidecar outbox full, client will resync")
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

// Text channel names as Discord has them: lower case, "-" for spaces.
var channelNameChars = regexp.MustCompile(`^[\p{L}\p{N}_-]{1,32}$`)

func textChannelName(raw string) (string, bool) {
	name := strings.ToLower(strings.Join(strings.Fields(raw), "-"))
	return name, channelNameChars.MatchString(name)
}

// manage changes the text channels (admins only) and sends everyone the
// new list.
func (c *Conn) manage(id mumble.Identity, req request) {
	h := c.hub
	if h.voice == nil || !h.voice.MayManage(id.Session) {
		c.manageError("you may not manage channels")
		return
	}
	h.manageMu.Lock()
	defer h.manageMu.Unlock()
	var err error
	switch req.Type {
	case "create-channel", "rename-channel":
		name, ok := textChannelName(req.Name)
		if !ok {
			c.manageError("a channel name has 1 to 32 letters, digits, - or _")
			return
		}
		if req.Type == "create-channel" {
			_, err = h.store.CreateChannel(name)
		} else {
			err = h.store.RenameChannel(req.Channel, name)
		}
	case "delete-channel":
		err = h.store.HideChannel(req.Channel)
	case "order-channels":
		err = h.store.SetPositions(h.fullOrder(req.Channels))
	}
	switch {
	case errors.Is(err, store.ErrNameTaken):
		c.manageError("a channel with this name exists")
		return
	case err != nil:
		c.manageError("cannot change the channel")
		return
	}
	if err := h.reloadChannels(); err != nil {
		slog.Error("load text channels", "err", err)
		return
	}
	h.mu.Lock()
	channels := h.channels
	h.mu.Unlock()
	h.broadcast(map[string]any{"type": "channels", "channels": channels})
}

func (h *Hub) reloadChannels() error {
	channels, err := h.store.Channels()
	if err != nil {
		return err
	}
	wire := make([]wireChannel, len(channels))
	for i, ch := range channels {
		wire[i] = wireChannel{ID: ch.ID, Name: ch.Name}
	}
	h.mu.Lock()
	h.channels = wire
	h.mu.Unlock()
	return nil
}

// fullOrder: the given text channels first, the others after them in their
// current order (a partial list must not leave two at one position).
func (h *Hub) fullOrder(first []int64) []int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	known := map[int64]bool{}
	for _, ch := range h.channels {
		known[ch.ID] = true
	}
	var out []int64
	for _, id := range first {
		if known[id] {
			out = append(out, id)
			delete(known, id)
		}
	}
	for _, ch := range h.channels {
		if known[ch.ID] {
			out = append(out, ch.ID)
		}
	}
	return out
}
