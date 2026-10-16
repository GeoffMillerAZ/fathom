package gallery

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designassets"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designevidence"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designreceipts"
)

func evidenceFixture(t *testing.T) (string, *designevidence.Catalog) {
	t.Helper()
	root, originals := referenceFixture(t)
	source, err := designassets.ReadFile(root, originals.List()[0].ID, "README.md")
	if err != nil {
		t.Fatal(err)
	}
	asset := func(id string, body []byte) designreceipts.Asset {
		return designreceipts.Asset{ID: id, SHA256: designreceipts.Digest(body), Bytes: len(body)}
	}
	content := map[string][]byte{"source": []byte(source.Content), "output": []byte("PRIVATE_OUTPUT_BODY_NOT_FOR_METADATA"), "support": []byte("synthetic tooling"), "observation": []byte("an assertion, not authenticated proof")}
	for _, allPassed := range []bool{false, true} {
		c := designreceipts.Contract{SchemaVersion: 1, Mode: "retrospective", Subject: designreceipts.Subject{ID: "reference.synthetic", Profile: "static-web", Variant: "links", SourceArchive: source.ArchiveID, SourceRevision: source.SHA256, ArtifactSHA256: asset("output", content["output"]).SHA256}, Inputs: designreceipts.Inputs{Source: []designreceipts.Asset{asset("source", content["source"])}, Transformer: []designreceipts.Asset{asset("support", content["support"])}, Config: []designreceipts.Asset{}, Patch: []designreceipts.Asset{}, Runtime: []designreceipts.Asset{}, Output: []designreceipts.Asset{asset("output", content["output"])}, Fixture: []designreceipts.Asset{asset("support", content["support"])}, Toolchain: []designreceipts.Asset{asset("support", content["support"])}, Baseline: []designreceipts.Asset{asset("support", content["support"])}}, Gates: map[string]designreceipts.Gate{}}
		if allPassed {
			c.Subject.ID = "reference.synthetic-claims"
		}
		r := designreceipts.Receipt{SchemaVersion: 1, SourceRevision: source.SHA256, ArtifactSHA256: c.Subject.ArtifactSHA256, Results: map[string]designreceipts.Result{}}
		for _, name := range []string{"reproducibility", "structure", "visual", "interaction", "lifecycle", "security", "data", "performance", "integration"} {
			c.Gates[name] = designreceipts.Gate{Applicable: true, Scope: "Synthetic <img src=x onerror=fixture()> gate", Slots: []designreceipts.Slot{{ID: "observation", Role: "report"}}}
			status, reason := "not-run", "Complete gate remains unproven."
			if allPassed {
				status, reason = "passed", "Recorded assertion only."
			}
			r.Results[name] = designreceipts.Result{Status: status, Reason: reason, Evidence: []designreceipts.Evidence{{Asset: asset("observation", content["observation"]), Role: "report"}}}
		}
		cb, err := designreceipts.EncodeContract(c)
		if err != nil {
			t.Fatal(err)
		}
		r.ContractSHA256 = designreceipts.Digest(cb)
		rb, err := designreceipts.EncodeReceipt(r)
		if err != nil {
			t.Fatal(err)
		}
		_, err = designevidence.Publish(root, source.Path, cb, rb, designreceipts.Expected{ContractSHA256: r.ContractSHA256, SourceRevision: source.SHA256, ArtifactSHA256: c.Subject.ArtifactSHA256}, content)
		if err != nil {
			t.Fatal(err)
		}
	}
	c, err := designevidence.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, c
}

func TestTargetEvidenceRoutesShareMetadataAndKeepClaimsSeparate(t *testing.T) {
	root, c := evidenceFixture(t)
	h := referenceHandler(root)
	before := conversionStoreSnapshot(t, root)
	q := designevidence.Query{Limit: 1}
	for pageNo := 0; pageNo < 2; pageNo++ {
		values := url.Values{"limit": {"1"}, "cursor": {q.Cursor}, "theme": {"dark"}}
		got := referenceDecode[designevidence.Page](t, referenceRequest(h, "GET", referenceLink("/api/target-evidence", values), nil))
		want, err := c.Search(q)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("page parity: %v", err)
		}
		w := referenceRequest(h, "GET", referenceLink("/target-evidence", values), nil)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "Recorded metadata only") {
			t.Fatalf("HTML listing: %d %s", w.Code, w.Body.String())
		}
		q.Cursor = got.NextCursor
	}
	for _, entry := range c.List() {
		path := "/api/target-evidence/" + entry.ID
		got := referenceDecode[designevidence.Detail](t, referenceRequest(h, "GET", path, nil))
		want, err := designevidence.Read(root, entry.ID)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("exact parity: %v", err)
		}
		body, _ := json.Marshal(got)
		if strings.Contains(string(body), "PRIVATE_OUTPUT_BODY_NOT_FOR_METADATA") || strings.Contains(string(body), "window.fixtureSentinel") {
			t.Fatal("metadata exposed bundled bytes")
		}
		if got.Rechecked.Verification != "not-verified" {
			t.Fatal("claim certified a target")
		}
		w := referenceRequest(h, "GET", "/target-evidence/"+entry.ID+"?theme=dark&limit=1", nil)
		for _, need := range []string{"Target compatibility not verified", "Nine recorded gates", "Recorded:", "&lt;img", "Original", "theme=dark"} {
			if !strings.Contains(w.Body.String(), need) {
				t.Fatalf("HTML missing %q", need)
			}
		}
		if w.Code != 200 || strings.Contains(w.Body.String(), "<img src=x") {
			t.Fatal("unescaped gate text")
		}
	}
	if !reflect.DeepEqual(before, conversionStoreSnapshot(t, root)) {
		t.Fatal("browsing changed the store")
	}
}

