package review_test

import (
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/archconformance"
	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// TestClaimsVocabularyMatchesSemanticLayer cross-checks review's mirrored
// findings-layer constants against internal/semantic's own — review cannot
// import internal/semantic in its main code (leaf-package rule), so this
// external test package (which CAN import both without creating a cycle)
// is the drift guard.
func TestClaimsVocabularyMatchesSemanticLayer(t *testing.T) {
	if review.ClaimsKind != semantic.KindFinding {
		t.Errorf("review.ClaimsKind = %q, want semantic.KindFinding %q", review.ClaimsKind, semantic.KindFinding)
	}
}

// ─── fixtures ─────────────────────────────────────────────────────────────

// fixtureNodes: pkg.A (changed) is called by pkg.Caller (unchanged); pkg.B
// is untouched and unrelated. TestA covers pkg.A.
func fixtureNodes() []core.SymbolNode {
	return []core.SymbolNode{
		{Service: "svc", Package: "pkg", Name: "A", Kind: core.KindGoFunc, FilePath: "pkg/a.go", Line: 10},
		{Service: "svc", Package: "pkg", Name: "Caller", Kind: core.KindGoFunc, FilePath: "pkg/caller.go", Line: 5, Calls: []string{"svc::pkg.A"}},
		{Service: "svc", Package: "pkg", Name: "TestA", Kind: core.KindGoFunc, FilePath: "pkg/a_test.go", Line: 1, Calls: []string{"svc::pkg.A"}},
		{Service: "svc", Package: "pkg", Name: "B", Kind: core.KindGoFunc, FilePath: "pkg/b.go", Line: 1},
	}
}

// ─── impact dimension ───────────────────────────────────────────────────────

func TestRun_Impact_ReportsDownstreamCallersNotChangedFilesThemselves(t *testing.T) {
	v := review.Run(review.Input{
		Service:      "svc",
		ChangedFiles: []string{"pkg/a.go"},
		Nodes:        fixtureNodes(),
	})
	d := dimension(t, v, review.DimImpact)
	if !d.Checked {
		t.Fatalf("impact should be checked, reason=%q", d.Reason)
	}
	if len(d.Tests) == 0 {
		t.Error("expected at least one affected test")
	}
	for _, f := range d.Findings {
		if f.File == "pkg/a.go" {
			t.Errorf("finding for the changed file itself should be excluded, got %+v", f)
		}
	}
	if len(d.Findings) == 0 {
		t.Error("expected downstream-caller findings (pkg.Caller, TestA)")
	}
}

func TestRun_Impact_UncheckedWhenNoSymbolsIndexed(t *testing.T) {
	v := review.Run(review.Input{Service: "svc", ChangedFiles: []string{"pkg/a.go"}})
	d := dimension(t, v, review.DimImpact)
	if d.Checked {
		t.Fatal("impact should be unchecked when the service has no indexed symbols")
	}
	if d.Reason == "" {
		t.Error("expected a non-empty reason")
	}
}

// ─── obligations dimension ─────────────────────────────────────────────────

func TestRun_Obligations_ScopedToChangedFiles(t *testing.T) {
	v := review.Run(review.Input{
		Service:              "svc",
		ChangedFiles:         []string{"pkg/a.go", "pkg/b.go"},
		ObligationsSupported: true,
		ObligationsByFile: map[string][]review.ObligationRule{
			"pkg/a.go": {{File: "pkg/a.go", Rule: "Sort outputs.", Src: "CLAUDE.md#L1-L1", Tier: "doc-claimed"}},
		},
	})
	d := dimension(t, v, review.DimObligations)
	if !d.Checked {
		t.Fatalf("obligations should be checked, reason=%q", d.Reason)
	}
	if len(d.Findings) != 1 {
		t.Fatalf("expected 1 obligation finding (only pkg/a.go has one), got %d: %+v", len(d.Findings), d.Findings)
	}
	if d.Findings[0].Message != "Sort outputs." {
		t.Errorf("Message = %q", d.Findings[0].Message)
	}
}

func TestRun_Obligations_UncheckedWhenBackendUnsupported(t *testing.T) {
	v := review.Run(review.Input{Service: "svc", ChangedFiles: []string{"pkg/a.go"}})
	d := dimension(t, v, review.DimObligations)
	if d.Checked {
		t.Fatal("obligations should be unchecked when the backend does not support semantic layers")
	}
	if d.Reason == "" {
		t.Error("expected a non-empty reason")
	}
}

// ─── arch_conformance dimension ─────────────────────────────────────────────

func TestRun_Arch_HonestyPassthrough(t *testing.T) {
	v := review.Run(review.Input{
		Service:      "svc",
		ChangedFiles: []string{"pkg/a.go"},
		ArchChecked:  false,
		ArchReason:   "no rules configured: no inline rules and no project rule pack",
	})
	d := dimension(t, v, review.DimArch)
	if d.Checked {
		t.Fatal("arch_conformance should be unchecked when ArchChecked is false")
	}
	if d.Reason == "" {
		t.Error("expected a non-empty reason")
	}
	if len(d.Findings) != 0 {
		t.Errorf("unchecked dimension must report zero findings, got %d", len(d.Findings))
	}
}

func TestRun_Arch_ReportsViolationsWithCitation(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "api", Name: "Handler", Kind: core.KindGoFunc, FilePath: "api/handler.go", Line: 7, Calls: []string{"svc::store.Query"}},
		{Service: "svc", Package: "store", Name: "Query", Kind: core.KindGoFunc, FilePath: "store/query.go", Line: 3},
	}
	v := review.Run(review.Input{
		Service:      "svc",
		ChangedFiles: []string{"api/handler.go"},
		Nodes:        nodes,
		ArchChecked:  true,
		ArchRules: []archconformance.Rule{
			{FromPkg: "api", ToPkg: "store", Allow: false},
		},
	})
	d := dimension(t, v, review.DimArch)
	if !d.Checked {
		t.Fatalf("expected checked=true, reason=%q", d.Reason)
	}
	if len(d.Findings) != 1 {
		t.Fatalf("expected 1 violation, got %d: %+v", len(d.Findings), d.Findings)
	}
	f := d.Findings[0]
	if f.File != "api/handler.go" {
		t.Errorf("File = %q, want api/handler.go", f.File)
	}
	if f.Citation == "" {
		t.Error("expected a non-empty citation")
	}
	if f.Severity != "error" {
		t.Errorf("Severity = %q, want error", f.Severity)
	}
}

