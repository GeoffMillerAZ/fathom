package contextbench

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
)

// This file adds the two things a single-trial, totals-only bench cannot
// give: repetition (so a task's variance is visible instead of assumed
// away) and pairing (so one pathological task cannot silently decide the
// verdict for the whole corpus).
//
// The 2026-07-24 home-bench run is the worked example. At n=1 per task per
// arm, fathom was cheaper on 8 of 13 tasks yet lost the aggregate ratio
// 1.071× because five tasks — one of them at 5.20× — erased the win. The
// audit that followed had to reason by hand about which perturbations were
// individually defensible. StatsByTaskArm and Pair make that reasoning a
// computation: per-task ratios, a median that an outlier cannot move, and
// an explicit count of which arm was cheaper where.
//
// Honesty floor (ADR-027, I38): a task that is missing from one arm is
// reported in Absent, never dropped. Dropping it would let a crashed or
// skipped run improve the surviving ratio — an instrument degrading toward
// "healthy", which is exactly the failure ADR-027 forbids. Criterion.Evaluate
// refuses to return a pass while Absent is non-empty.

// TaskArmStat aggregates one (task, arm) cell across however many trials
// the runner executed for it.
type TaskArmStat struct {
	TaskID      string  `json:"task_id"`
	Arm         string  `json:"arm"`
	Trials      int     `json:"trials"`
	Successes   int     `json:"successes"`
	SuccessRate float64 `json:"success_rate"`

	// Flaky is true when the cell neither always passed nor always failed.
	// A flaky cell is the signal that n=1 was hiding something: at one
	// trial per cell it is unreachable by construction.
	Flaky bool `json:"flaky"`

	MeanTokens   float64 `json:"mean_tokens"`
	MedianTokens float64 `json:"median_tokens"`
	// StdDevTokens is the population standard deviation (the trials are the
	// whole population of runs performed, not a sample drawn from a larger
	// set). It is 0 for a single trial — reported as 0, never as absent, so
	// a reader can tell "no spread measured" from "spread measured as zero"
	// by looking at Trials.
	StdDevTokens float64 `json:"stddev_tokens"`
	MeanWallMS   float64 `json:"mean_wall_ms"`

	// BorrowedTrials counts the cell's trials that were borrowed from an
	// earlier run rather than paid for by this one (BP-4).
	BorrowedTrials int `json:"borrowed_trials,omitempty"`

	// MeanPayloadBytes, MeanCostUSD and MeanToolSearchTurns carry the H2-3
	// cost half (BP-9/BP-10). Payload is the tool-result bytes returned to
	// the model — the discovery cost fathom controls, as opposed to
	// total_tokens, which O26 showed to be cache-read x turns.
	MeanPayloadBytes    float64 `json:"mean_payload_bytes,omitempty"`
	MeanCostUSD         float64 `json:"mean_cost_usd,omitempty"`
	MeanToolSearchTurns float64 `json:"mean_toolsearch_turns,omitempty"`
}

// StatsByTaskArm collapses raw per-trial results into one cell per
// (task, arm), sorted by task id then arm — the determinism contract, so
// the same results in any file order render the same table.
func StatsByTaskArm(results []ArmResult) []TaskArmStat {
	type key struct{ task, arm string }
	grouped := map[key][]ArmResult{}
	for _, r := range results {
		k := key{r.TaskID, r.Arm}
		grouped[k] = append(grouped[k], r)
	}

	keys := make([]key, 0, len(grouped))
	for k := range grouped {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].task != keys[j].task {
			return keys[i].task < keys[j].task
		}
		return keys[i].arm < keys[j].arm
	})

	out := make([]TaskArmStat, 0, len(keys))
	for _, k := range keys {
		rows := grouped[k]
		st := TaskArmStat{TaskID: k.task, Arm: k.arm, Trials: len(rows)}
		tokens := make([]float64, 0, len(rows))
		var sumTokens, sumWall, sumPayload, sumCost, sumToolSearch float64
		for _, r := range rows {
			if r.Success {
				st.Successes++
			}
			if r.Borrowed {
				st.BorrowedTrials++
			}
			tokens = append(tokens, float64(r.TotalTokens))
			sumTokens += float64(r.TotalTokens)
			sumWall += float64(r.WallTimeMS)
			sumPayload += float64(r.PayloadBytes)
			sumCost += r.CostUSD
			sumToolSearch += float64(r.ToolSearchTurns)
		}
		st.MeanPayloadBytes = sumPayload / float64(st.Trials)
		st.MeanCostUSD = sumCost / float64(st.Trials)
		st.MeanToolSearchTurns = sumToolSearch / float64(st.Trials)
		st.SuccessRate = float64(st.Successes) / float64(st.Trials)
		st.Flaky = st.Successes > 0 && st.Successes < st.Trials
		st.MeanTokens = sumTokens / float64(st.Trials)
		st.MeanWallMS = sumWall / float64(st.Trials)
		st.MedianTokens = median(tokens)
		var sq float64
		for _, v := range tokens {
			sq += (v - st.MeanTokens) * (v - st.MeanTokens)
		}
		st.StdDevTokens = math.Sqrt(sq / float64(st.Trials))
		out = append(out, st)
	}
	return out
}

