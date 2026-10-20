// Package thumbs makes small pictures of uploads with ffmpeg, if it is
// installed: a frame of a video, a smaller copy of a large picture.
// ffmpeg runs as a separate process, at low priority (nice), with a time
// limit, and on the stored file only: local files as the only protocol,
// the demuxer given by the upload's type (no playlists that point
// elsewhere). It writes to the temporary directory; the result is then
// copied next to the upload and renamed into place, so no half-written
// picture is ever served.
package thumbs

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Format is a picture format ffmpeg writes.
type Format struct {
	Type  string // MIME type
	Codec string
	Muxer string
	Alpha bool     // keeps transparency
	Args  []string // encoder options
}

var (
	WebP = Format{Type: "image/webp", Codec: "libwebp", Muxer: "webp", Alpha: true}
	JPEG = Format{Type: "image/jpeg", Codec: "mjpeg", Muxer: "mjpeg", Args: []string{"-pix_fmt", "yuvj420p", "-q:v", "4"}}
)

// demuxers by upload type: ffmpeg reads nothing else.
var demuxers = map[string]string{
	"video/mp4":  "mov",
	"video/webm": "matroska",
	"image/png":  "png_pipe",
	"image/jpeg": "jpeg_pipe",
	"image/webp": "webp_pipe",
}

// Supported: Make can read uploads of this type.
func Supported(ctype string) bool { _, ok := demuxers[ctype]; return ok }

// Job is one picture to make.
type Job struct {
	Src, Dst string // the stored upload; where the picture goes
	Type     string // the upload's type (see Supported)
	Max      int    // the longer side at most, in pixels
	Seek     bool   // a video's frame at 1 s, not the first
	// A JPEG's Exif orientation: ffmpeg is told to ignore it and the
	// picture is turned here, as browsers do.
	Orientation uint16
}

// exifFilters turn a picture by its Exif orientation.
var exifFilters = map[uint16]string{2: "hflip", 3: "hflip,vflip", 4: "vflip", 5: "transpose=0", 6: "transpose=1", 7: "transpose=3", 8: "transpose=2"}

// Args are ffmpeg's arguments for a job, writing to out.
func Args(f Format, j Job, out string) []string {
	a := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-protocol_whitelist", "file"}
	if j.Seek {
		a = append(a, "-ss", "1")
	}
	if strings.HasPrefix(j.Type, "image/") {
		a = append(a, "-noautorotate")
	}
	side := strconv.Itoa(j.Max)
	vf := "scale=w='min(" + side + ",iw)':h='min(" + side + ",ih)':force_original_aspect_ratio=decrease"
	if t, ok := exifFilters[j.Orientation]; ok {
		vf = t + "," + vf
	}
	a = append(a, "-f", demuxers[j.Type], "-i", "file:"+j.Src, "-frames:v", "1", "-an", "-sn", "-dn", "-vf", vf, "-c:v", f.Codec)
	if f.Args != nil {
		a = append(a, f.Args...)
	} else if f.Alpha && j.Type != "image/jpeg" && strings.HasPrefix(j.Type, "image/") {
		a = append(a, "-pix_fmt", "yuva420p")
	} else {
		a = append(a, "-pix_fmt", "yuv420p")
	}
	return append(a, "-f", f.Muxer, "-y", "file:"+out)
}

// Tool is an ffmpeg to run and the format it writes.
type Tool struct {
	Command []string // ffmpeg, behind nice where there is one
	Format  Format
}

// Find looks for ffmpeg (path, or on PATH) and asks which encoders it has:
// WebP if it can, else JPEG.
func Find(path string) (Tool, error) {
	return find(path, exec.LookPath, func(cmd []string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return exec.CommandContext(ctx, cmd[0], cmd[1:]...).Output()
	})
}

func find(path string, look func(string) (string, error), output func([]string) ([]byte, error)) (Tool, error) {
	ffmpeg, err := look(cmp.Or(path, "ffmpeg"))
	if err != nil {
		return Tool{}, err
	}
	out, err := output([]string{ffmpeg, "-hide_banner", "-encoders"})
	if err != nil {
		return Tool{}, fmt.Errorf("%s -encoders: %w", ffmpeg, err)
	}
	f, ok := formatOf(string(out))
	if !ok {
		return Tool{}, fmt.Errorf("%s has no WebP or JPEG encoder", ffmpeg)
	}
	cmd := []string{ffmpeg}
	if nice, err := look("nice"); err == nil {
		cmd = []string{nice, "-n", "10", ffmpeg}
	}
	return Tool{Command: cmd, Format: f}, nil
}

