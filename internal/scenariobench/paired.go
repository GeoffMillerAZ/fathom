// paired.go is wave N15 item 1's own addition: the scenario ledger's
// verdict block reported a treatment/control comparison (renderRunVerdict,
// verdict.go) but never the paired statistics the bench-confidence layer
// already computes for the home bench (internal/contextbench's own
// Pair/ComputeReliability, ADR-035) — a task-clustered percentile
// bootstrap over agentic-kit's reliability-stats primitive
// (PairedBootstrapCI), the SAME function contextbench already calls.
//
// Unlike contextbench's own ArmResult (one row per trial), a
// ScenarioLedgerRow is already the per-(task, arm) AGGREGATE over every
// trial (Trials/Successes summed, Readings.TotalTokens summed —
// AggregateRowReadings), so this file needs none of contextbench's own
// StatsByTaskArm grouping step: one row per (task, arm) IS one cluster's
// own summary already.
package scenariobench

import (
	"fmt"
	"sort"

	reliabilitystats "github.com/geoffmilleraz/agentic-kit/foundations/quality/reliability-stats"
)

// PairedRunSeed / PairedRunResamples are the scenario ledger's own paired
// bootstrap parameters. The seed mirrors contextbench's own
// DefaultBootstrapSeed convention (dated to when the figure shipped,
// rather than a bare magic number); the resample count is higher than
// contextbench's own default (2000) because a run with many exact ties
// (most LCA tasks tie 3/3 vs 3/3 on success) needs more replicates before
// the reported quantiles stop moving noticeably between reruns — verified
// against the operator's own independent bootstrap on lca-run-1's 39
// paired tasks (wave N15 item 1).
const (
	PairedRunSeed      int64 = 20260902
	PairedRunResamples       = 20000
)

// pairedRunLevel is the interval's confidence level — 95%, matching
// contextbench's own reliabilityLevel; not configurable for the same
// reason: a level a corpus could lower is a bar a corpus could lower.
const pairedRunLevel = 0.95

// minPairedTasks is evaluatePaired's own floor (wave N17) on how many
// tasks must actually pair across treatment and control before the
// bootstrap's own interval is trusted for a verdict — distinct from (and
// checked after) Criterion.MinTrials's floor on treatment TRIALS. Below
// it, a run has not measured enough paired evidence to decide either way,
// mirroring the unpaired branch's own MinTrials indeterminate.
const minPairedTasks = 3

// PairedRunStats is one run's paired bootstrap summary over its own sound
// (non-voided) rows: how many tasks paired across treatment and control,
// the mean per-task success-rate difference with its 95% CI, and the mean
// per-task token ratio with its own 95% CI.
type PairedRunStats struct {
	Treatment string
	Control   string
	// Tasks is the number of tasks present (with Trials > 0) in BOTH the
	// treatment and control arm — the paired-bootstrap's own cluster
	// count for SuccessDiff.
	Tasks int
	// SuccessDiff is the mean (over paired tasks) of each task's own
	// success-rate difference (treatment minus control), with its
	// bootstrap 95% CI.
	SuccessDiff reliabilitystats.Interval
	// TokenRatio is the mean (over paired tasks) of each task's own
	// per-trial token ratio (treatment / control), with its bootstrap 95%
	// CI. Zero-valued when TokenRatioTasks is 0 (no paired task recorded
	// tokens on both arms — a control-arm reading of 0 excludes that task
	// from the ratio the same way contextbench's own Pair does).
	TokenRatio reliabilitystats.Interval
	// TokenRatioTasks is the paired-task count TokenRatio was actually
	// bootstrapped over — never assumed equal to Tasks.
	TokenRatioTasks int
}

// ComputePairedRunStats pairs sound (InstrumentDefect == "", Trials > 0)
// rows by task id across treatment and control, then bootstraps the mean
// per-task success-rate difference and token ratio
// (reliabilitystats.PairedBootstrapCI) — task-clustered, so a task's own
// single aggregated ledger row is one unit of resampling evidence. A task
// present in only one arm is simply not paired (never an error on its
// own); an error is returned only when NO task pairs at all, since there
// is then nothing to resample.
func ComputePairedRunStats(rows []ScenarioLedgerRow, treatment, control string) (PairedRunStats, error) {
	byTask := map[string]map[string]ScenarioLedgerRow{}
	for _, r := range rows {
		if r.InstrumentDefect != "" || r.Trials == 0 {
			continue
		}
		if r.Arm != treatment && r.Arm != control {
			continue
		}
		if byTask[r.TaskID] == nil {
			byTask[r.TaskID] = map[string]ScenarioLedgerRow{}
		}
		byTask[r.TaskID][r.Arm] = r
	}
	taskIDs := make([]string, 0, len(byTask))
	for id := range byTask {
		taskIDs = append(taskIDs, id)
	}
	sort.Strings(taskIDs)

	var diffObs, ratioObs []reliabilitystats.Observation
	for _, id := range taskIDs {
		arms := byTask[id]
		t, tok := arms[treatment]
		c, cok := arms[control]
		if !tok || !cok {
			continue
		}
		tRate := float64(t.Successes) / float64(t.Trials)
		cRate := float64(c.Successes) / float64(c.Trials)
		diffObs = append(diffObs, reliabilitystats.Observation{Task: id, Diff: tRate - cRate})

		cTok := float64(c.Readings.TotalTokens) / float64(c.Trials)
		if cTok > 0 {
			tTok := float64(t.Readings.TotalTokens) / float64(t.Trials)
			ratioObs = append(ratioObs, reliabilitystats.Observation{Task: id, Diff: tTok / cTok})
		}
	}
	if len(diffObs) == 0 {
		return PairedRunStats{}, fmt.Errorf("computing paired run stats: no task paired across %q and %q", treatment, control)
	}

	out := PairedRunStats{Treatment: treatment, Control: control, Tasks: len(diffObs)}
	ci, err := reliabilitystats.PairedBootstrapCI(diffObs, PairedRunSeed, PairedRunResamples, pairedRunLevel)
	if err != nil {
		return PairedRunStats{}, fmt.Errorf("computing paired run stats: success-diff bootstrap: %w", err)
	}
	out.SuccessDiff = ci

	if len(ratioObs) > 0 {
		rci, err := reliabilitystats.PairedBootstrapCI(ratioObs, PairedRunSeed, PairedRunResamples, pairedRunLevel)
		if err != nil {
			return PairedRunStats{}, fmt.Errorf("computing paired run stats: token-ratio bootstrap: %w", err)
		}
		out.TokenRatio = rci
		out.TokenRatioTasks = len(ratioObs)
	}
	return out, nil
}

