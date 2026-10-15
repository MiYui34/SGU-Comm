package sidecar

import (
	"time"

	"kallisti.chat/server/internal/mumble"
)

// Clients send "typing" every 3 s while the user types; one per channel in
// this time goes on to the others (less than 3 s: late or early arrivals
// pass). Nothing is stored.
const typingEvery = 2500 * time.Millisecond

func (c *Conn) typing(id mumble.Identity, channel int64, voice uint32, isVoice bool) {
	now := c.hub.now()
	c.mu.Lock()
	if c.typed == nil {
		c.typed = map[int64]time.Time{}
	}
	last, seen := c.typed[channel]
	if seen && now.Sub(last) < typingEvery {
		c.mu.Unlock()
		return
	}
	c.typed[channel] = now
	c.mu.Unlock()
	event := map[string]any{"type": "typing", "channel": channel, "name": id.Name}
	for _, other := range c.hub.readers(voice, isVoice) {
		if other != c {
			other.send(event)
		}
	}
}