// median returns the middle value of vals, averaging the two middle values
// for an even count. vals is copied before sorting so the caller's slice is
// untouched. An empty slice yields 0.
func median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	s := append([]float64(nil), vals...)
	sort.Float64s(s)
	mid := len(s) / 2
	if len(s)%2 == 1 {
		return s[mid]
	}
	return (s[mid-1] + s[mid]) / 2
}

// PairedTask is one task compared head-to-head between two arms.
type PairedTask struct {
	TaskID              string  `json:"task_id"`
	TreatmentTrials     int     `json:"treatment_trials"`
	ControlTrials       int     `json:"control_trials"`
	TreatmentSuccesses  int     `json:"treatment_successes"`
	ControlSuccesses    int     `json:"control_successes"`
	TreatmentMeanTokens float64 `json:"treatment_mean_tokens"`
	ControlMeanTokens   float64 `json:"control_mean_tokens"`
	// TokenRatio is treatment mean ÷ control mean. It is 0 when the control
	// arm burned no tokens at all (an impossible-in-practice reading that
	// would otherwise divide by zero); the row is then excluded from the
	// median, and its exclusion is visible because Tasks counts it.
	TokenRatio float64 `json:"token_ratio"`
	Flaky      bool    `json:"flaky"`

	// PayloadRatio and CostRatio are H2-3's cost half per task (BP-10),
	// each 0 when the control arm recorded none of that quantity.
	PayloadRatio float64 `json:"payload_ratio,omitempty"`
	CostRatio    float64 `json:"cost_ratio,omitempty"`
}

// PairedSummary is the head-to-head verdict material for one run.
type PairedSummary struct {
	Treatment string `json:"treatment"`
	Control   string `json:"control"`

	// Tasks counts fully paired tasks — present in both arms. Absent lists
	// task ids that appeared in one arm only.
	Tasks  int      `json:"tasks"`
	Absent []string `json:"absent,omitempty"`

	// MinTrials is the smallest trial count in any compared cell: the
	// honest n for the run, since a corpus is only as replicated as its
	// thinnest cell.
	MinTrials int `json:"min_trials"`

	TreatmentSuccesses int     `json:"treatment_successes"`
	ControlSuccesses   int     `json:"control_successes"`
	TreatmentTrials    int     `json:"treatment_trials"`
	ControlTrials      int     `json:"control_trials"`
	TreatmentRate      float64 `json:"treatment_success_rate"`
	ControlRate        float64 `json:"control_success_rate"`

	TreatmentTokens int64 `json:"treatment_tokens"`
	ControlTokens   int64 `json:"control_tokens"`
	// AggregateTokenRatio is the totals-over-totals ratio the pre-registered
	// criteria are written against. MedianTokenRatio is the middle per-task
	// ratio, reported alongside it for interpretation only — never as the
	// gate, or a criterion could be loosened after seeing the data (law 12).
	AggregateTokenRatio float64 `json:"aggregate_token_ratio"`
	MedianTokenRatio    float64 `json:"median_token_ratio"`

	TasksTreatmentCheaper int `json:"tasks_treatment_cheaper"`
	TasksControlCheaper   int `json:"tasks_control_cheaper"`
	TasksTiedOnTokens     int `json:"tasks_tied_on_tokens"`

	FlakyTasks []string     `json:"flaky_tasks,omitempty"`
	PerTask    []PairedTask `json:"per_task"`

	// Scope says what the run selected (BP-8): ScopeFull is the whole
	// corpus, ScopeTargeted only the tasks a diff selected. Set by the
	// caller from the run's own flags, since the results themselves cannot
	// know what was left out.
	Scope string `json:"scope,omitempty"`

	// Cells counts every compared trial across both arms; BorrowedCells
	// counts those that came from an earlier run's key-matched cell
	// (BP-4/BP-5). A row whose cells are all borrowed measured nothing
	// new, and Evaluate says so.
	Cells         int `json:"cells,omitempty"`
	BorrowedCells int `json:"borrowed_cells,omitempty"`

	// H2-3's cost half (BP-10), computed exactly like AggregateTokenRatio:
	// totals per trial, treatment over control. Each is 0 when the control
	// arm recorded none of that quantity — e.g. a run scored without
	// --stream transcripts records no payload at all, which Evaluate
	// reports as indeterminate rather than as a ratio of zero.
	TreatmentPayloadBytes  int64   `json:"treatment_payload_bytes,omitempty"`
	ControlPayloadBytes    int64   `json:"control_payload_bytes,omitempty"`
	AggregatePayloadRatio  float64 `json:"aggregate_payload_ratio,omitempty"`
	TreatmentCostUSD       float64 `json:"treatment_cost_usd,omitempty"`
	ControlCostUSD         float64 `json:"control_cost_usd,omitempty"`
	AggregateCostRatio     float64 `json:"aggregate_cost_ratio,omitempty"`
	TreatmentToolSearchAvg float64 `json:"treatment_toolsearch_turns_mean,omitempty"`
	ControlToolSearchAvg   float64 `json:"control_toolsearch_turns_mean,omitempty"`
}

