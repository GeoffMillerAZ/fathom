package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestPlansContextRendersFreshAndNamesStalePointers pins WF-2: `fathom
// plans context <plan> --row <id>` renders the row's ctx: pack fresh
// against the live index — a resolvable symbol pointer gets its
// signature, file:line, and direct callers; a resolvable file pointer
// gets its line count; an unresolvable pointer is NAMED, never silently
// dropped — and the command always exits 0 (a pull tool, not a gate).
func TestPlansContextRendersFreshAndNamesStalePointers(t *testing.T) {
	storePath := seedPlansFixtureStore(t)
	pf := writeTempPlan(t, "# C\n\n"+
		"- [ ] Row with ctx pointers {id: T-1, ctx: validateCharge plans.go NoSuchThingXYZ, "+
		`reason: "why this row exists", trigger: "when the SDK ships"}`+"\n")

	var out, errBuf bytes.Buffer
	code := run([]string{
		"plans", "context", pf,
		"--row", "T-1", "--service", "payment-service", "--store", storePath,
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("plans context exit = %d, stderr: %s", code, errBuf.String())
	}
	got := out.String()

	// reason:/trigger: text carried through.
	if !strings.Contains(got, "why this row exists") {
		t.Errorf("missing reason: text:\n%s", got)
	}
	if !strings.Contains(got, "when the SDK ships") {
		t.Errorf("missing trigger: text:\n%s", got)
	}

	// Resolvable symbol pointer: signature + file:line + direct caller.
	// validateCharge is called exactly once, by Charge, in the faux
	// payment-service fixture (internal/connector/testdata/fauxtown/
	// payment-service/payment.go).
	if !strings.Contains(got, "validateCharge") {
		t.Errorf("missing symbol pointer render:\n%s", got)
	}
	if !strings.Contains(got, "payment-service/payment.go") {
		t.Errorf("missing file:line for validateCharge:\n%s", got)
	}
	if !strings.Contains(got, "Charge") {
		t.Errorf("missing validateCharge's direct caller (Charge):\n%s", got)
	}

	// Resolvable file pointer: line count (plans.go exists relative to
	// this package's directory, which is the test's working directory).
	if !strings.Contains(got, "plans.go") {
		t.Errorf("missing file pointer render:\n%s", got)
	}

	// Unresolvable pointer: named, not dropped, and exit stays 0.
	if !strings.Contains(got, "NoSuchThingXYZ") {
		t.Errorf("unresolvable pointer was dropped instead of named:\n%s", got)
	}
	if !strings.Contains(got, "unresolved (moved/renamed since planning)") {
		t.Errorf("missing the unresolved-pointer message:\n%s", got)
	}
}

// TestPlansContextBudgetElides pins the budget-cap contract
// (docs/design/budgeted-answers.md §2): a tiny budget forces an elision
// block naming what was dropped, and the command still exits 0.
func TestPlansContextBudgetElides(t *testing.T) {
	storePath := seedPlansFixtureStore(t)
	pf := writeTempPlan(t, "# C\n\n"+
		"- [ ] Row with many ctx pointers {id: T-1, ctx: validateCharge Charge ChargeRequest ChargeResult Processor plans.go}\n")

	var out, errBuf bytes.Buffer
	code := run([]string{
		"plans", "context", pf,
		"--row", "T-1", "--service", "payment-service", "--store", storePath,
		"--budget", "50",
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("plans context exit = %d, stderr: %s", code, errBuf.String())
	}
	got := out.String()
	if !strings.Contains(got, "elided") {
		t.Errorf("tiny budget did not produce an elision block:\n%s", got)
	}
}

// TestPlansContextUnknownRow is the CLI usage boundary: a row id absent
// from the plan is an error (nothing to render), unlike a resolution
// failure on a ctx: pointer within a found row.
func TestPlansContextUnknownRow(t *testing.T) {
	storePath := seedPlansFixtureStore(t)
	pf := writeTempPlan(t, "- [ ] Only row {id: T-1}\n")
	var out, errBuf bytes.Buffer
	code := run([]string{"plans", "context", pf, "--row", "NOPE", "--service", "payment-service", "--store", storePath}, &out, &errBuf)
	if code == 0 {
		t.Fatal("want non-zero exit for an unknown row id")
	}
}

// TestPlansContextDaemonUnreachable pins the operational fallback: no
// --store and an unreachable --url still exits 0, naming the failure
// inline instead of failing the command (a pull tool, not a gate).
func TestPlansContextDaemonUnreachable(t *testing.T) {
	pf := writeTempPlan(t, "- [ ] Row {id: T-1, ctx: SomeSymbol}\n")
	var out, errBuf bytes.Buffer
	code := run([]string{
		"plans", "context", pf,
		"--row", "T-1", "--service", "payment-service",
		"--url", "http://127.0.0.1:1", // nothing listens here
	}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("plans context with an unreachable daemon must still exit 0, got %d; stderr=%s", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "unresolved") {
		t.Errorf("expected the unreachable index to be named inline:\n%s", out.String())
	}
}
