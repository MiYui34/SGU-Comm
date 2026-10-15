package site

import (
	"bytes"
	"compress/gzip"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// Assets that compress well (the DeepFilterNet3 engine: 14.6 MB, 4 MB with
// gzip). Assets never change (a hash in the name): each is compressed once
// and kept.
var compressible = []string{".js", ".css", ".wasm", ".svg", ".json"}

type gzipCache struct {
	mu    sync.Mutex
	files map[string][]byte
}

// serve answers with the gzipped asset if the client takes gzip and the
// file compresses (false: serve it as it is).
func (c *gzipCache) serve(w http.ResponseWriter, r *http.Request, dir, clean string) bool {
	ext := filepath.Ext(clean)
	if !slices.Contains(compressible, ext) || !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		return false
	}
	body, ok := c.get(filepath.Join(dir, filepath.FromSlash(clean)))
	if !ok {
		return false
	}
	h := w.Header()
	h.Set("Content-Type", mime.TypeByExtension(ext))
	h.Set("Content-Encoding", "gzip")
	h.Add("Vary", "Accept-Encoding")
	h.Set("Content-Length", strconv.Itoa(len(body)))
	if r.Method != http.MethodHead {
		w.Write(body)
	}
	return true
}

func (c *gzipCache) get(file string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if body, ok := c.files[file]; ok {
		return body, true
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, false
	}
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	zw.Write(data)
	zw.Close()
	c.files[file] = buf.Bytes()
	return c.files[file], true
}