// RenderPairedRunStats renders one verdict-block line: paired task count,
// the mean success-rate diff with its 95% CI, and (when measured) the
// mean token ratio with its own 95% CI.
func RenderPairedRunStats(s PairedRunStats) string {
	line := fmt.Sprintf("Paired (bootstrap, n=%d, seed %d, %d resamples): success-rate diff %+.3f [%+.3f, %+.3f] (95%% CI)",
		s.Tasks, PairedRunSeed, PairedRunResamples, s.SuccessDiff.Point, s.SuccessDiff.Low, s.SuccessDiff.High)
	if s.TokenRatioTasks > 0 {
		line += fmt.Sprintf("; token ratio %.2f× [%.2f×, %.2f×]", s.TokenRatio.Point, s.TokenRatio.Low, s.TokenRatio.High)
	}
	return line + ".\n"
}

// evaluatePaired is Criterion.Evaluate's own branch for a Paired criterion
// (wave N17): VB-27's registered rule (docs/vision/29-verbs-not-lookups.md
// §5) is "a confidence interval that includes zero publishes the null",
// which the unpaired treatment>=control fallback cannot express — LCA's
// own criterion registers neither RequireSuccessAtLeastEqual nor
// MaxTokensPerSuccessRatio (lca.go's own doc comment: "a bare
// Criterion.Evaluate pass/fail is not that confidence interval"), so that
// fallback always reported "pass" once MinTrials was met, contradicting
// the pre-registered rule.
//
// Order: c.MinTrials still floors the treatment's own trial count first
// (the same floor the unpaired branch checks, so a run that measured
// almost nothing reports the same indeterminate reason either way); then
// ComputePairedRunStats pairs rows by task id, and minPairedTasks floors
// how many tasks actually paired — a bootstrap over one or two clustered
// tasks is not enough to trust the interval it reports. Only then does the
// success-rate diff's own 95% CI decide: including zero is null, excluding
// it above zero is pass (treatment's favour), excluding it below zero is
// fail.
func (c Criterion) evaluatePaired(rows []ScenarioLedgerRow) ScenarioEvaluation {
	ev := ScenarioEvaluation{CriterionID: c.ID}

	var tTrials int
	for _, r := range rows {
		if r.InstrumentDefect != "" {
			continue
		}
		if r.Arm == c.Treatment {
			tTrials += r.Trials
		}
	}
	if tTrials < c.MinTrials {
		ev.Verdict = ScenarioVerdictIndeterminate
		ev.Reason = fmt.Sprintf("treatment %q ran %d trial(s), below the registered floor of %d", c.Treatment, tTrials, c.MinTrials)
		return ev
	}

	stats, err := ComputePairedRunStats(rows, c.Treatment, c.Control)
	if err != nil {
		ev.Verdict = ScenarioVerdictIndeterminate
		ev.Reason = fmt.Sprintf("paired bootstrap unavailable: %s", err)
		return ev
	}
	if stats.Tasks < minPairedTasks {
		ev.Verdict = ScenarioVerdictIndeterminate
		ev.Reason = fmt.Sprintf("only %d task(s) paired across %q and %q, below the floor of %d", stats.Tasks, c.Treatment, c.Control, minPairedTasks)
		return ev
	}

	diff := stats.SuccessDiff
	switch {
	case diff.Low <= 0 && diff.High >= 0:
		ev.Verdict = ScenarioVerdictNull
		ev.Reason = fmt.Sprintf("paired 95%% CI of the success-rate difference includes zero: %+.3f [%+.3f, %+.3f]; the null is published", diff.Point, diff.Low, diff.High)
	case diff.Low > 0:
		ev.Verdict = ScenarioVerdictPass
		ev.Reason = fmt.Sprintf("paired 95%% CI of the success-rate difference excludes zero in the treatment's favour: %+.3f [%+.3f, %+.3f]", diff.Point, diff.Low, diff.High)
	default:
		ev.Verdict = ScenarioVerdictFail
		ev.Reason = fmt.Sprintf("paired 95%% CI of the success-rate difference excludes zero against the treatment: %+.3f [%+.3f, %+.3f]", diff.Point, diff.Low, diff.High)
	}
	return ev
}
