package scenariobench

import (
	"fmt"
	"sort"
)

// DryRunTaskVerdict is the instrument's discrimination check, never a tool-effect claim.
type DryRunTaskVerdict struct {
	TaskID string `json:"task_id"`
	Keep   bool   `json:"keep"`
	Reason string `json:"reason,omitempty"`
}

// dryRunControlVersions are the zero-turn control harnesses BX-36 reads. They
// share a run id by design, so the cell store keeps each one beside the others.
var dryRunControlVersions = map[string]bool{"null-1": true, "oracle-1": true, "wrong-patch-1": true}

// DryRunVerdict requires exactly three sound trials of each control for every
// task and corpus arm. It consumes stored cells without running any harness.
// Incomplete, duplicate or void evidence is an error, never a kept task.
// Flakiness takes precedence over stable failures of the expected discrimination.
func DryRunVerdict(corpus Corpus, cells []CellResult) ([]DryRunTaskVerdict, error) {
	versions := dryRunControlVersions
	type key struct{ task, arm, version string }
	grouped := map[key]map[int]CellResult{}
	tasks := map[string]bool{}
	for _, task := range corpus.Tasks {
		if task.Disposition != "dropped" {
			tasks[task.ID] = true
		}
	}
	for _, cell := range cells {
		if !tasks[cell.TaskID] || !versions[cell.HarnessVersion] {
			continue
		}
		if _, ok := corpus.Arms[cell.Arm]; !ok {
			continue
		}
		k := key{cell.TaskID, cell.Arm, cell.HarnessVersion}
		if grouped[k] == nil {
			grouped[k] = map[int]CellResult{}
		}
		if _, duplicate := grouped[k][cell.Trial]; duplicate {
			return nil, fmt.Errorf("dryrun %s/%s/%s: duplicate trial %d", k.task, k.arm, k.version, cell.Trial)
		}
		if cell.Trial < 1 || cell.Trial > 3 {
			return nil, fmt.Errorf("dryrun %s/%s/%s: expected trials 1 to 3", k.task, k.arm, k.version)
		}
		grouped[k][cell.Trial] = cell
	}
	ids := make([]string, 0, len(tasks))
	for id := range tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	arms := make([]string, 0, len(corpus.Arms))
	for arm := range corpus.Arms {
		arms = append(arms, arm)
	}
	sort.Strings(arms)
	if len(ids) == 0 || len(arms) == 0 {
		return nil, fmt.Errorf("dryrun requires selected tasks and arms")
	}
	verdicts := make([]DryRunTaskVerdict, 0, len(ids))
	for _, id := range ids {
		flaky, nullPassed, oracleFailed, wrongPassed := false, false, false, false
		for _, arm := range arms {
			for _, version := range []string{"null-1", "oracle-1", "wrong-patch-1"} {
				trials := grouped[key{id, arm, version}]
				if len(trials) != 3 {
					return nil, fmt.Errorf("dryrun %s/%s/%s: need 3 trials, got %d", id, arm, version, len(trials))
				}
				for trial := 1; trial <= 3; trial++ {
					cell := trials[trial]
					reason := cell.InstrumentDefect
					if reason == "" {
						reason = offlineControlDefect(cell)
					}
					if reason == "" {
						reason = CellLost(cell.Sessions, NewNullHarness())
					}
					if reason == "" && cell.Oracle.Calibration != nil {
						reason = cell.Oracle.Calibration.NotMeasurable
					}
					if reason == "" && cell.Oracle.Localization != nil {
						reason = cell.Oracle.Localization.NotMeasurable
					}
					if reason != "" {
						return nil, fmt.Errorf("dryrun %s/%s/%s/t%d: void: %s", id, arm, version, trial, reason)
					}
					if cell.Success != trials[1].Success {
						flaky = true
					}
					switch version {
					case "null-1":
						nullPassed = nullPassed || cell.Success
					case "oracle-1":
						oracleFailed = oracleFailed || !cell.Success
					case "wrong-patch-1":
						wrongPassed = wrongPassed || cell.Success
					}
				}
			}
		}
		reason := ""
		switch {
		case flaky:
			reason = "flaky"
		case nullPassed:
			reason = "null passed"
		case oracleFailed:
			reason = "oracle failed"
		case wrongPassed:
			reason = "wrong patch passed"
		}
		verdicts = append(verdicts, DryRunTaskVerdict{TaskID: id, Keep: reason == "", Reason: reason})
	}
	return verdicts, nil
}

// offlineControlDefect checks totals and sessions independently so inconsistent
// stored aggregates cannot make an agent-backed cell look like a free control.
func offlineControlDefect(cell CellResult) string {
	if cell.Readings.TotalTurns > 0 {
		return "control has positive total turns"
	}
	if cell.Readings.TotalCostUSD > 0 {
		return "control has positive total cost"
	}
	for _, session := range cell.Sessions {
		if session.Turns > 0 {
			return "control has positive session turns"
		}
		if session.CostUSD > 0 {
			return "control has positive session cost"
		}
	}
	return ""
}
