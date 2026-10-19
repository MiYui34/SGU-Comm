package whip

import (
	"strings"
	"testing"
)

// offerSDP is an offer like OBS's, with the media sections given.
func offerSDP(media ...string) string {
	return "v=0\r\no=- 1 2 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n" + strings.Join(media, "")
}

const (
	h264Media = "m=video 9 UDP/TLS/RTP/SAVPF 96 97\r\nc=IN IP4 0.0.0.0\r\na=mid:1\r\na=sendonly\r\n" +
		"a=rtpmap:96 VP8/90000\r\na=rtpmap:97 H264/90000\r\na=fmtp:97 profile-level-id=42e01f;packetization-mode=1;level-asymmetry-allowed=1\r\n"
	opusMedia = "m=audio 9 UDP/TLS/RTP/SAVPF 111\r\nc=IN IP4 0.0.0.0\r\na=mid:0\r\na=sendonly\r\n" +
		"a=rtpmap:111 opus/48000/2\r\na=fmtp:111 minptime=10;maxaveragebitrate=96000;stereo=1;sprop-stereo=1;useinbandfec=1\r\n"
	vp8Media = "m=video 9 UDP/TLS/RTP/SAVPF 96\r\nc=IN IP4 0.0.0.0\r\na=mid:1\r\na=rtpmap:96 VP8/90000\r\n"
	mode0    = "m=video 9 UDP/TLS/RTP/SAVPF 97\r\nc=IN IP4 0.0.0.0\r\na=mid:1\r\na=rtpmap:97 H264/90000\r\na=fmtp:97 profile-level-id=42e01f\r\n"
	aacMedia = "m=audio 9 UDP/TLS/RTP/SAVPF 98\r\nc=IN IP4 0.0.0.0\r\na=mid:0\r\na=rtpmap:98 MP4A-LATM/90000\r\n"
)

func TestCheckOffer(t *testing.T) {
	o, err := checkOffer(offerSDP(opusMedia, h264Media))
	if err != nil {
		t.Fatal(err)
	}
	if o.video.PayloadType != 97 || o.video.MimeType != "video/H264" || !strings.Contains(o.video.SDPFmtpLine, "packetization-mode=1") {
		t.Fatalf("video %+v", o.video)
	}
	if o.audio == nil || o.audio.PayloadType != 111 || o.audio.Channels != 2 || o.audio.ClockRate != 48000 {
		t.Fatalf("audio %+v", o.audio)
	}
	if o, err := checkOffer(offerSDP(h264Media)); err != nil || o.audio != nil {
		t.Fatalf("without sound: %+v %v", o, err)
	}
	for _, c := range []struct{ sdp, want string }{
		{"nonsense", "SDP"},
		{offerSDP(vp8Media), "H.264"},
		{offerSDP(mode0), "packetization-mode=1"},
		{offerSDP(opusMedia), "video"},
		{offerSDP(aacMedia, h264Media), "Opus"},
		{offerSDP(h264Media, h264Media), "one video"},
		{offerSDP(opusMedia, opusMedia, h264Media), "one audio"},
	} {
		if _, err := checkOffer(c.sdp); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: got %v, want %q", c.sdp, err, c.want)
		}
	}
}
