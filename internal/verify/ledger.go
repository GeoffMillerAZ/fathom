package verify

// ledger.go — the ledger oracle (ADR-034 decision 2, docs/plans/receipts.md
// RC-34). A plan row that claims a published bench reading — not a code
// symbol — may carry `{oracle: ledger, path: <repo path to a JSONL bench
// ledger>, run_id: <id>, task: <task id>, arm: <arm name>,
// min_trials: N}`. The row verifies when the ledger row it names:
//
//   - exists (matched on run_id + task + arm; when task and arm are both
//     left unset, matched on run_id alone against a ledger row that ALSO
//     carries no task_id/arm — the home bench's one-row-per-run shape,
//     docs/bench/ledger.jsonl);
//   - is not flagged `surface_unused: true`;
//   - carries no `instrument_defect` field; and
//   - ran at least min_trials (default 1) trials.
//
// Each condition names itself in the failure text, so a voided or
// under-powered reading cannot stand in as a claim's proof. Modeled on
// lexical.go's evalLexicalOracle/lintLexicalOracle split, and on the same
// "contract error is pure and shared" discipline.

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// ledgerCriterion is the criterion name a ledger row reports under.
const ledgerCriterion = "ledger_oracle"

// ledgerRow is the minimal shape read out of a committed bench ledger.
// Deliberately loose: encoding/json ignores every field this oracle does
// not name, so a line from scenariobench.ScenarioLedgerRow (readings,
// borrowed_cells, prediction, …) and a line from contextbench.LedgerRow
// (evaluation, per_task, gold_source_rate, …) both parse without error —
// this oracle never imports either package's own row type, and stays
// tolerant of fields added to either shape later (RC-34's instruction to
// keep the loader tolerant of unknown fields).
type ledgerRow struct {
	RunID  string `json:"run_id"`
	TaskID string `json:"task_id"`
	Arm    string `json:"arm"`

	// Trials is the scenario ledger's own per-(run,task,arm) trial count.
	// MinTrials is the home ledger's own per-cell replication floor (the
	// thinnest cell any run measured, contextbench.PairedSummary.MinTrials)
	// — the number this oracle must compare a claimed floor against for a
	// home row, never TreatmentTrials (tasks × trials, the run TOTAL: 33
	// successes-out-of on an 11-task run at n=3 says nothing about
	// replication and passes almost any floor by sheer task count).
	Trials    int `json:"trials"`
	MinTrials int `json:"min_trials"`

	SurfaceUnused    bool   `json:"surface_unused"`
	InstrumentDefect string `json:"instrument_defect"`

	// Evaluation is the criterion verdict the row was written with — a
	// home row's own registered-criterion judgment (contextbench.
	// Evaluation) or a scenario row's per-run ScenarioEvaluation. Read
	// here so a row whose own bench run failed or could not decide never
	// certifies a plan claim by existing and clearing the trial floor
	// alone.
	Evaluation struct {
		CriterionID string `json:"criterion_id"`
		Verdict     string `json:"verdict"`
		Reason      string `json:"reason"`
	} `json:"evaluation"`
}

// trialCount is the row's own reading of "how many trials did this
// measure, per cell" — Trials when the row carries one (a scenario ledger
// row, its own per-(task,arm) count), else MinTrials (a home ledger row's
// per-cell replication floor — never the run-total treatment_trials,
// which inflates with task count and says nothing about replication).
func (r ledgerRow) trialCount() int {
	if r.Trials > 0 {
		return r.Trials
	}
	return r.MinTrials
}

