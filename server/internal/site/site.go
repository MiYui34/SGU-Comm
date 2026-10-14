// Package site serves the web app and the bridge from one origin:
// WebSocket upgrades go to the bridge, everything else to the built web app.
package site

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// The web app needs only its own origin. Inline styles: Svelte sets style
// attributes. blob: images: upload previews. connect-src 'self' also allows
// ws(s) to the same host.
const csp = "default-src 'self'; img-src 'self' blob:; style-src 'self' 'unsafe-inline'; " +
	"object-src 'none'; base-uri 'none'; frame-ancestors 'none'"

// New returns the site handler. Without webDir, everything goes to the
// bridge (development: the web app runs on Vite).
func New(bridge http.Handler, webDir string) http.Handler {
	if webDir == "" {
		return bridge
	}
	files := http.FileServer(http.Dir(webDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			bridge.ServeHTTP(w, r)
			return
		}
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		clean := path.Clean("/" + r.URL.Path)
		if strings.HasPrefix(clean, "/assets/") {
			// Vite puts a content hash in asset names.
			h.Set("Cache-Control", "public, max-age=31536000, immutable")
			files.ServeHTTP(w, r)
			return
		}
		// Real files (favicon, ...) as they are; any other path is an app
		// route and gets index.html.
		if clean != "/" {
			if info, err := os.Stat(filepath.Join(webDir, filepath.FromSlash(clean))); err == nil && !info.IsDir() {
				files.ServeHTTP(w, r)
				return
			}
		}
		h.Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
	})
}