// Scope values a run may declare (BP-8).
const (
	ScopeFull     = "full"
	ScopeTargeted = "targeted"
)

// Pair compares two named arms over results. Only tasks present in both
// arms are compared; the rest are named in Absent (see the honesty note at
// the top of this file). Output ordering is by task id throughout.
func Pair(results []ArmResult, treatment, control string) PairedSummary {
	cells := map[string]map[string]TaskArmStat{}
	for _, st := range StatsByTaskArm(results) {
		if cells[st.TaskID] == nil {
			cells[st.TaskID] = map[string]TaskArmStat{}
		}
		cells[st.TaskID][st.Arm] = st
	}

	ids := make([]string, 0, len(cells))
	for id := range cells {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	p := PairedSummary{Treatment: treatment, Control: control}
	var ratios []float64
	for _, id := range ids {
		tc, tok := cells[id][treatment]
		cc, cok := cells[id][control]
		if !tok || !cok {
			// Present in one arm only: named, not dropped.
			if tok || cok {
				p.Absent = append(p.Absent, id)
			}
			continue
		}
		p.Tasks++
		if p.MinTrials == 0 || tc.Trials < p.MinTrials {
			p.MinTrials = tc.Trials
		}
		if cc.Trials < p.MinTrials {
			p.MinTrials = cc.Trials
		}

		p.Cells += tc.Trials + cc.Trials
		p.BorrowedCells += tc.BorrowedTrials + cc.BorrowedTrials

		row := PairedTask{
			TaskID:              id,
			TreatmentTrials:     tc.Trials,
			ControlTrials:       cc.Trials,
			TreatmentSuccesses:  tc.Successes,
			ControlSuccesses:    cc.Successes,
			TreatmentMeanTokens: tc.MeanTokens,
			ControlMeanTokens:   cc.MeanTokens,
			Flaky:               tc.Flaky || cc.Flaky,
		}
		if cc.MeanPayloadBytes > 0 {
			row.PayloadRatio = tc.MeanPayloadBytes / cc.MeanPayloadBytes
		}
		if cc.MeanCostUSD > 0 {
			row.CostRatio = tc.MeanCostUSD / cc.MeanCostUSD
		}
		if cc.MeanTokens > 0 {
			row.TokenRatio = tc.MeanTokens / cc.MeanTokens
			ratios = append(ratios, row.TokenRatio)
			switch {
			case tc.MeanTokens < cc.MeanTokens:
				p.TasksTreatmentCheaper++
			case tc.MeanTokens > cc.MeanTokens:
				p.TasksControlCheaper++
			default:
				p.TasksTiedOnTokens++
			}
		}
		if row.Flaky {
			p.FlakyTasks = append(p.FlakyTasks, id)
		}

		p.TreatmentSuccesses += tc.Successes
		p.ControlSuccesses += cc.Successes
		p.TreatmentTrials += tc.Trials
		p.ControlTrials += cc.Trials
		p.TreatmentTokens += int64(tc.MeanTokens * float64(tc.Trials))
		p.ControlTokens += int64(cc.MeanTokens * float64(cc.Trials))
		p.TreatmentPayloadBytes += int64(tc.MeanPayloadBytes * float64(tc.Trials))
		p.ControlPayloadBytes += int64(cc.MeanPayloadBytes * float64(cc.Trials))
		p.TreatmentCostUSD += tc.MeanCostUSD * float64(tc.Trials)
		p.ControlCostUSD += cc.MeanCostUSD * float64(cc.Trials)
		p.TreatmentToolSearchAvg += tc.MeanToolSearchTurns * float64(tc.Trials)
		p.ControlToolSearchAvg += cc.MeanToolSearchTurns * float64(cc.Trials)
		p.PerTask = append(p.PerTask, row)
	}

	if p.TreatmentTrials > 0 {
		p.TreatmentRate = float64(p.TreatmentSuccesses) / float64(p.TreatmentTrials)
	}
	if p.ControlTrials > 0 {
		p.ControlRate = float64(p.ControlSuccesses) / float64(p.ControlTrials)
	}
	// Totals are compared per-trial-normalized so an arm that ran more
	// trials than the other cannot look more expensive for that reason
	// alone.
	if p.TreatmentTrials > 0 && p.ControlTrials > 0 {
		if p.ControlTokens > 0 {
			p.AggregateTokenRatio = (float64(p.TreatmentTokens) / float64(p.TreatmentTrials)) / (float64(p.ControlTokens) / float64(p.ControlTrials))
		}
		if p.ControlPayloadBytes > 0 {
			p.AggregatePayloadRatio = (float64(p.TreatmentPayloadBytes) / float64(p.TreatmentTrials)) / (float64(p.ControlPayloadBytes) / float64(p.ControlTrials))
		}
		if p.ControlCostUSD > 0 {
			p.AggregateCostRatio = (p.TreatmentCostUSD / float64(p.TreatmentTrials)) / (p.ControlCostUSD / float64(p.ControlTrials))
		}
		p.TreatmentToolSearchAvg /= float64(p.TreatmentTrials)
		p.ControlToolSearchAvg /= float64(p.ControlTrials)
	}
	p.MedianTokenRatio = median(ratios)
	return p
}

// Verdict is the tri-state outcome of evaluating a pre-registered criterion.
type Verdict string

const (
	VerdictPass Verdict = "pass"
	VerdictFail Verdict = "fail"
	// VerdictIndeterminate means the run did not measure enough to decide.
	// It is never interchangeable with fail: a fail is a measured refutation
	// and belongs in the record as one; an indeterminate says the instrument
	// could not see, and demands a re-run.
	VerdictIndeterminate Verdict = "indeterminate"
)

// Criterion is a pre-registered, machine-evaluable win condition. It is
// written and committed BEFORE a run (law 12: the criterion may not move
// after the data arrives), and Evaluate is the only thing permitted to
// declare a win.
type Criterion struct {
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
	// RequireStrictlyMoreSuccesses demands treatment > control on success,
	// not merely ≥. Equal success is a fail, by construction: this is the
	// clause the 2026-07-24 home-bench run failed.
	RequireStrictlyMoreSuccesses bool `json:"require_strictly_more_successes"`
	// MaxTokenRatio caps AggregateTokenRatio (0 = unconstrained).
	MaxTokenRatio float64 `json:"max_token_ratio,omitempty"`
	// MaxPayloadRatio caps AggregatePayloadRatio and MaxCostRatio caps
	// AggregateCostRatio (0 = unconstrained) — H2-3's cost half (ADR-032
	// §3). They exist because total_tokens is cache-read x turns (O26): a
	// turn counter in a token costume, under which the discovery payload
	// fathom actually shrinks is invisible. A criterion that caps payload
	// but whose run recorded none is INDETERMINATE, never a pass: the
	// instrument did not look.
	MaxPayloadRatio float64 `json:"max_payload_ratio,omitempty"`
	MaxCostRatio    float64 `json:"max_cost_ratio,omitempty"`
	// MinTrials is the replication floor. A run thinner than this is
	// indeterminate rather than a win — the n=1 guard.
	MinTrials int `json:"min_trials,omitempty"`
	// Scoring names how a cell's success is decided: ScoringSubstring
	// (the default, H2-1's substring rules) or ScoringPrecision (H2-2's
	// fenced answer block with a precision term). A criterion registered
	// under one never re-scores data collected under the other.
	Scoring string `json:"scoring,omitempty"`
	// LeakRule, when present, excludes tasks the no-tools arm answers from
	// memory (LeakedTasks). Registered beside the criterion, before the
	// run, so the exclusion cannot be shaped by the data.
	LeakRule *LeakRule `json:"leak_rule,omitempty"`
	// Reliability, when present, seeds the reported figures
	// (ComputeReliability, PB-10). Evaluate does not read it: the figures
	// are reported, never gating.
	Reliability *ReliabilityConfig `json:"reliability,omitempty"`
}

// Scoring modes a criterion may name.
const (
	ScoringSubstring = "substring"
	ScoringPrecision = "precision"
)

// Evaluation is a criterion's verdict plus the sentence that justifies it.
type Evaluation struct {
	CriterionID string  `json:"criterion_id"`
	Verdict     Verdict `json:"verdict"`
	Reason      string  `json:"reason"`
}

// Evaluate applies c to p. Coverage is checked first and dominates: an
// incomplete run can report indeterminate or fail, never pass (ADR-027).
func (c Criterion) Evaluate(p PairedSummary) Evaluation {
	ev := Evaluation{CriterionID: c.ID}
	switch {
	case p.Tasks == 0:
		ev.Verdict = VerdictIndeterminate
		ev.Reason = "no task was present in both arms — nothing was compared"
		return ev
	case len(p.Absent) > 0:
		ev.Verdict = VerdictIndeterminate
		ev.Reason = fmt.Sprintf("coverage incomplete: %d task(s) ran in only one arm (%v) — a partial run may not report a win", len(p.Absent), p.Absent)
		return ev
	case c.MinTrials > 0 && p.MinTrials < c.MinTrials:
		ev.Verdict = VerdictIndeterminate
		ev.Reason = fmt.Sprintf("thinnest cell ran %d trial(s), below the pre-registered floor of %d", p.MinTrials, c.MinTrials)
		return ev
	case p.Scope == ScopeTargeted:
		// BP-8: a targeted run re-ran only the tasks a diff selected. Its
		// per-task rows are the point; a corpus-level verdict over a
		// subset chosen after the change is not a verdict at all.
		ev.Verdict = VerdictIndeterminate
		ev.Reason = fmt.Sprintf("scope targeted: %d task(s) were selected by a diff, not the whole corpus — a verdict needs a full run", p.Tasks)
		return ev
	case p.Scope != "" && p.Scope != ScopeFull && p.Scope != ScopeTargeted:
		ev.Verdict = VerdictIndeterminate
		ev.Reason = fmt.Sprintf("unknown run scope %q — only %q may produce a verdict", p.Scope, ScopeFull)
		return ev
	case p.Cells > 0 && p.BorrowedCells == p.Cells:
		// BP-5: every cell key-matched an earlier run, so this row is a
		// restatement of that run, not a measurement.
		ev.Verdict = VerdictIndeterminate
		ev.Reason = fmt.Sprintf("all %d compared cell(s) were borrowed from earlier runs by cell key — this row re-states an earlier measurement and is not one", p.Cells)
		return ev
	case c.MaxPayloadRatio > 0 && p.ControlPayloadBytes == 0:
		ev.Verdict = VerdictIndeterminate
		ev.Reason = "criterion caps payload_bytes but the run recorded none — score it from the stream transcripts (ledger -raw) before reading a verdict"
		return ev
	case c.MaxCostRatio > 0 && p.ControlCostUSD == 0:
		ev.Verdict = VerdictIndeterminate
		ev.Reason = "criterion caps cost_usd but the run recorded none"
		return ev
	}

	if c.RequireStrictlyMoreSuccesses && p.TreatmentSuccesses <= p.ControlSuccesses {
		ev.Verdict = VerdictFail
		ev.Reason = fmt.Sprintf("success %d vs %d — not strictly greater", p.TreatmentSuccesses, p.ControlSuccesses)
		return ev
	}
	if c.MaxTokenRatio > 0 && p.AggregateTokenRatio > c.MaxTokenRatio {
		ev.Verdict = VerdictFail
		ev.Reason = fmt.Sprintf("aggregate token ratio %.3f× exceeds the pre-registered cap of %.3f×", p.AggregateTokenRatio, c.MaxTokenRatio)
		return ev
	}
	if c.MaxPayloadRatio > 0 && p.AggregatePayloadRatio > c.MaxPayloadRatio {
		ev.Verdict = VerdictFail
		ev.Reason = fmt.Sprintf("aggregate payload_bytes ratio %.3f× exceeds the pre-registered cap of %.3f×", p.AggregatePayloadRatio, c.MaxPayloadRatio)
		return ev
	}
	if c.MaxCostRatio > 0 && p.AggregateCostRatio > c.MaxCostRatio {
		ev.Verdict = VerdictFail
		ev.Reason = fmt.Sprintf("aggregate cost_usd ratio %.3f× exceeds the pre-registered cap of %.3f×", p.AggregateCostRatio, c.MaxCostRatio)
		return ev
	}
	ev.Verdict = VerdictPass
	ev.Reason = fmt.Sprintf("success %d vs %d at %.3f× tokens over %d task(s), min %d trial(s) per cell",
		p.TreatmentSuccesses, p.ControlSuccesses, p.AggregateTokenRatio, p.Tasks, p.MinTrials)
	if c.MaxPayloadRatio > 0 {
		ev.Reason += fmt.Sprintf("; payload %.3f× (cap %.3f×), cost %.3f× (cap %.3f×), toolsearch turns %.2f vs %.2f (reported, never gated)",
			p.AggregatePayloadRatio, c.MaxPayloadRatio, p.AggregateCostRatio, c.MaxCostRatio, p.TreatmentToolSearchAvg, p.ControlToolSearchAvg)
	}
	if p.BorrowedCells > 0 {
		ev.Reason += fmt.Sprintf("; %d of %d cell(s) borrowed by key from earlier runs", p.BorrowedCells, p.Cells)
	}
	return ev
}

// criterionFile is the on-disk carrier for a corpus's machine-evaluable
// criterion. It sits beside the prose `pre_registered_criterion` block
// rather than replacing it: the prose is the registration a human reads and
// git dates, the struct is what Evaluate may act on. They are kept honest
// by TestMachineCriterionMatchesTheRegisteredProse, which pins the values.
type criterionFile struct {
	MachineCriterion *Criterion `json:"machine_criterion"`
	// RegisteredCriteria holds criteria registered AFTER the first run
	// under MachineCriterion, each with its own id (H2-2 and on). They sit
	// beside the original rather than replacing it: a criterion that has
	// scored data is history, and history is not rewritten.
	RegisteredCriteria []Criterion `json:"registered_criteria,omitempty"`
}

// LoadCriterionByID reads the criterion registered under id: the
// machine_criterion block when its id matches, else the entry of
// registered_criteria with that id. An id nobody registered is an error —
// the same law-12 refusal as LoadCriterion.
func LoadCriterionByID(path, id string) (Criterion, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Criterion{}, fmt.Errorf("reading corpus %s: %w", path, err)
	}
	var cf criterionFile
	if err := json.Unmarshal(raw, &cf); err != nil {
		return Criterion{}, fmt.Errorf("parsing corpus %s: %w", path, err)
	}
	if cf.MachineCriterion != nil && cf.MachineCriterion.ID == id {
		return *cf.MachineCriterion, nil
	}
	for _, c := range cf.RegisteredCriteria {
		if c.ID == id {
			return c, nil
		}
	}
	return Criterion{}, fmt.Errorf("corpus %s registers no criterion %q — a run cannot be scored against a criterion that was never registered", path, id)
}

