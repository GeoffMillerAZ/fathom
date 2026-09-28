package mcp_test

// obligations_specrefs_test.go — SR-27 (surface reckoning F-35, design §4):
// the retired spec_refs tool's one real consumer (the edit hook, 306 calls)
// now reads the same spec_id -> locations map from an additive spec_refs
// section on the obligations answer it already fetches.

import (
	"testing"
)

// TestEditHookReadsSpecRefsFromObligations is SR-27's acceptance test: an
// obligations call scoped to a file that cites a bracket spec ID, a
// milestone ID and a doc reference gets back an additive spec_refs section
// naming each ID's OTHER citing location elsewhere in the service — the
// exact information the edit hook used to fetch with a second spec_refs
// RPC per cited ID (cmd/fathom/edithook.go's fetchSpecRefs) — while the
// citing file itself is excluded, mirroring that RPC's own self-filter.
func TestEditHookReadsSpecRefsFromObligations(t *testing.T) {
	srv, root := snippetServer(t, nil)
	writeCorpusFile(t, root, "docs/spec-register.md", "[US-042] is the login flow.\nM18.2 ships the console.\nsee doc 31 for the schema.\n")
	writeCorpusFile(t, root, "internal/extractor/goextract.go", "// implements [US-042] and M18.2, see also doc 31\npackage extractor\n")

	p := obligationsPayload(t, srv, map[string]any{
		"service": "svc",
		"file":    "internal/extractor/goextract.go",
	})

	refs, ok := p["spec_refs"].(map[string]any)
	if !ok {
		t.Fatalf("spec_refs missing or wrong type: %v", p)
	}

	assertLocatesElsewhere := func(id string) {
		t.Helper()
		hits, ok := refs[id].([]any)
		if !ok || len(hits) == 0 {
			t.Fatalf("spec_refs[%q] missing or empty: %v", id, refs)
		}
		for _, raw := range hits {
			h, ok := raw.(map[string]any)
			if !ok {
				t.Fatalf("spec_refs[%q] hit is not an object: %v", id, raw)
			}
			if h["file"] == "internal/extractor/goextract.go" {
				t.Errorf("spec_refs[%q] must exclude the citing file itself, got %v", id, hits)
			}
		}
	}
	assertLocatesElsewhere("US-042")
	assertLocatesElsewhere("M18.2")
	assertLocatesElsewhere("doc 31")
}

// TestEditHookReadsSpecRefsFromObligations_NoCitations pins the omitted
// case: a file citing no spec ID carries no spec_refs key at all (never an
// empty object) — the same zero-output-on-clean contract obligations'
// other additive sections follow.
func TestEditHookReadsSpecRefsFromObligations_NoCitations(t *testing.T) {
	srv, root := snippetServer(t, nil)
	writeCorpusFile(t, root, "internal/extractor/goextract.go", "package extractor\n\nfunc Extract() {}\n")

	p := obligationsPayload(t, srv, map[string]any{
		"service": "svc",
		"file":    "internal/extractor/goextract.go",
	})
	if _, ok := p["spec_refs"]; ok {
		t.Errorf("spec_refs must be absent for a file citing no spec ID, got %v", p["spec_refs"])
	}
}

// TestEditHookReadsSpecRefsFromObligations_RootScopeOmitted pins the other
// gate obligationsSpecRefs shares with the raw-fallback prepend: a
// root-scope call (no file) never runs the search — there is no citing
// file to extract IDs from.
func TestEditHookReadsSpecRefsFromObligations_RootScopeOmitted(t *testing.T) {
	srv, root := snippetServer(t, nil)
	_ = root

	p := obligationsPayload(t, srv, map[string]any{"service": "svc"})
	if _, ok := p["spec_refs"]; ok {
		t.Errorf("spec_refs must be absent on a root-scope call, got %v", p["spec_refs"])
	}
}
