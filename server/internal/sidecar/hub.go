// Package sidecar serves the Kallisti features that Mumble does not have
// (text channels with history) over JSON text frames on the bridge
// WebSocket.
package sidecar

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
	// SetListenOnly makes a voice channel listen-only for all but the
	// given registered users, or not (admins and moderators).
	SetListenOnly(session, channel uint32, on bool, speakers []uint32) error
	// ListenOnly lists the listen-only voice channels (speaker lists are
	// never nil).
	ListenOnly() []ListenOnlyChannel
	// SyncTextChannels keeps a Mumble channel per text channel (in order)
	// and returns them.
	SyncTextChannels(want []Mirror) ([]uint32, error)
	// SendServerText sends HTML to one Mumble user, from the server.
	SendServerText(session uint32, html string)
	// MumbleClient: the session is a Mumble client (not a web client), and
	// who it is.
	MumbleClient(session uint32) (MumbleUser, bool)
}

// ListenOnlyChannel is a voice channel where only its speakers may speak.
type ListenOnlyChannel struct {
	Channel  uint32    `json:"channel"`
	Speakers []Speaker `json:"speakers"`
}

// Speaker is a registered user who may speak in a listen-only channel.
type Speaker struct {
	ID   uint32 `json:"id"`
	Name string `json:"name"`
}

// MumbleUser is who a Mumble client is (see Voice.MumbleClient).
type MumbleUser struct {
	Registered bool
	UserID     uint32 // with Registered
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
	// Guest messages of this run: message id -> voice server session (see
	// wrote).
	guestMessages map[int64]uint32
	// OnUploadDeleted removes the file of a deleted message's attachment.
	OnUploadDeleted func(key string)
	// Previews makes link previews (nil: none).
	Previews Previewer
	// SaveFile stores a picture from a Mumble message as an upload.
	SaveFile func(author, name, ctype string, data []byte) (store.Upload, error)
	// Per voice server session: the last message seen in each mirror.
	visitsMu sync.Mutex
	visits   map[uint32]*visit
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
	return &Hub{store: s, now: now, channels: toWireChannels(channels, false), conns: map[*Conn]struct{}{}, tokens: map[string]uploadToken{}}, nil
}

// SetVoice turns on stored voice channel chats. Call it before clients
// connect.
func (h *Hub) SetVoice(v Voice, publicBase string) {
	h.voice, h.publicBase = v, strings.TrimSuffix(publicBase, "/")
	h.manageMu.Lock()
	defer h.manageMu.Unlock()
	if err := h.syncMirrors(); err != nil {
		slog.Error("text channel mirrors", "err", err)
	}
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
	// The last typing signal passed on, per channel (see typing).
	typed map[int64]time.Time
	// Ends with the connection (work on its behalf stops); previewing:
	// previews on their way.
	ctx        context.Context
	stop       context.CancelFunc
	previewing int
}

func (h *Hub) Connect() *Conn {
	ctx, stop := context.WithCancel(context.Background())
	return &Conn{hub: h, out: make(chan []byte, outboxSize), limit: rate.NewLimiter(perSecond, burst), ctx: ctx, stop: stop}
}

// Outbox is closed when the connection closes.
func (c *Conn) Outbox() <-chan []byte { return c.out }

func (c *Conn) Close() {
	c.stop()
	c.mu.Lock()
	id := c.identity
	c.mu.Unlock()
	c.hub.mu.Lock()
	delete(c.hub.conns, c)
	// Murmur reuses session numbers: the next guest owns none of these.
	if id != nil && !id.Registered {
		for m, session := range c.hub.guestMessages {
			if session == id.Session {
				delete(c.hub.guestMessages, m)
			}
		}
	}
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
			ready["roles"], ready["listenOnly"] = c.hub.roles(), c.hub.listenOnly()
		}
		if id.Registered {
			ready["reads"], ready["summary"] = c.hub.readState(id)
		}
		c.send(ready)
	}
}

// Fail tells the client that text channels do not work on this connection.
func (c *Conn) Fail(reason string) { c.sendError(reason) }

type wireChannel struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// The Mumble channel that mirrors it (the web client hides it from
	// the voice channels).
	Mumble uint32 `json:"mumble,omitempty"`
	Topic  string `json:"topic,omitempty"`
}

type wireMessage struct {
	ID        int64          `json:"id"`
	Channel   int64          `json:"channel"`
	Author    string         `json:"author"`
	Text      string         `json:"text"`
	At        int64          `json:"at"` // Unix milliseconds
	Verified  bool           `json:"verified"`
	AuthorID  *uint32        `json:"authorId,omitempty"` // with verified (unknown for old messages)
	Upload    *wireUpload    `json:"upload,omitempty"`
	Edited    int64          `json:"edited,omitempty"` // Unix milliseconds
	Pinned    int64          `json:"pinned,omitempty"` // Unix milliseconds
	Reply     *wireQuote     `json:"reply,omitempty"`
	Reactions []wireReaction `json:"reactions,omitempty"`
}

