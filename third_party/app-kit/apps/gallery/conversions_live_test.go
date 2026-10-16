package gallery

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designattempts"
	"golang.org/x/net/html"
)

// This opt-in probe reads the owner's existing private store. Absence is an
// explicit skip, not default proof that purchased inputs exist or are compatible.
// Failure messages intentionally omit source bodies, member paths and raw errors.
func TestSavedConversionsGalleryLive(t *testing.T) {
	root := os.Getenv("APPKIT_PROBE_SAVED_CONVERSIONS_GALLERY_ROOT")
	if root == "" {
		t.Skip("set APPKIT_PROBE_SAVED_CONVERSIONS_GALLERY_ROOT for an opt-in read-only private-store probe")
	}
	if !filepath.IsAbs(root) {
		t.Fatal("the private-store probe requires an absolute configured root")
	}
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	catalog, err := designattempts.Load(root)
	if err != nil {
		t.Fatal("could not load the configured private attempt catalog")
	}
	if len(catalog.List()) == 0 || len(catalog.List()) > designattempts.MaxEntries {
		t.Fatal("the opted-in catalog must contain 1–4096 saved attempts")
	}
	handler := NewWithOptions(Options{DesignAssetsRoot: root})
	request := func(target string, status int) *httptest.ResponseRecorder {
		t.Helper()
		if ctx.Err() != nil {
			t.Fatal("private-store probe exceeded its context budget")
		}
		r := httptest.NewRequest(http.MethodGet, target, nil).WithContext(ctx)
		r.Host, r.RemoteAddr = "127.0.0.1:8765", "127.0.0.1:12345"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("private route returned status %d; expected %d (source suppressed)", w.Code, status)
		}
		if ctx.Err() != nil {
			t.Fatal("private-store probe exceeded its context budget")
		}
		if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("Content-Security-Policy") != "default-src 'none'; style-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'" {
			t.Fatal("private route lost its privacy or inert-content headers")
		}
		return w
	}
	query, seen := designattempts.Query{Limit: 50}, map[string]bool{}
	for n := 0; n < (designattempts.MaxEntries+49)/50; n++ {
		values := url.Values{"limit": {"50"}}
		if query.Cursor != "" {
			values.Set("cursor", query.Cursor)
		}
		w := request("/api/conversions?"+values.Encode(), http.StatusOK)
		var got designattempts.Page
		if json.Unmarshal(w.Body.Bytes(), &got) != nil {
			t.Fatal("conversion list did not return valid metadata JSON")
		}
		want, err := catalog.Search(query)
		if err != nil || !reflect.DeepEqual(got, want) || len(got.Entries) == 0 || len(got.Entries) > 50 {
			t.Fatal("conversion metadata page differs from the shared library")
		}
		for _, entry := range got.Entries {
			if seen[entry.ID] {
				t.Fatal("conversion list returned a duplicate attempt ID")
			}
			seen[entry.ID] = true
		}
		query.Cursor = got.NextCursor
		if query.Cursor == "" {
			break
		}
	}
	if query.Cursor != "" || len(seen) != len(catalog.List()) {
		t.Fatal("bounded discovery did not return every saved attempt")
	}
	liveConversionDOM(t, request("/conversions", http.StatusOK).Body.String())
	archives, statuses, replays := map[string]bool{}, map[string]int{}, map[string]int{}
	downloads, artifactBytes := 0, 0
	for _, entry := range catalog.List() {
		want, err := designattempts.Read(root, entry.ID)
		if err != nil {
			t.Fatalf("library exact read failed for attempt %s (source suppressed)", entry.ID)
		}
		var got designattempts.Detail
		if json.Unmarshal(request("/api/conversions/"+entry.ID, http.StatusOK).Body.Bytes(), &got) != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("exact API differs from canonical library detail for %s", entry.ID)
		}
		if got.Entry.Verification != "not-verified" || got.Replay != "matches-current" && got.Replay != "stale-transformer" {
			t.Fatal("exact API invented replay or compatibility evidence")
		}
		page := request("/conversions/"+entry.ID, http.StatusOK)
		liveConversionDOM(t, page.Body.String())
		if !strings.Contains(page.Body.String(), "not-verified") || !strings.Contains(page.Body.String(), got.Replay) {
			t.Fatalf("HTML omitted syntax/replay status for %s", entry.ID)
		}
		originalQuery := url.Values{"path": {entry.Source.Path}}
		var original designassets.TextFile
		if json.Unmarshal(request("/api/references/"+entry.Source.ArchiveID+"/source?"+originalQuery.Encode(), http.StatusOK).Body.Bytes(), &original) != nil || original.ArchiveID != entry.Source.ArchiveID || original.Path != entry.Source.Path || original.SHA256 != entry.Source.SHA256 || len(original.Content) != entry.Source.Size || fmt.Sprintf("%x", sha256.Sum256([]byte(original.Content))) != entry.Source.SHA256 {
			t.Fatal("exact original reference is missing or differs from attempt provenance")
		}
		originalQuery = url.Values{"file": {entry.Source.Path}}
		request("/references/"+entry.Source.ArchiveID+"?"+originalQuery.Encode(), http.StatusOK)
		if got.Translation.Artifact == nil {
			request("/conversions/"+entry.ID+"/artifact", http.StatusConflict)
		} else {
			artifact := got.Translation.Artifact
			w := request("/conversions/"+entry.ID+"/artifact", http.StatusOK)
			media, _, mediaErr := mime.ParseMediaType(w.Header().Get("Content-Type"))
			disposition, params, dispositionErr := mime.ParseMediaType(w.Header().Get("Content-Disposition"))
			if mediaErr != nil || media != "text/plain" || dispositionErr != nil || disposition != "attachment" || params["filename"] != artifact.Filename || w.Body.String() != artifact.Content || fmt.Sprintf("%x", sha256.Sum256(w.Body.Bytes())) != artifact.SHA256 {
				t.Fatalf("artifact download changed source or is not inert for %s", entry.ID)
			}
			downloads++
			artifactBytes += w.Body.Len()
		}
		archives[entry.Source.ArchiveID] = true
		statuses[entry.Status]++
		replays[got.Replay]++
	}
	t.Logf("read-only Gallery probe: revision=%s attempts=%d archives=%d translated=%d unsupported=%d current=%d stale=%d downloads=%d artifact_bytes=%d elapsed=%s; no runtime/visual compatibility claim",
		catalog.Revision, len(seen), len(archives), statuses["translated"], statuses["unsupported"], replays["matches-current"], replays["stale-transformer"], downloads, artifactBytes, time.Since(started).Round(time.Millisecond))
}

func liveConversionDOM(t *testing.T, body string) {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal("conversion HTML could not be parsed (source suppressed)")
	}
	mains, ids := 0, map[string]bool{}
	var walk func(*html.Node, bool)
	walk = func(n *html.Node, inMain bool) {
		if n.Type == html.ElementNode {
			if n.Data == "script" || n.Data == "iframe" || n.Data == "object" || n.Data == "embed" || strings.Contains(n.Data, "-") {
				t.Fatal("conversion source escaped its inert text pane")
			}
			if n.Data == "main" {
				if inMain {
					t.Fatal("conversion page contains nested main landmarks")
				}
				mains++
				inMain = true
			}
			for _, attr := range n.Attr {
				if strings.HasPrefix(strings.ToLower(attr.Key), "on") {
					t.Fatal("conversion source became an inline event handler")
				}
				if attr.Key == "id" {
					if ids[attr.Val] {
						t.Fatal("conversion page contains duplicate IDs")
					}
					ids[attr.Val] = true
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child, inMain)
		}
	}
	walk(doc, false)
	if mains != 1 {
		t.Fatal("conversion page must contain one main landmark")
	}
}
