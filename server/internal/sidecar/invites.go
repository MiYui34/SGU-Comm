package sidecar

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"log/slog"
	"time"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/store"
)

// Invite links to private channels (built-in server): admins, moderators
// and the channel's moderators make them; opening one makes the user a
// member. The link holds a random token (192 bits: guessing is hopeless,
// so the request rate limit is all the defence needed); the store keeps
// only its hash.
// Every bad link gets the same answer, which names no channel.

const (
	errInvite      = "This invite is not valid any more"
	maxInviteUses  = 1000
	maxInviteValid = 366 * 24 * time.Hour
)

type wireInvite struct {
	ID      int64  `json:"id"`
	Creator string `json:"creator"`
	Created int64  `json:"created"`           // Unix ms
	Expires int64  `json:"expires,omitempty"` // Unix ms; none: never
	MaxUses int    `json:"maxUses,omitempty"` // none: unlimited
	Uses    int    `json:"uses"`
}

func inviteHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// mayInvite: staff and the channel's moderators, for a private channel;
// else a manage error.
func (c *Conn) mayInvite(id mumble.Identity, channel uint32) bool {
	switch {
	case c.hub.voice == nil:
		c.manageError("invite links need the built-in Mumble server")
	case !c.hub.voice.ModeratesIn(id.Session, channel):
		c.manageError("only moderators manage invite links")
	case !c.hub.isPrivate(channel):
		c.manageError("invite links are for private channels")
	default:
		return true
	}
	return false
}

// sendInvites sends a private channel's valid links (to staff).
func (c *Conn) sendInvites(channel uint32) {
	list, err := c.hub.store.Invites(channel, c.hub.now())
	if err != nil {
		slog.Error("list invites", "err", err)
		c.manageError("cannot list the invite links")
		return
	}
	out := make([]wireInvite, len(list))
	for i, v := range list {
		out[i] = wireInvite{ID: v.ID, Creator: v.Creator, Created: v.CreatedAt.UnixMilli(), MaxUses: v.MaxUses, Uses: v.Uses}
		if !v.ExpiresAt.IsZero() {
			out[i].Expires = v.ExpiresAt.UnixMilli()
		}
	}
	c.send(map[string]any{"type": "invites", "voice": channel, "invites": out})
}

func (c *Conn) invites(id mumble.Identity, req request) {
	if c.mayInvite(id, req.Voice) {
		c.sendInvites(req.Voice)
	}
}

// createInvite makes a link valid for some seconds (0: for ever), for some
// uses (0: unlimited). Only its maker sees the token, once.
func (c *Conn) createInvite(id mumble.Identity, req request) {
	if !c.mayInvite(id, req.Voice) {
		return
	}
	if req.Seconds < 0 || req.Seconds > int64(maxInviteValid/time.Second) || req.Uses < 0 || req.Uses > maxInviteUses {
		c.manageError("invalid invite link")
		return
	}
	b := make([]byte, 24)
	rand.Read(b)
	token := base64.RawURLEncoding.EncodeToString(b)
	now := c.hub.now()
	inv := store.Invite{Channel: req.Voice, CreatorID: id.UserID, Creator: id.Name, CreatedAt: now, MaxUses: req.Uses}
	if req.Seconds > 0 {
		inv.ExpiresAt = now.Add(time.Duration(req.Seconds) * time.Second)
	}
	if _, err := c.hub.store.AddInvite(inviteHash(token), inv); err != nil {
		slog.Error("add invite", "err", err)
		c.manageError("cannot make the invite link")
		return
	}
	c.send(map[string]any{"type": "invite-created", "voice": req.Voice, "token": token})
	c.sendInvites(req.Voice)
}

func (c *Conn) deleteInvite(id mumble.Identity, req request) {
	channel, err := c.hub.store.InviteChannel(req.Invite)
	if err != nil {
		c.manageError("no such invite link")
		return
	}
	if !c.mayInvite(id, channel) {
		return
	}
	if _, err := c.hub.store.DeleteInvite(req.Invite); err != nil {
		c.manageError("no such invite link")
		return
	}
	c.sendInvites(channel)
}

// redeemInvite makes the user a member of the link's channel (the voice
// server tells all, see PrivateChanged) and says which channel to open.
// Members and staff just open it: no use counted.
func (c *Conn) redeemInvite(id mumble.Identity, token string) {
	h := c.hub
	if h.voice == nil || token == "" {
		c.manageError(errInvite)
		return
	}
	if !id.Registered { // members are registered users (guest accounts are)
		c.manageError("log in or join as a guest to use an invite link")
		return
	}
	now := h.now()
	inv, err := h.store.InviteByHash(inviteHash(token), now)
	if err != nil || !h.isPrivate(inv.Channel) {
		c.manageError(errInvite)
		return
	}
	if !h.voice.Sees(id.Session, inv.Channel) {
		// The use counts first, so no limit is overrun; a failed add
		// loses it.
		if err := h.store.UseInvite(inv.ID, now); err != nil {
			c.manageError(errInvite)
			return
		}
		if _, err := h.voice.AddMember(inv.Channel, id.UserID); err != nil {
			slog.Error("add a member by invite", "channel", inv.Channel, "err", err)
			c.manageError(errInvite)
			return
		}
	}
	m := map[string]any{"type": "invite-joined", "voice": inv.Channel}
	if t, ok := h.textOf(inv.Channel); ok {
		m["channel"] = t.ID
	}
	c.send(m)
}

// pruneInvites drops links that do not work any more (after access
// changes: also those of channels no longer private).
func (h *Hub) pruneInvites() {
	var private []uint32
	for _, p := range h.voice.Private() {
		private = append(private, p.Channel)
	}
	if err := h.store.PruneInvites(private, h.now()); err != nil {
		slog.Error("prune invites", "err", err)
	}
}
