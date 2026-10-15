package sidecar

import (
	"context"
	"regexp"
	"time"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/preview"
)

// Previewer makes link previews (see package preview).
type Previewer interface {
	Get(ctx context.Context, url string) *preview.Preview
}

const (
	maxPreviewAsks = pageSize // one page of history per request
	previewWait    = 15 * time.Second
)

type wirePreview struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Site        string `json:"site,omitempty"`
	Image       string `json:"image,omitempty"` // relative to the server
}

// The first http(s) link of a message text, as the web client links it:
// trailing punctuation, brackets and Markdown markers are not part of it.
var linkPattern = regexp.MustCompile(`https?://[^\s<>"]+[^\s<>".,:;!?)\]'*_~]`)

func firstLink(text string) string { return linkPattern.FindString(text) }

// previews answers with a "preview" event per asked message that has a
// link and that the user may read (preview null: none), as each is ready.
func (c *Conn) previews(id mumble.Identity, req request) {
	if c.hub.Previews == nil {
		return
	}
	asks := req.Messages[:min(len(req.Messages), maxPreviewAsks)]
	for _, mid := range asks {
		m, err := c.hub.store.MessageByID(mid)
		if err != nil {
			continue
		}
		link := firstLink(m.Text)
		voice, isVoice, ok := c.hub.chatOf(m.ChannelID)
		if link == "" || !ok || isVoice && !c.hub.voice.MayRead(id.Session, voice) {
			continue
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), previewWait)
			defer cancel()
			var out *wirePreview
			if p := c.hub.Previews.Get(ctx, link); p != nil {
				out = &wirePreview{URL: p.URL, Title: p.Title, Description: p.Description, Site: p.Site}
				if p.Image != "" {
					out.Image = "/api/link-image/" + p.Image
				}
			}
			c.send(map[string]any{"type": "preview", "channel": m.ChannelID, "message": m.ID, "preview": out})
		}()
	}
}
