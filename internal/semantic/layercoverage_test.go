package semantic

// layercoverage_test.go — O54 deliverable 2, unit-level: ObligationsLayerCoverage
// and NearestUncompiledGuide pinned directly, ahead of the fuller Compile()
// integration proof in compile_test.go and the MCP-level wiring test.

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

func TestObligationsLayerCoverage_AbsentWhenNeverCompiled(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	cov, found, err := ObligationsLayerCoverage(ctx, st, "svc", LayerObligations)
	if err != nil {
		t.Fatalf("ObligationsLayerCoverage: %v", err)
	}
	if found {
		t.Fatalf("found = true, want false (no compile_runs row at all) — cov=%+v", cov)
	}
}

func TestObligationsLayerCoverage_ReadsLatestRun(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	older := core.CompileRun{
		RunID: "r1", ServiceID: "svc", Layer: LayerObligations,
		RanAt: fixedNow().Add(-1), SectionsTotal: 10, SectionsCompiled: 10,
	}
	newer := core.CompileRun{
		RunID: "r2", ServiceID: "svc", Layer: LayerObligations,
		RanAt: fixedNow(), SectionsTotal: 60, SectionsCompiled: 18,
		BudgetStopped: true, UncompiledScopes: []string{"ADR.md", "docs/plans/README.md"},
	}
	if err := st.RecordCompileRun(ctx, older); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordCompileRun(ctx, newer); err != nil {
		t.Fatal(err)
	}
	cov, found, err := ObligationsLayerCoverage(ctx, st, "svc", LayerObligations)
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if cov.SectionsTotal != 60 || cov.SectionsCompiled != 18 || !cov.BudgetStopped {
		t.Fatalf("cov = %+v, want the NEWER run's numbers", cov)
	}
	if len(cov.UncompiledScopes) != 2 || cov.UncompiledScopes[0] != "ADR.md" {
		t.Fatalf("cov.UncompiledScopes = %v", cov.UncompiledScopes)
	}
}

func TestNearestUncompiledGuide(t *testing.T) {
	cov := LayerCoverage{
		SectionsTotal: 60, SectionsCompiled: 18, BudgetStopped: true,
		UncompiledScopes: []string{"ADR.md", "internal/CLAUDE.md", "internal/extractor/CLAUDE.md"},
	}
	cases := []struct {
		name      string
		file      string
		wantGuide string
		wantOK    bool
	}{
		{"root-scope call has no file", "", "", false},
		{"a file under the deepest uncompiled guide gets the DEEPEST match", "internal/extractor/goextract.go", "internal/extractor/CLAUDE.md", true},
		{"a file only under the shallower guide gets that one", "internal/mcp/tools.go", "internal/CLAUDE.md", true},
		{"a file under no uncompiled guide at all", "cmd/fathom/compile.go", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			guide, ok := NearestUncompiledGuide(cov, c.file)
			if ok != c.wantOK || guide != c.wantGuide {
				t.Errorf("NearestUncompiledGuide(file=%q) = (%q, %v), want (%q, %v)", c.file, guide, ok, c.wantGuide, c.wantOK)
			}
		})
	}
}

// TestNearestUncompiledGuide_RootTailNeverCountsAsAGuide: when the only
// uncompiled scope is a root-OTHER document (ADR.md — never the repo's own
// CLAUDE.md/AGENTS.md), NO file ever gets a coverage note from it — a
// root-other tail is not "the nearest guide" for anything, and there is no
// root GUIDE in the uncompiled set to fall back to either.
func TestNearestUncompiledGuide_RootTailNeverCountsAsAGuide(t *testing.T) {
	cov := LayerCoverage{SectionsTotal: 3, SectionsCompiled: 2, BudgetStopped: true, UncompiledScopes: []string{"ADR.md"}}
	if guide, ok := NearestUncompiledGuide(cov, "internal/extractor/goextract.go"); ok {
		t.Fatalf("NearestUncompiledGuide = (%q, true), want ok=false", guide)
	}
}

