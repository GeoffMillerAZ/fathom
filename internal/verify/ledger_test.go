package verify

// ledger_test.go — RC-34 (ADR-034 decision 2): the ledger oracle verifies a
// published bench reading against a committed JSONL ledger, golden fixtures
// under testdata/ledger/.

import (
	"os"
	"strings"
	"testing"
)

const ledgerMD = `# plan
- [x] Guarded win recorded {id: RG-1, oracle: ledger, path: testdata/ledger/scenarios-ledger.jsonl, run_id: guards-pilot-3, task: guards-i1, arm: full, min_trials: 2}
- [x] Control cell flagged unused {id: RG-2, oracle: ledger, path: testdata/ledger/scenarios-ledger.jsonl, run_id: guards-pilot-3, task: guards-i1, arm: control}
- [x] Instrument-defective row voided {id: RG-3, oracle: ledger, path: testdata/ledger/scenarios-ledger.jsonl, run_id: guards-pilot-3, task: guards-i11, arm: full}
- [x] Below the trial floor {id: RG-4, oracle: ledger, path: testdata/ledger/scenarios-ledger.jsonl, run_id: guards-pilot-3, task: guards-i31, arm: full, min_trials: 2}
- [x] Row never ran {id: RG-5, oracle: ledger, path: testdata/ledger/scenarios-ledger.jsonl, run_id: guards-pilot-3, task: guards-i99, arm: full}
- [x] Home bench keyed by run_id alone {id: RG-6, oracle: ledger, path: testdata/ledger/home-ledger.jsonl, run_id: 20260913T002832Z}
- [x] Home bench under a floor it cannot meet {id: RG-7, oracle: ledger, path: testdata/ledger/home-ledger.jsonl, run_id: 20260913T002832Z, min_trials: 50}
- [x] Two contracts {id: RG-8, symbol: Foo, oracle: ledger, path: testdata/ledger/scenarios-ledger.jsonl, run_id: guards-pilot-3, task: guards-i1, arm: full}
- [x] No path {id: RG-9, oracle: ledger, run_id: guards-pilot-3}
- [x] No run_id {id: RG-10, oracle: ledger, path: testdata/ledger/scenarios-ledger.jsonl}
- [x] Escapes the repo {id: RG-11, oracle: ledger, path: ../secrets.jsonl, run_id: guards-pilot-3}
- [x] Arm without a task {id: RG-12, oracle: ledger, path: testdata/ledger/scenarios-ledger.jsonl, run_id: guards-pilot-3, arm: full}
- [x] Non-numeric min_trials {id: RG-13, oracle: ledger, path: testdata/ledger/scenarios-ledger.jsonl, run_id: guards-pilot-3, task: guards-i1, arm: full, min_trials: many}
- [x] Ledger unparseable {id: RG-14, oracle: ledger, path: testdata/ledger/bad.jsonl, run_id: broken}
`

func TestParseMarkdownPlan_LedgerOracleKeys(t *testing.T) {
	p := ParseMarkdownPlan("svc", []byte(ledgerMD))
	r1 := p.Tasks[0]
	if r1.Oracle != "ledger" || r1.OraclePath != "testdata/ledger/scenarios-ledger.jsonl" ||
		r1.OracleRunID != "guards-pilot-3" || r1.OracleTaskID != "guards-i1" ||
		r1.OracleArm != "full" || r1.OracleMinTrials != "2" {
		t.Fatalf("ledger oracle keys parsed wrong: %+v", r1)
	}
}

