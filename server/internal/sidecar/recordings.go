package sidecar

import (
	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/recording"
)

// Recorder records voice channels (built-in server): admins and
// moderators start and stop recordings and download finished ones.
type Recorder interface {
	MayRecord(session uint32) bool
	StartRecording(session, channel uint32) error
	StopRecording(session, channel uint32) error
	// Recording: the channels being recorded. Finished: recordings that
	// may be downloaded, newest first. DownloadToken: a token for one
	// download (/api/recordings/{id}?token=).
	Recording() []uint32
	Finished() []recording.Info
	DownloadToken(session uint32) string
}

// recordingFor: the recorded channels the user sees (a list, never null).
func (h *Hub) recordingFor(id mumble.Identity) []uint32 {
	out := []uint32{}
	for _, ch := range h.Recorder.Recording() {
		if h.voice.Sees(id.Session, ch) {
			out = append(out, ch)
		}
	}
	return out
}

// RecordingChanged sends everyone the recorded channels it sees.
func (h *Hub) RecordingChanged() {
	if h.Recorder == nil || h.voice == nil {
		return
	}
	h.eachReady(func(c *Conn, id mumble.Identity) {
		c.send(map[string]any{"type": "recording", "channels": h.recordingFor(id)})
	})
}

// recordings answers the recording requests. The voice server checks the
// rights to start and stop and reports the change (RecordingChanged).
func (c *Conn) recordings(id mumble.Identity, req request) {
	r := c.hub.Recorder
	if r == nil || c.hub.voice == nil {
		c.manageError("recording needs the built-in Mumble server")
		return
	}
	var err error
	switch req.Type {
	case "start-recording":
		err = r.StartRecording(id.Session, req.Voice)
	case "stop-recording":
		err = r.StopRecording(id.Session, req.Voice)
	case "recordings", "recording-token":
		if !r.MayRecord(id.Session) {
			c.manageError("only admins and moderators download recordings")
		} else if req.Type == "recordings" {
			c.send(map[string]any{"type": "recordings", "recordings": r.Finished()})
		} else {
			c.send(map[string]any{"type": "recording-token", "token": r.DownloadToken(id.Session)})
		}
	}
	if err != nil {
		c.manageError(err.Error())
	}
}