// TestNearestUncompiledGuide_RootGuideFallback is the second review
// round's fix: a file with NO nested guide above it at all (most files in
// most repos) still gets a note when the repo ROOT's own CLAUDE.md/
// AGENTS.md is among the uncompiled scopes — the root guide governs every
// file by construction, so its own incompleteness is worth naming even
// with no nearer guide to blame instead. A nested guide, when one governs
// the path, still wins over the root fallback.
func TestNearestUncompiledGuide_RootGuideFallback(t *testing.T) {
	cov := LayerCoverage{
		UncompiledScopes: []string{"ADR.md", "CLAUDE.md", "internal/extractor/CLAUDE.md"},
	}
	cases := []struct {
		name      string
		file      string
		wantGuide string
		wantOK    bool
	}{
		{"a nested guide governs: it wins over the root fallback", "internal/extractor/goextract.go", "internal/extractor/CLAUDE.md", true},
		{"no nested guide governs anywhere: the root guide is the fallback", "pkg/random/thing.go", "CLAUDE.md", true},
		{"root-scope call: still no file to be nearest to", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			guide, ok := NearestUncompiledGuide(cov, c.file)
			if ok != c.wantOK || guide != c.wantGuide {
				t.Errorf("NearestUncompiledGuide(file=%q) = (%q, %v), want (%q, %v)", c.file, guide, ok, c.wantGuide, c.wantOK)
			}
		})
	}
}

// TestNearestUncompiledGuide_NoRootFallbackWhenRootGuideItselfIsCompiled:
// the root guide is NOT in UncompiledScopes (it compiled fine) — only
// ADR.md (root-other) is uncompiled, and no nested guide governs the file
// either. There is nothing left to fall back to.
func TestNearestUncompiledGuide_NoRootFallbackWhenRootGuideItselfIsCompiled(t *testing.T) {
	cov := LayerCoverage{UncompiledScopes: []string{"ADR.md"}}
	if guide, ok := NearestUncompiledGuide(cov, "pkg/random/thing.go"); ok {
		t.Fatalf("NearestUncompiledGuide = (%q, true), want ok=false — the root guide already compiled; ADR.md is never a guide", guide)
	}
}

// ─── O54 second review round: LiveLayerCoverage / corpus_grew ─────────────

func TestLiveLayerCoverage_AbsentWhenNeverCompiled(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	layer, err := LoadLayer(LayerObligations)
	if err != nil {
		t.Fatal(err)
	}
	cov, found, err := LiveLayerCoverage(ctx, st, "svc", layer, nil)
	if err != nil {
		t.Fatalf("LiveLayerCoverage: %v", err)
	}
	if found {
		t.Fatalf("found = true, want false (no compile_runs row at all) — cov=%+v", cov)
	}
}

