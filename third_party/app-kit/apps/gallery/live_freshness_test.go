package gallery

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

func liveDoc(t *testing.T, body string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func liveFindAll(n *html.Node, match func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && match(n) {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}

func liveAttr(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

func liveHas(key string) func(*html.Node) bool {
	return func(n *html.Node) bool { _, ok := liveAttr(n, key); return ok }
}

func TestLiveSpecimenPageComposesRegionsWithoutNavigationCost(t *testing.T) {
	h := New()
	res, body := get(t, h, "/live?theme=dark")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /live = %d", res.StatusCode)
	}
	doc := liveDoc(t, body)
	if n := len(liveFindAll(doc, func(n *html.Node) bool { return n.Data == "h1" })); n != 1 {
		t.Fatalf("specimen page has %d h1 elements", n)
	}
	pages := liveFindAll(doc, liveHas("data-live-page"))
	if len(pages) != 1 {
		t.Fatal("want one live page status")
	}
	if u, _ := liveAttr(pages[0], "data-live-url"); u != "/live/events" {
		t.Errorf("stream url = %q", u)
	}
	if ev, _ := liveAttr(pages[0], "data-live-events"); ev != "change" {
		t.Errorf("followed events = %q", ev)
	}
	if cursor, _ := liveAttr(pages[0], "data-live-last-event-id"); cursor != "0" {
		t.Errorf("render-time cursor = %q, want the fixture position 0", cursor)
	}
	regions := liveFindAll(doc, liveHas("data-live-source"))
	if len(regions) != len(liveSources) {
		t.Fatalf("regions = %d", len(regions))
	}
	for i, r := range regions {
		if src, _ := liveAttr(r, "data-live-source"); src != liveSources[i].Key {
			t.Errorf("region %d follows %q", i, src)
		}
	}
	if strings.Count(body, "w.AppKitLive = { version: 1") != 1 {
		t.Error("live.js must be included exactly once")
	}
	if !strings.Contains(body, `href="?theme=light"`) && !strings.Contains(body, `href="/live?theme=light"`) {
		t.Error("theme links must stay on the specimen")
	}
	for _, link := range nav {
		if link.Href == "/live" || link.Href == "/page-heading" {
			t.Fatal("specimens must not grow the budgeted top navigation")
		}
	}
	_, overview := get(t, h, "/overview")
	for _, href := range []string{`href="/live"`, `href="/page-heading"`} {
		if !strings.Contains(overview, href) {
			t.Errorf("overview does not link %s", href)
		}
	}
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, "/live", nil))
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /live = %d", method, w.Code)
		}
	}
}

// Every utility the new helpers emit must exist in the Gallery's built CSS, or
// the reserved status line and responsive heading silently lose their layout.
func TestLiveAndHeadingUtilitiesAreInBuiltCSS(t *testing.T) {
	css, err := os.ReadFile("ui/app.css")
	if err != nil {
		t.Fatal(err)
	}
	h := New()
	paths := []string{"/live"}
	for _, v := range pageHeadingVariants {
		paths = append(paths, "/page-heading?variant="+v.Key)
	}
	seen := map[string]bool{}
	for _, path := range paths {
		_, body := get(t, h, path)
		doc := liveDoc(t, body)
		roots := liveFindAll(doc, func(n *html.Node) bool {
			_, a := liveAttr(n, "data-page-heading")
			_, b := liveAttr(n, "data-live-page")
			_, c := liveAttr(n, "data-live-source")
			return a || b || c
		})
		if len(roots) == 0 {
			t.Fatalf("%s rendered no helper markup", path)
		}
		for _, root := range roots {
			for _, n := range liveFindAll(root, func(*html.Node) bool { return true }) {
				v, _ := liveAttr(n, "class")
				for _, c := range strings.Fields(v) {
					seen[c] = true
				}
			}
		}
	}
	for c := range seen {
		var b strings.Builder
		b.WriteByte('.')
		for _, r := range c {
			if !(r == '-' || r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
				b.WriteByte('\\')
			}
			b.WriteRune(r)
		}
		if !strings.Contains(string(css), b.String()) {
			t.Errorf("built CSS lacks %q (selector %s); run task gallery:css", c, b.String())
		}
	}
	for _, required := range []string{"invisible", "col-start-1", "row-start-1", "sm:flex-row", "*:min-h-11"} {
		if !seen[required] {
			t.Errorf("helpers no longer emit %q; update this contract deliberately", required)
		}
	}
}

type liveStream struct {
	lines  chan string
	cancel context.CancelFunc
	resp   *http.Response
}