// wireQuote is the start of the message a reply answers (author empty:
// deleted since).
type wireQuote struct {
	ID     int64  `json:"id"`
	Author string `json:"author"`
	Text   string `json:"text"`
}

const quoteLength = 100

func shorten(text string, n int) string {
	if utf8.RuneCountInString(text) <= n {
		return text
	}
	return string([]rune(text)[:n]) + "…"
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
	w := wireMessage{ID: m.ID, Channel: m.ChannelID, Author: m.Author, Text: m.Text, At: m.CreatedAt.UnixMilli(), Verified: m.Verified, AuthorID: m.AuthorID}
	if !m.EditedAt.IsZero() {
		w.Edited = m.EditedAt.UnixMilli()
	}
	if !m.PinnedAt.IsZero() {
		w.Pinned = m.PinnedAt.UnixMilli()
	}
	if r := m.Reply; r != nil {
		w.Reply = &wireQuote{ID: r.ID, Author: r.Author, Text: shorten(r.Text, quoteLength)}
	}
	if u := m.Upload; u != nil {
		w.Upload = &wireUpload{URL: FileURL(*u), Name: u.Name, Type: u.Type, Size: u.Size}
	}
	if len(m.Reactions) > 0 {
		w.Reactions = toWireReactions(m.Reactions)
	}
	return w
}

type request struct {
	Type      string   `json:"type"`
	Name      string   `json:"name"`     // create-channel, rename-channel
	Channels  []int64  `json:"channels"` // order-channels
	Voice     uint32   `json:"voice"`    // voice-chat: the Mumble channel
	Channel   int64    `json:"channel"`
	Before    int64    `json:"before"`
	Text      string   `json:"text"`
	Upload    string   `json:"upload"`    // key of an upload to attach
	Message   int64    `json:"message"`   // edit-message, delete-message
	Reply     int64    `json:"reply"`     // send: the message it answers
	Author    string   `json:"author"`    // search: only this author
	User      uint32   `json:"user"`      // set-moderator: registered user id
	Moderator bool     `json:"moderator"` // set-moderator
	Users     []uint32 `json:"users"`     // set-listen-only: who may speak
	Messages  []int64  `json:"messages"`  // previews
	Emoji     string   `json:"emoji"`     // react
	On        bool     `json:"on"`        // react: add (or remove); pin
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
	case "create-channel", "rename-channel", "delete-channel", "order-channels", "set-topic":
		c.manage(*id, req)
		return
	case "mark-read":
		c.markRead(*id, req)
		return
	case "search":
		c.search(*id, req)
		return
	case "edit-message", "delete-message":
		c.changeMessage(*id, req)
		return
	case "react":
		c.react(*id, req)
		return
	case "pin":
		c.pin(*id, req)
		return
	case "previews":
		c.previews(*id, req)
		return
	case "set-moderator":
		// The voice server reports the change (RolesChanged).
		if c.hub.voice == nil {
			c.manageError("roles need the built-in Mumble server")
		} else if err := c.hub.voice.SetModerator(id.Session, req.User, req.Moderator); err != nil {
			c.manageError(err.Error())
		}
		return
	case "set-listen-only":
		// The voice server reports the change (ListenOnlyChanged).
		if c.hub.voice == nil {
			c.manageError("listen-only channels need the built-in Mumble server")
		} else if err := c.hub.voice.SetListenOnly(id.Session, req.Voice, req.On, req.Users); err != nil {
			c.manageError(err.Error())
		}
		return
	}
	if req.Type != "send" && req.Type != "history" && req.Type != "typing" && req.Type != "pins" {
		c.sendError("unknown request type")
		return
	}
	ch, ok := c.hub.chatOf(req.Channel)
	if !ok {
		if req.Type != "typing" { // a channel deleted while typing
			c.sendError("unknown channel")
		}
		return
	}
	if !c.hub.may(*id, ch, req.Type != "history" && req.Type != "pins") {
		if req.Type != "typing" { // only typing: no error to show
			c.sendError("you may not use this channel's chat")
		}
		return
	}
	switch req.Type {
	case "history":
		c.history(req)
	case "pins":
		c.pins(req)
	case "send":
		c.post(*id, req, ch)
	case "typing":
		c.typing(*id, ch)
	}
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
// In a text channel's mirror, it goes to the text channel (all web users).
// Images become attachments: the first on the text, others on messages of
// their own.
func (h *Hub) MumbleText(from mumble.Identity, channel uint32, text string, images []Image) {
	if utf8.RuneCountInString(text) > MaxTextLength {
		text = string([]rune(text)[:MaxTextLength])
	}
	var where chat
	if c, ok := h.textOf(channel); ok {
		where = chat{id: c.ID}
	} else {
		id, err := h.store.VoiceChat(channel)
		if err != nil {
			slog.Error("voice channel chat", "channel", channel, "err", err)
			return
		}
		where = chat{id: id, voice: channel, isVoice: true}
	}
	store1 := func(text string, upload int64) {
		m, err := h.store.AddMessage(store.NewMessage{ChannelID: where.id, Author: from.Name, Verified: from.Registered, AuthorID: from.UserID, Text: text, At: h.now(), UploadID: upload})
		if err != nil {
			slog.Error("store a Mumble message", "err", err)
			return
		}
		h.sendTo(where, map[string]any{"type": "message", "message": toWire(m)})
	}
	// Only registered users' images are kept (with their upload quota):
	// guests can use any name. Mumble clients see them live anyway.
	if len(images) == 0 || h.SaveFile == nil || !from.Registered {
		if text != "" {
			store1(text, 0)
		}
		return
	}
	for i, img := range images {
		u, err := h.SaveFile(from.Name, fmt.Sprintf("image-%d%s", i+1, imageExt(img.Type)), img.Type, img.Data)
		if err != nil {
			slog.Error("store a Mumble image", "err", err)
			continue
		}
		// If storing the message fails, the upload has no message: the
		// cleanup of unused uploads removes it.
		store1(text, u.ID)
		text = ""
	}
	if text != "" { // no image could be stored
		store1(text, 0)
	}
}

