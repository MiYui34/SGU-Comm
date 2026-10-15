// Package preview makes link previews: the server fetches a page's title,
// description and image, so readers' browsers never contact the other host.
// Only public addresses on the web ports are fetched (also after DNS and
// redirects), with limits on time and size.
package preview

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

const (
	timeout        = 5 * time.Second
	maxRedirects   = 3
	maxPage        = 512 << 10 // bytes read of a page (the head is enough)
	maxImage       = 512 << 10
	maxTitle       = 200 // runes
	maxDescription = 300
	maxSite        = 100
	cacheFor       = time.Hour
	maxEntries     = 500
	parallel       = 4 // fetches at the same time
)

// Preview is what a card shows. Image is the key of the fetched image (see
// ServeHTTP), empty without one.
type Preview struct {
	URL, Title, Description, Site, Image string
}

type picture struct {
	data []byte
	typ  string
}

type entry struct {
	done    chan struct{}
	preview *Preview // nil: no preview
	at      time.Time
}

// Fetcher fetches and caches previews (also failures, so a link is tried
// once an hour at most).
type Fetcher struct {
	client *http.Client
	allow  func(netip.AddrPort) bool // tests allow local servers
	now    func() time.Time
	sem    chan struct{}
	mu     sync.Mutex
	pages  map[string]*entry
	images map[string]picture
}

func New() *Fetcher {
	f := &Fetcher{allow: publicAddr, now: time.Now, sem: make(chan struct{}, parallel),
		pages: map[string]*entry{}, images: map[string]picture{}}
	dialer := &net.Dialer{Timeout: timeout, Control: func(_, address string, _ syscall.RawConn) error {
		ap, err := netip.ParseAddrPort(address)
		if err != nil || !f.allow(ap) {
			return errors.New("not a public web address")
		}
		return nil
	}}
	f.client = &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:                  nil, // straight to the host: the check above sees its address
			DialContext:            dialer.DialContext,
			TLSHandshakeTimeout:    timeout,
			ResponseHeaderTimeout:  timeout,
			MaxResponseHeaderBytes: 64 << 10,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > maxRedirects {
				return errors.New("too many redirects")
			}
			return webURL(req.URL)
		},
	}
	return f
}

var blocked = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"), // carrier-grade NAT (also VPNs)
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("64:ff9b::/96"), // NAT64: IPv4 behind it
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("fec0::/10"),
}

// publicAddr: a global address (no local network, loopback or cloud
// metadata) on the HTTP or HTTPS port.
func publicAddr(ap netip.AddrPort) bool {
	ip := ap.Addr()
	if ip.Is4In6() {
		return false
	}
	if p := ap.Port(); p != 80 && p != 443 {
		return false
	}
	return ip.IsGlobalUnicast() && !ip.IsPrivate() &&
		!slices.ContainsFunc(blocked, func(p netip.Prefix) bool { return p.Contains(ip) })
}

func webURL(u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" || u.Host == "" || u.User != nil {
		return errors.New("not a web address")
	}
	return nil
}

// Get returns the preview of a page (nil: none). A fetch of the same page
// that is on its way is waited for.
func (f *Fetcher) Get(ctx context.Context, raw string) *Preview {
	f.mu.Lock()
	e, ok := f.pages[raw]
	if ok && f.now().Sub(e.at) > cacheFor {
		select {
		case <-e.done: // old: fetch again
			f.forget(raw)
			ok = false
		default:
		}
	}
	if !ok {
		f.trim()
		e = &entry{done: make(chan struct{}), at: f.now()}
		f.pages[raw] = e
		go f.fill(raw, e)
	}
	f.mu.Unlock()
	select {
	case <-e.done:
		return e.preview
	case <-ctx.Done():
		return nil
	}
}

