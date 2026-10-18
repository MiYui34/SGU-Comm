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

// UserChanged tells everyone about a registration made or renamed (name),
// or removed ("") (see mumbled.Config.OnUserChanged).
func (h *Hub) UserChanged(id uint32, name string) {
	if h.voice == nil {
		return
	}
	if name == "" {
		h.broadcast(map[string]any{"type": "member", "id": id, "gone": true})
		return
	}
	if acc, err := h.store.AccountByName(name); err == nil && acc.Temporary {
		return
	}
	h.broadcast(map[string]any{"type": "member", "id": id, "name": name})
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