// ─── spec_coverage dimension ────────────────────────────────────────────────

func TestRun_SpecCoverage_FlagsUncoveredRefInDiff(t *testing.T) {
	v := review.Run(review.Input{
		Service:      "svc",
		ChangedFiles: []string{"pkg/a.go"},
		ChangedSources: []core.SourceFile{
			{Path: "pkg/a.go", Content: []byte("// implements [US-001]\nfunc A() {}")},
		},
	})
	d := dimension(t, v, review.DimSpecCoverage)
	if !d.Checked {
		t.Fatalf("spec_coverage should always be checked, reason=%q", d.Reason)
	}
	if len(d.Findings) != 1 {
		t.Fatalf("expected 1 uncovered spec finding, got %d: %+v", len(d.Findings), d.Findings)
	}
}

func TestRun_SpecCoverage_CleanWhenNoRefsInDiff(t *testing.T) {
	v := review.Run(review.Input{
		Service:      "svc",
		ChangedFiles: []string{"pkg/a.go"},
		ChangedSources: []core.SourceFile{
			{Path: "pkg/a.go", Content: []byte("func A() {}")},
		},
	})
	d := dimension(t, v, review.DimSpecCoverage)
	if !d.Checked {
		t.Fatal("spec_coverage should be checked even with zero refs")
	}
	if len(d.Findings) != 0 {
		t.Errorf("expected zero findings, got %d", len(d.Findings))
	}
}

// ─── determinism ────────────────────────────────────────────────────────────

func TestRun_Deterministic(t *testing.T) {
	in := review.Input{
		Service:              "svc",
		ChangedFiles:         []string{"pkg/b.go", "pkg/a.go"},
		Nodes:                fixtureNodes(),
		ObligationsSupported: true,
		ObligationsByFile: map[string][]review.ObligationRule{
			"pkg/a.go": {{File: "pkg/a.go", Rule: "Rule one."}},
		},
	}
	v1 := review.Run(in)
	v2 := review.Run(in)
	if v1.Summary != v2.Summary {
		t.Errorf("Summary differs across identical runs:\n%q\n%q", v1.Summary, v2.Summary)
	}
	if len(v1.ChangedFiles) != 2 || v1.ChangedFiles[0] != "pkg/a.go" {
		t.Errorf("ChangedFiles should be sorted: %v", v1.ChangedFiles)
	}
}

// ─── C2-1: findings bridge (AttachClaims / ApplyClaimStatus) ────────────────

