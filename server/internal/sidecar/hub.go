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
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/time/rate"

	"kallisti.chat/server/internal/msgrules"
	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/push"
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
	// MessagesOf: whose messages the actor may delete at once: a user by
	// session (0: a registered user by id); registered users come back by
	// user id, guests by session.
	MessagesOf(actor, session, user uint32) (registered bool, id uint32, err error)
	// SetModerator makes a registered user a moderator, or not (admins).
	SetModerator(session, user uint32, on bool) error
	// SetAdmin makes a registered user an admin (admins), or not
	// (SuperUser).
	SetAdmin(session, user uint32, on bool) error
	// Roles: registered user ids of admins and moderators.
	Roles() (admins, moderators []uint32)
	// ModeratesIn: the user is staff or a moderator of the Mumble channel
	// (or one above it). ChannelModerators lists each channel's own
	// moderators; SetChannelModerator makes a registered user one, or not
	// (admins and moderators).
	ModeratesIn(session, channel uint32) bool
	ChannelModerators() []ChannelModerators
	SetChannelModerator(session, user, channel uint32, on bool) error
	// SetListenOnly makes a voice channel listen-only for all but the
	// given registered users, or not (admins and moderators).
	SetListenOnly(session, channel uint32, on bool, speakers []uint32) error
	// ListenOnly lists the listen-only voice channels (speaker lists are
	// never nil).
	ListenOnly() []ListenOnlyChannel
	// SyncTextTree keeps a Mumble channel per category and per text
	// channel (in its category's, in order) and returns them.
	SyncTextTree(cats []MirrorCategory, want []Mirror) (catIDs, ids []uint32, err error)
	// Sees: the user may see a Mumble channel (others are hidden from it;
	// a private text channel is its mirror's access). Private lists the
	// private channels; SetPrivate makes one private with these members
	// (registered user ids), or public; CreatePrivateChannel makes a
	// private voice channel (below parent; channel moderators in their
	// channels). Only admins and moderators change them.
	Sees(session, channel uint32) bool
	// UserSees and UserMayRead are Sees and MayRead for a registered user
	// by id, also when it is not connected (push notifications).
	UserSees(user, channel uint32) bool
	UserMayRead(user, channel uint32) bool
	// UserBanned: a registered user is banned (it gets no pushes).
	UserBanned(user uint32) bool
	Private() []PrivateChannel
	SetPrivate(session, channel uint32, on bool, members []uint32) error
	CreatePrivateChannel(session, parent uint32, name string) (uint32, error)
	// AddMember adds a registered user to a private channel's members (by
	// an invite link, see invites.go); false: it already was.
	AddMember(channel, user uint32) (bool, error)
	// SendServerText sends HTML to one Mumble user, from the server.
	SendServerText(session uint32, html string)
	// SendPrivateText sends plain text to a registered user's Mumble
	// clients as a private message from a session (named in the text if
	// it is gone); false: nobody got it.
	SendPrivateText(from uint32, fromName string, toUser uint32, text string) bool
	// UserNames: the names of registered users (unknown ids are left out).
	UserNames(ids []uint32) map[uint32]string
	// Users: all registered users' names but SuperUser's, by id. Bio: a
	// registered user's saved comment ("": none). Pictures: the registered
	// users with a saved picture (at /api/avatars/<name>).
	Users() map[uint32]string
	Bio(user uint32) string
	Pictures() map[uint32]bool
	// Online: the connected registered users but SuperUser, by id, in any
	// channel.
	Online() map[uint32]OnlineUser
	// MumbleClient is who the session is (also a web session's), and
	// whether it is a Mumble client (not a web client).
	MumbleClient(session uint32) (MumbleUser, bool)
	// Ban bans a user for a time (0: forever), if the actor may.
	Ban(actor, session uint32, reason string, d time.Duration) error
	// SetTimeout times a user out for a time, or ends it (0), if the
	// actor may; TimedOut: the session's user is timed out, until when;
	// Timeouts: the running time-outs.
	SetTimeout(actor, session uint32, d time.Duration) error
	TimedOut(session uint32) (time.Time, bool)
	Timeouts() []Timeout
	// The same for channel time-outs: in a Mumble channel and below it
	// (staff and the channel's moderators give them).
	SetChannelTimeout(actor, session, channel uint32, d time.Duration) error
	ChannelTimedOut(session, channel uint32) (time.Time, bool)
	ChannelTimeouts() []Timeout
	// SetGuestsReadOnly: guests may only read, or write again (admins).
	// ReadOnly: the session's user is a guest that may not write now.
	SetGuestsReadOnly(session uint32, on bool) error
	GuestsReadOnly() bool
	ReadOnly(session uint32) bool
	// SetCaptcha: the level of the browser check for new accounts
	// (admins); Captcha: the level now.
	SetCaptcha(session uint32, level string) error
	Captcha() string
	// Connections: every connection over the recent window (admins).
	Connections(session uint32) ([]Connection, error)
	// CheckMessage: a session's message to post (with the word
	// replacements), or why the message rules refuse it. MessageRules,
	// SetMessageRules: the rules (admins).
	CheckMessage(session uint32, m msgrules.Message) (string, error)
	MessageRules(session uint32) (msgrules.Rules, error)
	SetMessageRules(session uint32, r msgrules.Rules) (msgrules.Rules, error)
}

