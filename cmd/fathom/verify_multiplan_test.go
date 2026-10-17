package main

// verify_multiplan_test.go — the pre-push verify step is ONE `fathom verify`
// call over every plan (one shared call graph, not one process per plan):
// exit 1 when any plan has an unproven claimed row, 0 when all hold, and the
// text output names each plan that prints rows.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeScratchPlan(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestVerifyManyPlansOneCallNamesEachFailingPlan(t *testing.T) {
	scratchEnv(t)
	store, good := scratchRunFixture(t)
	bad := writeScratchPlan(t, "bad.md", "# Bad\n\n- [x] Missing {id: BAD-1, symbol: NoSuchSymbolAnywhere}\n")
	good2 := writeScratchPlan(t, "good2.md", "# Good 2\n\n- [x] Charge handler {id: SC-9, symbol: Charge, skip_test: true, skip: \"fixture\"}\n")

	var out, errb bytes.Buffer
	if code := run([]string{"verify", "--strict", "--service", "payment-service", "--store", store, "--quiet",
		"--plan", good, "--plan", bad, "--plan", good2}, &out, &errb); code != 1 {
		t.Fatalf("a plan with an unproven claimed row must exit 1, got %d\nstdout=%s\nstderr=%s", code, out.String(), errb.String())
	}
	if !strings.Contains(out.String(), "plan "+bad) || !strings.Contains(out.String(), "BAD-1") {
		t.Errorf("output must name the failing plan and its row:\n%s", out.String())
	}
	if strings.Contains(out.String(), "plan "+good) {
		t.Errorf("a plan with no printed rows must not be named under --quiet:\n%s", out.String())
	}

	out.Reset()
	errb.Reset()
	if code := run([]string{"verify", "--strict", "--service", "payment-service", "--store", store, "--quiet",
		"--plan", good, "--plan", good2}, &out, &errb); code != 0 {
		t.Fatalf("all plans proven must exit 0, got %d\nstdout=%s\nstderr=%s", code, out.String(), errb.String())
	}
}

func TestVerifySinglePlanOutputHasNoPlanHeader(t *testing.T) {
	scratchEnv(t)
	store, _ := scratchRunFixture(t)
	bad := writeScratchPlan(t, "bad.md", "# Bad\n\n- [x] Missing {id: BAD-1, symbol: NoSuchSymbolAnywhere}\n")
	var out, errb bytes.Buffer
	if code := run([]string{"verify", "--strict", "--service", "payment-service", "--store", store, "--quiet", "--plan", bad}, &out, &errb); code != 1 {
		t.Fatalf("exit %d, want 1\n%s", code, out.String())
	}
	if strings.Contains(out.String(), "plan "+bad) {
		t.Errorf("a single-plan run keeps its output unchanged:\n%s", out.String())
	}
}
