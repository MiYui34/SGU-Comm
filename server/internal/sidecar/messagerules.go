package sidecar

import (
	"errors"
	"strings"
	"unicode/utf8"

	"kallisti.chat/server/internal/msgrules"
	"kallisti.chat/server/internal/mumble"
)

// Message rules: the built-in server checks sends, edits and uploads (see
// Voice.CheckMessage; it also checks its Mumble clients' texts), and
// replaces words in their texts. Without it (Murmur) there are none.

// refused: the rules refuse a send, an edit or an upload; the author sees
// why. Else req has the text to post.
func (c *Conn) refused(id mumble.Identity, req *request) bool {
	var m msgrules.Message
	switch req.Type {
	case "send":
		m = msgrules.Message{Text: req.Text, Upload: req.Upload != "", MentionAll: mentionAllOf(req.Text) != "", Post: true}
	case "edit-message":
		m = msgrules.Message{Text: req.Text, MentionAll: mentionAllOf(req.Text) != ""}
	case "upload-token":
		m = msgrules.Message{Upload: true}
	default:
		return false
	}
	if c.hub.voice == nil {
		return false
	}
	text, err := c.hub.voice.CheckMessage(id.Session, m)
	if err == nil && text != req.Text && utf8.RuneCountInString(strings.TrimSpace(text)) > MaxTextLength {
		err = errors.New("This message is too long with the word replacements")
	}
	if err != nil {
		if req.Type == "send" { // the text comes back for the composer
			c.send(map[string]any{"type": "error", "message": err.Error(), "about": "refused", "channel": req.Channel, "text": req.Text})
		} else {
			c.sendError(err.Error())
		}
		return true
	}
	req.Text = text
	return false
}

// messageRules answers an admin: the rules, or new ones saved.
func (c *Conn) messageRules(id mumble.Identity, req request) {
	if c.hub.voice == nil {
		c.manageError("message rules need the built-in Mumble server")
		return
	}
	var r msgrules.Rules
	var err error
	if req.Type == "set-message-rules" && req.Rules != nil {
		r, err = c.hub.voice.SetMessageRules(id.Session, *req.Rules)
	} else {
		r, err = c.hub.voice.MessageRules(id.Session)
	}
	if err != nil {
		c.manageError(err.Error())
		return
	}
	if r.Words == nil { // empty lists, not null: the client wants lists
		r.Words = []string{}
	}
	if r.Domains == nil {
		r.Domains = []string{}
	}
	if r.Replacements == nil {
		r.Replacements = []msgrules.Replacement{}
	}
	c.send(map[string]any{"type": "message-rules", "rules": r})
}
