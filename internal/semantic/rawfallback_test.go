package semantic

// rawfallback_test.go — wave O1 (ADR-037), failing-first. Pins
// RawFallbackRules directly (unit level) ahead of the fuller
// pkg/client/MCP-level proofs in pkg/client/semantic_test.go and
// internal/mcp/rawfallback_tool_test.go.

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// rawFallbackTestLayer loads the real embedded obligations layer — the same
// one production compiles against — so promptForLayer's identity and
// allDigestsMatch behave exactly as they do at serve time.
func rawFallbackTestLayer(t *testing.T) LayerDef {
	t.Helper()
	layer, err := LoadLayer(LayerObligations)
	if err != nil {
		t.Fatalf("LoadLayer(obligations): %v", err)
	}
	return layer
}

// freshClaimFor builds a core.SemanticClaim that allDigestsMatch will accept
// as covering sec — the "already compiled, fresh" case a raw candidate must
// never duplicate.
func freshClaimFor(t *testing.T, layer LayerDef, sec Section) core.SemanticClaim {
	t.Helper()
	_, promptVersion, err := promptForLayer(layer)
	if err != nil {
		t.Fatalf("promptForLayer: %v", err)
	}
	return core.SemanticClaim{
		ClaimID: "cl-" + sec.Subject, ServiceID: "svc",
		Layer: layer.Name, LayerVersion: "v1", Kind: KindObligation,
		Subject: sec.Subject, Rule: "compiled rule text",
		Scope: []string{"**"}, Tier: core.TierDocClaimed, TierProfile: "engineering",
		Status: core.ClaimFresh, InputDigest: sec.Digest, PromptVersion: promptVersion,
	}
}

func TestRawFallbackRules_NestedGuideRankedAheadOfRootOther(t *testing.T) {
	layer := rawFallbackTestLayer(t)

	rootDoc := core.SourceFile{Path: "CLAUDE.md", Content: []byte("# Rules\n\n- Root rule, compiled already.")}
	nestedDoc := core.SourceFile{Path: "internal/CLAUDE.md", Content: []byte("# internal guide\n\n- Register every extractor in fusion.New.")}
	adrDoc := core.SourceFile{Path: "ADR.md", Content: []byte("## ADR-001: Some decision\n\nSome unrelated decision text.")}

	rootSec := SplitMarkdownSections(rootDoc.Path, rootDoc.Content)[0]
	claim := freshClaimFor(t, layer, rootSec) // the ROOT section is already compiled and fresh

	corpus := []core.SourceFile{rootDoc, nestedDoc, adrDoc}
	rows, err := RawFallbackRules([]core.SemanticClaim{claim}, layer, corpus, "internal/extractor/goextract.go", 2, "compiled layer incomplete: budget_stopped; raw nearest guide served")
	if err != nil {
		t.Fatalf("RawFallbackRules: %v", err)
	}
	if len(rows) == 0 {
		t.Fatalf("expected raw rows for the uncompiled nested guide, got none")
	}
	if rows[0].Tier != "raw" {
		t.Errorf("rows[0].Tier = %q, want %q", rows[0].Tier, "raw")
	}
	if rows[0].Layer != LayerObligations {
		t.Errorf("rows[0].Layer = %q, want %q", rows[0].Layer, LayerObligations)
	}
	if rows[0].Src != "internal/CLAUDE.md#L1-L3" {
		t.Errorf("rows[0].Src = %q, want the nested guide's own citation", rows[0].Src)
	}
	if rows[0].Why == "" {
		t.Error("rows[0].Why should be set on a raw row")
	}
	// The ALREADY-COMPILED root section must never re-appear as a raw
	// candidate — the whole point of the digest/prompt-version gate.
	for _, r := range rows {
		if r.Src == "CLAUDE.md#L1-L3" {
			t.Errorf("compiled root section re-appeared as a raw row: %+v", r)
		}
	}
	// ADR.md (root-other, tier 3) must never rank AHEAD of the nested guide
	// (tier 1/2) — nearest first, regardless of source order.
	if len(rows) > 1 {
		for i := 1; i < len(rows); i++ {
			if rows[i].Src == "internal/CLAUDE.md#L1-L3" {
				t.Errorf("nested guide row not ranked first: rows=%+v", rows)
			}
		}
	}
}