// TestFindingsBridge_RefutationDurability proves the pure-logic half of the
// C2-1 durable-findings guarantee: an unchanged finding re-emitted by a
// later review computes the SAME ClaimID, and once ApplyClaimStatus is told
// that ID is refuted (as it would be after a real `fathom claim refute` /
// ClaimFeedback write persisted via the store — see cmd/fathom's end-to-end
// review_test.go for the full store round trip), the finding carries
// Status "refuted" and is excluded from its dimension's active Count —
// never silently resurrected.
func TestFindingsBridge_RefutationDurability(t *testing.T) {
	buildVerdict := func() review.Verdict {
		return review.Run(review.Input{
			Service:      "svc",
			ChangedFiles: []string{"api/handler.go"},
			Nodes: []core.SymbolNode{
				{Service: "svc", Package: "api", Name: "Handler", Kind: core.KindGoFunc, FilePath: "api/handler.go", Line: 7, Calls: []string{"svc::store.Query"}},
				{Service: "svc", Package: "store", Name: "Query", Kind: core.KindGoFunc, FilePath: "store/query.go", Line: 3},
			},
			ArchChecked: true,
			ArchRules:   []archconformance.Rule{{FromPkg: "api", ToPkg: "store", Allow: false}},
		})
	}
	fixedNow := func() time.Time { return time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC) }

	// Review #1: emit the finding, attach claims.
	v1 := buildVerdict()
	claims1, _ := review.AttachClaims(&v1, fixedNow, "wm1")
	if len(claims1) == 0 {
		t.Fatal("expected at least one persisted claim")
	}
	id := v1.Dimensions[archIdx(t, v1)].Findings[0].ClaimID
	if id == "" {
		t.Fatal("expected a non-empty ClaimID after AttachClaims")
	}

	// Simulate an external refute (fathom claim refute / ClaimFeedback):
	// the store now holds this ClaimID with Status refuted.
	existing := []core.SemanticClaim{{ClaimID: id, Status: core.ClaimRefuted}}

	// Review #2: an unchanged finding re-emitted from the same diff must
	// compute the IDENTICAL ClaimID.
	v2 := buildVerdict()
	claims2, _ := review.AttachClaims(&v2, fixedNow, "wm2")
	id2 := v2.Dimensions[archIdx(t, v2)].Findings[0].ClaimID
	if id2 != id {
		t.Fatalf("ClaimID changed across identical re-review: %q vs %q", id, id2)
	}
	if len(claims2) != len(claims1) {
		t.Fatalf("claim count changed across identical re-review: %d vs %d", len(claims1), len(claims2))
	}

	review.ApplyClaimStatus(&v2, existing)
	d := v2.Dimensions[archIdx(t, v2)]
	if d.Findings[0].Status != string(core.ClaimRefuted) {
		t.Errorf("Status = %q, want %q", d.Findings[0].Status, core.ClaimRefuted)
	}
	if d.Count != 0 {
		t.Errorf("Count = %d, want 0 (the only finding is refuted)", d.Count)
	}
	// The finding must still be VISIBLE for audit, not dropped.
	if len(d.Findings) != 1 {
		t.Errorf("refuted finding should remain in Findings, got %d entries", len(d.Findings))
	}
}

func TestFindingsBridge_FreshFindingStaysActive(t *testing.T) {
	v := review.Run(review.Input{
		Service:      "svc",
		ChangedFiles: []string{"api/handler.go"},
		Nodes: []core.SymbolNode{
			{Service: "svc", Package: "api", Name: "Handler", Kind: core.KindGoFunc, FilePath: "api/handler.go", Line: 7, Calls: []string{"svc::store.Query"}},
			{Service: "svc", Package: "store", Name: "Query", Kind: core.KindGoFunc, FilePath: "store/query.go", Line: 3},
		},
		ArchChecked: true,
		ArchRules:   []archconformance.Rule{{FromPkg: "api", ToPkg: "store", Allow: false}},
	})
	fixedNow := func() time.Time { return time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC) }
	review.AttachClaims(&v, fixedNow, "wm1")
	review.ApplyClaimStatus(&v, nil) // nothing persisted yet
	d := v.Dimensions[archIdx(t, v)]
	if d.Findings[0].Status != "" {
		t.Errorf("Status = %q, want empty (never persisted before)", d.Findings[0].Status)
	}
	if d.Count != 1 {
		t.Errorf("Count = %d, want 1", d.Count)
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────

func dimension(t *testing.T, v review.Verdict, name string) review.Dimension {
	t.Helper()
	for _, d := range v.Dimensions {
		if d.Name == name {
			return d
		}
	}
	t.Fatalf("dimension %q not found in verdict: %+v", name, v)
	return review.Dimension{}
}

func archIdx(t *testing.T, v review.Verdict) int {
	t.Helper()
	for i, d := range v.Dimensions {
		if d.Name == review.DimArch {
			return i
		}
	}
	t.Fatalf("arch_conformance dimension not found")
	return -1
}