func TestTargetEvidencePrivateBoundaryAndMalformedQueries(t *testing.T) {
	root, c := evidenceFixture(t)
	h := referenceHandler(root)
	for _, route := range []string{"/target-evidence", "/api/target-evidence", "/target-evidence/" + c.List()[0].ID, "/api/target-evidence/" + c.List()[0].ID} {
		for _, method := range []string{"POST", "HEAD", "PUT", "DELETE"} {
			if w := referenceRequest(h, method, route, nil); w.Code != 405 {
				t.Fatalf("%s %s: %d", method, route, w.Code)
			}
		}
		for _, headers := range []map[string]string{{"Origin": "https://example.invalid"}, {"Sec-Fetch-Site": "cross-site"}, {"Host": "example.invalid"}} {
			if w := referenceRequest(h, "GET", route, headers); w.Code != 403 {
				t.Fatalf("boundary %s: %d", route, w.Code)
			}
		}
	}
	for _, query := range []string{"limit=0", "limit=51", "limit=1&limit=2", "unknown=value", "theme=purple", "profile=wasm", "subject_id=%00", "cursor=stale", "archive_id=not-an-archive"} {
		if w := referenceRequest(h, "GET", "/api/target-evidence?"+query, nil); w.Code != 400 {
			t.Fatalf("query %s: %d", query, w.Code)
		}
	}
	if w := referenceRequest(h, "GET", "/api/target-evidence/"+c.List()[0].ID+"?subject_id=x", nil); w.Code != 400 {
		t.Fatal("exact accepted list filters")
	}
	if w := referenceRequest(h, "GET", "/api/target-evidence/evidence%40"+strings.TrimPrefix(c.List()[0].ID, "evidence@"), nil); w.Code != 400 {
		t.Fatalf("encoded alias: %d", w.Code)
	}
}

func TestTargetEvidenceEmptyDisabledAndCorruptRecovery(t *testing.T) {
	for _, api := range []string{"", "/api"} {
		want := http.StatusOK // Existing private-library HTML setup page, not evidence.
		if api != "" {
			want = http.StatusServiceUnavailable
		}
		if w := referenceRequest(referenceHandler(""), "GET", api+"/target-evidence", nil); w.Code != want || !strings.Contains(w.Body.String(), "private library") {
			t.Fatalf("disabled: %d", w.Code)
		}
	}
	root, _ := referenceFixture(t)
	if got := referenceDecode[designevidence.Page](t, referenceRequest(referenceHandler(root), "GET", "/api/target-evidence", nil)); got.Total != 0 {
		t.Fatal("empty evidence store has records")
	}
	root, c := evidenceFixture(t)
	entry := c.List()[0]
	if err := os.Remove(filepath.Join(root, "evidence", "entries", strings.TrimPrefix(entry.ID, "evidence@")+".json")); err != nil {
		t.Fatal(err)
	}
	w := referenceRequest(referenceHandler(root), "GET", "/target-evidence/"+entry.ID, nil)
	if w.Code != 500 || !strings.Contains(w.Body.String(), "Open original reference") {
		t.Fatalf("corrupt recovery: %d %s", w.Code, w.Body.String())
	}
	original, err := url.Parse(evidenceOriginalURL(entry, nil))
	if err != nil {
		t.Fatal(err)
	}
	original.Fragment = "" // Browsers do not send URL fragments in HTTP requests.
	if w := referenceRequest(referenceHandler(root), "GET", original.String(), nil); w.Code != http.StatusOK {
		t.Fatalf("original unavailable after evidence failure: %d", w.Code)
	}
	if w := referenceRequest(New(), "GET", "/api/catalog", nil); w.Code != http.StatusOK {
		t.Fatalf("first-party catalog unavailable: %d", w.Code)
	}
}