// Image is a picture from a Mumble message (see mumbled.Image).
type Image struct {
	Type string
	Data []byte
}

func imageExt(typ string) string {
	switch typ {
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	}
	return ".jpg"
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

func (c *Conn) post(id mumble.Identity, req request, ch chat) {
	text, ok := validText(req.Text, req.Upload != "")
	if !ok {
		c.sendError(textError)
		return
	}
	n := store.NewMessage{ChannelID: req.Channel, Author: id.Name, Verified: id.Registered, AuthorID: id.UserID, Text: text, At: c.hub.now(), ReplyTo: req.Reply}
	if req.Reply != 0 {
		if orig, err := c.hub.store.MessageByID(req.Reply); err != nil || orig.ChannelID != req.Channel {
			c.sendError("the message to answer is not in this channel")
			return
		}
	}
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
	if !id.Registered {
		c.hub.rememberGuest(m.ID, id.Session)
	}
	c.hub.sendTo(ch, map[string]any{"type": "message", "message": toWire(m)})
	if ch.isVoice {
		c.hub.voice.SendChannelText(id.Session, ch.voice, c.hub.mumbleText(m))
	} else if mirror := c.hub.mirrorOf(req.Channel); mirror != 0 && c.hub.voice != nil {
		c.hub.voice.SendChannelText(id.Session, mirror, c.hub.mumbleText(m))
	}
}

// mumbleText is a stored message as Mumble clients see it: an upload as a
// link (or its name, without a public address).
func (h *Hub) mumbleText(m store.Message) string {
	text := m.Text
	if m.Upload != nil {
		file := "[" + m.Upload.Name + "]"
		if h.publicBase != "" {
			file = h.publicBase + FileURL(*m.Upload)
		}
		text = strings.TrimSpace(text + " " + file)
	}
	// Mumble clients have no replies: a short quote first.
	if r := m.Reply; r != nil && r.Author != "" {
		text = "> " + r.Author + ": " + shorten(r.Text, 60) + " — " + text
	}
	return text
}

// readers are the connections whose user may read a chat (a text
// channel: everyone).

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

// listenOnly lists the listen-only voice channels (a list, never null in
// JSON; Voice gives speaker lists).
func (h *Hub) listenOnly() []ListenOnlyChannel {
	return append([]ListenOnlyChannel{}, h.voice.ListenOnly()...)
}

// ListenOnlyChanged sends everyone the listen-only voice channels.
func (h *Hub) ListenOnlyChanged() {
	if h.voice == nil {
		return
	}
	h.broadcast(map[string]any{"type": "listen-only", "channels": h.listenOnly()})
}

func (h *Hub) broadcast(v any) {
	for _, c := range h.all() {
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

const maxTopicLength = 200

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
	case "set-topic":
		topic := strings.TrimSpace(req.Text)
		if utf8.RuneCountInString(topic) > maxTopicLength {
			c.manageError("a topic has at most 200 characters")
			return
		}
		err = h.store.SetTopic(req.Channel, topic)
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
	if err := h.syncMirrors(); err != nil {
		slog.Error("text channel mirrors", "err", err)
		if err := h.reloadChannels(); err != nil {
			slog.Error("load text channels", "err", err)
			return
		}
	}
	h.mu.Lock()
	channels := h.channels
	h.mu.Unlock()
	h.broadcast(map[string]any{"type": "channels", "channels": channels})
}

// toWireChannels: mirrors only with the built-in server (the ids may be
// left from it when Murmur runs now).
func toWireChannels(channels []store.Channel, mirrors bool) []wireChannel {
	wire := make([]wireChannel, len(channels))
	for i, ch := range channels {
		wire[i] = wireChannel{ID: ch.ID, Name: ch.Name, Topic: ch.Topic}
		if mirrors {
			wire[i].Mumble = ch.Mumble
		}
	}
	return wire
}

func (h *Hub) reloadChannels() error {
	channels, err := h.store.Channels()
	if err != nil {
		return err
	}
	h.mu.Lock()
	h.channels = toWireChannels(channels, h.voice != nil)
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
