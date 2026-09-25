package gallery

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/internal/designassets"
)

func referenceFixture(t *testing.T) (string, *designassets.Catalog) {
	t.Helper()
	source, root := t.TempDir(), t.TempDir()
	for _, name := range []string{"catalyst-ui-kit.zip", "studio.zip", "marketing-v4.zip"} {
		var body bytes.Buffer
		z := zip.NewWriter(&body)
		for _, f := range []struct{ name, body string }{
			{"README.md", "# Synthetic reference\r\n<script>window.fixtureSentinel=true</script>\n"},
			{"package.json", `{"name":"<img src=x onerror=fixture()>","dependencies":{"react":"^19.0.0"},"devDependencies":{"tailwindcss":"^4.0.0"}}`},
			{"src/one.tsx", "export const one = '<button>Synthetic</button>';\n"},
			{"src/two.tsx", "export const two = 'synthetic';\n"},
			{"src/<img src=x onerror=fixture()>.tsx", "export const escaped = 'synthetic';\n"},
			{"theme.css", ":root { --color-synthetic: olive; }"},
			{"large.txt", strings.Repeat("LARGE_INLINE_PRIVATE_MARKER", 1400)},
			{"huge.txt", strings.Repeat("x", int(designassets.MaxTextBytes)+1)},
			{".env", "SYNTHETIC_SECRET=fixture-only"},
			{"image.png", "\x89PNG\x00binary"},
			{"key.txt", "-----BEGIN PRIVATE KEY-----\nsynthetic only"},
		} {
			w, err := z.Create(f.name)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = w.Write([]byte(f.body)); err != nil {
				t.Fatal(err)
			}
		}
		if err := z.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(source, name), body.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
	c, err := designassets.Import(context.Background(), source, root)
	if err != nil {
		t.Fatal(err)
	}
	return root, c
}

func referenceHandler(root string) http.Handler {
	mux := http.NewServeMux()
	registerReferences(mux, root)
	return mux
}
func referenceRequest(h http.Handler, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, nil)
	r.Host = "127.0.0.1:8765"
	r.RemoteAddr = "127.0.0.1:12345"
	for key, value := range headers {
		if key == "Host" {
			r.Host = value
		} else {
			r.Header.Set(key, value)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func referenceDecode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	if w.Code != 200 {
		t.Fatalf("status=%d, body=%s", w.Code, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("missing private JSON headers: %v", w.Header())
	}
	var out T
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestReferencesJSONParityAndPagination(t *testing.T) {
	root, c := referenceFixture(t)
	h := referenceHandler(root)
	q := designassets.Query{Limit: 1}
	seen := map[string]bool{}
	for pageNo := 0; pageNo < 4; pageNo++ {
		v := url.Values{"limit": {"1"}, "cursor": {q.Cursor}}
		w := referenceRequest(h, "GET", referenceLink("/api/references/search", v), nil)
		got := referenceDecode[designassets.Page](t, w)
		want, err := c.Search(q)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("search parity: %+v %v", got, err)
		}
		if strings.Contains(w.Body.String(), "fixtureSentinel") || strings.Contains(w.Body.String(), `"files"`) {
			t.Fatal("search exposed source/inventory bodies")
		}
		for _, item := range got.Items {
			if seen[item.ID] {
				t.Fatal("duplicate page ID")
			}
			seen[item.ID] = true
		}
		if got.NextCursor == "" {
			break
		}
		q.Cursor = got.NextCursor
	}
	if len(seen) != len(c.List()) {
		t.Fatal("pagination omitted archive")
	}
	for _, a := range c.List() {
		base := "/api/references/" + url.PathEscape(a.ID)
		got := referenceDecode[referenceMetadata](t, referenceRequest(h, "GET", base, nil))
		verified, err := designassets.Verify(root, a.ID)
		if err != nil || got.Revision != c.Revision || !reflect.DeepEqual(got.Asset, designassets.Summarize(verified)) || !reflect.DeepEqual(got.Projects, verified.Projects) || got.Notice == "" {
			t.Fatal("exact metadata parity failed")
		}
		fileQuery := designassets.FileQuery{ID: a.ID, Prefix: "src/", Kind: "source", Limit: 1}
		count := 0
		for pageNo := 0; pageNo < 4; pageNo++ {
			v := url.Values{"prefix": {"src/"}, "file_kind": {"source"}, "file_limit": {"1"}, "file_cursor": {fileQuery.Cursor}, "kind": {"template"}, "limit": {"50"}, "cursor": {"unrelated-search-cursor"}}
			w := referenceRequest(h, "GET", referenceLink(base+"/files", v), nil)
			page := referenceDecode[designassets.FilePage](t, w)
			want, err := c.FilesPage(fileQuery)
			if err != nil || !reflect.DeepEqual(page, want) || w.Header().Get("X-Reference-Integrity") != "metadata-only" {
				t.Fatalf("file parity/filter collision: %+v %v", page, err)
			}
			count += len(page.Items)
			if page.NextCursor == "" {
				break
			}
			fileQuery.Cursor = page.NextCursor
		}
		if count != 3 {
			t.Fatalf("file pagination got %d source files", count)
		}
		text := referenceDecode[designassets.TextFile](t, referenceRequest(h, "GET", referenceLink(base+"/source", url.Values{"path": {"README.md"}}), nil))
		wantText, err := designassets.ReadFile(root, a.ID, "README.md")
		if err != nil || !reflect.DeepEqual(text, *wantText) {
			t.Fatal("source bytes/hash parity failed")
		}
	}
	for _, suffix := range []string{"?q=unmatched", "?family=studio&kind=ui-kit"} {
		page := referenceDecode[designassets.Page](t, referenceRequest(h, "GET", "/api/references/search"+suffix, nil))
		if page.Total != 0 || page.Items == nil {
			t.Fatal("no-match must be empty results, not missing data")
		}
	}
}

func TestReferencesAccessBoundaryAndMethods(t *testing.T) {
	root, c := referenceFixture(t)
	h := referenceHandler(root)
	id := c.List()[0].ID
	routes := []string{"/references", "/references/" + id, "/api/references/search", "/api/references/" + id, "/api/references/" + id + "/files", "/api/references/" + id + "/source?path=README.md"}
	for _, target := range routes {
		for _, headers := range []map[string]string{
			{"Host": "example.com"}, {"Host": "localhost.evil"}, {"Host": "127.0.0.1.evil"}, {"Host": "localhost."}, {"Host": "localhost:0"}, {"Host": "localhost:99999"}, {"Host": "user@localhost"}, {"Host": "localhost:08765"},
			{"Origin": "https://evil.example"}, {"Origin": "null"}, {"Origin": "http://127.0.0.1:8766"}, {"Origin": "https://127.0.0.1:8765"}, {"Origin": "http://localhost:8765"}, {"Origin": "http://user:pass@127.0.0.1:8765"}, {"Origin": "http://127.0.0.1:8765/path"},
			{"Sec-Fetch-Site": "cross-site"}, {"Sec-Fetch-Site": "same-site"}, {"Origin": "http://127.0.0.1:8765", "Sec-Fetch-Site": "cross-site"},
		} {
			w := referenceRequest(h, "GET", target, headers)
			if w.Code != 403 {
				t.Errorf("%s headers=%v got %d", target, headers, w.Code)
			}
			if strings.Contains(w.Body.String(), "fixtureSentinel") || strings.Contains(w.Body.String(), "^19.0.0") || strings.Contains(w.Body.String(), id) {
				t.Fatal("denied response leaked reference")
			}
			if w.Header().Get("Access-Control-Allow-Origin") != "" || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("denied response missing privacy headers")
			}
		}
		for _, method := range []string{"HEAD", "POST", "PUT", "DELETE", "OPTIONS"} {
			w := referenceRequest(h, method, target, nil)
			if w.Code != 405 || w.Header().Get("Allow") != "GET" {
				t.Errorf("method %s %s status=%d", method, target, w.Code)
			}
		}
	}
	for _, headers := range []map[string]string{nil, {"Origin": "http://127.0.0.1:8765"}, {"Sec-Fetch-Site": "same-origin"}, {"Sec-Fetch-Site": "none"}, {"Host": "localhost"}, {"Host": "LOCALHOST:8765", "Origin": "http://localhost:8765"}, {"Host": "[::1]:8765", "Origin": "http://[::1]:8765"}, {"Host": "127.0.0.1:80", "Origin": "http://127.0.0.1"}} {
		if w := referenceRequest(h, "GET", "/api/references/search", headers); w.Code != 200 {
			t.Errorf("valid local request rejected %v: %d", headers, w.Code)
		}
	}
	for _, header := range []string{"Origin", "Sec-Fetch-Site"} {
		r := httptest.NewRequest("GET", "/api/references/search", nil)
		r.Host = "127.0.0.1:8765"
		r.RemoteAddr = "127.0.0.1:12345"
		r.Header.Add(header, "same-origin")
		r.Header.Add(header, "same-origin")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("multiple authority headers accepted")
		}
	}
	for _, peer := range []string{"", "192.0.2.1:12345", "[2001:db8::1]:12345", "localhost:12345", "127.0.0.1", "127.0.0.1:bogus"} {
		r := httptest.NewRequest("GET", "/api/references/search", nil)
		r.Host = "127.0.0.1:8765"
		r.RemoteAddr = peer
		r.Header.Set("Origin", "http://127.0.0.1:8765")
		r.Header.Set("X-Forwarded-For", "127.0.0.1")
		r.Header.Set("Forwarded", "for=127.0.0.1;host=127.0.0.1:8765")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Errorf("nonlocal/unknown peer accepted through spoofed forwarding metadata: %q", peer)
		}
	}
	mux := http.NewServeMux()
	registerCatalog(mux)
	registerReferences(mux, root)
	if w := referenceRequest(mux, "GET", "/api/catalog/search", map[string]string{"Host": "example.com", "Origin": "https://elsewhere.example"}); w.Code != 200 {
		t.Fatal("private restrictions leaked into first-party catalog")
	}
}