// formatOf picks the format from `ffmpeg -encoders` (lines of flags, name,
// description).
func formatOf(encoders string) (Format, bool) {
	has := map[string]bool{}
	for line := range strings.Lines(encoders) {
		if f := strings.Fields(line); len(f) >= 2 {
			has[f[1]] = true
		}
	}
	switch {
	case has[WebP.Codec]:
		return WebP, true
	case has[JPEG.Codec]:
		return JPEG, true
	}
	return Format{}, false
}

const (
	// A picture larger than this is not kept (they are some 100 KB).
	maxOutput = 8 << 20
	// Text of ffmpeg's errors kept for the log.
	maxStderr = 1024
)

// Maker runs jobs: at most a few ffmpeg processes at once, from a bounded
// queue (Enqueue) or directly (Make).
type Maker struct {
	tool    Tool
	Timeout time.Duration
	run     func(ctx context.Context, args []string) error
	slots   chan struct{}
	queue   chan task
}

type task struct {
	job  Job
	done func(ctype string, err error)
}

// New starts workers that take jobs from a queue of queued at most, until
// ctx ends.
func New(ctx context.Context, tool Tool, workers, queued int) *Maker {
	m := &Maker{tool: tool, Timeout: 20 * time.Second, slots: make(chan struct{}, workers), queue: make(chan task, queued)}
	m.run = m.exec
	for range workers {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case t := <-m.queue:
					t.done(m.Make(ctx, t.job))
				}
			}
		}()
	}
	return m
}

// Format is what the pictures are.
func (m *Maker) Format() Format { return m.tool.Format }

// Enqueue queues a job; done gets the picture's type, or an error. False:
// the queue is full (the job is dropped).
func (m *Maker) Enqueue(j Job, done func(ctype string, err error)) bool {
	select {
	case m.queue <- task{j, done}:
		return true
	default:
		return false
	}
}

// Make makes a picture now (waiting for a free slot) and returns its type.
func (m *Maker) Make(ctx context.Context, j Job) (string, error) {
	select {
	case m.slots <- struct{}{}:
		defer func() { <-m.slots }()
	case <-ctx.Done():
		return "", ctx.Err()
	}
	if !Supported(j.Type) {
		return "", fmt.Errorf("cannot read %s", j.Type)
	}
	tmp, err := os.CreateTemp("", "kallisti-preview-*")
	if err != nil {
		return "", err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())
	ctx, cancel := context.WithTimeout(ctx, m.Timeout)
	defer cancel()
	if err := m.run(ctx, Args(m.tool.Format, j, tmp.Name())); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("ffmpeg: %w", ctx.Err())
		}
		return "", err
	}
	if err := place(tmp.Name(), j.Dst); err != nil {
		return "", err
	}
	return m.tool.Format.Type, nil
}

// exec runs ffmpeg with no environment but PATH (none of the server's
// secrets); it is killed when ctx ends.
func (m *Maker) exec(ctx context.Context, args []string) error {
	cmd := exec.CommandContext(ctx, m.tool.Command[0], slices.Concat(m.tool.Command[1:], args)...)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
	cmd.WaitDelay = time.Second
	var stderr limited
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg: %w: %s", err, bytes.TrimSpace(stderr.b))
	}
	return nil
}

// place copies the picture next to dst and renames it into place (the
// temporary directory can be another file system).
func place(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if st, err := in.Stat(); err != nil || st.Size() == 0 || st.Size() > maxOutput {
		return cmp.Or(err, errors.New("ffmpeg wrote no picture or a huge one"))
	}
	out, err := os.CreateTemp(filepath.Dir(dst), ".preview-*")
	if err != nil {
		return err
	}
	_, err = io.Copy(out, io.LimitReader(in, maxOutput))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(out.Name(), dst)
	}
	if err != nil {
		os.Remove(out.Name())
	}
	return err
}

// limited keeps the first maxStderr bytes written.
type limited struct{ b []byte }

func (l *limited) Write(p []byte) (int, error) {
	l.b = append(l.b, p[:min(len(p), maxStderr-len(l.b))]...)
	return len(p), nil
}
