package mumbled

import (
	"errors"
	"html"
	"log/slog"
	"maps"
	"slices"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
	"kallisti.chat/server/internal/recording"
)

// Channel recordings (see package recording): staff start and stop them;
// the voice path hands every packet spoken to a recorded channel to its
// recorder, which never blocks.

// MayRecord: the user may record channels and download recordings (admins
// and moderators).
func (s *Server) MayRecord(session uint32) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.state.Users[session]
	return u != nil && s.cfg.Recordings != nil && s.state.isStaff(u)
}

// Recording: the channels being recorded.
func (s *Server) Recording() []uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Sorted(maps.Keys(s.recordings))
}

// recordableLocked: why the actor may not record the channel (nil: it
// may).
func (s *Server) recordableLocked(actor, channel uint32) error {
	u, ch := s.state.Users[actor], s.state.Channels[channel]
	switch {
	case s.cfg.Recordings == nil:
		return errors.New("recording is not set up on this server")
	case u == nil || !s.state.isStaff(u):
		return errors.New("only admins and moderators record channels")
	case ch == nil || channel == 0 || s.state.mirror(channel):
		return errors.New("only voice channels can be recorded")
	}
	return nil
}

var errRecording = errors.New("this channel is already being recorded")

// startableLocked: why the actor may not start recording the channel.
func (s *Server) startableLocked(actor, channel uint32) error {
	if err := s.recordableLocked(actor, channel); err != nil {
		return err
	}
	if s.recordings[channel] != nil || s.closing.Load() {
		return errRecording
	}
	return nil
}

// StartRecording starts recording a voice channel (one recording per
// channel).
func (s *Server) StartRecording(actor, channel uint32) error {
	s.mu.Lock()
	if err := s.startableLocked(actor, channel); err != nil {
		s.mu.Unlock()
		return err
	}
	by, name := s.state.Users[actor].Name, s.state.Channels[channel].Name
	s.mu.Unlock()
	rec, err := s.cfg.Recordings.Start(channel, name, by, s.recordingEnded)
	if err != nil {
		slog.Error("recording: start", "err", err)
		return errors.New("cannot start the recording")
	}
	s.mu.Lock()
	// Again: the channel may have gone, or another recording started.
	if err := s.startableLocked(actor, channel); err != nil {
		s.mu.Unlock()
		rec.Stop("cancelled")
		return err
	}
	if rec.Ended() { // at a limit already: recordingEnded found nothing
		s.mu.Unlock()
		return errors.New("the recording ended at once")
	}
	if s.recordings == nil {
		s.recordings = map[uint32]*recording.Recording{}
	}
	s.recordings[channel] = rec
	s.channelTextLocked(channel, "🔴 Recording started by "+html.EscapeString(by)+". Everything said in this channel is recorded.")
	s.mu.Unlock()
	s.recordingChanged()
	return nil
}

// StopRecording stops a channel's recording; its files wait for download.
func (s *Server) StopRecording(actor, channel uint32) error {
	s.mu.Lock()
	err := s.recordableLocked(actor, channel)
	rec := s.recordings[channel]
	if err == nil && rec == nil {
		err = errors.New("this channel is not being recorded")
	}
	if err != nil {
		s.mu.Unlock()
		return err
	}
	delete(s.recordings, channel)
	s.mu.Unlock()
	s.endRecording(channel, rec, "stopped")
	return nil
}

// endRecording stops a recording taken out of s.recordings (not under
// s.mu) and tells everyone, unless a new one started meanwhile.
func (s *Server) endRecording(channel uint32, rec *recording.Recording, reason string) {
	info := rec.Stop(reason)
	s.mu.Lock()
	if s.recordings[channel] == nil {
		s.channelTextLocked(channel, recordingStopped(info.Reason))
	}
	s.mu.Unlock()
	s.recordingChanged()
}

func recordingStopped(reason string) string {
	if reason == "stopped" {
		return "⏹ Recording stopped."
	}
	return "⏹ Recording stopped (" + html.EscapeString(reason) + ")."
}

// recordingEnded: a recording stopped by itself (at a limit).
func (s *Server) recordingEnded(info recording.Info) {
	s.mu.Lock()
	if rec := s.recordings[info.Channel]; rec == nil || rec.ID() != info.ID {
		s.mu.Unlock()
		return
	}
	delete(s.recordings, info.Channel)
	s.channelTextLocked(info.Channel, recordingStopped(info.Reason))
	s.mu.Unlock()
	s.recordingChanged()
}

// stopRecordingsLocked takes the removed channels' recordings out; they
// end in the background.
func (s *Server) stopRecordingsLocked(channels []uint32, reason string) {
	for _, ch := range channels {
		if rec := s.recordings[ch]; rec != nil {
			delete(s.recordings, ch)
			s.recordingEnds.Go(func() { s.endRecording(ch, rec, reason) })
		}
	}
}

// stopAllRecordings finishes every recording (server shutdown).
func (s *Server) stopAllRecordings() {
	s.mu.Lock()
	recs := s.recordings
	s.recordings = nil
	s.mu.Unlock()
	for _, rec := range recs {
		rec.Stop("server shutdown")
	}
	s.recordingEnds.Wait()
}

// channelTextLocked tells the Mumble clients in a channel (web clients
// show the indicator instead).
func (s *Server) channelTextLocked(channel uint32, text string) {
	for _, u := range s.state.Users {
		if u.Channel == channel {
			s.mumbleTextLocked(u.Session, channel, text)
		}
	}
}

func (s *Server) mumbleTextLocked(session, channel uint32, text string) {
	if c := s.clients[session]; c != nil && !c.web {
		c.sendMsg(mumble.TypeTextMessage, &pb.TextMessage{ChannelId: []uint32{channel}, Message: proto.String(text)})
	}
}

// tellRecordingLocked tells a Mumble client that entered a channel that
// it is being recorded.
func (s *Server) tellRecordingLocked(session, channel uint32) {
	if s.recordings[channel] != nil {
		s.mumbleTextLocked(session, channel, "🔴 This channel is being recorded.")
	}
}

// recorderForLocked: the recording a packet from the user to a target goes to
// (nil: none): what it says to its own channel, if it may speak there.
func (s *Server) recorderForLocked(u *User, target uint32) *recording.Recording {
	rec := s.recordings[u.Channel]
	if rec == nil || target != 0 || u.SelfMute || u.Mute || u.Suppress || !s.state.Has(u, u.Channel, Speak) {
		return nil
	}
	return rec
}

func (s *Server) recordingChanged() {
	if s.cfg.OnRecordingChanged != nil {
		s.cfg.OnRecordingChanged()
	}
}