func TestRawFallbackRules_NoGapReturnsNil(t *testing.T) {
	layer := rawFallbackTestLayer(t)
	doc := core.SourceFile{Path: "CLAUDE.md", Content: []byte("# Rules\n\n- Only rule, already compiled.")}
	sec := SplitMarkdownSections(doc.Path, doc.Content)[0]
	claim := freshClaimFor(t, layer, sec)

	rows, err := RawFallbackRules([]core.SemanticClaim{claim}, layer, []core.SourceFile{doc}, "pkg/foo.go", 2, "why")
	if err != nil {
		t.Fatalf("RawFallbackRules: %v", err)
	}
	if rows != nil {
		t.Errorf("expected nil rows for a fully-compiled corpus, got %+v", rows)
	}
}

func TestRawFallbackRules_RespectsMax(t *testing.T) {
	layer := rawFallbackTestLayer(t)
	docA := core.SourceFile{Path: "internal/CLAUDE.md", Content: []byte("# internal\n\n- Rule A.")}
	docB := core.SourceFile{Path: "internal/store/CLAUDE.md", Content: []byte("# store\n\n- Rule B, deeper and nearer to the file.")}
	corpus := []core.SourceFile{docA, docB}

	rows, err := RawFallbackRules(nil, layer, corpus, "internal/store/backend.go", 1, "why")
	if err != nil {
		t.Fatalf("RawFallbackRules: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1 (max respected)", len(rows))
	}
	if rows[0].Src != "internal/store/CLAUDE.md#L1-L3" {
		t.Errorf("rows[0].Src = %q, want the DEEPER (nearer) guide to win the single slot", rows[0].Src)
	}
}

func TestRawFallbackRules_GuardClauses(t *testing.T) {
	layer := rawFallbackTestLayer(t)
	doc := core.SourceFile{Path: "CLAUDE.md", Content: []byte("# Rules\n\n- A rule.")}

	cases := []struct {
		name string
		file string
		max  int
		docs []core.SourceFile
	}{
		{"empty file (root scope)", "", 2, []core.SourceFile{doc}},
		{"zero max", "pkg/foo.go", 0, []core.SourceFile{doc}},
		{"negative max", "pkg/foo.go", -1, []core.SourceFile{doc}},
		{"empty corpus", "pkg/foo.go", 2, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rows, err := RawFallbackRules(nil, layer, c.docs, c.file, c.max, "why")
			if err != nil {
				t.Fatalf("RawFallbackRules: %v", err)
			}
			if rows != nil {
				t.Errorf("expected nil rows, got %+v", rows)
			}
		})
	}
}

func TestRawFallbackRules_UnrelatedGuideNeverMatches(t *testing.T) {
	// A nested guide that governs a DIFFERENT subtree entirely must never
	// show up as a raw candidate for file — impliedGuideScope/
	// rankByGoverningScope must classify it as no-match (tier < 0), not
	// root-tier-by-accident.
	layer := rawFallbackTestLayer(t)
	doc := core.SourceFile{Path: "cmd/fathom/CLAUDE.md", Content: []byte("# cmd/fathom\n\n- UI doctrine.")}
	rows, err := RawFallbackRules(nil, layer, []core.SourceFile{doc}, "internal/store/backend.go", 2, "why")
	if err != nil {
		t.Fatalf("RawFallbackRules: %v", err)
	}
	if rows != nil {
		t.Errorf("expected nil rows — cmd/fathom/CLAUDE.md does not govern internal/store, got %+v", rows)
	}
}
