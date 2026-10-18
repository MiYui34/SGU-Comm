package sidecar

import (
	"cmp"
	"log/slog"
	"slices"

	"kallisti.chat/server/internal/names"
)

// The member list shows registered users while they are offline too:
// ready lists them, "member" events follow changes. Guest accounts are
// left out (throwaway accounts, online ones show anyway).

// members are the registered users but guest accounts, by id.
func (h *Hub) members() []wirePeer {
	guests, err := h.store.TemporaryNames()
	if err != nil {
		slog.Error("guest accounts", "err", err)
	}
	out := []wirePeer{}
	for id, name := range h.voice.Users() {
		if !guests[names.Key(name)] {
			out = append(out, wirePeer{id, name})
		}
	}
	slices.SortFunc(out, func(a, b wirePeer) int { return cmp.Compare(a.ID, b.ID) })
	return out
}

// syncOnline tells all but one connection (which gets ready) the members
// who came online, changed session or went offline since last told, and
// returns them: session by user id. Members online in channels a viewer
// cannot see are listed too, without the channel (as Discord). Under
// statusMu, so ready and these events keep their order.
func (h *Hub) syncOnline(skip *Conn) map[uint32]uint32 {
	guests, err := h.store.TemporaryNames()
	if err != nil {
		slog.Error("guest accounts", "err", err)
	}
	now := map[uint32]uint32{}
	for id, u := range h.voice.Online() {
		if !guests[names.Key(u.Name)] {
			now[id] = u.Session
		}
	}
	var events []map[string]any
	for id, session := range now {
		if h.online[id] != session {
			events = append(events, map[string]any{"type": "online", "id": id, "session": session})
		}
	}
	for id := range h.online {
		if _, ok := now[id]; !ok {
			events = append(events, map[string]any{"type": "online", "id": id})
		}
	}
	for _, c := range h.all() {
		if c == skip {
			continue
		}
		for _, e := range events {
			c.send(e)
		}
	}
	h.online = now
	return now
}

// onlineChanged follows joins, leaves and registrations.
func (h *Hub) onlineChanged() {
	h.statusMu.Lock()
	defer h.statusMu.Unlock()
	h.syncOnline(nil)
}

// UserChanged tells everyone about a registration made or renamed (name),
// or removed ("") (see mumbled.Config.OnUserChanged). A new member is
// told online first, so clients do not list it offline for a moment.
func (h *Hub) UserChanged(id uint32, name string) {
	if h.voice == nil {
		return
	}
	h.onlineChanged()
	if name == "" {
		h.broadcast(map[string]any{"type": "member", "id": id, "gone": true})
		return
	}
	if acc, err := h.store.AccountByName(name); err == nil && acc.Temporary {
		return
	}
	h.broadcast(map[string]any{"type": "member", "id": id, "name": name})
}

// AccountPermanent lists a guest account that became a normal account (see
// auth.Service.OnPermanent): its registration has the account's name.
func (h *Hub) AccountPermanent(name string) {
	if h.voice == nil {
		return
	}
	for id, n := range h.voice.Users() {
		if names.Key(n) == names.Key(name) {
			h.UserChanged(id, n)
			return
		}
	}
}

// bio answers a registered user's saved bio (the Mumble comment), for the
// menu of an offline user.
func (c *Conn) bio(user uint32) {
	if c.hub.voice == nil {
		c.sendError("bios need the built-in Mumble server")
		return
	}
	c.send(map[string]any{"type": "bio", "user": user, "text": c.hub.voice.Bio(user)})
}