func openLiveStream(t *testing.T, base, lastID string) *liveStream {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/live/events", nil)
	req.Header.Set("Accept", "text/event-stream")
	if lastID != "" {
		req.Header.Set("Last-Event-ID", lastID)
	}
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	s := &liveStream{lines: make(chan string, 256), cancel: cancel, resp: resp}
	t.Cleanup(func() { cancel(); resp.Body.Close() })
	if resp.StatusCode != http.StatusOK {
		close(s.lines)
		return s
	}
	go func() {
		defer close(s.lines)
		r := bufio.NewReader(resp.Body)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			s.lines <- strings.TrimSuffix(line, "\n")
		}
	}()
	return s
}

// frame reads lines up to the next blank line, skipping comments and retry.
func (s *liveStream) frame(t *testing.T) []string {
	t.Helper()
	var out []string
	deadline := time.After(3 * time.Second)
	for {
		select {
		case line, ok := <-s.lines:
			if !ok {
				t.Fatalf("stream ended while reading a frame: %v", out)
			}
			if line == "" {
				if len(out) > 0 {
					return out
				}
				continue
			}
			if strings.HasPrefix(line, ":") || strings.HasPrefix(line, "retry:") {
				continue
			}
			out = append(out, line)
		case <-deadline:
			t.Fatalf("timed out waiting for a frame; got %v", out)
		}
	}
}

func (s *liveStream) ended(t *testing.T) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case _, ok := <-s.lines:
			if !ok {
				return // frames still in flight are drained; the stream must close
			}
		case <-deadline:
			t.Fatal("stream was not closed")
		}
	}
}

func livePost(t *testing.T, base, path string, values url.Values, fetch bool) (int, string, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, base+path, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", base)
	if fetch {
		req.Header.Set("X-Live-Fixture", "fetch")
	}
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header.Get("Location"), string(body)
}

func frameOf(id int, sources string) []string {
	return []string{"id: " + strconv.Itoa(id), "event: change", `data: {"sources":[` + sources + `]}`}
}

func sameFrame(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("frame = %q, want %q", got, want)
	}
}

func TestLiveFixtureStreamsReplaysAndFaults(t *testing.T) {
	srv := httptest.NewServer(New())
	defer srv.Close()
	base := srv.URL

	a := openLiveStream(t, base, "")
	if a.resp.Header.Get("Content-Type") != "text/event-stream" || a.resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("stream headers: %v", a.resp.Header)
	}
	status, location, _ := livePost(t, base, "/live/publish", url.Values{"source": {"plans:fixture"}}, false)
	if status != http.StatusSeeOther || location != "/live" {
		t.Fatalf("script-free publish = %d %q", status, location)
	}
	sameFrame(t, a.frame(t), frameOf(1, `"plans:fixture"`))
	status, _, text := livePost(t, base, "/live/publish", url.Values{"source": {"*"}}, true)
	if status != http.StatusOK || text != "Published event 2 for *." {
		t.Fatalf("fetch publish = %d %q", status, text)
	}
	sameFrame(t, a.frame(t), frameOf(2, `"*"`))

	b := openLiveStream(t, base, "1")
	sameFrame(t, b.frame(t), frameOf(2, `"*"`))
	future := openLiveStream(t, base, "99")
	sameFrame(t, future.frame(t), frameOf(2, `"*"`))

	for i := 0; i < liveReplayWindow+3; i++ {
		livePost(t, base, "/live/publish", url.Values{"source": {"index:fixture"}}, true)
	}
	last := 2 + liveReplayWindow + 3
	gap := openLiveStream(t, base, "1")
	sameFrame(t, gap.frame(t), frameOf(last, `"*"`))
	recent := openLiveStream(t, base, strconv.Itoa(last-1))
	sameFrame(t, recent.frame(t), frameOf(last, `"index:fixture"`))

	status, _, _ = livePost(t, base, "/live/drop", nil, true)
	if status != http.StatusOK {
		t.Fatalf("drop = %d", status)
	}
	for _, s := range []*liveStream{a, b, future, gap, recent} {
		s.ended(t)
	}

	if status, _, _ = livePost(t, base, "/live/outage", url.Values{"count": {"2"}}, true); status != http.StatusOK {
		t.Fatalf("outage = %d", status)
	}
	for i := 0; i < 2; i++ {
		if s := openLiveStream(t, base, ""); s.resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("outage connection %d = %d", i, s.resp.StatusCode)
		}
	}
	var open []*liveStream
	for i := 0; i < liveMaxStreams; i++ {
		s := openLiveStream(t, base, "")
		if s.resp.StatusCode != http.StatusOK {
			t.Fatalf("stream %d = %d", i, s.resp.StatusCode)
		}
		open = append(open, s)
	}
	if s := openLiveStream(t, base, ""); s.resp.StatusCode != http.StatusTooManyRequests || s.resp.Header.Get("Retry-After") != "1" {
		t.Fatalf("over-capacity stream = %d", s.resp.StatusCode)
	}
	for _, s := range open {
		s.cancel()
	}

	_, page := getURL(t, base+"/live")
	if !strings.Contains(page, `data-live-last-event-id="`+strconv.Itoa(last)+`"`) {
		t.Error("the rendered page must carry the stream position it shows")
	}
	if !strings.Contains(page, "revision 1") || !strings.Contains(page, "revision "+strconv.Itoa(liveReplayWindow+4)) {
		t.Error("refreshed page does not render the published revisions")
	}
}