// LoadCriterion reads the criterion a run is scored under when none is
// named: the NEWEST entry of registered_criteria when the corpus has any,
// else the machine_criterion block. Registration is a committed act that
// precedes the run it binds (H2-3 was registered 2026-09-12 to score the
// 2026-09-13 run), so "newest registered" is the pre-registered bar, never
// a post-hoc choice; the older criteria stay loadable by id and their
// recorded verdicts stand as history. A corpus without any criterion is an
// error, not a default: inventing a criterion at evaluation time is
// precisely the post-hoc move law 12 forbids.
func LoadCriterion(path string) (Criterion, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Criterion{}, fmt.Errorf("reading corpus %s: %w", path, err)
	}
	var cf criterionFile
	if err := json.Unmarshal(raw, &cf); err != nil {
		return Criterion{}, fmt.Errorf("parsing corpus %s: %w", path, err)
	}
	if n := len(cf.RegisteredCriteria); n > 0 {
		return cf.RegisteredCriteria[n-1], nil
	}
	if cf.MachineCriterion == nil {
		return Criterion{}, fmt.Errorf("corpus %s has no machine_criterion block: a run cannot be scored against a criterion that was never registered", path)
	}
	if cf.MachineCriterion.ID == "" {
		return Criterion{}, fmt.Errorf("corpus %s: machine_criterion has no id", path)
	}
	return *cf.MachineCriterion, nil
}
