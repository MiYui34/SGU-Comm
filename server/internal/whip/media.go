package whip

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4/pkg/media/samplebuilder"
	"golang.org/x/time/rate"

	"kallisti.chat/server/internal/stream"
)

// clock turns a track's RTP timestamps into stream microseconds, counted
// from the arrival of its first packet (arrival: microseconds since the
// session began). Picture and sound each start at their first arrival, so
// they are in step up to the difference in network delay.
type clock struct {
	rate    int64
	started bool
	last    uint32
	ticks   int64
	base    int64
}

func (c *clock) micros(ts uint32, arrival int64) uint64 {
	if !c.started {
		c.started, c.last, c.base = true, ts, arrival
	}
	c.ticks += int64(int32(ts - c.last)) // wraps around
	c.last = ts
	return uint64(max(0, c.base+c.ticks*1_000_000/c.rate))
}

// videoIn builds access units from RTP packets (reordered, H.264
// depacketized to Annex B).
type videoIn struct {
	sb    *samplebuilder.SampleBuilder
	clock clock
	avc   avc
}

const (
	maxLatePackets = 1024 // a 1 MB keyframe (the hub's limit) in 1200-byte packets fits
	maxLate        = 500 * time.Millisecond
)

func newVideoIn() *videoIn {
	return &videoIn{
		sb:    samplebuilder.New(maxLatePackets, &h264Depacketizer{}, 90_000, samplebuilder.WithMaxTimeDelay(maxLate)),
		clock: clock{rate: 90_000},
	}
}

const maxUnit = 1 << 20 // bytes of a picture, and of a NAL unit in fragments (the hub's frame limit)

// h264Depacketizer bounds Pion's FU-A assembly, which appends fragments
// until an end one: a start fragment drops an unfinished unit, and one past
// maxUnit is dropped (a picture of only it comes out empty: lost, so a
// keyframe is asked for).
type h264Depacketizer struct {
	codecs.H264Packet
	frag int  // bytes of the unit in fragments so far
	open bool // its start came
}

func (d *h264Depacketizer) Unmarshal(payload []byte) ([]byte, error) {
	if len(payload) < 2 || payload[0]&0x1f != 28 { // not FU-A
		d.open = false
		return d.H264Packet.Unmarshal(payload)
	}
	if payload[1]&0x80 != 0 {
		d.H264Packet, d.frag, d.open = codecs.H264Packet{}, 0, true
	}
	if d.frag += len(payload); !d.open || d.frag > maxUnit {
		d.H264Packet, d.open = codecs.H264Packet{}, false
		return []byte{}, nil
	}
	if payload[1]&0x40 != 0 {
		d.open = false
	}
	return d.H264Packet.Unmarshal(payload)
}

// timed is a picture with its stream timestamp; lost: pictures before it
// were lost or broken (viewers need a keyframe).
type timed struct {
	picture
	ts   uint64
	lost bool
}

func (v *videoIn) push(p *rtp.Packet, arrival int64) []timed {
	if !v.clock.started { // pictures come out a packet later
		v.clock.micros(p.Timestamp, arrival)
	}
	v.sb.Push(p)
	var out []timed
	for s := v.sb.Pop(); s != nil; s = v.sb.Pop() {
		ts := v.clock.micros(s.PacketTimestamp, arrival)
		pic, err := v.avc.unit(s.Data)
		if err != nil || len(s.Data) == 0 || len(s.Data) > maxUnit {
			out = append(out, timed{lost: true})
			continue
		}
		out = append(out, timed{picture: pic, ts: ts, lost: s.PrevDroppedPackets > 0})
	}
	return out
}

// feed passes a WHIP session's media to the stream hub: the stream starts
// with the first keyframe (its SPS gives the size) and changes with new
// sizes. Pictures come from one goroutine, sound from another.
type feed struct {
	hub     *stream.Hub
	session uint32
	audio   string // "opus" or ""
	hooks   stream.IngestHooks
	bytes   *rate.Limiter
	sounds  *rate.Limiter
	now     func() time.Time
	config  stream.Config // the pictures' goroutine's
	ingest  atomic.Pointer[stream.Ingest]
	sClock  clock // the sound goroutine's
	// The stream starts and stops under mu: a stop while it starts wins.
	mu      sync.Mutex
	stopped bool
}

const soundsPerSec = 100 // as from a stream connection

func newFeed(hub *stream.Hub, session uint32, audio bool, bytesPerSec int, hooks stream.IngestHooks) *feed {
	f := &feed{hub: hub, session: session, hooks: hooks, now: time.Now, sClock: clock{rate: 48_000},
		bytes:  rate.NewLimiter(rate.Limit(bytesPerSec), 2*bytesPerSec),
		sounds: rate.NewLimiter(soundsPerSec, soundsPerSec/2)}
	if audio {
		f.audio = "opus"
	}
	return f
}

// started: the stream is live (it had its first keyframe).
func (f *feed) started() bool { return f.ingest.Load() != nil }

// picture forwards a picture; an error ends the session (the stream
// cannot start, or a new size is refused).
func (f *feed) picture(t timed) error {
	in := f.ingest.Load()
	if t.lost && in != nil {
		in.Dropped()
	}
	if t.data == nil {
		return nil
	}
	changed := false
	if t.codec != "" {
		c := stream.Config{Codec: t.codec, Width: t.width, Height: t.height, Audio: f.audio}
		changed, f.config = c != f.config, c
	}
	switch {
	case in == nil && (!t.key || f.config.Codec == ""):
		return nil // waiting for a keyframe (asked for, see session)
	case in == nil:
		var err error
		if in, err = f.start(); err != nil {
			return err
		}
	case changed:
		if err := in.Configure(f.config); err != nil {
			return sizeError(err, f.config)
		}
	}
	if !f.bytes.AllowN(f.now(), len(t.data)) {
		in.Dropped()
		return nil
	}
	in.Frame(t.key, t.ts, t.data)
	return nil
}

func sizeError(err error, c stream.Config) error {
	return fmt.Errorf("%w (the picture: %dx%d, %s)", err, c.Width, c.Height, c.Codec)
}

// sound forwards an Opus packet (once the stream is live).
func (f *feed) sound(p *rtp.Packet, arrival int64) {
	ts := f.sClock.micros(p.Timestamp, arrival)
	if in := f.ingest.Load(); in != nil && len(p.Payload) > 0 && f.sounds.Allow() {
		in.Sound(ts, p.Payload)
	}
}

// stop ends the stream, if it started.
func (f *feed) stop() {
	f.mu.Lock()
	f.stopped = true
	f.mu.Unlock()
	if in := f.ingest.Load(); in != nil {
		in.Stop()
	}
}

func (f *feed) start() (*stream.Ingest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stopped {
		return nil, errors.New("stopped")
	}
	in, err := f.hub.StartIngest(f.session, f.config, f.hooks)
	if err != nil {
		return nil, sizeError(err, f.config)
	}
	f.ingest.Store(in)
	slog.Info("stream from OBS is live", "session", f.session, "codec", f.config.Codec, "width", f.config.Width, "height", f.config.Height, "audio", f.config.Audio)
	return in, nil
}
