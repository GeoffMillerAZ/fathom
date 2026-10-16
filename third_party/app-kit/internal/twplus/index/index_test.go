package index_test

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/twplus/index"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/twplus/normalize"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/twplus/parse"
)

func ptr(s string) *string { return &s }

// buildIndex assembles a tiny corpus and returns an open read-only handle.
func buildIndex(t *testing.T) *index.Index {
	t.Helper()
	path := filepath.Join(t.TempDir(), "components.sqlite")
	b, err := index.NewBuilder(path)
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	comps := []parse.Component{
		{
			Path: []string{"Marketing", "Page Sections", "Pricing Sections", "Three tiers"},
			Raw: parse.RawComponent{Name: "Three tiers", Snippets: []parse.RawSnippet{
				{Code: `<div class="grid grid-cols-3 pricing tiers">React pricing</div>`, Language: "jsx", Name: "react", Mode: ptr("light"), Version: 4, SupportsDarkMode: true},
				{Code: `<div class="grid grid-cols-3 pricing tiers">HTML pricing</div>`, Language: "html", Name: "html", Mode: ptr("light"), Version: 4, SupportsDarkMode: true},
			}},
		},
		{
			Path: []string{"Application UI", "Elements", "Badges", "Flat"},
			Raw: parse.RawComponent{Name: "Flat", Snippets: []parse.RawSnippet{
				// Two modes share identical code → one deduped blob.
				{Code: `<span class="badge rounded-md">B</span>`, Language: "html", Name: "html", Mode: ptr("light"), Version: 4, SupportsDarkMode: false},
				{Code: `<span class="badge rounded-md">B</span>`, Language: "html", Name: "html", Mode: ptr("dark"), Version: 4, SupportsDarkMode: false},
			}},
		},
	}
	for _, c := range comps {
		comp, rows, err := normalize.Build(c)
		if err != nil {
			t.Fatalf("normalize.Build %s: %v", c.FullName(), err)
		}
		if err := b.AddComponent(comp, rows); err != nil {
			t.Fatalf("AddComponent %s: %v", c.FullName(), err)
		}
	}
	stats, err := b.Finalize()
	if err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	if stats.Variants != 4 {
		t.Fatalf("variants = %d, want 4", stats.Variants)
	}
	if stats.Blobs != 3 { // 2 pricing + 1 deduped badge
		t.Fatalf("blobs = %d, want 3 (badge light/dark dedupe to one)", stats.Blobs)
	}
	ix, err := index.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { ix.Close() })
	return ix
}

func TestSearchRanksKnownItem(t *testing.T) {
	ix := buildIndex(t)
	res, err := ix.SearchComponents("pricing tiers", index.Filters{}, 10)
	if err != nil {
		t.Fatalf("SearchComponents: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("no results for 'pricing tiers'")
	}
	if res[0].FullName != "Marketing.Page Sections.Pricing Sections.Three tiers" {
		t.Errorf("top result = %q, want the pricing component", res[0].FullName)
	}
}

func TestSearchFilters(t *testing.T) {
	ix := buildIndex(t)
	res, err := ix.SearchComponents("pricing", index.Filters{Framework: "react"}, 10)
	if err != nil {
		t.Fatalf("SearchComponents: %v", err)
	}
	for _, r := range res {
		if r.Framework != "react" {
			t.Errorf("framework filter leaked: got %q", r.Framework)
		}
	}
}

func TestSearchInjectionSafe(t *testing.T) {
	ix := buildIndex(t)
	// '-' and quotes are FTS5 operators; the sanitizer must treat them as text.
	if _, err := ix.SearchComponents(`rounded-md "OR" badge*`, index.Filters{}, 10); err != nil {
		t.Fatalf("query with FTS operators should not error: %v", err)
	}
}

func TestGetComponent(t *testing.T) {
	ix := buildIndex(t)
	r, err := ix.GetComponent("Marketing.Page Sections.Pricing Sections.Three tiers", "react", 4, "light")
	if err != nil {
		t.Fatalf("GetComponent: %v", err)
	}
	if r.Framework != "react" || r.TailwindVersion != 4 || r.Mode != "light" {
		t.Errorf("got axes %s/%d/%s", r.Framework, r.TailwindVersion, r.Mode)
	}
	if r.Code != `<div class="grid grid-cols-3 pricing tiers">React pricing</div>` {
		t.Errorf("decompressed code mismatch: %q", r.Code)
	}
	if len(r.Available) != 2 {
		t.Errorf("available variants = %d, want 2", len(r.Available))
	}
}

func TestGetComponentNotFound(t *testing.T) {
	ix := buildIndex(t)
	if _, err := ix.GetComponent("Nope.Nope.Nope.Nope", "", 0, ""); err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
}

func TestVerifyClean(t *testing.T) {
	ix := buildIndex(t)
	rep, err := ix.Verify()
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !rep.OK() {
		t.Fatalf("clean index reported problems: %+v", rep.Problems)
	}
	if rep.Components != 2 || rep.Variants != 4 || rep.Blobs != 3 {
		t.Errorf("report counts = %d/%d/%d, want 2/4/3", rep.Components, rep.Variants, rep.Blobs)
	}
	if rep.BlobsChecked != 3 {
		t.Errorf("blobs checked = %d, want 3", rep.BlobsChecked)
	}
	if rep.FTSRows != rep.Variants {
		t.Errorf("fts rows = %d, want %d (one per variant)", rep.FTSRows, rep.Variants)
	}
}

func TestVariantHashesCoverEveryVariant(t *testing.T) {
	ix := buildIndex(t)
	hashes, err := ix.VariantHashes()
	if err != nil {
		t.Fatalf("VariantHashes: %v", err)
	}
	if len(hashes) != 4 {
		t.Fatalf("variant hashes = %d, want 4", len(hashes))
	}
	// The hash recorded for a known variant must match the get_component code's
	// sha256 — the fidelity invariant the verify command relies on.
	k := index.VariantKey{FullName: "Marketing.Page Sections.Pricing Sections.Three tiers", Framework: "react", TailwindVersion: 4, Mode: "light"}
	got, ok := hashes[k]
	if !ok {
		t.Fatalf("no hash for %+v", k)
	}
	r, err := ix.GetComponent(k.FullName, k.Framework, k.TailwindVersion, k.Mode)
	if err != nil {
		t.Fatalf("GetComponent: %v", err)
	}
	sum := sha256.Sum256([]byte(r.Code))
	if want := hex.EncodeToString(sum[:]); got != want {
		t.Errorf("recorded hash %s != sha256(code) %s", got, want)
	}
}

func TestSectionsAndCounts(t *testing.T) {
	ix := buildIndex(t)
	c, v, b, err := ix.Counts()
	if err != nil {
		t.Fatalf("Counts: %v", err)
	}
	if c != 2 || v != 4 || b != 3 {
		t.Errorf("counts = %d/%d/%d, want 2/4/3", c, v, b)
	}
	secs, err := ix.Sections()
	if err != nil {
		t.Fatalf("Sections: %v", err)
	}
	if len(secs) != 2 {
		t.Errorf("sections = %d, want 2", len(secs))
	}
}
