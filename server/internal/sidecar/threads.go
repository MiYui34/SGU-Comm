package sidecar

import (
	"errors"
	"log/slog"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/names"
	"kallisti.chat/server/internal/store"
)

// Threads: a side conversation started from a message of a text channel,
// a chat of its own (see store.Thread) with the channel's rights: who
// reads the channel reads its threads, who writes there writes in them
// (slow mode and message rules too), its moderators moderate them. The
// maker renames and archives its thread; staff and the channel's
// moderators also delete it. Mumble clients never see threads.
// Registered users follow the threads they start, write in or are
// mentioned in: those count as unread for them. Threads without messages
// for a while (a staff setting) are archived; a new message opens them
// again.

type wireThread struct {
	ID       int64      `json:"id"`
	Channel  int64      `json:"channel"` // the text channel
	Start    int64      `json:"start"`   // the message it started from
	Name     string     `json:"name"`
	Author   string     `json:"author"`
	AuthorID *uint32    `json:"authorId,omitempty"`
	At       int64      `json:"at"`   // Unix milliseconds
	Last     int64      `json:"last"` // the newest message (or At)
	Replies  int        `json:"replies"`
	Archived bool       `json:"archived,omitempty"`
	Quote    *wireQuote `json:"quote,omitempty"` // the start message (author empty: deleted)
}

func toWireThread(t store.Thread) wireThread {
	return wireThread{ID: t.ID, Channel: t.Parent, Start: t.Start, Name: t.Name, Author: t.Author, AuthorID: t.AuthorID,
		At: t.CreatedAt.UnixMilli(), Last: t.LastAt.UnixMilli(), Replies: t.Replies, Archived: t.Archived,
		Quote: &wireQuote{ID: t.Start, Author: t.StartAuthor, Text: shorten(t.StartText, 300)}}
}

func toWireThreads(ts []store.Thread) []wireThread {
	out := make([]wireThread, len(ts))
	for i, t := range ts {
		out[i] = toWireThread(t)
	}
	return out
}

const maxThreadName = 100

// threadName: one line of 1 to maxThreadName characters.
func threadName(raw string) (string, bool) {
	name := strings.Join(strings.Fields(raw), " ")
	return name, name != "" && utf8.RuneCountInString(name) <= maxThreadName
}

// threadArchives: the choices for archiving idle threads, in seconds.
var threadArchives = []int64{3600, 24 * 3600, 3 * 24 * 3600, 7 * 24 * 3600}

// threadChat is a thread's chat, if its text channel exists.
func (h *Hub) threadChat(t store.Thread) (chat, bool) {
	return chat{id: t.ID, parent: t.Parent}, h.channelExists(t.Parent)
}

// threadsFor: the active threads of the text channels the user reads.
func (h *Hub) threadsFor(id mumble.Identity) []wireThread {
	all, err := h.store.Threads(0, false, 0, 0)
	if err != nil {
		slog.Error("threads", "err", err)
	}
	return toWireThreads(slices.DeleteFunc(all, func(t store.Thread) bool {
		ch, ok := h.threadChat(t)
		return !ok || !h.may(id, ch, false)
	}))
}

// followsOf: the threads a registered user follows (a list, never null).
func (h *Hub) followsOf(user uint32) []int64 {
	ids, err := h.store.Followed(user)
	if err != nil {
		slog.Error("followed threads", "err", err)
	}
	return append([]int64{}, ids...)
}

// sendFollows tells a registered user's connections what it follows.
func (h *Hub) sendFollows(user uint32) {
	event := map[string]any{"type": "follows", "threads": h.followsOf(user)}
	h.eachReady(func(c *Conn, id mumble.Identity) {
		if id.Registered && id.UserID == user {
			c.send(event)
		}
	})
}

// follow makes a registered user follow a thread (or not) and tells it.
func (h *Hub) follow(user uint32, thread int64, on bool) {
	changed, err := h.store.Follow(user, thread, on)
	if err != nil {
		slog.Error("follow a thread", "err", err)
	}
	if changed {
		h.sendFollows(user)
	}
}

// threadChanged sends a thread's readers the thread as it is now.
func (h *Hub) threadChanged(thread int64) {
	t, err := h.store.Thread(thread)
	if err != nil {
		return // deleted meanwhile
	}
	if ch, ok := h.threadChat(t); ok {
		h.sendTo(ch, map[string]any{"type": "thread", "thread": toWireThread(t)})
	}
}

// threadFollowers: before a message in a thread goes out, its registered
// author and those it mentions or answers (who may read it, not by
// @everyone) follow the thread.
func (h *Hub) threadFollowers(author mumble.Identity, ch chat, m store.Message) {
	if author.Registered {
		h.follow(author.UserID, ch.id, true)
	}
	if h.voice == nil || m.Reply == nil && !strings.Contains(m.Text, "@") {
		return
	}
	m.MentionAll = ""
	text := strings.ToLower(m.Text)
	for user, name := range h.voice.Users() {
		// Most users are not named: no pattern for them.
		answered := m.Reply != nil && names.Key(m.Reply.Author) == names.Key(name)
		if !answered && !strings.Contains(text, "@"+strings.ToLower(name)) {
			continue
		}
		if pings(m, false, name, user) && h.userMay(user, ch) {
			h.follow(user, ch.id, true)
		}
	}
}

// threadPosted: after a message in a thread, an archived thread opens
// again, and readers get the new reply count.
func (h *Hub) threadPosted(ch chat) {
	if _, err := h.store.SetThreadArchived(ch.id, false); err != nil {
		slog.Error("open a thread", "err", err)
	}
	h.threadChanged(ch.id)
}

