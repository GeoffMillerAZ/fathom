package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
)

// plansFixtureFiles returns the two committed golden plan fixtures
// (cmd/fathom/testdata/plans/fixture-{a,b}.md) as absolute paths, working-
// directory independent. See fixture-a.md's doc comment for why its task
// IDs are deliberately out of document order (E3-2's ordering contract).
func plansFixtureFiles(t *testing.T) []string {
	t.Helper()
	return []string{
		filepath.Join("testdata", "plans", "fixture-a.md"),
		filepath.Join("testdata", "plans", "fixture-b.md"),
	}
}

// seedPlansFixtureStore ingests the faux payment-service fixture (the same
// corpus verify_test.go uses — Charge, validateCharge, ChargeRequest, …)
// into a fresh store so the fixture plans' claimed symbols resolve.
func seedPlansFixtureStore(t *testing.T) string {
	t.Helper()
	storePath := filepath.Join(t.TempDir(), "test.duckdb")
	var ing bytes.Buffer
	code := run([]string{
		"ingest", "--service", "payment-service",
		"--path", fauxPaymentPath(),
		"--store", storePath, "--mode", "faux",
	}, &ing, &ing)
	if code != 0 {
		t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}
	return storePath
}

func planArgs(sub string, storePath string, files []string, extra ...string) []string {
	args := []string{"plans", sub, "--service", "payment-service", "--store", storePath, "--json"}
	for _, f := range files {
		args = append(args, "--plan", f)
	}
	return append(args, extra...)
}

// ─── plans status (E3-1) ────────────────────────────────────────────────────

type planStatusRow struct {
	Plan                string `json:"plan"`
	Total               int    `json:"total"`
	Proven              int    `json:"proven"`
	ClaimedUnproven     int    `json:"claimed_unproven"`
	Open                int    `json:"open"`
	DeferredWithTrigger int    `json:"deferred_with_trigger"`
	SkippedOther        int    `json:"skipped_other"`
}

func TestCmdPlansStatus_Rollup(t *testing.T) {
	storePath := seedPlansFixtureStore(t)
	files := plansFixtureFiles(t)

	var out, errBuf bytes.Buffer
	code := run(planArgs("status", storePath, files), &out, &errBuf)
	if code != 0 {
		t.Fatalf("plans status exit = %d, stderr: %s", code, errBuf.String())
	}

	var rows []planStatusRow
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatalf("plans status --json not valid JSON: %v\n%s", err, out.String())
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 plan rollups (a, b), got %d: %+v", len(rows), rows)
	}

	// Sorted by plan name: fixture-a.md before fixture-b.md.
	a, b := rows[0], rows[1]
	if !bytes.HasSuffix([]byte(a.Plan), []byte("fixture-a.md")) {
		t.Errorf("rows[0].Plan = %q, want fixture-a.md", a.Plan)
	}
	if !bytes.HasSuffix([]byte(b.Plan), []byte("fixture-b.md")) {
		t.Errorf("rows[1].Plan = %q, want fixture-b.md", b.Plan)
	}

	wantA := planStatusRow{Total: 6, Proven: 1, ClaimedUnproven: 1, Open: 2, DeferredWithTrigger: 1, SkippedOther: 1}
	if a.Total != wantA.Total || a.Proven != wantA.Proven || a.ClaimedUnproven != wantA.ClaimedUnproven ||
		a.Open != wantA.Open || a.DeferredWithTrigger != wantA.DeferredWithTrigger || a.SkippedOther != wantA.SkippedOther {
		t.Errorf("fixture-a rollup = %+v, want %+v", a, wantA)
	}
	wantB := planStatusRow{Total: 2, Proven: 1, ClaimedUnproven: 0, Open: 1, DeferredWithTrigger: 0, SkippedOther: 0}
	if b.Total != wantB.Total || b.Proven != wantB.Proven || b.ClaimedUnproven != wantB.ClaimedUnproven ||
		b.Open != wantB.Open || b.DeferredWithTrigger != wantB.DeferredWithTrigger || b.SkippedOther != wantB.SkippedOther {
		t.Errorf("fixture-b rollup = %+v, want %+v", b, wantB)
	}

	// Determinism: identical bytes on a second run.
	var out2 bytes.Buffer
	if code := run(planArgs("status", storePath, files), &out2, &errBuf); code != 0 {
		t.Fatalf("second plans status exit = %d, stderr: %s", code, errBuf.String())
	}
	if out.String() != out2.String() {
		t.Errorf("plans status --json not byte-identical across runs:\nfirst:\n%s\nsecond:\n%s", out.String(), out2.String())
	}
}

func TestCmdPlansStatus_HumanTable(t *testing.T) {
	storePath := seedPlansFixtureStore(t)
	files := plansFixtureFiles(t)
	args := []string{"plans", "status", "--service", "payment-service", "--store", storePath}
	for _, f := range files {
		args = append(args, "--plan", f)
	}
	var out, errBuf bytes.Buffer
	if code := run(args, &out, &errBuf); code != 0 {
		t.Fatalf("plans status exit = %d, stderr: %s", code, errBuf.String())
	}
	if out.Len() == 0 {
		t.Fatal("human table output is empty")
	}
	for _, want := range []string{"fixture-a.md", "fixture-b.md", "proven"} {
		if !bytes.Contains(out.Bytes(), []byte(want)) {
			t.Errorf("human table missing %q:\n%s", want, out.String())
		}
	}
}

