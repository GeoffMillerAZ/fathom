package extractor

import "testing"

// TestLoadDialectsKeepsTheCUEFamily: the CUE family's manifests stay in
// internal/extractor (ADR-092, LL-7) although its readers moved to
// internal/extractor/cuelang, so LoadDialects lists the family whether or not
// a binary links that package. A caller that wants only what it can read
// filters by NativeDialectExtractor.
func TestLoadDialectsKeepsTheCUEFamily(t *testing.T) {
	ds, err := LoadDialects()
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, d := range ds {
		have[d.ID] = true
	}
	for _, id := range []string{"cue", "timoni", "fathom-concerns", "fathom-layer-packs"} {
		if !have[id] {
			t.Errorf("LoadDialects lost %q", id)
		}
	}
}
