package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

const genPlanFixture = `# Fixture
- [x] Grammar lands [FR-001] {id: G2-1, symbol: ParseMarkdownPlan, package: internal/verify, min_tier: verified}
- [~] Long haul {id: W-2, reason: "kit-side done, donor NOT deleted"}
- [-] Later {id: W-3, disposition: deferred, reason: "waiting on O6", trigger: "skip share <30%"}
- [ ] Open {id: W-4, symbol: GateRequireAll, skip_test: true, skip: "cmd oracle pending", weight: 3, accept: "WHEN asked, THE SYSTEM SHALL gate"}
- [x] Measured {id: W-5, oracle: lexical, path: docs/measurements/m.md, pattern: "p95 ≈ 329"}
- [x] Positional untyped line
- [x] The harness API {id: W-6, symbol: RunHarness, library: true, reason: "gate harness: TestHarness runs it"}
`

func TestGenPlanCue_RoundTrip(t *testing.T) {
	plan := verify.ParseMarkdownPlan("svc", []byte(genPlanFixture))
	doc := renderPlanCUE(plan)

	// Determinism.
	if doc != renderPlanCUE(plan) {
		t.Fatal("renderPlanCUE is not deterministic")
	}
	if !strings.Contains(doc, "Code generated") {
		t.Error("emitted CUE must carry the generated-only header")
	}

	back, err := planFromCUE([]byte(doc))
	if err != nil {
		t.Fatalf("planFromCUE: %v\n%s", err, doc)
	}
	// The row's Markdown position is not part of the contract, so the
	// typed shadow does not carry it.
	stripMarkdownPosition(&plan)
	if !reflect.DeepEqual(plan, back) {
		t.Fatalf("round-trip loss:\n md: %+v\ncue: %+v\ndoc:\n%s", plan, back, doc)
	}
}

func TestCmdGenPlanCue_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	planPath := filepath.Join(dir, "plan.md")
	outPath := filepath.Join(dir, "plan.cue")
	if err := os.WriteFile(planPath, []byte(genPlanFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := cmdGenPlanCue([]string{"--plan", planPath, "--service", "svc", "--output", outPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "round-trip verified") {
		t.Errorf("stdout = %q, want round-trip confirmation", stdout.String())
	}
	emitted, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(emitted), `disposition: "deferred"`) {
		t.Error("v2 disposition missing from emitted CUE")
	}
	// Missing flags → operational exit 2.
	if code := cmdGenPlanCue(nil, &stdout, &stderr); code != 2 {
		t.Errorf("missing flags: exit %d, want 2", code)
	}
}
