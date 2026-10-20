package thumbs

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"kallisti.chat/server/internal/mediasize"
)

func TestArgs(t *testing.T) {
	video := Args(WebP, Job{Src: "/up/k", Type: "video/mp4", Max: 480, Seek: true}, "/tmp/o")
	want := "-nostdin -hide_banner -loglevel error -protocol_whitelist file -ss 1 -f mov -i file:/up/k -frames:v 1 -an -sn -dn " +
		"-vf scale=w='min(480,iw)':h='min(480,ih)':force_original_aspect_ratio=decrease -c:v libwebp -pix_fmt yuv420p -f webp -y file:/tmp/o"
	if got := strings.Join(video, " "); got != want {
		t.Errorf("video:\n%s\nwant\n%s", got, want)
	}
	for _, c := range []struct {
		name   string
		format Format
		job    Job
		has    []string
		hasNot []string
	}{
		{"first frame", WebP, Job{Type: "video/webm", Max: 480}, []string{"-f matroska"}, []string{"-ss", "-noautorotate"}},
		{"png keeps alpha", WebP, Job{Type: "image/png", Max: 1000}, []string{"-noautorotate -f png_pipe", "-pix_fmt yuva420p", "min(1000,iw)"}, nil},
		{"jpeg turned by exif", JPEG, Job{Type: "image/jpeg", Max: 1000, Orientation: 6}, []string{"-f jpeg_pipe", "-vf transpose=1,scale=", "-c:v mjpeg -pix_fmt yuvj420p -q:v 4 -f mjpeg"}, nil},
		{"jpeg mirrored", JPEG, Job{Type: "image/jpeg", Max: 1000, Orientation: 2}, []string{"-vf hflip,scale="}, nil},
		{"webp", WebP, Job{Type: "image/webp", Max: 1000}, []string{"-f webp_pipe"}, nil},
	} {
		got := strings.Join(Args(c.format, c.job, "/tmp/o"), " ")
		for _, s := range c.has {
			if !strings.Contains(got, s) {
				t.Errorf("%s: %q lacks %q", c.name, got, s)
			}
		}
		for _, s := range c.hasNot {
			if strings.Contains(got, s) {
				t.Errorf("%s: %q has %q", c.name, got, s)
			}
		}
	}
}

func TestFormatOf(t *testing.T) {
	for _, c := range []struct {
		encoders string
		want     Format
		ok       bool
	}{
		{" V....D libwebp_anim  libwebp WebP image\n V....D libwebp  libwebp WebP image\n VFS..D mjpeg  MJPEG\n", WebP, true},
		{" VFS..D mjpeg                MJPEG (Motion JPEG)\n", JPEG, true},
		{" V....D png  PNG\n", Format{}, false},
	} {
		if got, ok := formatOf(c.encoders); got.Type != c.want.Type || ok != c.ok {
			t.Errorf("%q: %v %v", c.encoders, got, ok)
		}
	}
}

func TestFind(t *testing.T) {
	encoders := func(out string) func([]string) ([]byte, error) {
		return func([]string) ([]byte, error) { return []byte(out), nil }
	}
	none := func(string) (string, error) { return "", exec.ErrNotFound }
	if _, err := find("", none, encoders(" V mjpeg x\n")); err == nil {
		t.Error("no ffmpeg on PATH: found")
	}
	var asked []string
	look := func(name string) (string, error) {
		asked = append(asked, name)
		return "/bin/" + filepath.Base(name), nil
	}
	tool, err := find("/opt/ff/ffmpeg", look, encoders(" V libwebp x\n"))
	if err != nil || tool.Format.Type != "image/webp" || !slices.Equal(tool.Command, []string{"/bin/nice", "-n", "10", "/bin/ffmpeg"}) {
		t.Errorf("found %+v %v", tool, err)
	}
	if asked[0] != "/opt/ff/ffmpeg" {
		t.Errorf("looked for %v", asked)
	}
	onlyFFmpeg := func(name string) (string, error) {
		if name == "nice" {
			return "", exec.ErrNotFound
		}
		return "/bin/ffmpeg", nil
	}
	if tool, _ := find("", onlyFFmpeg, encoders(" V mjpeg x\n")); !slices.Equal(tool.Command, []string{"/bin/ffmpeg"}) || tool.Format.Type != "image/jpeg" {
		t.Errorf("without nice: %+v", tool)
	}
	if _, err := find("", onlyFFmpeg, encoders(" V png x\n")); err == nil {
		t.Error("no usable encoder: found")
	}
}

