package semantic

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// symbolSrc is a tiny "governing document" that is ALSO real code — a
// layer whose corpus cites actual symbol declarations, the case SU-21
// exists for. Line numbers (1-based):
//
//	1: package pkg
//	2: (blank)
//	3: // Widget does the thing.
//	4: func Widget() {
//	5: 	doStuff()
//	6: }
const symbolSrc = `package pkg

// Widget does the thing.
func Widget() {
	doStuff()
}
`

func seedWidgetSymbol(t *testing.T, st store.Store, line, endLine int) {
	t.Helper()
	err := st.PutObservations(context.Background(), []core.Observation{
		{
			ServiceID: "svc", SourceRef: "internal/pkg/widget.go", SourceKind: core.KindGoFunc,
			Symbol: "Widget", Package: "pkg", Line: line, EndLine: endLine,
		},
	})
	if err != nil {
		t.Fatalf("seedWidgetSymbol: %v", err)
	}
}

// TestCompile_AnchorsSymbolWhenCiteResolvesToExactlyOneSymbol (SU-21, ADR-041
// decision 7): a claim compiled from a document that IS a real source file
// gets an "anchors" link when its citation resolves to exactly one indexed
// symbol's declared range.
func TestCompile_AnchorsSymbolWhenCiteResolvesToExactlyOneSymbol(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	seedWidgetSymbol(t, st, 4, 6)

	c := newCompiler(t, st, llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[
  {"rule":"Widget must always call doStuff exactly once.","scope":["internal/pkg/**"],"cite_start":4,"cite_end":6}
]}`,
		Usage: llmclient.Usage{InputTokens: 10, OutputTokens: 5},
	}})

	run, err := c.Compile(ctx, CompileInput{
		ServiceID:    "svc",
		Docs:         []core.SourceFile{{Path: "internal/pkg/widget.go", Content: []byte(symbolSrc)}},
		IndexedFiles: []string{"internal/pkg/widget.go", "internal/other/thing.go"},
		Watermark:    "manifest:aaa",
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if run.ClaimsEmitted != 1 {
		t.Fatalf("ClaimsEmitted = %d, want 1", run.ClaimsEmitted)
	}

	claims, err := st.ClaimsForService(ctx, "svc", LayerObligations)
	if err != nil || len(claims) != 1 {
		t.Fatalf("ClaimsForService: %v, %d claims", err, len(claims))
	}
	claimID := claims[0].ClaimID

	links, err := st.ClaimLinks(ctx, "svc", []string{claimID})
	if err != nil {
		t.Fatalf("ClaimLinks: %v", err)
	}
	wantKey := core.SymbolKey("svc", core.QualifiedPackage("internal/pkg/widget.go", "pkg"), "Widget")
	wantHash, ok := core.HashLines([]byte(symbolSrc), 4, 6)
	if !ok {
		t.Fatal("HashLines setup failed")
	}
	var found bool
	for _, l := range links {
		if l.Rel == core.RelAnchors {
			found = true
			if l.TargetRef != wantKey {
				t.Errorf("anchors TargetRef = %q, want %q", l.TargetRef, wantKey)
			}
			if l.TargetHash != wantHash {
				t.Errorf("anchors TargetHash = %q, want %q", l.TargetHash, wantHash)
			}
		}
	}
	if !found {
		t.Fatalf("no anchors link written; links=%+v", links)
	}
}

// TestCompile_NoAnchorWhenCiteIsAmbiguousOrDoesNotResolve pins the "never
// guess" side: a citation over a prose document (no matching symbols at
// all) never gets an anchors link — byte-identical to before SU-21.
func TestCompile_NoAnchorWhenCiteIsAmbiguousOrDoesNotResolve(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	c := newCompiler(t, st, llmclient.MockEntry{Response: llmclient.Response{
		Text: `{"claims":[
  {"rule":"Inject clocks; never call time.Now() in query logic.","scope":["internal/query/**"],"cite_start":3,"cite_end":3}
]}`,
		Usage: llmclient.Usage{InputTokens: 10, OutputTokens: 5},
	}})
	run, err := c.Compile(ctx, compileInput(false))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if run.ClaimsEmitted != 1 {
		t.Fatalf("ClaimsEmitted = %d, want 1", run.ClaimsEmitted)
	}
	claims, _ := st.ClaimsForService(ctx, "svc", LayerObligations)
	links, err := st.ClaimLinks(ctx, "svc", []string{claims[0].ClaimID})
	if err != nil {
		t.Fatalf("ClaimLinks: %v", err)
	}
	for _, l := range links {
		if l.Rel == core.RelAnchors {
			t.Fatalf("prose citation must never get an anchors link, got %+v", l)
		}
	}
}

// seedAnchoredClaim writes a claim citing symbolSrc lines [4,6] (Widget's
// declared range) with an EMPTY AnchorText — deliberately, so the existing
// T5-1/O13 literal-text re-anchor path (findAnchorText) cannot fire and the
// SU-21 symbol-anchor fallback is the ONLY mechanism under test — plus the
// paired "anchors" link AnchorSymbol would have written at mint.
func seedAnchoredClaim(t *testing.T, st store.Store, claimID string) {
	t.Helper()
	claim := core.SemanticClaim{
		ClaimID: claimID, ServiceID: "svc", Layer: LayerObligations, LayerVersion: "v1", Kind: KindObligation,
		Subject: "internal/pkg/widget.go#widget", Rule: "Widget must always call doStuff exactly once.",
		Scope: []string{"internal/pkg/**"}, Tier: core.TierDocClaimed, TierProfile: "engineering",
		Status: core.ClaimFresh, ReproKey: claimID,
	}
	hash, ok := core.HashLines([]byte(symbolSrc), 4, 6)
	if !ok {
		t.Fatal("HashLines setup failed")
	}
	symKey := core.SymbolKey("svc", core.QualifiedPackage("internal/pkg/widget.go", "pkg"), "Widget")
	links := []core.ClaimLink{
		{ClaimID: claimID, Rel: core.RelCites, TargetRef: core.CiteRef("internal/pkg/widget.go", 4, 6), TargetHash: hash},
		{ClaimID: claimID, Rel: core.RelScopes, TargetRef: "internal/pkg/**"},
		{ClaimID: claimID, Rel: core.RelAnchors, TargetRef: symKey, TargetHash: hash},
	}
	if err := st.PutClaims(context.Background(), []core.SemanticClaim{claim}, links); err != nil {
		t.Fatalf("seedAnchoredClaim: %v", err)
	}
}

// TestSweep_ReAnchorsViaSymbolWhenTextSearchCannot (SU-21, ADR-041 decision
// 7): the symbol moves 20 lines down (unrelated prose inserted above it,
// so its OWN body text is unchanged but the AnchorText mechanism has none
// to search with — seedAnchoredClaim leaves it empty) — Sweep still heals
// the drifted citation by resolving the claim's "anchors" link against the
// service's CURRENT symbol index and re-pointing the cites link at the
// symbol's new range, instead of flipping the claim stale.
func TestSweep_ReAnchorsViaSymbolWhenTextSearchCannot(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	seedWidgetSymbol(t, st, 4, 6)
	seedAnchoredClaim(t, st, "c1")

	// Move Widget 20 lines down: 19 blank-ish filler lines inserted above
	// it, content otherwise byte-identical.
	filler := ""
	for i := 0; i < 19; i++ {
		filler += "// filler\n"
	}
	movedSrc := "package pkg\n\n" + filler + "// Widget does the thing.\nfunc Widget() {\n\tdoStuff()\n}\n"
	newLine, newEnd := 23, 25
	if got, ok := core.HashLines([]byte(movedSrc), newLine, newEnd); !ok || got != mustHash(t, []byte(symbolSrc), 4, 6) {
		t.Fatalf("test setup: moved range [%d,%d] does not reproduce the original symbol body", newLine, newEnd)
	}
	// The index reflects the post-move state (as a fresh ingest would).
	seedWidgetSymbol(t, st, newLine, newEnd)

	fetch := func(path string) ([]byte, bool) {
		if path == "internal/pkg/widget.go" {
			return []byte(movedSrc), true
		}
		return nil, false
	}
	rep, err := Sweep(ctx, st, "svc", fetch, nil)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if rep.ReAnchored != 1 {
		t.Fatalf("ReAnchored = %d, want 1: %+v", rep.ReAnchored, rep)
	}
	if rep.Stale != 0 {
		t.Fatalf("Stale = %d, want 0 (healed via the symbol anchor)", rep.Stale)
	}

	claims, err := st.ClaimsForService(ctx, "svc", "")
	if err != nil || len(claims) != 1 || claims[0].Status != core.ClaimFresh {
		t.Fatalf("claim after sweep: %v, claims=%+v", err, claims)
	}

	links, err := st.ClaimLinks(ctx, "svc", []string{"c1"})
	if err != nil {
		t.Fatalf("ClaimLinks: %v", err)
	}
	wantRef := core.CiteRef("internal/pkg/widget.go", newLine, newEnd)
	var gotRef string
	for _, l := range links {
		if l.Rel == core.RelCites {
			gotRef = l.TargetRef
		}
	}
	if gotRef != wantRef {
		t.Errorf("cites TargetRef after sweep = %q, want %q", gotRef, wantRef)
	}

	events, err := st.ClaimEvents(ctx, "c1")
	if err != nil {
		t.Fatalf("ClaimEvents: %v", err)
	}
	var reanchored int
	for _, ev := range events {
		if ev.Verdict == "re-anchored" {
			reanchored++
		}
	}
	if reanchored != 1 {
		t.Errorf("re-anchored events = %d, want 1: %+v", reanchored, events)
	}
}

func mustHash(t *testing.T, content []byte, start, end int) string {
	t.Helper()
	h, ok := core.HashLines(content, start, end)
	if !ok {
		t.Fatalf("HashLines(%d,%d) failed", start, end)
	}
	return h
}

// K18 (wave lf2, ADR-048 amendment 2026-10-10): a Python file's `<module>`
// symbol spans line 1 to its last line, so it encloses every citation in the
// file. It is the file's granularity, never a claim's anchor: a citation over
// a top-level function still resolves to that function alone, and one over
// top-level code alone resolves to nothing, as before the module symbol.
func TestEnclosingSymbol_ModuleSymbolIsNeverAnAnchor(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	err := st.PutObservations(ctx, []core.Observation{
		{ServiceID: "svc", SourceRef: "pkg/widget.py", SourceKind: "PythonModule", Symbol: core.ModuleSymbolName, Package: "pkg/widget", Line: 1, EndLine: 12},
		{ServiceID: "svc", SourceRef: "pkg/widget.py", SourceKind: "PythonFunc", Symbol: "widget", Package: "pkg/widget", Line: 4, EndLine: 6},
	})
	if err != nil {
		t.Fatalf("PutObservations: %v", err)
	}
	idx, err := LoadSymbolIndex(ctx, st, "svc")
	if err != nil {
		t.Fatalf("LoadSymbolIndex: %v", err)
	}
	if got, ok := idx.EnclosingSymbol("pkg/widget.py", 4, 6); !ok || got.Name != "widget" {
		t.Errorf("EnclosingSymbol(4,6) = (%q, %v), want (\"widget\", true): the module symbol made a top-level function's citation ambiguous", got.Name, ok)
	}
	if got, ok := idx.EnclosingSymbol("pkg/widget.py", 9, 10); ok {
		t.Errorf("EnclosingSymbol(9,10) = (%q, true), want no anchor for top-level code", got.Name)
	}
}