func TestReferencesMissingCorruptAndExactIntegrity(t *testing.T) {
	markerOnly := t.TempDir()
	marker, err := os.ReadFile(filepath.Join("..", "..", "data", "design-assets", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(markerOnly, "go.mod"), marker, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{"", filepath.Join(t.TempDir(), "missing"), markerOnly} {
		h := referenceHandler(root)
		w := referenceRequest(h, "GET", "/references?theme=dark", nil)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "assets-import") || !strings.Contains(w.Body.String(), "/catalog") {
			t.Fatal("missing library has no usable setup state")
		}
		for _, path := range []string{"/api/references/search", "/api/references/missing", "/api/references/missing/files", "/api/references/missing/source?path=README.md"} {
			w := referenceRequest(h, "GET", path, nil)
			if w.Code != 503 {
				t.Errorf("missing API status=%d", w.Code)
			}
			if root != "" && strings.Contains(w.Body.String(), root) {
				t.Fatal("setup response leaked absolute store path")
			}
		}
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "catalog.json"), []byte("corrupt private catalog"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/references", "/api/references/search"} {
		w := referenceRequest(referenceHandler(root), "GET", path, nil)
		if w.Code != 500 || !strings.Contains(w.Body.String(), "repair") || strings.Contains(w.Body.String(), root) {
			t.Fatalf("corrupt store response status=%d", w.Code)
		}
	}
	root, c := referenceFixture(t)
	a := c.List()[0]
	h := referenceHandler(root)
	if err := os.WriteFile(filepath.Join(root, "archives", a.SHA256+".zip"), []byte("corrupt synthetic archive"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/references/" + a.ID, "/api/references/" + a.ID, "/api/references/" + a.ID + "/source?path=README.md"} {
		w := referenceRequest(h, "GET", path, nil)
		if w.Code != 500 || strings.Contains(w.Body.String(), root) || strings.Contains(w.Body.String(), "fixtureSentinel") {
			t.Errorf("exact read did not fail safely %s status=%d", path, w.Code)
		}
	}
	for _, path := range []string{"/api/references/search", "/api/references/" + a.ID + "/files"} {
		if w := referenceRequest(h, "GET", path, nil); w.Code != 200 {
			t.Fatal("metadata listing unexpectedly claims fresh ZIP verification")
		}
	}
}

func TestReferencesInvalidQueriesAndSourceRefusal(t *testing.T) {
	root, c := referenceFixture(t)
	h := referenceHandler(root)
	a := c.List()[0]
	base := "/api/references/" + a.ID
	for _, suffix := range []string{"?limit=0", "?limit=-1", "?limit=51", "?limit=bogus", "?kind=runtime", "?cursor=bogus", "?q=a&q=b", "?q=%zz", "?q=" + strings.Repeat("x", 17000)} {
		for _, path := range []string{"/references", "/api/references/search"} {
			if w := referenceRequest(h, "GET", path+suffix, nil); w.Code != 400 {
				t.Errorf("bad search %s status=%d", suffix, w.Code)
			}
		}
	}
	for _, suffix := range []string{"?file_limit=0", "?file_limit=51", "?file_limit=bogus", "?file_kind=invalid", "?file_cursor=bogus", "?prefix=../", "?prefix=/", "?prefix=a%5Cb"} {
		if w := referenceRequest(h, "GET", base+"/files"+suffix, nil); w.Code != 400 {
			t.Errorf("bad file query %s status=%d", suffix, w.Code)
		}
	}
	for _, path := range []string{"/references/missing", "/api/references/missing", "/api/references/missing/files", "/api/references/missing/source?path=README.md"} {
		if w := referenceRequest(h, "GET", path, nil); w.Code != 404 {
			t.Errorf("unknown ID %s status=%d", path, w.Code)
		}
	}
	for _, path := range []string{"../catalog.json", "/etc/passwd", "README.md/../package.json", "src\\one.tsx", "missing", ""} {
		if w := referenceRequest(h, "GET", referenceLink(base+"/source", url.Values{"path": {path}}), nil); w.Code != 404 {
			t.Errorf("unsafe/unknown path %q status=%d", path, w.Code)
		}
	}
	for _, path := range []string{".env", "image.png", "huge.txt", "key.txt"} {
		w := referenceRequest(h, "GET", referenceLink(base+"/source", url.Values{"path": {path}}), nil)
		if w.Code != 400 || strings.Contains(w.Body.String(), "SYNTHETIC_SECRET") || strings.Contains(w.Body.String(), "BEGIN PRIVATE KEY") {
			t.Errorf("unsafe text %s status=%d", path, w.Code)
		}
	}
	first, _ := c.Search(designassets.Query{Limit: 1})
	if w := referenceRequest(h, "GET", referenceLink("/api/references/search", url.Values{"limit": {"2"}, "cursor": {first.NextCursor}}), nil); w.Code != 400 {
		t.Fatal("changed search cursor accepted")
	}
	firstFiles, _ := c.FilesPage(designassets.FileQuery{ID: a.ID, Limit: 1})
	if w := referenceRequest(h, "GET", referenceLink(base+"/files", url.Values{"file_limit": {"2"}, "file_cursor": {firstFiles.NextCursor}}), nil); w.Code != 400 {
		t.Fatal("changed file cursor accepted")
	}
}

func TestReferencesEscapingAndSafeNavigationContext(t *testing.T) {
	root, c := referenceFixture(t)
	h := referenceHandler(root)
	a := c.List()[0]
	v := url.Values{"q": {"<img src=x onerror=fixture()>"}, "kind": {"ui-kit"}, "family": {"catalyst"}, "limit": {"1"}, "cursor": {"search-context-only"}, "theme": {"dark"}, "prefix": {"src/"}, "file_kind": {"source"}, "file_limit": {"1"}, "file": {"README.md"}, "back": {"https://evil.example/escape"}, "next": {"//evil.example/escape"}}
	path := referenceLink("/references/"+a.ID, v)
	w := referenceRequest(h, "GET", path, nil)
	body := w.Body.String()
	if w.Code != 200 {
		t.Fatalf("detail status=%d: %s", w.Code, body)
	}
	for _, literal := range []string{"<script>window.fixtureSentinel=true</script>", "<img src=x onerror=fixture()>"} {
		if strings.Contains(body, literal) {
			t.Fatal("source/metadata entered page without escaping")
		}
	}
	if !strings.Contains(body, "&lt;script&gt;window.fixtureSentinel=true&lt;/script&gt;") {
		t.Fatal("escaped original source not visible")
	}
	if !strings.Contains(body, `data-theme="dark"`) {
		t.Fatal("theme lost")
	}
	links := regexp.MustCompile(`href="([^"]+)"`).FindAllStringSubmatch(body, -1)
	var back, next, source string
	for _, match := range links {
		link := html.UnescapeString(match[1])
		if strings.Contains(link, "evil.example") {
			t.Fatal("untrusted back/next became a link")
		}
		u, err := url.Parse(link)
		if err != nil {
			t.Fatal(err)
		}
		if u.Path == "/references" && u.Query().Get("cursor") == "search-context-only" {
			back = link
		}
		if u.Path == "/references/"+a.ID && u.Query().Get("file_cursor") != "" {
			next = link
		}
		if strings.HasSuffix(u.Path, "/source") {
			source = link
		}
	}
	if back == "" || next == "" || source == "" {
		t.Fatal("missing back/file-pagination/source links")
	}
	backURL, _ := url.Parse(back)
	nextURL, _ := url.Parse(next)
	sourceURL, _ := url.Parse(source)
	if backURL.Query().Get("file") != "" || backURL.Query().Get("prefix") != "" || backURL.Query().Get("theme") != "dark" || nextURL.Query().Get("cursor") != "search-context-only" || nextURL.Query().Get("file_limit") != "1" || sourceURL.IsAbs() || sourceURL.Query().Get("path") != "README.md" {
		t.Fatal("search/file/theme context collided or source link escaped origin")
	}
	if w := referenceRequest(h, "GET", next, nil); w.Code != 200 {
		t.Fatal("next file page not usable with retained search cursor")
	}
	if w := referenceRequest(h, "GET", referenceLink("/references", url.Values{"q": {"<script>fixture()</script>"}}), nil); w.Code != 200 || strings.Contains(w.Body.String(), "<script>fixture()</script>") {
		t.Fatal("search text unescaped")
	}
}

func TestReferencesLargeTextUsesExplicitFallbackWithoutTruncation(t *testing.T) {
	root, c := referenceFixture(t)
	h := referenceHandler(root)
	a := c.List()[0]
	w := referenceRequest(h, "GET", referenceLink("/references/"+a.ID, url.Values{"file": {"large.txt"}}), nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "32 KiB") || !strings.Contains(w.Body.String(), "No source was truncated") || !strings.Contains(w.Body.String(), "assets-get") || strings.Contains(w.Body.String(), "LARGE_INLINE_PRIVATE_MARKER") {
		t.Fatal("large HTML source silently truncated or embedded")
	}
	got := referenceDecode[designassets.TextFile](t, referenceRequest(h, "GET", referenceLink("/api/references/"+a.ID+"/source", url.Values{"path": {"large.txt"}}), nil))
	want, err := designassets.ReadFile(root, a.ID, "large.txt")
	if err != nil || got.Content != want.Content || got.Size != int64(len(want.Content)) {
		t.Fatal("JSON fallback is not complete exact text")
	}
}

func TestReferencesRealHTTPHeadersAndReadOnlyStore(t *testing.T) {
	root, c := referenceFixture(t)
	before, err := os.ReadFile(filepath.Join(root, "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(referenceHandler(root))
	defer server.Close()
	response, err := http.Get(server.URL + "/api/references/search")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || response.StatusCode != 200 || !strings.Contains(string(body), c.Revision) {
		t.Fatal("real loopback HTTP lookup failed")
	}
	if response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("Referrer-Policy") != "no-referrer" || response.Header.Get("X-Frame-Options") != "DENY" {
		t.Fatal("real response missing privacy headers")
	}
	if response.Header.Get("Content-Security-Policy") != "default-src 'none'; style-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'" {
		t.Fatal("private source route CSP permits unneeded executable content")
	}
	after, err := os.ReadFile(filepath.Join(root, "catalog.json"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("read-only request modified manifest")
	}
	for _, a := range c.List() {
		if _, err := designassets.Verify(root, a.ID); err != nil {
			t.Fatal("read-only requests modified original archive")
		}
	}
}