// ArchiveIdleThreads archives the threads idle for longer than the
// setting (call it now and then).
func (h *Hub) ArchiveIdleThreads() {
	after, err := h.store.ThreadArchiveAfter()
	if err != nil {
		slog.Error("thread archive setting", "err", err)
		return
	}
	ids, err := h.store.ArchiveIdleThreads(h.now().Add(-after))
	if err != nil {
		slog.Error("archive threads", "err", err)
	}
	for _, id := range ids {
		h.threadChanged(id)
	}
}

// threadArchive: the setting in seconds.
func (h *Hub) threadArchive() int64 {
	after, err := h.store.ThreadArchiveAfter()
	if err != nil {
		slog.Error("thread archive setting", "err", err)
	}
	return int64(after / time.Second)
}

// threadRequest answers the thread requests.
func (c *Conn) threadRequest(id mumble.Identity, req request) {
	h := c.hub
	switch req.Type {
	case "create-thread":
		c.createThread(id, req)
		return
	case "set-thread-archive":
		switch {
		case h.voice == nil || !h.voice.MayManage(id.Session):
			c.manageError("only admins and moderators change this setting")
		case !slices.Contains(threadArchives, req.Seconds):
			c.manageError("invalid archive time")
		default:
			if err := h.store.SetThreadArchiveAfter(time.Duration(req.Seconds) * time.Second); err != nil {
				c.manageError("cannot change the setting")
				return
			}
			h.broadcast(map[string]any{"type": "thread-archive", "seconds": req.Seconds})
		}
		return
	case "threads": // a text channel's archived threads, a page
		ch, ok := h.chatOf(req.Channel)
		if !ok || ch.isVoice || ch.direct || ch.parent != 0 || !h.may(id, ch, false) {
			c.sendError("unknown channel")
			return
		}
		ts, err := h.store.Threads(req.Channel, true, req.Before, pageSize+1)
		if err != nil {
			c.sendError("cannot load the threads")
			return
		}
		more := len(ts) > pageSize
		c.send(map[string]any{"type": "archived-threads", "channel": req.Channel, "threads": toWireThreads(ts[:min(len(ts), pageSize)]), "more": more})
		return
	}
	t, err := h.store.Thread(req.Channel)
	ch, ok := h.threadChat(t)
	if err != nil || !ok || !h.may(id, ch, false) {
		c.sendError("unknown thread")
		return
	}
	own := id.Registered && t.AuthorID != nil && *t.AuthorID == id.UserID && h.may(id, ch, true)
	mod := h.mayModerate(id, ch)
	switch req.Type {
	case "follow-thread":
		if !id.Registered {
			c.sendError("join with an account (or a guest account) to follow threads")
			return
		}
		h.follow(id.UserID, t.ID, req.On)
	case "rename-thread":
		name, valid := threadName(req.Name)
		switch {
		case !own && !mod:
			c.sendError("you may not rename this thread")
		case !valid:
			c.sendError("a thread name has 1 to 100 characters")
		case h.store.RenameThread(t.ID, name) != nil:
			c.sendError("cannot rename the thread")
		default:
			h.threadChanged(t.ID)
		}
	case "archive-thread":
		if !own && !mod {
			c.sendError("you may not archive this thread")
		} else if changed, err := h.store.SetThreadArchived(t.ID, req.On); err != nil {
			c.sendError("cannot archive the thread")
		} else if changed {
			h.threadChanged(t.ID)
		}
	case "delete-thread":
		if !mod {
			c.sendError("only moderators delete threads")
			return
		}
		keys, err := h.store.DeleteThread(t.ID)
		if err != nil {
			c.sendError("cannot delete the thread")
			return
		}
		for _, k := range keys {
			if h.OnUploadDeleted != nil {
				h.OnUploadDeleted(k)
			}
		}
		h.sendTo(ch, map[string]any{"type": "thread-deleted", "channel": t.Parent, "id": t.ID})
	}
}

// createThread starts a thread from a message of a text channel (those
// who may write there); its maker follows it.
func (c *Conn) createThread(id mumble.Identity, req request) {
	h := c.hub
	m, err := h.store.MessageByID(req.Message)
	ch, ok := h.chatOf(m.ChannelID)
	if err != nil || !ok || ch.isVoice || ch.direct || ch.parent != 0 || !h.may(id, ch, true) {
		c.sendError("you may not start a thread here")
		return
	}
	name, valid := threadName(req.Name)
	if !valid {
		c.sendError("a thread name has 1 to 100 characters")
		return
	}
	var author *uint32
	if id.Registered {
		author = &id.UserID
	}
	t, err := h.store.CreateThread(m.ChannelID, m.ID, name, id.Name, author, h.now())
	switch {
	case errors.Is(err, store.ErrThreadExists):
		c.sendError(err.Error())
		return
	case err != nil:
		c.sendError("cannot start the thread")
		return
	}
	if id.Registered {
		h.follow(id.UserID, t.ID, true)
	}
	h.threadChanged(t.ID)
}

// threadsOf: the threads of messages found (a list, never null).
func (h *Hub) threadsOf(msgs []store.Message, parents map[int64]int64) []wireThread {
	out := []wireThread{}
	for _, m := range msgs {
		if _, ok := parents[m.ChannelID]; !ok || slices.ContainsFunc(out, func(t wireThread) bool { return t.ID == m.ChannelID }) {
			continue
		}
		if t, err := h.store.Thread(m.ChannelID); err == nil {
			out = append(out, toWireThread(t))
		}
	}
	return out
}
