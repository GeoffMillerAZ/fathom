package main

// u2remedy_test.go — PC-7 (docs/plans/proof-credibility.md): the U2 remedy
// text and the U2 counter are pinned to each other. The counter
// (planRowStates) derives U2 from the verify ledger as
// claimed − verified − failing, so the only ways a claimed row leaves U2
// are the ones that make verify count it as VERIFIED: a symbol contract
// or a lexical oracle (ADR-030). A `skip:` reason does not — and the
// remedy must say so, because it once promised otherwise.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/intent"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

func TestU2RemedyMatchesCounter(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "evidence.txt"), []byte("the measurement: p95 329\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Three claimed rows: a justified skip with no symbol, a lexical
	// oracle, and a symbol the (empty) index refutes.
	md := "- [x] Doc row {id: R-1, skip_test: true, skip: \"the evidence is evidence.txt, reviewed by hand\"}\n" +
		"- [x] Doc row proven {id: R-2, oracle: lexical, path: evidence.txt, pattern: \"p95 329\"}\n" +
		"- [x] Code row {id: R-3, symbol: Missing}\n"
	planPath := filepath.Join(root, "plan.md")
	if err := os.WriteFile(planPath, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := verify.ParseMarkdownPlan("svc", []byte(md))
	rep := verify.VerifyPlan(plan, emptyFileEvidence{files: verify.DirFiles(root)})
	byID := map[string]verify.TaskVerdict{}
	for _, tv := range rep.Tasks {
		byID[tv.TaskID] = tv
	}
	if byID["R-1"].Status != verify.Unknown || byID["R-1"].Label != "no-contract" {
		t.Fatalf("a skip: reason must NOT lift a row out of no-contract: %+v", byID["R-1"])
	}
	if byID["R-2"].Status != verify.Pass {
		t.Fatalf("a lexical oracle must verify the row: %+v", byID["R-2"])
	}
	if byID["R-3"].Status != verify.Fail {
		t.Fatalf("a refuted symbol must fail: %+v", byID["R-3"])
	}

	// The counter: the ledger row this report writes reads 3 claimed,
	// 1 verified, 1 failing → exactly one U2 row, the skip-reasoned one.
	ledger := filepath.Join(root, "verify-ledger.jsonl")
	t.Setenv("FATHOM_VERIFY_LEDGER", ledger)
	cwd, _ := os.Getwd()
	appendVerifyLedger([]string{planPath}, "svc", []verify.Report{rep})
	rows, _ := planRowStates(cwd)
	var u2 []intent.PlanRowState
	for _, r := range rows {
		if r.Unknown {
			u2 = append(u2, r)
		}
	}
	if len(u2) != 1 || !strings.HasPrefix(u2[0].ID, "1 claimed row(s) with no contract") {
		t.Fatalf("counter: want exactly one U2 row for the skip-reasoned claim, got %+v", rows)
	}

	// The remedy text, as the audit renders it for that row.
	audit := intent.RunAudit(intent.AuditInput{PlanRows: rows, Now: time.Now().UTC()})
	var remedy string
	for _, cl := range audit.Unproven {
		if cl.Kind == "U2" {
			remedy = cl.Remedy
		}
	}
	if remedy == "" {
		t.Fatal("no U2 claim rendered")
	}
	for _, want := range []string{"does not lift", "symbol", "oracle: lexical", "ADR-030"} {
		if !strings.Contains(remedy, want) {
			t.Errorf("U2 remedy must name the two real exits and disclaim skip:; lacks %q: %s", want, remedy)
		}
	}
	if strings.Contains(remedy, "or mark skip_test") {
		t.Errorf("U2 remedy still promises a skip: reason lifts the debt — the counter never honored it: %s", remedy)
	}
}

// emptyFileEvidence is an empty index that can read a working tree — the
// shape the CLI's store-mode evidence has for a service with no symbols.
type emptyFileEvidence struct {
	files verify.FileEvidence
}

func (e emptyFileEvidence) FindSymbol(string) []core.SymbolNode   { return nil }
func (e emptyFileEvidence) TestReaches(string) []string           { return nil }
func (e emptyFileEvidence) Reachable(string, string) bool         { return false }
func (e emptyFileEvidence) Contradictions() []core.Contradiction  { return nil }
func (e emptyFileEvidence) SpecImplementors(string) []string      { return nil }
func (e emptyFileEvidence) ReadRepoFile(p string) ([]byte, error) { return e.files.ReadRepoFile(p) }