// ledgerContractError returns the static reason a ledger row's contract is
// malformed, or "" when its shape is sound. Pure; shared by lintLedgerOracle
// and ledgerOracle so the two can never disagree about what is refused.
func ledgerContractError(t Task) string {
	switch {
	case t.Oracle != "ledger":
		return fmt.Sprintf("unknown oracle %q (want ledger)", t.Oracle)
	case t.Expect.Symbol != "":
		return "one contract per row: symbol and oracle both set"
	case t.OraclePath == "":
		return "ledger oracle without a path"
	case filepath.IsAbs(t.OraclePath) || strings.HasPrefix(t.OraclePath, "../") || strings.Contains(t.OraclePath, "/../") || t.OraclePath == "..":
		return fmt.Sprintf("path %q must be repo-relative", t.OraclePath)
	case t.OracleRunID == "":
		return "ledger oracle without a run_id"
	case t.OracleArm != "" && t.OracleTaskID == "":
		return "ledger oracle names an arm without a task"
	}
	if t.OracleMinTrials != "" {
		if n, err := strconv.Atoi(t.OracleMinTrials); err != nil || n < 0 {
			return fmt.Sprintf("min_trials %q must be a non-negative integer", t.OracleMinTrials)
		}
	}
	return ""
}

// ledgerMinTrials returns the row's min_trials floor: the annotated value,
// or 1 when unset. Called only after ledgerContractError has passed, so the
// Atoi it repeats can never fail here.
func ledgerMinTrials(t Task) int {
	if t.OracleMinTrials == "" {
		return 1
	}
	n, err := strconv.Atoi(t.OracleMinTrials)
	if err != nil {
		return 1
	}
	return n
}

// lintLedgerOracle reports the contract-shape refusals for one ledger row,
// in the same words ledgerOracle fails with.
func lintLedgerOracle(t Task) []string {
	if t.Oracle == "" {
		return nil
	}
	if msg := ledgerContractError(t); msg != "" {
		return []string{fmt.Sprintf("%s: ledger oracle refused — %s", t.ID, msg)}
	}
	return nil
}