// fakeFFmpeg writes a shell script that acts as ffmpeg; out is the output
// file (the last argument, without "file:").
func fakeFFmpeg(t *testing.T, body string) Tool {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ffmpeg")
	script := "#!/bin/sh\nfor a; do out=$a; done; out=${out#file:}\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return Tool{Command: []string{path}, Format: WebP}
}

// setup gives an upload directory with a source file, and an empty
// temporary directory (TMPDIR) to check for leftovers.
func setup(t *testing.T) (dir, tmp string, job Job) {
	dir, tmp = t.TempDir(), t.TempDir()
	t.Setenv("TMPDIR", tmp)
	os.WriteFile(filepath.Join(dir, "key"), []byte("video"), 0o600)
	return dir, tmp, Job{Src: filepath.Join(dir, "key"), Dst: filepath.Join(dir, "key.preview"), Type: "video/mp4", Max: 480}
}

func names(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func TestMakeStoresFile(t *testing.T) {
	dir, tmp, job := setup(t)
	m := New(t.Context(), fakeFFmpeg(t, `printf thumb > "$out"`), 2, 4)
	ctype, err := m.Make(t.Context(), job)
	if err != nil || ctype != "image/webp" {
		t.Fatalf("make: %q %v", ctype, err)
	}
	if b, _ := os.ReadFile(job.Dst); string(b) != "thumb" {
		t.Fatalf("stored %q", b)
	}
	if got := names(t, dir); !slices.Equal(got, []string{"key", "key.preview"}) {
		t.Errorf("upload directory: %v", got)
	}
	if got := names(t, tmp); len(got) != 0 {
		t.Errorf("left in the temporary directory: %v", got)
	}
}

func TestFailuresLeaveNoFile(t *testing.T) {
	for name, body := range map[string]string{
		"exit status": `printf partial > "$out"; exit 1`,
		"empty":       `: > "$out"`,
	} {
		dir, tmp, job := setup(t)
		m := New(t.Context(), fakeFFmpeg(t, body), 1, 1)
		if _, err := m.Make(t.Context(), job); err == nil {
			t.Errorf("%s: no error", name)
		}
		if got := names(t, dir); !slices.Equal(got, []string{"key"}) {
			t.Errorf("%s: upload directory: %v", name, got)
		}
		if got := names(t, tmp); len(got) != 0 {
			t.Errorf("%s: temporary directory: %v", name, got)
		}
	}
}

func TestTimeoutKills(t *testing.T) {
	dir, tmp, job := setup(t)
	m := New(t.Context(), fakeFFmpeg(t, `printf partial > "$out"; exec sleep 30`), 1, 1)
	m.Timeout = 200 * time.Millisecond
	start := time.Now()
	_, err := m.Make(t.Context(), job)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 5*time.Second {
		t.Fatalf("after %v: %v", time.Since(start), err)
	}
	if got := names(t, dir); !slices.Equal(got, []string{"key"}) {
		t.Errorf("upload directory: %v", got)
	}
	if got := names(t, tmp); len(got) != 0 {
		t.Errorf("temporary directory: %v", got)
	}
}

func TestConcurrencyLimit(t *testing.T) {
	dir, _, job := setup(t)
	m := New(t.Context(), Tool{Command: []string{"ffmpeg"}, Format: WebP}, 2, 10)
	var running, most atomic.Int32
	m.run = func(ctx context.Context, args []string) error {
		n := running.Add(1)
		for {
			old := most.Load()
			if n <= old || most.CompareAndSwap(old, n) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		running.Add(-1)
		return os.WriteFile(strings.TrimPrefix(args[len(args)-1], "file:"), []byte("x"), 0o600)
	}
	var wg sync.WaitGroup
	for i := range 6 {
		j := job
		j.Dst = filepath.Join(dir, string(rune('a'+i)))
		wg.Add(1)
		if !m.Enqueue(j, func(_ string, err error) {
			if err != nil {
				t.Error(err)
			}
			wg.Done()
		}) {
			t.Fatal("queue full")
		}
	}
	// Made directly (old uploads) too: the same limit.
	if _, err := m.Make(t.Context(), job); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if most.Load() != 2 {
		t.Errorf("at most %d at once, want 2", most.Load())
	}
}

func TestQueueBounded(t *testing.T) {
	_, _, job := setup(t)
	m := New(t.Context(), Tool{Command: []string{"ffmpeg"}, Format: WebP}, 1, 1)
	started, release := make(chan struct{}), make(chan struct{})
	m.run = func(context.Context, []string) error {
		started <- struct{}{}
		<-release
		return errors.New("failed")
	}
	done := make(chan error, 2)
	report := func(_ string, err error) { done <- err }
	m.Enqueue(job, report)
	<-started
	if !m.Enqueue(job, report) {
		t.Fatal("second job refused")
	}
	if m.Enqueue(job, report) {
		t.Fatal("third job queued, the queue holds one")
	}
	close(release)
	<-started
	if err := <-done; err == nil {
		t.Error("failure not reported")
	}
	<-done
}

// With a real ffmpeg (skipped without one): a video thumbnail and a
// picture preview, in the format the build can write.
func TestRealFFmpeg(t *testing.T) {
	tool, err := Find("")
	if err != nil {
		t.Skip("no ffmpeg:", err)
	}
	dir := t.TempDir()
	var b bytes.Buffer
	png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 2000, 1000)))
	os.WriteFile(filepath.Join(dir, "png"), b.Bytes(), 0o600)
	// A JPEG with Exif orientation 6 (turned by 90 degrees): turned once.
	b.Reset()
	jpeg.Encode(&b, image.NewGray(image.Rect(0, 0, 40, 30)), nil)
	ifd := "MM\x00\x2a\x00\x00\x00\x08\x00\x01\x01\x12\x00\x03\x00\x00\x00\x01\x00\x06\x00\x00\x00\x00\x00\x00"
	app1 := "\xff\xe1\x00" + string(rune(2+6+len(ifd))) + "Exif\x00\x00" + ifd
	os.WriteFile(filepath.Join(dir, "jpeg"), slices.Concat(b.Bytes()[:2], []byte(app1), b.Bytes()[2:]), 0o600)
	m := New(t.Context(), tool, 1, 1)
	for _, c := range []struct {
		job  Job
		want mediasize.Info
	}{
		{Job{Src: "../mediasize/testdata/faststart.mp4", Type: "video/mp4", Max: 480}, mediasize.Info{Width: 64, Height: 48}},
		{Job{Src: "../mediasize/testdata/rotated.mp4", Type: "video/mp4", Max: 480}, mediasize.Info{Width: 48, Height: 64}},
		{Job{Src: "../mediasize/testdata/vp9.webm", Type: "video/webm", Max: 480}, mediasize.Info{Width: 64, Height: 48}},
		{Job{Src: filepath.Join(dir, "png"), Type: "image/png", Max: 1000}, mediasize.Info{Width: 1000, Height: 500}},
		{Job{Src: filepath.Join(dir, "jpeg"), Type: "image/jpeg", Max: 1000, Orientation: 6}, mediasize.Info{Width: 30, Height: 40}},
	} {
		c.job.Dst = filepath.Join(dir, "out")
		ctype, err := m.Make(t.Context(), c.job)
		if err != nil {
			t.Errorf("%s: %v", c.job.Src, err)
			continue
		}
		f, _ := os.ReadFile(c.job.Dst)
		if got := mediasize.Probe(bytes.NewReader(f), int64(len(f)), ctype); got != c.want {
			t.Errorf("%s: %s %+v, want %+v", c.job.Src, ctype, got, c.want)
		}
	}
}