func getURL(t *testing.T, u string) (int, string) {
	t.Helper()
	resp, err := http.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestLiveFixtureRefusesUnsafeOrMalformedRequests(t *testing.T) {
	srv := httptest.NewServer(New())
	defer srv.Close()
	base := srv.URL
	request := func(method, path string, header map[string]string, body string) int {
		req, _ := http.NewRequest(method, base+path, strings.NewReader(body))
		for k, v := range header {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultTransport.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	stream := map[string]string{"Accept": "text/event-stream"}
	form := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
	for _, tc := range []struct {
		name, method, path, body string
		header                   map[string]string
		want                     int
	}{
		{"stream without accept", "GET", "/live/events", "", nil, http.StatusNotAcceptable},
		{"stream POST", "POST", "/live/events", "", stream, http.StatusMethodNotAllowed},
		{"stream query", "GET", "/live/events?x=1", "", stream, http.StatusBadRequest},
		{"stream bad cursor", "GET", "/live/events", "", map[string]string{"Accept": "text/event-stream", "Last-Event-ID": "abc"}, http.StatusBadRequest},
		{"cross-site stream", "GET", "/live/events", "", map[string]string{"Accept": "text/event-stream", "Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
		{"publish GET", "GET", "/live/publish", "", nil, http.StatusMethodNotAllowed},
		{"unknown source", "POST", "/live/publish", "source=other", form, http.StatusBadRequest},
		{"two sources", "POST", "/live/publish", "source=*&source=*", form, http.StatusBadRequest},
		{"foreign origin", "POST", "/live/publish", "source=*", map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Origin": "https://example.com"}, http.StatusForbidden},
		{"oversized outage", "POST", "/live/outage", "count=99", form, http.StatusBadRequest},
		{"negative outage", "POST", "/live/outage", "count=-1", form, http.StatusBadRequest},
		{"large body", "POST", "/live/publish", "source=" + strings.Repeat("x", 2048), form, http.StatusBadRequest},
		{"unknown variant", "GET", "/page-heading?variant=wide", "", nil, http.StatusBadRequest},
		{"repeated variant", "GET", "/page-heading?variant=full&variant=long", "", nil, http.StatusBadRequest},
		{"heading POST", "POST", "/page-heading", "", nil, http.StatusMethodNotAllowed},
	} {
		if got := request(tc.method, tc.path, tc.header, tc.body); got != tc.want {
			t.Errorf("%s: %d, want %d", tc.name, got, tc.want)
		}
	}
	// A non-loopback peer cannot publish even with a matching origin.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/live/publish", strings.NewReader("source=*"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	New().ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("remote publish = %d", w.Code)
	}
}

func TestPageHeadingSpecimenVariants(t *testing.T) {
	h := New()
	for _, v := range pageHeadingVariants {
		t.Run(v.Key, func(t *testing.T) {
			res, body := get(t, h, "/page-heading?variant="+v.Key+"&theme=dark")
			if res.StatusCode != http.StatusOK {
				t.Fatalf("status %d", res.StatusCode)
			}
			doc := liveDoc(t, body)
			h1 := liveFindAll(doc, func(n *html.Node) bool { return n.Data == "h1" })
			if len(h1) != 1 {
				t.Fatalf("variant renders %d h1 elements", len(h1))
			}
			if len(liveFindAll(doc, liveHas("data-page-heading"))) != 1 {
				t.Fatal("variant must render one page heading")
			}
			current := liveFindAll(doc, func(n *html.Node) bool {
				v, _ := liveAttr(n, "aria-current")
				return n.Data == "a" && v == "page" && n.Parent != nil && strings.Contains(attrOr(n.Parent, "aria-label"), "variants")
			})
			if len(current) != 1 {
				t.Fatal("variant switcher must mark exactly one current variant")
			}
			if !strings.Contains(body, `href="/page-heading?theme=light&amp;variant=`+v.Key+`"`) {
				t.Error("theme links must keep the selected variant")
			}
		})
	}
	_, long := get(t, h, "/page-heading?variant=long")
	if strings.Contains(long, "javascript:") || !strings.Contains(long, `aria-disabled="true"`) {
		t.Error("unsafe specimen action must render the shared inert label")
	}
	_, def := get(t, h, "/page-heading")
	if !strings.Contains(def, "Plans for fathom") {
		t.Error("default variant should be full")
	}
}

func attrOr(n *html.Node, key string) string {
	v, _ := liveAttr(n, key)
	return v
}