// ledgerOracle evaluates one ledger row against files and returns the
// criterion plus the verdict label a failure or unknown carries. files ==
// nil is Unknown with the reason, never a pass and never a fail: the
// contract is sound, the instrument simply cannot read the tree from here.
func ledgerOracle(t Task, files FileEvidence) (Criterion, string) {
	if msg := ledgerContractError(t); msg != "" {
		return Criterion{ledgerCriterion, Fail, msg}, "contract-invalid"
	}
	if files == nil {
		return Criterion{ledgerCriterion, Unknown, "no repo file access on this evidence source — run fathom verify from the repo (working tree) to evaluate the ledger oracle"}, "oracle-unevaluated"
	}
	body, err := files.ReadRepoFile(t.OraclePath)
	if err != nil {
		return Criterion{ledgerCriterion, Fail, fmt.Sprintf("path %q not readable in the working tree: %v", t.OraclePath, err)}, "path-absent"
	}

	homeShaped := t.OracleTaskID == "" && t.OracleArm == ""
	var match *ledgerRow
	var homeMatches []ledgerRow
	lineNo := 0
	for _, line := range strings.Split(string(body), "\n") {
		lineNo++
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var r ledgerRow
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return Criterion{ledgerCriterion, Fail, fmt.Sprintf("%s line %d is not valid JSON: %v", t.OraclePath, lineNo, err)}, "ledger-unparseable"
		}
		if r.RunID != t.OracleRunID {
			continue
		}
		if homeShaped {
			// Matched on run_id alone (the home bench's one-row-per-run
			// ledger): the row itself must ALSO carry no task_id/arm, so a
			// scenario-shaped row that happens to share the run_id string
			// is never mistaken for the row this claim named. Every such
			// row is collected — a run_id scored under two registered
			// criteria (e.g. H2-1 and H2-3) writes one row each, and
			// silently taking "the last one" would let the claim pick
			// whichever verdict happened to be written second.
			if r.TaskID != "" || r.Arm != "" {
				continue
			}
			homeMatches = append(homeMatches, r)
			continue
		}
		if r.TaskID != t.OracleTaskID || r.Arm != t.OracleArm {
			continue
		}
		rr := r
		match = &rr // last match in file order wins: the freshest reading
	}

	if homeShaped {
		switch len(homeMatches) {
		case 0:
			// match stays nil; falls through to the absent-row report below.
		case 1:
			match = &homeMatches[0]
		default:
			if t.OracleCriterion == "" {
				return Criterion{ledgerCriterion, Fail, fmt.Sprintf("run_id=%s is ambiguous in %s: %d rows match (criteria: %s) — add criterion: <id> to pick one", t.OracleRunID, t.OraclePath, len(homeMatches), criterionIDs(homeMatches))}, "ambiguous-run-id"
			}
			var picked []ledgerRow
			for _, r := range homeMatches {
				if r.Evaluation.CriterionID == t.OracleCriterion {
					picked = append(picked, r)
				}
			}
			switch len(picked) {
			case 0:
				return Criterion{ledgerCriterion, Fail, fmt.Sprintf("run_id=%s in %s has no row for criterion %q — rows present: %s", t.OracleRunID, t.OraclePath, t.OracleCriterion, criterionIDs(homeMatches))}, "ledger-row-absent"
			case 1:
				match = &picked[0]
			default:
				return Criterion{ledgerCriterion, Fail, fmt.Sprintf("run_id=%s criterion=%s is still ambiguous in %s: %d rows match", t.OracleRunID, t.OracleCriterion, t.OraclePath, len(picked))}, "ambiguous-run-id"
			}
		}
	}

	if match == nil {
		want := fmt.Sprintf("run_id=%s", t.OracleRunID)
		if t.OracleTaskID != "" || t.OracleArm != "" {
			want = fmt.Sprintf("run_id=%s task=%s arm=%s", t.OracleRunID, t.OracleTaskID, t.OracleArm)
		}
		return Criterion{ledgerCriterion, Fail, fmt.Sprintf("no ledger row for %s in %s", want, t.OraclePath)}, "ledger-row-absent"
	}
	rowName := fmt.Sprintf("%s/%s/%s", match.RunID, match.TaskID, match.Arm)
	if match.SurfaceUnused {
		return Criterion{ledgerCriterion, Fail, fmt.Sprintf("ledger row %s is flagged surface_unused: true", rowName)}, "surface-unused"
	}
	if match.InstrumentDefect != "" {
		return Criterion{ledgerCriterion, Fail, fmt.Sprintf("ledger row %s carries instrument_defect: %q — voided, not proof", rowName, match.InstrumentDefect)}, "instrument-defect"
	}
	if verdict := match.Evaluation.Verdict; (verdict == "fail" || verdict == "indeterminate") && t.OracleAcceptVerdict != "any" {
		return Criterion{ledgerCriterion, Fail, fmt.Sprintf("ledger row %s carries evaluation.verdict=%s: %s — add accept_verdict: any to certify despite it", rowName, verdict, match.Evaluation.Reason)}, "verdict-" + verdict
	}
	minTrials := ledgerMinTrials(t)
	if trials := match.trialCount(); trials < minTrials {
		return Criterion{ledgerCriterion, Fail, fmt.Sprintf("ledger row %s ran %d trial(s), below the required floor of %d", rowName, trials, minTrials)}, "below-trial-floor"
	}
	return Criterion{ledgerCriterion, Pass, fmt.Sprintf("ledger row %s: %d trial(s) (>= required %d), not voided", rowName, match.trialCount(), minTrials)}, ""
}

// criterionIDs renders the criterion_id each of rows carries, in file
// order, for an ambiguity message — "(unset)" for a row written before
// evaluations carried one, never a blank that reads as a typo.
func criterionIDs(rows []ledgerRow) string {
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		id := r.Evaluation.CriterionID
		if id == "" {
			id = "(unset)"
		}
		ids = append(ids, id)
	}
	return strings.Join(ids, ", ")
}
