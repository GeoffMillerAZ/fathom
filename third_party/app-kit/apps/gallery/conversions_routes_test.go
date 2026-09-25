package gallery

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/internal/designassets"
	"github.com/geoffmilleraz/app-kit/internal/designattempts"
	"github.com/geoffmilleraz/app-kit/internal/designconvert"
	"golang.org/x/net/html"
)

// conversionFixture creates exactly two real, published attempts from synthetic
// originals. No purchased source, generated execution or browser runtime needed.
func conversionFixture(t *testing.T) (string, *designattempts.Catalog) {
	t.Helper()
	source, root := t.TempDir(), t.TempDir()
	var archive bytes.Buffer
	z := zip.NewWriter(&archive)
	for _, member := range []struct{ name, content string }{
		{"html/static.html", "<section>\r\n<p title=\"&quot; onmouseover=&quot;fixture()\">&lt;script&gt;fixture()&lt;/script&gt;</p>\n<p>Second source line</p>\n<p>Third source line</p>\n</section>\n"},
		{"html/<img src=x onerror=fixture()>.html", "<el-dialog>\n<button>Unported source</button>\n</el-dialog>\n<script>window.fixtureSentinel=true</script>\n"},
	} {
		w, err := z.Create(member.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(member.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "marketing-v4.zip"), archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	assets, err := designassets.Import(context.Background(), source, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"html/static.html", "html/<img src=x onerror=fixture()>.html"} {
		original, err := designassets.ReadFile(root, assets.List()[0].ID, name)
		if err != nil {
			t.Fatal(err)
		}
		result, err := designconvert.Translate(designconvert.Source{ArchiveID: original.ArchiveID, Path: original.Path,
			SHA256: original.SHA256, Size: len(original.Content)}, []byte(original.Content))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := designattempts.Publish(root, result); err != nil {
			t.Fatal(err)
		}
	}
	c, err := designattempts.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, entry := range c.List() {
		counts[entry.Status]++
	}
	if len(c.List()) != 2 || counts["translated"] != 1 || counts["unsupported"] != 1 {
		t.Fatal("synthetic fixture must contain one translated and one unsupported attempt")
	}
	return root, c
}

func conversionEntry(t *testing.T, c *designattempts.Catalog, status string) designattempts.Entry {
	t.Helper()
	for _, entry := range c.List() {
		if entry.Status == status {
			return entry
		}
	}
	t.Fatalf("fixture missing %s attempt", status)
	return designattempts.Entry{}
}

func conversionStoreSnapshot(t *testing.T, root string) map[string][32]byte {
	t.Helper()
	files := map[string][32]byte{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[name] = sha256.Sum256(body)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func conversionDOM(t *testing.T, body string) (*html.Node, []*url.URL) {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	mainCount, ids := 0, map[string]bool{}
	var links []*url.URL
	var walk func(*html.Node, bool)
	walk = func(n *html.Node, inMain bool) {
		if n.Type == html.ElementNode {
			if n.Data == "script" || n.Data == "iframe" || n.Data == "object" || n.Data == "el-dialog" || n.Data == "img" {
				t.Fatalf("source became an executable/active page element: %s", n.Data)
			}
			if n.Data == "main" {
				if inMain {
					t.Fatal("nested main landmark")
				}
				mainCount++
				inMain = true
			}
			for _, attr := range n.Attr {
				if strings.HasPrefix(strings.ToLower(attr.Key), "on") {
					t.Fatal("source became an inline event handler")
				}
				if attr.Key == "id" {
					if ids[attr.Val] {
						t.Fatal("duplicate HTML ID", attr.Val)
					}
					ids[attr.Val] = true
				}
				if attr.Key == "href" {
					u, err := url.Parse(attr.Val)
					if err != nil || u.IsAbs() || u.Host != "" || strings.HasPrefix(attr.Val, "//") {
						t.Fatal("comparison navigation escaped same origin")
					}
					links = append(links, u)
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child, inMain)
		}
	}
	walk(doc, false)
	if mainCount != 1 {
		t.Fatalf("want one main landmark, got %d", mainCount)
	}
	return doc, links
}

func TestConversionsJSONParityPaginationAndArtifactDownload(t *testing.T) {
	root, c := conversionFixture(t)
	before := conversionStoreSnapshot(t, root)
	h := referenceHandler(root)
	q, seen := designattempts.Query{Limit: 1}, map[string]bool{}
	for n := 0; n < 2; n++ {
		values := url.Values{"limit": {"1"}, "theme": {"dark"}}
		if q.Cursor != "" {
			values.Set("cursor", q.Cursor)
		}
		w := referenceRequest(h, "GET", "/api/conversions?"+values.Encode(), nil)
		page := referenceDecode[designattempts.Page](t, w)
		want, err := c.Search(q)
		if err != nil || !reflect.DeepEqual(page, want) || len(page.Entries) != 1 {
			t.Fatal("conversion metadata API diverges from shared catalog", err)
		}
		if strings.Contains(w.Body.String(), `"translation"`) || strings.Contains(w.Body.String(), `"content"`) || strings.Contains(w.Body.String(), "Second source line") {
			t.Fatal("metadata list exposed source/artifact body")
		}
		seen[page.Entries[0].ID], q.Cursor = true, page.NextCursor
	}
	if len(seen) != 2 || q.Cursor != "" {
		t.Fatal("pagination omitted an attempt or failed to terminate")
	}
	for _, entry := range c.List() {
		for _, selector := range []designattempts.Query{{ArchiveID: entry.Source.ArchiveID, Path: entry.Source.Path}, {Status: entry.Status}} {
			v := url.Values{"archive_id": {selector.ArchiveID}, "path": {selector.Path}, "status": {selector.Status}}
			page := referenceDecode[designattempts.Page](t, referenceRequest(h, "GET", "/api/conversions?"+v.Encode(), nil))
			want, err := c.Search(selector)
			if err != nil || !reflect.DeepEqual(page, want) {
				t.Fatal("exact source/status selectors differ from shared catalog", err)
			}
		}
		got := referenceDecode[designattempts.Detail](t, referenceRequest(h, "GET", "/api/conversions/"+entry.ID, nil))
		want, err := designattempts.Read(root, entry.ID)
		if err != nil || !reflect.DeepEqual(got, want) || got.Entry.Source != entry.Source || got.Replay != "matches-current" || got.Entry.Verification != "not-verified" {
			t.Fatal("exact API changed identity, full envelope or replay", err)
		}
		download := referenceRequest(h, "GET", "/conversions/"+entry.ID+"/artifact", nil)
		if entry.Status == "unsupported" {
			if download.Code != http.StatusConflict {
				t.Fatalf("unsupported attempt fabricated downloadable artifact: %d", download.Code)
			}
			continue
		}
		media, _, err := mime.ParseMediaType(download.Header().Get("Content-Type"))
		disposition, params, dispositionErr := mime.ParseMediaType(download.Header().Get("Content-Disposition"))
		if download.Code != 200 || err != nil || media != "text/plain" || dispositionErr != nil || disposition != "attachment" || params["filename"] != want.Translation.Artifact.Filename {
			t.Fatal("artifact download is not an inert named text attachment")
		}
		if !bytes.Equal(download.Body.Bytes(), []byte(want.Translation.Artifact.Content)) || fmt.Sprintf("%x", sha256.Sum256(download.Body.Bytes())) != entry.ArtifactSHA256 {
			t.Fatal("download changed saved artifact bytes or hash")
		}
		if download.Header().Get("Cache-Control") != "no-store" || download.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("artifact attachment lost private headers")
		}
	}
	if !reflect.DeepEqual(before, conversionStoreSnapshot(t, root)) {
		t.Fatal("read-only conversion routes changed original or attempt storage")
	}
	if w := referenceRequest(NewWithOptions(Options{DesignAssetsRoot: root}), "GET", "/conversions", nil); w.Code != 200 {
		t.Fatal("conversion route not integrated into Gallery options")
	}
}

func TestConversionsAccessBoundaryAndMethods(t *testing.T) {
	root, c := conversionFixture(t)
	h, id := referenceHandler(root), conversionEntry(t, c, "translated").ID
	routes := []string{"/conversions", "/conversions/" + id, "/conversions/" + id + "/artifact", "/api/conversions", "/api/conversions/" + id}
	for _, target := range routes {
		for _, headers := range []map[string]string{
			{"Host": "evil.example"}, {"Host": "localhost.evil"}, {"Origin": "https://evil.example"}, {"Origin": "null"},
			{"Origin": "http://127.0.0.1:8766"}, {"Sec-Fetch-Site": "cross-site"}, {"Sec-Fetch-Site": "same-site"},
		} {
			w := referenceRequest(h, "GET", target, headers)
			if w.Code != 403 || strings.Contains(w.Body.String(), id) || strings.Contains(w.Body.String(), "Second source line") {
				t.Fatalf("private conversion guard failed: %s status=%d", target, w.Code)
			}
			if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("Access-Control-Allow-Origin") != "" || w.Header().Get("Content-Security-Policy") != "default-src 'none'; style-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'" {
				t.Fatal("private conversion guard headers missing")
			}
		}
		for _, method := range []string{"HEAD", "POST", "PUT", "DELETE", "OPTIONS"} {
			w := referenceRequest(h, method, target, nil)
			if w.Code != 405 || w.Header().Get("Allow") != "GET" {
				t.Fatalf("non-GET conversion route accepted: %s %s status=%d", method, target, w.Code)
			}
		}
		for _, peer := range []string{"", "192.0.2.1:9999", "[2001:db8::1]:9999", "localhost:9999"} {
			r := httptest.NewRequest("GET", target, nil)
			r.Host, r.RemoteAddr = "127.0.0.1:8765", peer
			r.Header.Set("X-Forwarded-For", "127.0.0.1")
			r.Header.Set("Forwarded", "for=127.0.0.1;host=127.0.0.1:8765")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != 403 {
				t.Fatalf("spoofed loopback authority accepted remote peer %q", peer)
			}
		}
	}
}

func TestConversionsMalformedQueriesAndCursorBinding(t *testing.T) {
	root, c := conversionFixture(t)
	h, id := referenceHandler(root), conversionEntry(t, c, "translated").ID
	for _, route := range []string{"/conversions", "/api/conversions", "/conversions/" + id, "/api/conversions/" + id, "/conversions/" + id + "/artifact"} {
		for _, query := range []string{
			"limit=0", "limit=-1", "limit=51", "limit=bogus", "limit=1&limit=2", "theme=unknown", "theme=%ff", "theme=%00",
			"status=verified", "archive_id=bad", "path=../escape", "path=a%5Cb", "path=%ff", "path=%00", "path=%zz",
			"unknown=value", "back=https://evil.example", "path=a;status=translated", "path=" + strings.Repeat("x", 17000),
		} {
			if w := referenceRequest(h, "GET", route+"?"+query, nil); w.Code != 400 {
				t.Errorf("bad conversion query accepted: %s?%s status=%d", route, query, w.Code)
			}
		}
	}
	for _, query := range []string{"source_line=0", "source_line=-1", "source_line=bogus", "artifact_line=0", "artifact_line=99999999999999999999999", "lines=0", "lines=401", "lines=bogus", "source_line=1&source_line=2"} {
		if w := referenceRequest(h, "GET", "/conversions/"+id+"?"+query, nil); w.Code != 400 {
			t.Errorf("bad pane bound accepted: %s status=%d", query, w.Code)
		}
	}
	for _, route := range []string{"/conversions/", "/api/conversions/"} {
		for _, encoded := range []string{strings.Replace(id, "syntax@", "syntax%40", 1), "%73" + id[1:], id + "%2fextra"} {
			if w := referenceRequest(h, "GET", route+encoded, nil); w.Code != 400 {
				t.Errorf("encoded exact path alias accepted: %s status=%d", route+encoded, w.Code)
			}
		}
	}
	first, err := c.Search(designattempts.Query{Limit: 1})
	if err != nil || first.NextCursor == "" {
		t.Fatal("fixture missing continuation", err)
	}
	for _, values := range []url.Values{{"limit": {"2"}, "cursor": {first.NextCursor}}, {"limit": {"1"}, "status": {"translated"}, "cursor": {first.NextCursor}}, {"cursor": {"malformed"}}} {
		if w := referenceRequest(h, "GET", "/api/conversions?"+values.Encode(), nil); w.Code != 400 {
			t.Fatal("changed query or malformed cursor accepted")
		}
	}
}

func TestConversionsMissingAndCorruptRecovery(t *testing.T) {
	unknown := "syntax@" + strings.Repeat("0", 64)
	for _, root := range []string{"", filepath.Join(t.TempDir(), "absent")} {
		h := referenceHandler(root)
		w := referenceRequest(h, "GET", "/conversions", nil)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "assets-import") {
			t.Fatal("missing private root has no useful HTML setup state")
		}
		for _, route := range []string{"/api/conversions", "/api/conversions/" + unknown} {
			w := referenceRequest(h, "GET", route, nil)
			if w.Code != 503 || root != "" && strings.Contains(w.Body.String(), root) {
				t.Fatal("missing private root API must be unavailable without absolute paths")
			}
		}
	}
	root, _ := referenceFixture(t)
	h := referenceHandler(root)
	empty := referenceDecode[designattempts.Page](t, referenceRequest(h, "GET", "/api/conversions", nil))
	if empty.Total != 0 || empty.Entries == nil || empty.NextCursor != "" {
		t.Fatal("absent optional attempt store must be an explicit empty page")
	}
	if w := referenceRequest(h, "GET", "/conversions", nil); w.Code != 200 {
		t.Fatal("empty saved attempts page failed")
	}
	if _, err := os.Stat(filepath.Join(root, "translations")); !os.IsNotExist(err) {
		t.Fatal("read-only route created an attempt store")
	}
	root, c := conversionFixture(t)
	h = referenceHandler(root)
	for _, route := range []string{"/conversions/", "/api/conversions/"} {
		if w := referenceRequest(h, "GET", route+unknown, nil); w.Code != 404 {
			t.Fatal("unknown attempt must be 404")
		}
	}
	entry := conversionEntry(t, c, "translated")
	filename := filepath.Join(root, "translations", "entries", strings.TrimPrefix(entry.ID, "syntax@")+".json")
	if err := os.WriteFile(filename, []byte("corrupt synthetic receipt"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, route := range []string{"/conversions/" + entry.ID, "/api/conversions/" + entry.ID, "/conversions/" + entry.ID + "/artifact"} {
		w := referenceRequest(h, "GET", route, nil)
		if w.Code != 500 || strings.Contains(w.Body.String(), root) || strings.Contains(w.Body.String(), "Second source line") {
			t.Fatalf("known corrupt attempt did not fail safely: %s status=%d", route, w.Code)
		}
		if route == "/conversions/"+entry.ID {
			_, links := conversionDOM(t, w.Body.String())
			found := false
			for _, link := range links {
				if link.Path == "/references/"+entry.Source.ArchiveID && link.Query().Get("file") == entry.Source.Path {
					found = true
				}
			}
			if !found {
				t.Fatal("failed saved attempt hides its exact original recovery link")
			}
		}
	}
	originalURL := "/references/" + entry.Source.ArchiveID + "?" + url.Values{"file": {entry.Source.Path}}.Encode()
	if w := referenceRequest(h, "GET", originalURL, nil); w.Code != 200 {
		t.Fatal("corrupt attempt hid the original reference")
	}
	if err := os.WriteFile(filepath.Join(root, "translations", "catalog.json"), []byte("corrupt target index"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, route := range []string{"/references", originalURL} {
		if w := referenceRequest(h, "GET", route, nil); w.Code != 200 {
			t.Fatal("target index corruption disabled source-native recovery")
		}
	}
}

func TestConversionsEscapingAndHistoricalStatus(t *testing.T) {
	root, c := conversionFixture(t)
	h := referenceHandler(root)
	for _, route := range []string{"/conversions?theme=dark", "/conversions/" + conversionEntry(t, c, "translated").ID + "?theme=dark", "/conversions/" + conversionEntry(t, c, "unsupported").ID + "?theme=dark"} {
		w := referenceRequest(h, "GET", route, nil)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `data-theme="dark"`) || !strings.Contains(w.Body.String(), "not-verified") {
			t.Fatalf("comparison lost status/theme or failed: %s status=%d", route, w.Code)
		}
		conversionDOM(t, w.Body.String())
	}
	entry := conversionEntry(t, c, "translated")
	detail, err := designattempts.Read(root, entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Model a canonical same-schema historical receipt, never a public Publish
	// bypass. Preserve the existing current attempt alongside this test revision.
	detail.Translation.Transformer.GoVersion = "historical-test-go"
	body, err := json.Marshal(detail.Translation)
	if err != nil {
		t.Fatal(err)
	}
	entry.ID = fmt.Sprintf("syntax@%x", sha256.Sum256(body))
	entry.Transformer = detail.Translation.Transformer
	if err := os.WriteFile(filepath.Join(root, "translations", "entries", strings.TrimPrefix(entry.ID, "syntax@")+".json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	entries := append(c.List(), entry)
	slices.SortFunc(entries, func(a, b designattempts.Entry) int { return strings.Compare(a.ID, b.ID) })
	identity, _ := json.Marshal(struct {
		SchemaVersion int                    `json:"schema_version"`
		Entries       []designattempts.Entry `json:"entries"`
	}{1, entries})
	index, _ := json.Marshal(designattempts.Catalog{SchemaVersion: 1, Revision: fmt.Sprintf("%x", sha256.Sum256(identity)), Entries: entries})
	if err := os.WriteFile(filepath.Join(root, "translations", "catalog.json"), index, 0600); err != nil {
		t.Fatal(err)
	}
	want, err := designattempts.Read(root, entry.ID)
	if err != nil || want.Replay != "stale-transformer" {
		t.Fatal("historical canary is not a valid same-schema receipt", err)
	}
	got := referenceDecode[designattempts.Detail](t, referenceRequest(h, "GET", "/api/conversions/"+entry.ID, nil))
	if !reflect.DeepEqual(got, want) || got.Entry.Verification != "not-verified" {
		t.Fatal("historical exact API invented current replay or compatibility")
	}
	w := referenceRequest(h, "GET", "/conversions/"+entry.ID, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "stale-transformer") || !strings.Contains(w.Body.String(), "not-verified") {
		t.Fatal("historical comparison concealed stale transformer evidence")
	}
}

func TestConversionsIndependentPanesAndNavigationContext(t *testing.T) {
	root, c := conversionFixture(t)
	h := referenceHandler(root)
	entry := conversionEntry(t, c, "translated")
	detail, err := designattempts.Read(root, entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	original, err := designassets.ReadFile(root, entry.Source.ArchiveID, entry.Source.Path)
	if err != nil {
		t.Fatal(err)
	}
	first, err := c.Search(designattempts.Query{ArchiveID: entry.Source.ArchiveID, Limit: 1})
	if err != nil || first.NextCursor == "" {
		t.Fatal("fixture missing source-specific metadata continuation", err)
	}
	values := url.Values{"archive_id": {entry.Source.ArchiveID}, "limit": {"1"}, "cursor": {first.NextCursor}, "theme": {"dark"}, "source_line": {"2"}, "artifact_line": {"3"}, "lines": {"1"}}
	base := "/conversions/" + entry.ID
	w := referenceRequest(h, "GET", base+"?"+values.Encode(), nil)
	if w.Code != 200 {
		t.Fatalf("independent pane request failed: %d", w.Code)
	}
	doc, links := conversionDOM(t, w.Body.String())
	panes := map[string]string{}
	var text func(*html.Node) string
	text = func(n *html.Node) string {
		if n.Type == html.TextNode {
			return n.Data
		}
		var out strings.Builder
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			out.WriteString(text(child))
		}
		return out.String()
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "pre" {
			for _, attr := range n.Attr {
				if attr.Key == "data-conversion-pane" {
					if _, duplicate := panes[attr.Val]; duplicate {
						t.Fatal("duplicate source-comparison pane")
					}
					panes[attr.Val] = text(n)
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	// HTML parsing normalizes CRLF by specification; JSON/download checks above
	// separately require original byte identity without that presentation step.
	sourceLine := strings.SplitAfter(original.Content, "\n")[1]
	artifactLine := strings.SplitAfter(detail.Translation.Artifact.Content, "\n")[2]
	if len(panes) != 2 || panes["source"] != strings.ReplaceAll(sourceLine, "\r\n", "\n") || panes["artifact"] != strings.ReplaceAll(artifactLine, "\r\n", "\n") {
		t.Fatal("source/artifact panes mixed identities, offsets or escaped content")
	}
	back, sourceNext, artifactNext := false, false, false
	for _, link := range links {
		query := link.Query()
		if link.Path == "/conversions" && query.Get("cursor") == first.NextCursor {
			back = query.Get("archive_id") == entry.Source.ArchiveID && query.Get("limit") == "1" && query.Get("theme") == "dark" && query.Get("source_line") == "" && query.Get("artifact_line") == "" && query.Get("lines") == ""
		}
		if link.Path != base || query.Get("lines") != "1" || query.Get("theme") != "dark" || query.Get("archive_id") != entry.Source.ArchiveID || query.Get("cursor") != first.NextCursor {
			continue
		}
		if query.Get("source_line") == "3" && query.Get("artifact_line") == "3" {
			sourceNext = true
			if got := referenceRequest(h, "GET", link.RequestURI(), nil); got.Code != 200 {
				t.Fatal("source next link is not usable")
			}
		}
		if query.Get("source_line") == "2" && query.Get("artifact_line") == "4" {
			artifactNext = true
			if got := referenceRequest(h, "GET", link.RequestURI(), nil); got.Code != 200 {
				t.Fatal("artifact next link is not usable")
			}
		}
	}
	if !back || !sourceNext || !artifactNext {
		t.Fatal("back/search/theme state or independent pane continuation lost")
	}
}
