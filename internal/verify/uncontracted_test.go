package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// uncontracted_test.go — a claimed row with no contract at all verifies
// no-contract (Unknown) exactly as a skip-only row does, and the default gate
// lets both through. skiponly_test.go pins the waiver shape; these pin the bare
// shape (S-10 in skipOnlyMD: `{id: S-10}` and nothing else), the lint and the
// gate agreeing on EVERY claimed no-contract row, and this repository's own
// top-level plans carrying none (PC-20: the check `task validate` runs on
// every push, because `fathom verify` without --strict never fails on one).

func TestLintPlan_ClaimedBareRowHasNoContract(t *testing.T) {
	p := ParseMarkdownPlan("svc", []byte(skipOnlyMD))
	var got []string
	for _, f := range LintPlan(p) {
		if strings.HasPrefix(f, "S-10: ") {
			got = append(got, f)
		}
	}
	if len(got) != 1 {
		t.Fatalf("S-10 (claimed, no annotation beyond its id) findings = %v, want exactly one", got)
	}
	for _, word := range []string{"no-contract", "--strict", "symbol", "oracle", "gone"} {
		if !strings.Contains(got[0], word) {
			t.Errorf("finding %q does not name %q (the remedy must be in the message)", got[0], word)
		}
	}
	if strings.Contains(got[0], "waiver") {
		t.Errorf("finding %q calls a bare row a waiver; S-10 carries no skip", got[0])
	}
}

// Every claimed row VerifyPlan labels no-contract is one the lint names, with
// or without a waiver, and claimedWithoutContract is true exactly for those.
func TestLintPlan_EveryClaimedNoContractRowIsNamed(t *testing.T) {
	p := ParseMarkdownPlan("svc", []byte(skipOnlyMD))
	named := map[string]bool{}
	for _, f := range LintPlan(p) {
		if strings.Contains(f, "no-contract") {
			named[strings.SplitN(f, ":", 2)[0]] = true
		}
	}
	byID := map[string]Task{}
	for _, tk := range p.Tasks {
		byID[tk.ID] = tk
	}
	noContract := 0
	for _, v := range VerifyPlan(p, noEvidence{}).Tasks {
		want := v.Claimed && v.Label == "no-contract"
		if want {
			noContract++
		}
		if got := claimedWithoutContract(byID[v.TaskID]); got != want {
			t.Errorf("%s: claimedWithoutContract = %v, VerifyPlan claimed no-contract = %v (label %q)", v.TaskID, got, want, v.Label)
		}
		if want && !named[v.TaskID] {
			t.Errorf("%s: claimed no-contract row was not named by the lint", v.TaskID)
		}
		if named[v.TaskID] && !want {
			t.Errorf("%s: lint names it no-contract but VerifyPlan labels it %q (claimed=%v)", v.TaskID, v.Label, v.Claimed)
		}
	}
	if noContract != 3 { // S-1, S-2 (waivers) and S-10 (bare)
		t.Fatalf("fixture drift: %d claimed no-contract rows, want 3", noContract)
	}
}

// TestRepoPlans_EveryClaimedRowCarriesAContract is what holds PC-20 once it is
// claimed: every top-level plan (the pre-push gate's own scope — README.md and
// archive/ excluded) has no claimed row that `fathom verify` would read
// no-contract. The pre-push `verify` step runs without --strict and exits 0 on
// such a row; this test, under `task validate`, is the part that fails.
func TestRepoPlans_EveryClaimedRowCarriesAContract(t *testing.T) {
	dir := filepath.Join("..", "..", "docs", "plans")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("docs/plans unreadable (%v) — an unread plan must not pass as a clean one", err)
	}
	var bad []string
	read := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") || name == "README.md" {
			continue
		}
		md, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("docs/plans/%s unreadable (%v) — an unread plan must not pass as a clean one", name, err)
		}
		read++
		for _, tk := range ParseMarkdownPlan("fathom", md).Tasks {
			if claimedWithoutContract(tk) {
				bad = append(bad, fmt.Sprintf("docs/plans/%s:%d %s", name, tk.Line, tk.ID))
			}
		}
	}
	if read == 0 {
		t.Fatal("no top-level plan was read — the check would pass vacuously")
	}
	if len(bad) > 0 {
		t.Errorf("%d claimed row(s) with no contract (verify reads them no-contract; --strict exits 1): give each symbol:, oracle: lexical|ledger, or gone:, or uncheck it:\n  %s",
			len(bad), strings.Join(bad, "\n  "))
	}
}