func TestEvalLedgerOracle_VerifiesAndRefuses(t *testing.T) {
	p := ParseMarkdownPlan("svc", []byte(ledgerMD))
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	rep := VerifyPlan(p, dirEvidence{noEvidence{}, DirFiles(wd)})
	want := map[string]struct {
		status CriterionStatus
		label  string
		detail string
	}{
		"RG-1":  {Pass, "verified", "2 trial(s)"},
		"RG-2":  {Fail, "surface-unused", "surface_unused: true"},
		"RG-3":  {Fail, "instrument-defect", "instrument_defect"},
		"RG-4":  {Fail, "below-trial-floor", "below the required floor of 2"},
		"RG-5":  {Fail, "ledger-row-absent", "no ledger row"},
		"RG-6":  {Pass, "verified", "33 trial(s)"}, // trialCount falls back to treatment_trials=33
		"RG-7":  {Fail, "below-trial-floor", "below the required floor of 50"},
		"RG-8":  {Fail, "contract-invalid", "one contract per row"},
		"RG-9":  {Fail, "contract-invalid", "without a path"},
		"RG-10": {Fail, "contract-invalid", "without a run_id"},
		"RG-11": {Fail, "contract-invalid", "repo-relative"},
		"RG-12": {Fail, "contract-invalid", "arm without a task"},
		"RG-13": {Fail, "contract-invalid", "non-negative integer"},
		"RG-14": {Fail, "ledger-unparseable", "not valid JSON"},
	}
	for _, tv := range rep.Tasks {
		w, ok := want[tv.TaskID]
		if !ok {
			t.Fatalf("unexpected task %s", tv.TaskID)
		}
		if tv.Status != w.status || tv.Label != w.label {
			t.Errorf("%s: status/label = %s/%s, want %s/%s (%+v)", tv.TaskID, tv.Status, tv.Label, w.status, w.label, tv.Criteria)
		}
		var led *Criterion
		for i := range tv.Criteria {
			if tv.Criteria[i].Name == ledgerCriterion {
				led = &tv.Criteria[i]
			}
		}
		if led == nil {
			t.Errorf("%s: no %s criterion", tv.TaskID, ledgerCriterion)
			continue
		}
		if !strings.Contains(led.Detail, w.detail) {
			t.Errorf("%s: detail %q lacks %q", tv.TaskID, led.Detail, w.detail)
		}
	}
}

func TestEvalLedgerOracle_NoFileEvidenceIsUnknownNeverPass(t *testing.T) {
	p := ParseMarkdownPlan("svc", []byte("- [x] Row {id: RG-1, oracle: ledger, path: testdata/ledger/scenarios-ledger.jsonl, run_id: guards-pilot-3, task: guards-i1, arm: full}\n"))
	rep := VerifyPlan(p, noEvidence{})
	tv := rep.Tasks[0]
	if tv.Status != Unknown || tv.Label != "oracle-unevaluated" {
		t.Fatalf("without FileEvidence the row must be Unknown/oracle-unevaluated, got %s/%s", tv.Status, tv.Label)
	}
	if g := Gate(rep, true); g.Pass {
		t.Error("--strict must refuse an unevaluated oracle row")
	}
	if g := Gate(rep, false); !g.Pass {
		t.Error("non-strict must fail open on an unevaluated oracle row")
	}
}

func TestLintLedgerOracle_RefusesTheSameShapesVerifyFails(t *testing.T) {
	p := ParseMarkdownPlan("svc", []byte(ledgerMD))
	findings := LintPlan(p)
	joined := strings.Join(findings, "\n")
	for _, id := range []string{"RG-8", "RG-9", "RG-10", "RG-11", "RG-12", "RG-13"} {
		if !strings.Contains(joined, id+": ledger oracle refused") {
			t.Errorf("lint must refuse %s; got:\n%s", id, joined)
		}
	}
	for _, id := range []string{"RG-1", "RG-2", "RG-3", "RG-4", "RG-5", "RG-6", "RG-7", "RG-14"} {
		if strings.Contains(joined, id+": ledger oracle refused") {
			t.Errorf("lint must not refuse a sound contract %s (its truth is verify's job); got:\n%s", id, joined)
		}
	}
}

// dirEvidence pairs an empty index with DirFiles, so the ledger oracle's
// golden fixtures are read as real committed files (testdata/ledger/) rather
// than the in-memory memFiles map lexical_test.go's fixtures use.
type dirEvidence struct {
	noEvidence
	dir DirFiles
}

func (e dirEvidence) ReadRepoFile(p string) ([]byte, error) { return e.dir.ReadRepoFile(p) }
