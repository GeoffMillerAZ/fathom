package fusion_test

// TestTypedEdgesNameDroppedOutOfScopeTargets is VS-9 (docs/plans/
// verify-spine.md): the typed-edges pass counts and reports every edge it
// dropped because the target's declaring file was outside the connector's
// discovered set (targetAllowed false) — and when such a file EXISTS on
// disk, the report names it as a discovery failure, never a clean "0
// failed". This is the likeliest cause of the 2026-08-23 cpp-toolchain
// flip: TestSameNamedMethodsDoNotFlipReachability (internal/analysis/
// callgraph) independently establishes the syntax-pass ambiguity resolver
// was already safe, so a silently wedged connector-scope drop — not a
// first-match-wins resolver — is the mechanism this test targets.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// certifies: invariant I24 (asserts)
func TestTypedEdgesNameDroppedOutOfScopeTargets(t *testing.T) {
	dir := typedFixtureDir(t)
	ctx := context.Background()
	st := store.NewMemoryStore()

	// embediface.go's UseWrapper dispatches (through the embedded Adapter
	// interface) to AdapterA.Complete/AdapterB.Complete, both declared in
	// adapter.go — a different file than the caller. Omit adapter.go from
	// the allowed set while keeping every other fixture file: adapter.go
	// still EXISTS on disk, so this is a discovery gap, not a legitimate
	// exclusion.
	names, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil || len(names) == 0 {
		t.Fatalf("globbing fixture .go files: %v (n=%d)", err, len(names))
	}
	allowed := make(map[string]bool, len(names))
	for _, n := range names {
		base := filepath.Base(n)
		if base == "adapter.go" {
			continue
		}
		allowed[base] = true
	}

	report, err := fusion.EnrichTypedEdges(ctx, st, "typedfixture", dir, allowed)
	if err != nil {
		t.Fatalf("EnrichTypedEdges: %v", err)
	}

	if report.EdgesDroppedOutOfScope == 0 {
		t.Error("EdgesDroppedOutOfScope = 0, want > 0 (UseWrapper's dispatch targets in adapter.go were excluded)")
	}
	if report.DiscoveryGaps == 0 {
		t.Error("DiscoveryGaps = 0, want > 0 — adapter.go exists on disk despite being excluded, a discovery failure, not a clean zero")
	}
	foundSample, foundGap := false, false
	for _, f := range report.DroppedOutOfScopeSample {
		if f == "adapter.go" {
			foundSample = true
		}
	}
	for _, f := range report.DiscoveryGapFiles {
		if f == "adapter.go" {
			foundGap = true
		}
	}
	if !foundSample {
		t.Errorf("DroppedOutOfScopeSample = %v, want to name adapter.go", report.DroppedOutOfScopeSample)
	}
	if !foundGap {
		t.Errorf("DiscoveryGapFiles = %v, want to name adapter.go", report.DiscoveryGapFiles)
	}
}