func (f *Fetcher) fill(raw string, e *entry) {
	defer close(e.done)
	f.sem <- struct{}{}
	defer func() { <-f.sem }()
	p, image, err := f.fetch(raw)
	if err != nil {
		slog.Debug("link preview", "url", raw, "err", err)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if image.data != nil {
		p.Image = randomKey()
		f.images[p.Image] = image
	}
	e.preview = &p
}

func (f *Fetcher) fetch(raw string) (Preview, picture, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return Preview{}, picture{}, err
	}
	if err := webURL(u); err != nil {
		return Preview{}, picture{}, err
	}
	res, err := f.get(u.String(), "text/html")
	if err != nil {
		return Preview{}, picture{}, err
	}
	defer res.Body.Close()
	ctype := res.Header.Get("Content-Type")
	if !strings.HasPrefix(ctype, "text/html") && !strings.HasPrefix(ctype, "application/xhtml") {
		return Preview{}, picture{}, errors.New("not a page")
	}
	body, err := charset.NewReader(io.LimitReader(res.Body, maxPage), ctype)
	if err != nil {
		return Preview{}, picture{}, err
	}
	p, imageURL := parse(body, res.Request.URL)
	if p.Title == "" {
		return Preview{}, picture{}, errors.New("no title")
	}
	p.URL = raw
	return p, f.image(imageURL), nil
}

func (f *Fetcher) get(u, accept string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", "Kallisti link preview")
	res, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, errors.New(res.Status)
	}
	return res, nil
}

var imageTypes = []string{"image/png", "image/jpeg", "image/gif", "image/webp"}

// image fetches a preview picture: only real images (by their bytes), and
// small ones.
func (f *Fetcher) image(u string) picture {
	if u == "" {
		return picture{}
	}
	res, err := f.get(u, "image/*")
	if err != nil {
		return picture{}
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, maxImage+1))
	if err != nil || len(data) > maxImage {
		return picture{}
	}
	if typ := http.DetectContentType(data); slices.Contains(imageTypes, typ) {
		return picture{data, typ}
	}
	return picture{}
}

// parse reads a page's head: Open Graph tags first, else the title and
// description. imageURL is absolute (http or https), or empty.
func parse(r io.Reader, base *url.URL) (p Preview, imageURL string) {
	meta := map[string]string{}
	var title string
	z := html.NewTokenizer(r)
	for inTitle := false; ; {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			goto done
		case html.TextToken:
			if inTitle && title == "" {
				title = string(z.Text())
			}
		case html.StartTagToken, html.SelfClosingTagToken, html.EndTagToken:
			name, hasAttr := z.TagName()
			switch string(name) {
			case "body":
				goto done
			case "head":
				if tt == html.EndTagToken {
					goto done
				}
			case "title":
				inTitle = tt == html.StartTagToken
			case "meta":
				var key, content string
				for hasAttr {
					var k, v []byte
					k, v, hasAttr = z.TagAttr()
					switch string(k) {
					case "property", "name":
						key = strings.ToLower(string(v))
					case "content":
						content = string(v)
					}
				}
				if _, seen := meta[key]; key != "" && !seen {
					meta[key] = content
				}
			}
		}
	}
done:
	first := func(keys ...string) string {
		for _, k := range keys {
			if v := clean(meta[k]); v != "" {
				return v
			}
		}
		return ""
	}
	p.Title = shorten(cmpOr(first("og:title", "twitter:title"), clean(title)), maxTitle)
	p.Description = shorten(first("og:description", "twitter:description", "description"), maxDescription)
	p.Site = shorten(first("og:site_name"), maxSite)
	if img := first("og:image", "og:image:url", "twitter:image"); img != "" {
		if u, err := base.Parse(img); err == nil && webURL(u) == nil {
			imageURL = u.String()
		}
	}
	return p, imageURL
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// clean makes one line of text: no control characters, single spaces.
func clean(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

func shorten(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return strings.TrimSpace(string(r[:n])) + "…"
	}
	return s
}

func randomKey() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// trim keeps the cache small: old entries go, then any (with their images).
// Callers hold mu.
func (f *Fetcher) trim() {
	if len(f.pages) < maxEntries {
		return
	}
	for raw, e := range f.pages {
		if f.now().Sub(e.at) > cacheFor || len(f.pages) >= maxEntries {
			select {
			case <-e.done:
				f.forget(raw)
			default: // on its way: kept
			}
		}
	}
}

// forget removes a page and its image. Callers hold mu.
func (f *Fetcher) forget(raw string) {
	if e := f.pages[raw]; e != nil && e.preview != nil && e.preview.Image != "" {
		delete(f.images, e.preview.Image)
	}
	delete(f.pages, raw)
}

// ServeHTTP serves fetched images at /api/link-image/<key>.
func (f *Fetcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	img, ok := f.images[strings.TrimPrefix(r.URL.Path, "/api/link-image/")]
	f.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	h := w.Header()
	h.Set("Content-Type", img.typ)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "private, max-age=3600")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(img.data))
}
