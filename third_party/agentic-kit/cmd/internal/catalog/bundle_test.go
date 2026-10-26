package catalog

import (
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/catalog-kit"
)

func TestRealSourceBundleCoverage(t *testing.T) {
	entries, err := Collect("../../..")
	if err != nil {
		t.Fatal(err)
	}
	b, err := ResolveBundle(entries, "fixture-revision")
	if err != nil {
		t.Fatal(err)
	}
	packages := map[string]bool{}
	for _, r := range catalogkit.Rows(b) {
		for _, kind := range r.Facets["kind"] {
			if kind == "package" {
				packages[r.ID] = true
			}
		}
	}
	if len(packages) != len(entries) || len(entries) < 100 {
		t.Fatalf("coverage mismatch %d/%d", len(packages), len(entries))
	}
	for _, e := range entries {
		if !packages[e.RelPath] {
			t.Fatalf("missing live source package %s", e.RelPath)
		}
	}
	entries[0], entries[len(entries)-1] = entries[len(entries)-1], entries[0]
	other, err := ResolveBundle(entries, "fixture-revision")
	if err != nil || other.Digest != b.Digest {
		t.Fatalf("source ordering changed bundle: %v", err)
	}
	if _, err := ResolveBundle([]Entry{{RelPath: "same", Name: "one"}, {RelPath: "same", Name: "two"}}, ""); err == nil {
		t.Fatal("ambiguous package path accepted")
	}
}
