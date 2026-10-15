package sidecar

import (
	"html"
	"log/slog"
	"strings"
)

// Text channel mirrors: each text channel has a Mumble channel where
// Mumble clients read and write it (the voice server keeps them, see
// Voice.SyncTextChannels). Mumble users get what they missed when they
// enter one.

// Mirror is a text channel's Mumble channel (0: none yet) and name.
type Mirror struct {
	Channel uint32
	Name    string
	Topic   string
}

// noChannel is the voice server's "not on the server" (mumbled.NoChannel).
const noChannel = ^uint32(0)

// historySize: what a Mumble user gets on entering a mirror, at most.
const historySize = 20

// syncMirrors makes the mirrors match the text channels, saves their
// Mumble channels and reloads the list. Call it with manageMu held.
func (h *Hub) syncMirrors() error {
	channels, err := h.store.Channels()
	if err != nil {
		return err
	}
	want := make([]Mirror, len(channels))
	for i, c := range channels {
		want[i] = Mirror{Channel: c.Mumble, Name: c.Name, Topic: c.Topic}
	}
	ids, err := h.voice.SyncTextChannels(want)
	if err != nil {
		return err
	}
	for i, c := range channels {
		if i < len(ids) && ids[i] != 0 && ids[i] != c.Mumble {
			if err := h.store.SetMumbleChannel(c.ID, ids[i]); err != nil {
				return err
			}
		}
	}
	return h.reloadChannels()
}

// textOf finds the text channel a Mumble channel mirrors.
func (h *Hub) textOf(mumble uint32) (wireChannel, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, c := range h.channels {
		if c.Mumble != 0 && c.Mumble == mumble {
			return c, true
		}
	}
	return wireChannel{}, false
}

// mirrorOf is the Mumble channel of a text channel (0: none).
func (h *Hub) mirrorOf(text int64) uint32 {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, c := range h.channels {
		if c.ID == text {
			return c.Mumble
		}
	}
	return 0
}

// UserMoved hears the voice server's users change channels (see
// mumbled.Config.OnUserMoved). Per connection it keeps the last message
// a user saw in each mirror: Mumble clients keep their chat log until
// they disconnect. Moves arrive a little late: a message written just
// then may come twice (live and in the history) or count as seen.
func (h *Hub) UserMoved(session, from, to uint32) {
	h.visitsMu.Lock()
	defer h.visitsMu.Unlock()
	if from == noChannel || to == noChannel {
		delete(h.visits, session)
	}
	if to == noChannel {
		return
	}
	if c, ok := h.textOf(from); ok {
		h.seen(session)[c.ID] = h.lastMessage(c.ID)
	}
	if c, ok := h.textOf(to); ok {
		since, visited := h.seen(session)[c.ID]
		h.sendMissed(session, c, since, visited)
	}
}

func (h *Hub) seen(session uint32) map[int64]int64 {
	if h.visits == nil {
		h.visits = map[uint32]map[int64]int64{}
	}
	if h.visits[session] == nil {
		h.visits[session] = map[int64]int64{}
	}
	return h.visits[session]
}

func (h *Hub) lastMessage(channel int64) int64 {
	msgs, err := h.store.History(channel, 0, 1)
	if err != nil || len(msgs) == 0 {
		return 0
	}
	return msgs[0].ID
}

// sendMissed sends the recent messages of a text channel to a Mumble
// user, as one text: all of the last historySize on a first visit, else
// those after since.
func (h *Hub) sendMissed(session uint32, c wireChannel, since int64, visited bool) {
	msgs, err := h.store.History(c.ID, 0, historySize)
	if err != nil {
		slog.Error("text channel history for Mumble", "err", err)
		return
	}
	var b strings.Builder
	n := 0
	for _, m := range msgs {
		if visited && m.ID <= since {
			continue
		}
		at := m.CreatedAt.UTC().Format("Jan 2 15:04")
		b.WriteString("<br><b>" + html.EscapeString(m.Author) + "</b> <i>" + at + " UTC</i>: " + html.EscapeString(h.mumbleText(m)))
		n++
	}
	if n == 0 {
		return
	}
	head := "Earlier in #"
	if visited {
		head = "While you were away in #"
	}
	h.voice.SendServerText(session, "<b>"+head+html.EscapeString(c.Name)+":</b>"+b.String())
}
