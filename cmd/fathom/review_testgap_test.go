package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
)

// BD-5 (docs/plans/blast-depth.md): `fathom review` prints the affected
// tests and the test gap after the dimension summary, cited file:line.
func TestRenderTestGap(t *testing.T) {
	var out bytes.Buffer
	renderTestGap(&out, impact.TestGapResult{
		Checked: true,
		AffectedTests: []impact.TestReach{
			{Test: "svc::pkg.TestFoo", File: "pkg/a_test.go", Line: 1, Citation: "pkg/a_test.go#L1-L1", Reaches: []string{"svc::pkg.A", "svc::pkg.B"}},
			{Test: "svc::pkg.TestRun", File: "pkg/r_test.go", Line: 4, Citation: "pkg/r_test.go#L4-L4", Reaches: []string{"svc::pkg.Complete"}, ViaDispatch: true},
		},
		Gap: []impact.SymbolGap{{Symbol: "svc::pkg.Orphan", File: "pkg/orphan.go", Line: 1, Citation: "pkg/orphan.go#L1-L1"}},
	})
	got := out.String()
	for _, want := range []string{
		"[x] test_impact: 2 affected test(s), 1 changed symbol(s) with no test",
		"  - run svc::pkg.TestFoo (pkg/a_test.go#L1-L1) reaches svc::pkg.A, svc::pkg.B",
		"  - run svc::pkg.TestRun (pkg/r_test.go#L4-L4) reaches svc::pkg.Complete via interface dispatch",
		"  - gap svc::pkg.Orphan (pkg/orphan.go#L1-L1): no test reaches it",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderTestGap output missing %q:\n%s", want, got)
		}
	}

	out.Reset()
	renderTestGap(&out, impact.TestGapResult{Reason: "no symbols indexed for this service — ingest it before reviewing"})
	if !strings.Contains(out.String(), "[ ] test_impact: not checked") || !strings.Contains(out.String(), "reason: no symbols indexed") {
		t.Errorf("unchecked rendering = %q", out.String())
	}

	// Determinism: the same input renders identically.
	var again bytes.Buffer
	renderTestGap(&again, impact.TestGapResult{Reason: "no symbols indexed for this service — ingest it before reviewing"})
	if again.String() != out.String() {
		t.Fatal("renderTestGap is not deterministic")
	}
}
