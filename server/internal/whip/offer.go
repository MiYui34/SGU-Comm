package whip

import (
	"errors"
	"strconv"
	"strings"

	"github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v4"
)

// offer: the codecs taken from an encoder's offer, exactly as offered (so
// any H.264 profile matches).
type offer struct {
	video webrtc.RTPCodecParameters
	audio *webrtc.RTPCodecParameters // nil: no sound
}

var errCodecs = errors.New("Kallisti takes one H.264 video track (packetization-mode=1) and at most one Opus audio track")

// checkOffer takes one H.264 video and at most one Opus audio section.
func checkOffer(s string) (offer, error) {
	var d sdp.SessionDescription
	if err := d.UnmarshalString(s); err != nil {
		return offer{}, errors.New("bad SDP offer")
	}
	var o offer
	videos := 0
	for _, m := range d.MediaDescriptions {
		codecs := mediaCodecs(m)
		switch m.MediaName.Media {
		case "video":
			if videos++; videos > 1 {
				return offer{}, errors.New("only one video track, please: " + errCodecs.Error())
			}
			if _, ok := m.Attribute("simulcast"); ok {
				return offer{}, errors.New("turn simulcast off in OBS: " + errCodecs.Error())
			}
			h264 := false
			for _, c := range codecs {
				if strings.EqualFold(c.name, "H264") {
					h264 = true
					if strings.Contains(c.fmtp, "packetization-mode=1") {
						o.video = c.params("video/H264", 0)
						break
					}
				}
			}
			switch {
			case !h264:
				return offer{}, errors.New("no H.264 video: " + errCodecs.Error())
			case o.video.PayloadType == 0:
				return offer{}, errors.New("H.264 needs packetization-mode=1: " + errCodecs.Error())
			}
		case "audio":
			if o.audio != nil {
				return offer{}, errors.New("only one audio track, please: " + errCodecs.Error())
			}
			for _, c := range codecs {
				if strings.EqualFold(c.name, "opus") && c.rate == 48000 {
					p := c.params("audio/opus", 2)
					o.audio = &p
					break
				}
			}
			if o.audio == nil {
				return offer{}, errors.New("no Opus audio: " + errCodecs.Error())
			}
		default:
			return offer{}, errCodecs
		}
	}
	if videos == 0 {
		return offer{}, errors.New("no video track: " + errCodecs.Error())
	}
	return o, nil
}

type sdpCodec struct {
	pt         uint8
	name, fmtp string
	rate       uint32
}

func (c sdpCodec) params(mime string, channels uint16) webrtc.RTPCodecParameters {
	return webrtc.RTPCodecParameters{
		PayloadType:        webrtc.PayloadType(c.pt),
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: mime, ClockRate: c.rate, Channels: channels, SDPFmtpLine: c.fmtp},
	}
}

// mediaCodecs: a media section's rtpmap lines (with their fmtp), in order.
func mediaCodecs(m *sdp.MediaDescription) []sdpCodec {
	var out []sdpCodec
	fmtp := map[string]string{}
	for _, a := range m.Attributes {
		if a.Key == "fmtp" {
			pt, v, _ := strings.Cut(a.Value, " ")
			fmtp[pt] = v
		}
	}
	for _, a := range m.Attributes {
		if a.Key != "rtpmap" {
			continue
		}
		pt, v, _ := strings.Cut(a.Value, " ")
		parts := strings.Split(v, "/")
		n, err := strconv.ParseUint(pt, 10, 7)
		if err != nil || len(parts) < 2 {
			continue
		}
		rate, _ := strconv.ParseUint(parts[1], 10, 32)
		out = append(out, sdpCodec{pt: uint8(n), name: parts[0], fmtp: fmtp[pt], rate: uint32(rate)})
	}
	return out
}