// TestLiveLayerCoverage_CorpusGrowsAfterAFullCompile is the second review
// round's fix: sections_total/sections_compiled from the LATEST
// compile_runs row go stale the moment the corpus grows past what that
// run counted — a full compile of a 1-section corpus reads back as
// "100% covered" forever, even after a second CLAUDE.md is added and
// never compiled at all. LiveLayerCoverage recomputes from the CURRENT
// corpus and reports the new section under CorpusGrew and
// UncompiledScopes; NearestUncompiledGuide treats it exactly like any
// other uncompiled guide.
func TestLiveLayerCoverage_CorpusGrowsAfterAFullCompile(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()

	original := core.SourceFile{Path: "cmd/fathom/CLAUDE.md", Content: []byte("# guide\n\n- Wire every new verb into verbs.go.\n")}
	indexed := []string{"cmd/fathom/compile.go", "internal/extractor/goextract.go", "docs/x.md"}

	mock := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[{"rule":"Wire every new verb into verbs.go.","scope":["cmd/fathom/**"],"cite_start":3,"cite_end":3}]}`,
	}})
	c := &Compiler{St: st, LLM: mock, Model: "mock-model", Now: fixedNow}
	run, err := c.Compile(ctx, CompileInput{
		ServiceID: "svc", Docs: []core.SourceFile{original}, IndexedFiles: indexed, Watermark: "manifest:aaa",
	})
	if err != nil || run.SectionsCompiled != 1 {
		t.Fatalf("seed compile: run=%+v err=%v", run, err)
	}

	layer, err := LoadLayer(LayerObligations)
	if err != nil {
		t.Fatal(err)
	}

	// The corpus has not grown yet: live coverage matches the recorded run.
	cov, found, err := LiveLayerCoverage(ctx, st, "svc", layer, []core.SourceFile{original})
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if cov.SectionsTotal != 1 || cov.SectionsCompiled != 1 || cov.CorpusGrew != 0 {
		t.Fatalf("cov = %+v, want {SectionsTotal:1 SectionsCompiled:1 CorpusGrew:0 ...}", cov)
	}
	if len(cov.UncompiledScopes) != 0 {
		t.Fatalf("cov.UncompiledScopes = %v, want empty", cov.UncompiledScopes)
	}

	// A new guide is added to the corpus AFTER the compile — a stale,
	// store-only ObligationsLayerCoverage would still report 1/1 (100%
	// covered) here; LiveLayerCoverage must not.
	grown := core.SourceFile{Path: "internal/CLAUDE.md", Content: []byte("# guide\n\n- Register every extractor in fusion.New.\n")}
	cov2, found, err := LiveLayerCoverage(ctx, st, "svc", layer, []core.SourceFile{original, grown})
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if cov2.SectionsTotal != 2 {
		t.Fatalf("cov2.SectionsTotal = %d, want 2 (live, not the stale recorded 1)", cov2.SectionsTotal)
	}
	if cov2.SectionsCompiled != 1 {
		t.Fatalf("cov2.SectionsCompiled = %d, want 1 (only the original doc)", cov2.SectionsCompiled)
	}
	if cov2.CorpusGrew != 1 {
		t.Fatalf("cov2.CorpusGrew = %d, want 1 (the new guide has never been seen by any run)", cov2.CorpusGrew)
	}
	if want := "internal/CLAUDE.md"; len(cov2.UncompiledScopes) != 1 || cov2.UncompiledScopes[0] != want {
		t.Fatalf("cov2.UncompiledScopes = %v, want [%q]", cov2.UncompiledScopes, want)
	}

	// A never-seen guide counts as uncompiled for NearestUncompiledGuide
	// too — corpus growth is not a special case at that layer.
	if guide, ok := NearestUncompiledGuide(cov2, "internal/extractor/goextract.go"); !ok || guide != "internal/CLAUDE.md" {
		t.Fatalf("NearestUncompiledGuide = (%q, %v), want (\"internal/CLAUDE.md\", true)", guide, ok)
	}
	// The ORIGINAL doc's own file is unaffected — its guide is fully
	// covered, so no note fires for it.
	if guide, ok := NearestUncompiledGuide(cov2, "cmd/fathom/compile.go"); ok {
		t.Fatalf("NearestUncompiledGuide(cmd/fathom/compile.go) = (%q, true), want ok=false", guide)
	}
}

// IV-7 / O1135 (I38): a compiled claim whose source file left the corpus
// (deleted at the indexed commit) is named in MissingSources; a live file
// is not.
func TestLiveLayerCoverage_NamesSourceFilesGoneFromTheCorpus(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	gone := core.SourceFile{Path: "capstan/CLAUDE.md", Content: []byte("# guide\n\n- Wire every new verb into verbs.go.\n")}
	kept := core.SourceFile{Path: "AGENTS.md", Content: []byte("# guide\n\n- Keep the gate green.\n")}
	mock := llmclient.NewMockClient(llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[{"rule":"Wire every new verb into verbs.go.","scope":["capstan/**"],"cite_start":3,"cite_end":3}]}`,
	}}, llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[{"rule":"Keep the gate green.","scope":["internal/**"],"cite_start":3,"cite_end":3}]}`,
	}})
	c := &Compiler{St: st, LLM: mock, Model: "mock-model", Now: fixedNow}
	run0, cerr0 := c.Compile(ctx, CompileInput{ServiceID: "svc", Docs: []core.SourceFile{gone, kept}, IndexedFiles: []string{"capstan/a.go", "internal/b.go", "docs/x.md", "docs/y.md", "docs/z.md"}, Watermark: "manifest:aaa"})
	if cerr0 != nil || run0.SectionsCompiled != 2 {
		t.Fatalf("seed compile: %+v %v", run0, cerr0)
	}
	layer, err := LoadLayer(LayerObligations)
	if err != nil {
		t.Fatal(err)
	}
	cov, found, err := LiveLayerCoverage(ctx, st, "svc", layer, []core.SourceFile{kept})
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	cls, cerr1 := st.ClaimsForService(ctx, "svc", LayerObligations)
	_ = cerr1
	if len(cls) != 2 {
		t.Fatalf("seed claims = %d, want 2 (run %+v)", len(cls), run0.RejectionReasons)
	}
	if len(cov.MissingSources) != 1 || cov.MissingSources[0] != "capstan/CLAUDE.md" {
		t.Fatalf("MissingSources = %v, want [capstan/CLAUDE.md]", cov.MissingSources)
	}
}
