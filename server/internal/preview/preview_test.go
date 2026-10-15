package preview

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestParse(t *testing.T) {
	base, _ := url.Parse("https://example.org/news/1")
	p, img := parse(strings.NewReader(`<!doctype html><html><head>
		<title>Fallback</title>
		<meta property="og:title" content="  The   &amp; Title ">
		<meta property="og:description" content="What it is about.">
		<meta property="og:site_name" content="Example">
		<meta property="og:image" content="/img/cover.png">
		</head><body><meta property="og:title" content="not this"></body></html>`), base)
	if p.Title != "The & Title" || p.Description != "What it is about." || p.Site != "Example" {
		t.Fatalf("%+v", p)
	}
	if img != "https://example.org/img/cover.png" {
		t.Fatalf("image %q", img)
	}
	p, img = parse(strings.NewReader(`<html><head><title>Only a title</title>
		<meta name="description" content="`+strings.Repeat("long ", 200)+`"></head>`), base)
	if p.Title != "Only a title" || len([]rune(p.Description)) > maxDescription+1 || img != "" {
		t.Fatalf("%+v %q", p, img)
	}
	// Only http(s) images.
	if _, img := parse(strings.NewReader(`<meta property="og:image" content="javascript:alert(1)">`), base); img != "" {
		t.Fatalf("image %q", img)
	}
}

func TestPublicAddresses(t *testing.T) {
	for addr, want := range map[string]bool{
		"93.184.216.34:443": true, "[2606:2800:220:1::1]:443": true,
		"93.184.216.34:22": false, "127.0.0.1:443": false, "10.1.2.3:80": false, "192.168.1.1:80": false,
		"172.16.0.1:80": false, "169.254.169.254:80": false, "100.100.1.1:443": false, "0.0.0.0:80": false,
		"[::1]:443": false, "[fd00::1]:443": false, "[fe80::1]:443": false, "[::ffff:127.0.0.1]:443": false,
		"[64:ff9b::a00:1]:443": false,
	} {
		if got := publicAddr(netip.MustParseAddrPort(addr)); got != want {
			t.Errorf("%s: %v", addr, got)
		}
	}
}

// local lets tests fetch from httptest (on 127.0.0.1).
func local(netip.AddrPort) bool { return true }

func TestFetchWithImage(t *testing.T) {
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 30)...)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/page":
			hits.Add(1)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<head><title>Hello</title><meta property="og:image" content="/pic"></head>`)
		case "/pic":
			w.Write(png)
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusFound)
		case "/file":
			w.Header().Set("Content-Type", "application/zip")
			fmt.Fprint(w, "PK")
		}
	}))
	defer srv.Close()
	f := New()
	f.allow = local
	p := f.Get(context.Background(), srv.URL+"/page")
	if p == nil || p.Title != "Hello" || p.Image == "" || p.URL != srv.URL+"/page" {
		t.Fatalf("preview %+v", p)
	}
	if again := f.Get(context.Background(), srv.URL+"/page"); again == nil || hits.Load() != 1 {
		t.Fatalf("not cached: %d fetches", hits.Load())
	}
	w := httptest.NewRecorder()
	f.ServeHTTP(w, httptest.NewRequest("GET", "/api/link-image/"+p.Image, nil))
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" || w.Body.Len() != len(png) {
		t.Fatalf("image: %d %v", w.Code, w.Header())
	}
	w = httptest.NewRecorder()
	f.ServeHTTP(w, httptest.NewRequest("GET", "/api/link-image/nope", nil))
	if w.Code != 404 {
		t.Fatalf("unknown image: %d", w.Code)
	}
	for _, path := range []string{"/loop", "/file"} {
		if p := f.Get(context.Background(), srv.URL+path); p != nil {
			t.Fatalf("%s: %+v", path, p)
		}
	}
}

func TestLocalAddressesAreRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<title>secret admin page</title>`)
	}))
	defer srv.Close()
	if p := New().Get(context.Background(), srv.URL); p != nil {
		t.Fatalf("fetched a local page: %+v", p)
	}
	if p := New().Get(context.Background(), "ftp://example.org/"); p != nil {
		t.Fatalf("ftp: %+v", p)
	}
}