func TestCmdPlansStatus_MissingService(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"plans", "status", "--plan", "x.md"}, &out, &errBuf)
	if code == 0 {
		t.Fatal("want non-zero exit when --service is missing")
	}
}

// ─── plans next (E3-2) ──────────────────────────────────────────────────────

type planNextRow struct {
	Plan    string `json:"plan"`
	ID      string `json:"id"`
	Title   string `json:"title"`
	Symbol  string `json:"symbol"`
	Package string `json:"package"`
	State   string `json:"state"`
	Detail  string `json:"detail,omitempty"`
}

// TestCmdPlansNext_Ordering is E3-2's ordering contract: rows come out
// (plan name, then DOCUMENT order) — never TaskID-lexical order. fixture-a's
// frontier-eligible rows are authored as A-2, A-7, A-1 in that document
// order (A-9 is proven/excluded; A-5, A-6 are skipped/excluded), which is
// NOT alphabetical — a regression to `sort.Slice` by TaskID would produce
// A-1, A-2, A-7 instead and this test would catch it.
func TestCmdPlansNext_Ordering(t *testing.T) {
	storePath := seedPlansFixtureStore(t)
	files := plansFixtureFiles(t)

	var out, errBuf bytes.Buffer
	code := run(planArgs("next", storePath, files), &out, &errBuf)
	if code != 0 {
		t.Fatalf("plans next exit = %d, stderr: %s", code, errBuf.String())
	}

	var rows []planNextRow
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatalf("plans next --json not valid JSON: %v\n%s", err, out.String())
	}

	wantIDs := []string{"A-2", "A-7", "A-1", "B-2"}
	if len(rows) != len(wantIDs) {
		t.Fatalf("frontier rows = %d, want %d: %+v", len(rows), len(wantIDs), rows)
	}
	for i, id := range wantIDs {
		if rows[i].ID != id {
			t.Errorf("rows[%d].ID = %q, want %q (full order: %v)", i, rows[i].ID, id, idsOf(rows))
		}
	}

	byID := map[string]planNextRow{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	if byID["A-2"].State != "claimed-failing" {
		t.Errorf("A-2 state = %q, want claimed-failing", byID["A-2"].State)
	}
	if byID["A-2"].Detail == "" {
		t.Error("A-2 (claimed-failing) must carry a failure detail")
	}
	if byID["A-7"].State != "open" {
		t.Errorf("A-7 state = %q, want open", byID["A-7"].State)
	}
	if byID["A-1"].State != "in-progress" {
		t.Errorf("A-1 state = %q, want in-progress", byID["A-1"].State)
	}
	if byID["A-7"].Symbol != "Charge" {
		t.Errorf("A-7 symbol anchor = %q, want Charge", byID["A-7"].Symbol)
	}
	if byID["B-2"].Plan == "" {
		t.Error("B-2 row missing its plan file anchor")
	}

	// Determinism.
	var out2 bytes.Buffer
	if code := run(planArgs("next", storePath, files), &out2, &errBuf); code != 0 {
		t.Fatalf("second plans next exit = %d, stderr: %s", code, errBuf.String())
	}
	if out.String() != out2.String() {
		t.Errorf("plans next --json not byte-identical across runs")
	}
}

func idsOf(rows []planNextRow) []string {
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	return ids
}

// TestCmdPlansNext_ClaimedUnknownSurfaces is critic finding 4: a plan whose
// only row is claimed with no contract verifies Unknown — scripts/ci.sh's
// `verify --strict` BLOCKS on exactly that row, so a frontier that answered
// "no open work — every row is claimed-and-verified or skipped" would say
// done while CI says blocked. Claimed-Unknown rows must surface in the
// frontier as state "claimed-unknown".
func TestCmdPlansNext_ClaimedUnknownSurfaces(t *testing.T) {
	storePath := seedPlansFixtureStore(t)
	pf := writeTempPlan(t, "# U\n\n- [x] Mystery work with no contract {id: U-1}\n")

	// JSON: exactly one frontier row, state claimed-unknown.
	var out, errBuf bytes.Buffer
	code := run(planArgs("next", storePath, []string{pf}), &out, &errBuf)
	if code != 0 {
		t.Fatalf("plans next exit = %d, stderr: %s", code, errBuf.String())
	}
	var rows []planNextRow
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatalf("plans next --json not valid JSON: %v\n%s", err, out.String())
	}
	if len(rows) != 1 {
		t.Fatalf("frontier rows = %d, want 1 (the claimed-Unknown row): %+v", len(rows), rows)
	}
	if rows[0].ID != "U-1" || rows[0].State != "claimed-unknown" {
		t.Errorf("row = {ID: %q, State: %q}, want {U-1, claimed-unknown}", rows[0].ID, rows[0].State)
	}
	if rows[0].Detail == "" {
		t.Error("claimed-unknown row must carry a detail (e.g. no-contract) explaining why it is unproven")
	}

	// Human output must not declare the plan done.
	args := []string{"plans", "next", "--service", "payment-service", "--store", storePath, "--plan", pf}
	var human, humanErr bytes.Buffer
	if code := run(args, &human, &humanErr); code != 0 {
		t.Fatalf("plans next (human) exit = %d, stderr: %s", code, humanErr.String())
	}
	if bytes.Contains(human.Bytes(), []byte("no open work")) {
		t.Errorf("frontier claims 'no open work' on a plan CI --strict would block:\n%s", human.String())
	}
	if !bytes.Contains(human.Bytes(), []byte("claimed-unknown")) {
		t.Errorf("human output missing the claimed-unknown row:\n%s", human.String())
	}
}