// LiveStream is a screen share or camera in a voice channel, by its id
// (the streamer's session; a camera's has stream.CameraBit set too).
type LiveStream struct {
	Session uint32 `json:"session"`
	Channel uint32 `json:"channel"`
	Preview string `json:"preview,omitempty"` // a JPEG data URL; for those who may enter the channel
}

// StreamsChanged sends everyone the live streams in channels it sees
// (previews come with StreamPreview).
func (h *Hub) StreamsChanged(streams []LiveStream) {
	if h.voice == nil {
		return
	}
	h.eachReady(func(c *Conn, id mumble.Identity) {
		out := []LiveStream{}
		for _, s := range streams {
			if h.voice.Sees(id.Session, s.Channel) {
				out = append(out, LiveStream{Session: s.Session, Channel: s.Channel})
			}
		}
		c.send(map[string]any{"type": "streams", "streams": out})
	})
}

// StreamPreview sends a stream's new thumbnail to those who may enter its
// channel.
func (h *Hub) StreamPreview(s LiveStream) {
	h.sendTo(chat{voice: s.Channel, isVoice: true}, map[string]any{"type": "stream-preview", "session": s.Session, "image": s.Preview})
}

// liveStreams for one user: previews only of channels it may enter.
func (h *Hub) liveStreams(id mumble.Identity) []LiveStream {
	out := []LiveStream{}
	for _, s := range h.LiveStreams() {
		if !h.voice.Sees(id.Session, s.Channel) {
			continue
		}
		if !h.voice.MayRead(id.Session, s.Channel) {
			s.Preview = ""
		}
		out = append(out, s)
	}
	return out
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

// OnlineUser is a connected registered user (see Voice.Online).
type OnlineUser struct {
	Name    string
	Session uint32
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
	// Text channels and their categories; changed by admins (with the
	// built-in server). Under mu.
	channels   []wireChannel
	categories []wireCategory
	mu         sync.Mutex
	// One channel change at a time: store, reload and broadcast in order.
	manageMu sync.Mutex
	// Guest messages of this run: message id -> voice server session (see
	// wrote).
	guestMessages map[int64]uint32
	// UploadMax: the size limit per upload, for clients to check first.
	UploadMax int64
	// OnUploadDeleted removes the file of a deleted message's attachment.
	OnUploadDeleted func(key string)
	// Avatars: online users' pictures are at /api/avatars/<name> too, so
	// clients need not fetch them on the voice connection.
	Avatars bool
	// Previews makes link previews (nil: none).
	Previews Previewer
	// Screen shares (built-in server): a token for the stream connection,
	// and the live streams.
	StreamToken func(session uint32) string
	LiveStreams func() []LiveStream
	// Channel recordings (built-in server; nil: none).
	Recorder Recorder
	// SaveFile stores a picture from a Mumble message as an upload.
	SaveFile func(author, name, ctype string, data []byte) (store.Upload, error)
	// Push sends a push notification to a user's browsers, by name (nil:
	// none; see push.go). It must not block.
	Push func(name string, n push.Note)
	// Per voice server session: the last message seen in each mirror.
	visitsMu sync.Mutex
	visits   map[uint32]*visit
	conns    map[*Conn]struct{}
	tokens   map[string]uploadToken
	// One status change at a time, from the change to its broadcast (taken
	// before mu and Conn.mu; see status.go). Also guards online: the
	// members online as last told, with their sessions (see members.go).
	statusMu sync.Mutex
	online   map[uint32]uint32
	// Custom emoji (see emoji.go). Under mu; emojiMu: one reload at a time.
	emoji   []wireEmoji
	emojiMu sync.Mutex
	slow    slowPosts // see slowmode.go
}

// emoji: a token for adding this custom emoji (not for a file).
type uploadToken struct {
	author  string
	emoji   string
	expires time.Time
}

func NewHub(s *store.Store, now func() time.Time) (*Hub, error) {
	h := &Hub{store: s, now: now, conns: map[*Conn]struct{}{}, tokens: map[string]uploadToken{}}
	if err := h.reloadChannels(); err != nil {
		return nil, err
	}
	return h, h.loadEmoji()
}

// SetVoice turns on stored voice channel chats. Call it before clients
// connect.
func (h *Hub) SetVoice(v Voice, publicBase string) {
	h.voice, h.publicBase = v, strings.TrimSuffix(publicBase, "/")
	h.manageMu.Lock()
	defer h.manageMu.Unlock()
	if err := h.syncMirrors(0); err != nil {
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
	limitLog time.Time // the last "over the limit" log line (one a minute)
	mu       sync.Mutex
	identity *mumble.Identity
	ready    bool
	closed   bool
	// Messages were dropped (full outbox): the client must reload.
	lagged  bool
	dropped uint64 // messages lost so far (see Overflows)
	// The last typing signal passed on, per channel (see typing).
	typed map[int64]time.Time
	// The chosen status (kept for registered users) and no input for a
	// while (see status.go).
	status store.UserStatus
	away   bool
	sent   statusView // the status others were told (see syncStatuses)
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
	if !c.closed {
		c.closed = true
		close(c.out)
	}
	c.mu.Unlock()
	c.statusGone()
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
	wasReady := c.ready && !becomesReady
	c.mu.Unlock()
	c.hub.mu.Unlock()
	if wasReady && c.hub.voice != nil {
		c.sendAccess(id) // a registration can change what it sees
	}
	if becomesReady {
		c.sendReady(id, c.hub.readyFor(id))
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
	// Seconds between a user's posts (built-in server; see slowmode.go).
	SlowMode int64 `json:"slowMode,omitempty"`
	Category int64 `json:"category,omitempty"` // see categories.go
}

type wireMessage struct {
	ID         int64          `json:"id"`
	Channel    int64          `json:"channel"`
	Author     string         `json:"author"`
	Text       string         `json:"text"`
	At         int64          `json:"at"` // Unix milliseconds
	Verified   bool           `json:"verified"`
	AuthorID   *uint32        `json:"authorId,omitempty"` // with verified (unknown for old messages)
	Upload     *wireUpload    `json:"upload,omitempty"`
	Edited     int64          `json:"edited,omitempty"` // Unix milliseconds
	Pinned     int64          `json:"pinned,omitempty"` // Unix milliseconds
	Reply      *wireQuote     `json:"reply,omitempty"`
	Reactions  []wireReaction `json:"reactions,omitempty"`
	MentionAll string         `json:"mentionAll,omitempty"` // "here" or "everyone"
	NoPing     bool           `json:"noPing,omitempty"`     // a reply that does not ping
	Thread     *wireThread    `json:"thread,omitempty"`     // the thread started from it
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
	// Pixels, milliseconds: unknown if missing.
	Width    int   `json:"width,omitempty"`
	Height   int   `json:"height,omitempty"`
	Duration int64 `json:"duration,omitempty"`
	// A smaller picture (see files' previews): a video's poster, a large
	// picture for the chat.
	Preview string `json:"preview,omitempty"`
}

// FileURL is where the server serves an upload (relative to the server).
func FileURL(u store.Upload) string {
	return "/files/" + u.Key + "/" + url.PathEscape(u.Name)
}

// previewURL is where its preview is ("": none).
func previewURL(u store.Upload) string {
	if !u.HasPreview() {
		return ""
	}
	return FileURL(u) + "?preview"
}

// UploadPreview: an upload got its preview. If it is attached already,
// its message goes out again with it (as an edit, but not marked edited);
// otherwise the message will carry it when sent.
func (h *Hub) UploadPreview(key string) {
	m, err := h.store.MessageByUpload(key)
	if err != nil {
		return
	}
	if ch, ok := h.chatOf(m.ChannelID); ok {
		h.sendTo(ch, map[string]any{"type": "message-edited", "message": toWire(m)})
	}
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
		w.Upload = &wireUpload{URL: FileURL(*u), Name: u.Name, Type: u.Type, Size: u.Size, Width: u.Width, Height: u.Height, Duration: u.Duration, Preview: previewURL(*u)}
	}
	if len(m.Reactions) > 0 {
		w.Reactions = toWireReactions(m.Reactions)
	}
	w.MentionAll, w.NoPing = m.MentionAll, m.NoPing
	if m.Thread != nil {
		t := toWireThread(*m.Thread)
		t.Quote = nil // this message
		w.Thread = &t
	}
	return w
}

type request struct {
	Type      string          `json:"type"`
	Name      string          `json:"name"`       // create-channel, rename-channel; emoji-token, delete-emoji
	Channels  []int64         `json:"channels"`   // order-channels
	Category  int64           `json:"category"`   // create-channel, set-category, rename-category, delete-category
	Cats      []int64         `json:"categories"` // order-categories
	Voice     uint32          `json:"voice"`      // voice-chat, start-recording, stop-recording: the Mumble channel; create-private-voice: the parent
	Channel   int64           `json:"channel"`
	Before    int64           `json:"before"`
	Text      string          `json:"text"`
	Upload    string          `json:"upload"`    // key of an upload to attach
	Message   int64           `json:"message"`   // edit-message, delete-message, create-thread
	Reply     int64           `json:"reply"`     // send: the message it answers
	NoPing    bool            `json:"noPing"`    // send: the reply does not ping
	Author    string          `json:"author"`    // search: only this author
	User      uint32          `json:"user"`      // set-moderator, set-admin, dm, bio, delete-messages: registered user id
	Moderator bool            `json:"moderator"` // set-moderator
	Admin     bool            `json:"admin"`     // set-admin
	Users     []uint32        `json:"users"`     // set-listen-only: who may speak
	Messages  []int64         `json:"messages"`  // previews
	Emoji     string          `json:"emoji"`     // react
	On        bool            `json:"on"`        // react: add (or remove); pin; away; set-guests-read-only
	Status    string          `json:"status"`    // set-status: "online", "idle" or "dnd"
	Level     string          `json:"level"`     // set-captcha
	Until     int64           `json:"until"`     // set-status: end of the custom status (Unix ms)
	Session   uint32          `json:"session"`   // ban, time-out, delete-messages: the user's voice server session
	Seconds   int64           `json:"seconds"`   // ban (0: forever), time-out (0: end it), delete-messages (the last seconds), set-slow-mode (0: off)
	Private   bool            `json:"private"`   // create-channel: a private one
	Uses      int             `json:"uses"`      // create-invite (0: unlimited; seconds 0: for ever)
	Invite    int64           `json:"invite"`    // delete-invite
	Token     string          `json:"token"`     // redeem-invite
	Rules     *msgrules.Rules `json:"rules"`     // set-message-rules
}

// Handle answers one request from the browser.
// overLimit answers a request over the rate limit, marked so the client
// does not show it as a lasting error (a send comes back for the composer),
// and logs which requests do it, once a minute per connection.
func (c *Conn) overLimit(id mumble.Identity, req request) {
	const text = "too many requests, slow down"
	c.mu.Lock()
	logIt := time.Since(c.limitLog) > time.Minute
	if logIt {
		c.limitLog = time.Now()
	}
	c.mu.Unlock()
	if logIt {
		slog.Warn("requests over the limit", "user", id.Name, "session", id.Session, "type", req.Type)
	}
	if req.Type == "send" {
		c.send(map[string]any{"type": "error", "message": text, "about": "refused", "channel": req.Channel, "text": req.Text})
		return
	}
	c.send(map[string]any{"type": "error", "message": text, "about": "rate"})
}

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
		c.overLimit(*id, req)
		return
	}
	if c.readOnly(*id, req) || c.refused(*id, &req) {
		return
	}
	if req.Type == "upload-token" {
		c.send(map[string]any{"type": "upload-token", "token": c.hub.issueUploadToken(id.Name, "")})
		return
	}
	if chatWrites[req.Type] && c.timedOut(*id, req) {
		return
	}
	if req.Type == "voice-chat" {
		c.voiceChat(*id, req.Voice)
		return
	}
	switch req.Type {
	case "create-channel", "rename-channel", "delete-channel", "order-channels", "set-topic", "set-slow-mode",
		"create-category", "rename-category", "delete-category", "order-categories", "set-category":
		c.manage(*id, req)
		return
	case "mark-read":
		c.markRead(*id, req)
		return
	case "resync":
		c.resendReady(*id)
		return
	case "create-thread", "threads", "get-thread", "rename-thread", "archive-thread", "delete-thread", "follow-thread", "set-thread-archive":
		c.threadRequest(*id, req)
		return
	case "search":
		c.search(*id, req)
		return
	case "edit-message", "delete-message":
		c.changeMessage(*id, req)
		return
	case "delete-messages":
		c.deleteMessagesOf(*id, req)
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
	case "set-moderator", "set-admin":
		// The voice server reports the change (RolesChanged).
		if c.hub.voice == nil {
			c.manageError("roles need the built-in Mumble server")
			return
		}
		set, on := c.hub.voice.SetModerator, req.Moderator
		if req.Type == "set-admin" {
			set, on = c.hub.voice.SetAdmin, req.Admin
		}
		if err := set(id.Session, req.User, on); err != nil {
			c.manageError(err.Error())
		}
		return
	case "ban":
		c.ban(*id, req)
		return
	case "time-out":
		c.timeOut(*id, req)
		return
	case "channel-time-out":
		c.channelTimeOut(*id, req)
		return
	case "set-channel-moderator":
		c.setChannelModerator(*id, req)
		return
	case "dm":
		c.direct(*id, req.User)
		return
	case "bio":
		c.bio(req.User)
		return
	case "set-status":
		c.setStatus(*id, req)
		return
	case "away":
		c.setAway(*id, req.On)
		return
	case "emoji-token":
		c.emojiToken(*id, req.Name)
		return
	case "delete-emoji":
		c.deleteEmoji(*id, req.Name)
		return
	case "stream-token":
		if c.hub.StreamToken == nil {
			c.sendError("screen sharing needs the built-in Mumble server")
		} else {
			c.send(map[string]any{"type": "stream-token", "token": c.hub.StreamToken(id.Session)})
		}
		return
	case "set-private":
		c.setPrivate(*id, req)
		return
	case "start-recording", "stop-recording", "recordings", "recording-token":
		c.recordings(*id, req)
		return
	case "create-private-voice":
		c.createPrivateVoice(*id, req.Voice, req.Name)
		return
	case "invites":
		c.invites(*id, req)
		return
	case "create-invite":
		c.createInvite(*id, req)
		return
	case "delete-invite":
		c.deleteInvite(*id, req)
		return
	case "redeem-invite":
		c.redeemInvite(*id, req.Token)
		return
	case "set-guests-read-only":
		// The voice server reports the change (GuestsReadOnlyChanged).
		if c.hub.voice == nil {
			c.manageError("this setting needs the built-in Mumble server")
		} else if err := c.hub.voice.SetGuestsReadOnly(id.Session, req.On); err != nil {
			c.manageError(err.Error())
		}
		return
	case "set-captcha":
		if c.hub.voice == nil {
			c.manageError("this setting needs the built-in Mumble server")
		} else if err := c.hub.voice.SetCaptcha(id.Session, req.Level); err != nil {
			c.manageError(err.Error())
		} else {
			c.hub.broadcast(map[string]any{"type": "captcha", "level": req.Level})
		}
		return
	case "connections":
		c.connections(id.Session)
		return
	case "message-rules", "set-message-rules":
		c.messageRules(*id, req)
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
	h.storeMumble(from, where, text, images)
}

// storeMumble stores a message from a Mumble client in a chat and sends it
// to the chat's readers. It returns how many messages it stored and the
// last one's id.
func (h *Hub) storeMumble(from mumble.Identity, where chat, text string, images []Image) (n int, last int64) {
	store1 := func(text string, upload int64) {
		m, err := h.store.AddMessage(store.NewMessage{ChannelID: where.id, Author: from.Name, Verified: from.Registered, AuthorID: from.UserID, Text: text, At: h.now(), UploadID: upload})
		if err != nil {
			slog.Error("store a Mumble message", "err", err)
			return
		}
		n, last = n+1, m.ID
		h.sendTo(where, map[string]any{"type": "message", "message": toWire(m)})
		h.pushFor(where, m)
	}
	// Only registered users' images are kept (with their upload quota):
	// guests can use any name. Mumble clients see them live anyway.
	if len(images) == 0 || h.SaveFile == nil || !from.Registered {
		if text != "" {
			store1(text, 0)
		}
		return n, last
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
	return n, last
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
	n := store.NewMessage{ChannelID: req.Channel, Thread: ch.parent != 0, Author: id.Name, Verified: id.Registered, AuthorID: id.UserID, Text: text, At: c.hub.now(), ReplyTo: req.Reply, NoPing: req.NoPing && req.Reply != 0}
	// Not in direct chats: there every message counts already.
	if !ch.direct && c.hub.voice != nil && c.hub.voice.MayManage(id.Session) {
		n.MentionAll = mentionAllOf(text)
	}
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
	if c.slowMode(id, ch, req.Text) {
		return
	}
	m, err := c.hub.store.AddMessage(n)
	if errors.Is(err, store.ErrNotFound) { // a thread deleted meanwhile
		c.sendError("unknown channel")
		return
	}
	if err != nil {
		c.sendError("cannot store message")
		return
	}
	if !id.Registered {
		c.hub.rememberGuest(m.ID, id.Session)
	}
	if ch.direct {
		c.hub.announceDirect(ch)
	}
	if ch.parent != 0 {
		c.hub.threadFollowers(id, ch, m)
	}
	c.hub.sendTo(ch, map[string]any{"type": "message", "message": toWire(m)})
	if ch.direct {
		c.hub.relayDirect(id, ch, m)
	}
	if ch.parent != 0 {
		c.hub.threadPosted(ch)
	}
	c.hub.pushFor(ch, m)
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

// roles are the registered user ids of admins and moderators, and the
// moderators of the channels the user sees.
type roles struct {
	Type              string              `json:"type,omitempty"` // "roles" as an event of its own
	Admins            []uint32            `json:"admins"`
	Moderators        []uint32            `json:"moderators"`
	ChannelModerators []ChannelModerators `json:"channelModerators"`
}

// roles for everyone; channel moderators come with forUser.
func (h *Hub) roles() roles {
	admins, moderators := h.voice.Roles()
	// Lists, never null in JSON.
	return roles{Admins: append([]uint32{}, admins...), Moderators: append([]uint32{}, moderators...), ChannelModerators: h.voice.ChannelModerators()}
}

func (r roles) forUser(h *Hub, id mumble.Identity) roles {
	r.ChannelModerators = slices.DeleteFunc(slices.Clone(r.ChannelModerators), func(m ChannelModerators) bool { return !h.voice.Sees(id.Session, m.Channel) })
	if r.ChannelModerators == nil {
		r.ChannelModerators = []ChannelModerators{}
	}
	return r
}

// RolesChanged sends everyone the new admins and moderators.
func (h *Hub) RolesChanged() {
	if h.voice == nil {
		return
	}
	all := h.roles()
	all.Type = "roles"
	h.eachReady(func(c *Conn, id mumble.Identity) { c.send(all.forUser(h, id)) })
	h.PrivateChanged() // staff see all channels
}

// listenOnly lists the listen-only voice channels (a list, never null in
// JSON; Voice gives speaker lists).
func (h *Hub) listenOnly() []ListenOnlyChannel {
	return append([]ListenOnlyChannel{}, h.voice.ListenOnly()...)
}

// listenOnlyFor: the listen-only channels the user sees.
func (h *Hub) listenOnlyFor(id mumble.Identity) []ListenOnlyChannel {
	return slices.DeleteFunc(h.listenOnly(), func(c ListenOnlyChannel) bool { return !h.voice.Sees(id.Session, c.Channel) })
}

// ListenOnlyChanged sends everyone the listen-only voice channels it sees.
func (h *Hub) ListenOnlyChanged() {
	if h.voice == nil {
		return
	}
	h.eachReady(func(c *Conn, id mumble.Identity) {
		c.send(map[string]any{"type": "listen-only", "channels": h.listenOnlyFor(id)})
	})
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
			c.dropped++
			return
		}
		c.out <- resync
		c.lagged = false
	}
	select {
	case c.out <- b:
	default:
		c.lagged = true
		c.dropped++
		slog.Warn("sidecar outbox full, client will resync")
	}
}

// Overflows is the number of messages each connection lost to a full
// outbox, by voice server session (only those that lost some).
func (h *Hub) Overflows() map[uint32]uint64 {
	out := map[uint32]uint64{}
	for _, c := range h.all() {
		c.mu.Lock()
		if c.dropped > 0 && c.identity != nil {
			out[c.identity.Session] += c.dropped
		}
		c.mu.Unlock()
	}
	return out
}

func (h *Hub) issueUploadToken(author, emoji string) string {
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
	h.tokens[token] = uploadToken{author: author, emoji: emoji, expires: now.Add(tokenLifetime)}
	return token
}

// RedeemUploadToken returns the author a file upload token was issued to.
func (h *Hub) RedeemUploadToken(token string) (string, bool) {
	t, ok := h.redeem(token)
	if !ok || t.emoji != "" {
		return "", false
	}
	return t.author, true
}

// redeem: a token works once and only before it expires.
func (h *Hub) redeem(token string) (uploadToken, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	t, ok := h.tokens[token]
	delete(h.tokens, token)
	return t, ok && !h.now().After(t.expires)
}

// Text channel names as Discord has them: lower case, "-" for spaces.
var channelNameChars = regexp.MustCompile(`^[\p{L}\p{N}_-]{1,32}$`)

func textChannelName(raw string) (string, bool) {
	name := strings.ToLower(strings.Join(strings.Fields(raw), "-"))
	return name, channelNameChars.MatchString(name)
}

const maxTopicLength = 200

// manage changes the text channels and categories (see mayManage) and
// sends everyone the new list.
func (c *Conn) manage(id mumble.Identity, req request) {
	h := c.hub
	// Authorized with the lock held: a channel moved meanwhile is checked
	// in its new category.
	h.manageMu.Lock()
	defer h.manageMu.Unlock()
	if !h.mayManage(id, req) {
		c.manageError("you may not manage channels")
		return
	}
	var err error
	taken := "a channel with this name exists"
	var private int64 // a new private text channel
	// Private channels change too: a new one, or a private one deleted.
	access := false
	switch req.Type {
	case "create-channel", "rename-channel":
		name, ok := textChannelName(req.Name)
		if !ok {
			c.manageError("a channel name has 1 to 32 letters, digits, - or _")
			return
		}
		if req.Type == "create-channel" {
			var ch store.Channel
			// Only staff bring back a deleted channel's history (it may
			// have been private).
			if ch, err = h.store.CreateChannel(name, req.Category, h.voice.MayManage(id.Session)); err == nil && req.Private {
				private, access = ch.ID, true
			}
		} else {
			err = h.store.RenameChannel(req.Channel, name)
		}
	case "delete-channel":
		access = h.isPrivate(h.mirrorOf(req.Channel))
		err = h.store.HideChannel(req.Channel)
	case "set-topic":
		topic := strings.TrimSpace(req.Text)
		if utf8.RuneCountInString(topic) > maxTopicLength {
			c.manageError("a topic has at most 200 characters")
			return
		}
		err = h.store.SetTopic(req.Channel, topic)
	case "set-slow-mode":
		if !slices.Contains(slowModes, req.Seconds) {
			c.manageError("invalid slow mode")
			return
		}
		err = h.store.SetSlowMode(req.Channel, req.Seconds)
	case "order-channels":
		channels, _ := h.orderIDs()
		order := fullOrder(req.Channels, channels)
		if len(req.Channels) > 0 && h.categoryOf(req.Channels[0]) != 0 {
			order = inSlots(req.Channels, channels) // one category's: the others keep their places
		}
		err = h.store.SetPositions(order)
	case "create-category", "rename-category":
		name, ok := categoryName(req.Name)
		if !ok {
			c.manageError("a category name has 1 to 32 letters, digits, spaces or - _ = ( ) [ ] { } @ |")
			return
		}
		taken = "a category with this name exists"
		if req.Type == "create-category" {
			_, err = h.store.CreateCategory(name)
		} else {
			err = h.store.RenameCategory(req.Category, name)
		}
	case "delete-category": // its channels stay, without a category
		access, err = true, h.store.DeleteCategory(req.Category)
	case "order-categories":
		_, cats := h.orderIDs()
		err = h.store.SetCategoryPositions(fullOrder(req.Cats, cats))
	case "set-category": // its moderators change
		access, err = true, h.store.SetChannelCategory(req.Channel, req.Category)
	}
	switch {
	case errors.Is(err, store.ErrNameTaken):
		c.manageError(taken)
		return
	case err != nil:
		c.manageError("cannot change the channel")
		return
	}
	if err := h.syncMirrors(private); err != nil {
		slog.Error("text channel mirrors", "err", err)
	}
	if private != 0 && h.mirrorOf(private) == 0 {
		// Without its private mirror it would be public.
		if err := h.store.HideChannel(private); err != nil {
			slog.Error("hide a text channel", "err", err)
		}
		c.manageError("cannot make the private channel")
	}
	if err := h.reloadChannels(); err != nil {
		slog.Error("load text channels", "err", err)
		return
	}
	h.eachReady(func(c *Conn, id mumble.Identity) {
		if access {
			c.sendAccess(id)
		} else {
			c.send(h.channelsEvent(id))
		}
	})
}

// toWireChannels: mirrors only with the built-in server (the ids may be
// left from it when Murmur runs now).
func toWireChannels(channels []store.Channel, mirrors bool) []wireChannel {
	wire := make([]wireChannel, len(channels))
	for i, ch := range channels {
		wire[i] = wireChannel{ID: ch.ID, Name: ch.Name, Topic: ch.Topic, Category: ch.Category}
		if mirrors {
			wire[i].Mumble, wire[i].SlowMode = ch.Mumble, ch.SlowMode
		}
	}
	return wire
}

func (h *Hub) reloadChannels() error {
	channels, err := h.store.Channels()
	if err != nil {
		return err
	}
	cats, err := h.store.Categories()
	if err != nil {
		return err
	}
	wire := make([]wireCategory, len(cats))
	for i, c := range cats {
		wire[i] = wireCategory{ID: c.ID, Name: c.Name}
		if h.voice != nil {
			wire[i].Mumble = c.Mumble
		}
	}
	h.mu.Lock()
	h.channels, h.categories = toWireChannels(channels, h.voice != nil), wire
	h.mu.Unlock()
	return nil
}

// orderIDs: the text channels' and the categories' ids, in order.
func (h *Hub) orderIDs() (channels, categories []int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, c := range h.channels {
		channels = append(channels, c.ID)
	}
	for _, c := range h.categories {
		categories = append(categories, c.ID)
	}
	return channels, categories
}

// known: the given ids that are in all, once each.
func known(given, all []int64) []int64 {
	var out []int64
	for _, id := range given {
		if slices.Contains(all, id) && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// inSlots: all, with the given ids in the places where they stood, in
// the given order.
func inSlots(given, all []int64) []int64 {
	order := known(given, all)
	out, next := slices.Clone(all), order
	for i, id := range all {
		if slices.Contains(order, id) {
			out[i], next = next[0], next[1:]
		}
	}
	return out
}

// fullOrder: the given ids of all first, the others after them in their
// current order (a partial list must not leave two at one position).
func fullOrder(first, all []int64) []int64 {
	out := known(first, all)
	for _, id := range all {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}
