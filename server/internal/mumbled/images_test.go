package mumbled

import (
	"encoding/base64"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"kallisti.chat/server/internal/mumble"
	"kallisti.chat/server/internal/mumble/pb"
)

// A PNG header: enough for the type check.
var tinyPNG = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00")

func dataImage(typ string, data []byte) string {
	return `<img src="data:` + typ + `;base64,` + base64.StdEncoding.EncodeToString(data) + `" />`
}

func TestExtractImages(t *testing.T) {
	msg := `Now playing <b>x</b><br>` + dataImage("image/png", tinyPNG) +
		dataImage("image/svg+xml", []byte(`<svg onload="x()"/>`)) + `<img src="data:image/png;base64,!!!">` +
		`<img src="https://tracker.example/x.png">`
	rest, imgs := extractImages(msg)
	if len(imgs) != 1 || imgs[0].Type != "image/png" || string(imgs[0].Data) != string(tinyPNG) {
		t.Fatalf("images %+v", imgs)
	}
	if strings.Contains(rest, "<img") || plainText(rest) != "Now playing x" {
		t.Fatalf("rest %q", rest)
	}
}

func TestImagesReachClientsAndTheHook(t *testing.T) {
	var mu sync.Mutex
	var heard []Image
	var heardText string
	cfg := Config{OnChannelText: func(t ChannelText) {
		text, images := t.Text, t.Images
		mu.Lock()
		heard, heardText = images, text
		mu.Unlock()
	}}
	_, addr := startServerAndListen(t, "127.0.0.1", cfg, openACL)
	alice := dial(t, addr, "alice")
	bob := dial(t, addr, "bob")
	// Over 5000 characters with the image: allowed for image messages.
	big := append(append([]byte{}, tinyPNG...), make([]byte, 6000)...)
	alice.send(mumble.TypeTextMessage, &pb.TextMessage{ChannelId: []uint32{0}, Message: proto.String("cover:" + dataImage("image/png", big))})
	var m pb.TextMessage
	proto.Unmarshal(bob.expect(mumble.TypeTextMessage), &m)
	if !strings.HasPrefix(m.GetMessage(), "cover:<br><img src=\"data:image/png;base64,") {
		t.Fatalf("bob got %.80q", m.GetMessage())
	}
	eventually(t, func() bool { mu.Lock(); defer mu.Unlock(); return len(heard) == 1 })
	mu.Lock()
	defer mu.Unlock()
	if heardText != "cover:" || len(heard[0].Data) != len(big) {
		t.Fatalf("hook: %q %d", heardText, len(heard[0].Data))
	}
}

func TestLongTextWithoutImagesIsStillRefused(t *testing.T) {
	var mu sync.Mutex
	n := 0
	cfg := Config{OnChannelText: func(ChannelText) { mu.Lock(); n++; mu.Unlock() }}
	_, addr := startServerAndListen(t, "127.0.0.1", cfg, openACL)
	alice := dial(t, addr, "alice")
	alice.send(mumble.TypeTextMessage, &pb.TextMessage{ChannelId: []uint32{0}, Message: proto.String(strings.Repeat("x", 6000) + "<img")})
	alice.send(mumble.TypeTextMessage, &pb.TextMessage{ChannelId: []uint32{0}, Message: proto.String("short")})
	eventually(t, func() bool { mu.Lock(); defer mu.Unlock(); return n == 1 })
}

func TestNoUserHTMLGetsThrough(t *testing.T) {
	for _, in := range []string{`<<b>img src="http://evil/x.png">`, `<scr<b>ipt>x</script>`} {
		if out := plainText(in); strings.Contains(out, "<") {
			t.Errorf("%q: %q", in, out)
		}
	}
	if plainText("a < b") != "a &lt; b" {
		t.Errorf("a < b: %q", plainText("a < b"))
	}
}

func TestPlainTextIsLinear(t *testing.T) {
	// Each removed tag exposes the next one: one pass, not one per tag.
	n := 50000
	start := time.Now()
	out := plainText(strings.Repeat("<", n) + strings.Repeat("b>", n))
	if d := time.Since(start); d > time.Second {
		t.Fatalf("took %v", d)
	}
	if strings.Contains(out, "<") {
		t.Fatalf("a tag is left: %q...", out[:20])
	}
}

func TestUnpaddedBase64(t *testing.T) {
	raw := base64.RawStdEncoding.EncodeToString(tinyPNG)
	if _, imgs := extractImages(`<img src="data:image/png;base64,` + raw + `">`); len(imgs) != 1 {
		t.Fatal("unpadded image dropped")
	}
}

func TestTreeMessagesStoreNoImages(t *testing.T) {
	var mu sync.Mutex
	var withImages, without int
	cfg := Config{OnChannelText: func(t ChannelText) {
		images := t.Images
		mu.Lock()
		if len(images) > 0 {
			withImages++
		} else {
			without++
		}
		mu.Unlock()
	}}
	_, addr := startServerAndListen(t, "127.0.0.1", cfg, openACL)
	alice := dial(t, addr, "alice")
	alice.createChannel(0, "A", false)
	alice.send(mumble.TypeTextMessage, &pb.TextMessage{TreeId: []uint32{0}, Message: proto.String("x" + dataImage("image/png", tinyPNG))})
	eventually(t, func() bool { mu.Lock(); defer mu.Unlock(); return without == 2 })
	mu.Lock()
	defer mu.Unlock()
	if withImages != 0 {
		t.Fatalf("images stored for %d tree channels", withImages)
	}
}
