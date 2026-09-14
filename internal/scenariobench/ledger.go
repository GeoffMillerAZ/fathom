// The scenario ledger (SB-18): one committed JSONL row per (run, task,
// arm) — docs/bench/scenarios-ledger.jsonl — rendered as a marker-fenced
// table into docs/bench/README.md's "## Scenario runs" section, mirroring
// scripts/bench-ledger-render.sh's own marker convention so both ledgers
// read the same way.
package scenariobench

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ScenarioLedgerRow is one task's outcome for one arm, in one run.
type ScenarioLedgerRow struct {
	RunID          string   `json:"run_id"`
	GeneratedAt    string   `json:"generated_at"`
	Commit         string   `json:"commit"`
	CorpusSHA256   string   `json:"corpus_sha256"`
	TaskID         string   `json:"task_id"`
	Category       Category `json:"category"`
	Arm            string   `json:"arm"`
	ModelID        string   `json:"model_id"`
	HarnessVersion string   `json:"harness_version"`

	Trials    int `json:"trials"`
	Successes int `json:"successes"`

	// Prediction is the arm's pre-registered expectation for this task
	// (Arm.Prediction, or Task.Predictions[arm]), carried beside the
	// measured outcome so a reader sees the bet next to the result.
	Prediction string `json:"prediction,omitempty"`

	Readings Readings `json:"readings"`
	CostUSD  float64  `json:"cost_usd,omitempty"`
	// SurfaceUnused is true when the arm names a non-empty Surface but
	// Readings.SurfaceCalls is 0 across every cell filed into this row:
	// the arm's own MCP server connected (else the cell would be LOST by
	// the instrument-truth gate) but was never actually called — the
	// second documented defect (docs/bench/scenarios.md "Instrument
	// truth"), distinct from the first (never connected at all).
	SurfaceUnused bool `json:"surface_unused,omitempty"`
	// InstrumentDefect names the instrument fact that voids this row's
	// reading (ADR-034 decision 2, RC-36) — a row carrying it measured
	// something other than what it claims (O41: the first round's rows
	// ran with no MCP tools) and is excluded from every verdict, listed
	// only as voided. Empty for a sound row.
	InstrumentDefect string `json:"instrument_defect,omitempty"`

	Borrowed []BorrowedScenarioCell `json:"borrowed_cells,omitempty"`
	Notes    string                 `json:"notes,omitempty"`

	// GoldSourceRate is VB-4 (ADR-035 decision 5): the fraction of this
	// row's own cells (successes and failures alike — "which tool touched
	// the gold signal at all", E1/O23 as a number) whose
	// Readings.GoldSourceTool equalled each key. GoldSourceNone ("none")
	// is a normal key: the agent knew or guessed on some cells.
	// GoldSourceNotMeasured names cells scored before this reading
	// existed — never silently folded into GoldSourceNone, which is
	// itself a finding, not an absence of one.
	GoldSourceRate map[string]float64 `json:"gold_source_rate,omitempty"`
	// GoldSourcePasses narrows the same reading to cells that PASSED:
	// among this row's Successes, how many did each tool actually earn
	// (ADR-035 decision 5: "a tool passes only if it was the gold
	// source"). What the verdict block's "gold source: <tool> earned N of
	// M passes" line sums across a run's treatment-arm rows. Nil when the
	// row has no successes.
	GoldSourcePasses map[string]int `json:"gold_source_passes,omitempty"`

	// PerTrial preserves each cell's own structured readings verbatim, one
	// entry per trial filed into this row (AggregateRowReadings, cell.go)
	// — the detail Readings' own summed counts and averaged ratios
	// deliberately do not carry, so a reader who needs the raw per-trial
	// shape still has it. Empty for a row filed before this field existed.
	PerTrial []TrialReading `json:"per_trial,omitempty"`
}

// TrialReading is one trial's own structured readings, carried on the
// ledger row in full even though the row's own Readings summarizes most
// of these fields away (a count, a mean, a rate) rather than repeating
// them per trial.
type TrialReading struct {
	Trial          int                  `json:"trial"`
	GoldSourceTool string               `json:"gold_source_tool,omitempty"`
	PlanCoverage   *PlanCoverageReading `json:"plan_coverage,omitempty"`
	FalseDone      *FalseDoneReading    `json:"false_done,omitempty"`
	Drift          *DriftReading        `json:"drift,omitempty"`
	Resumption     *ResumptionReading   `json:"resumption,omitempty"`
	Overlap        *OverlapReading      `json:"overlap,omitempty"`
	// Localization is VB-25: this trial's own Readings.Localization,
	// preserved verbatim (the row's own LocalizationHitAt1Rate etc. only
	// average it away).
	Localization *LocalizationReading `json:"localization,omitempty"`
}

// GoldSourceNotMeasured is the gold_source_rate bucket for a cell whose
// Readings.GoldSourceTool was never computed (scored before VB-4) — kept
// distinct from GoldSourceNone ("no tool result held it, the agent knew
// or guessed") so an old cell is never silently read as a negative
// finding.
const GoldSourceNotMeasured = "not_measured"

// AppendScenarioLedger appends one row to the JSONL ledger at path,
// creating the file (and its parent directory) if it does not exist.
func AppendScenarioLedger(path string, row ScenarioLedgerRow) error {
	line, err := json.Marshal(row)
	if err != nil {
		return fmt.Errorf("marshaling scenario ledger row %s/%s: %w", row.TaskID, row.Arm, err)
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening scenario ledger %s: %w", path, err)
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("appending to scenario ledger %s: %w", path, err)
	}
	return nil
}

// LoadScenarioLedger reads every row from the JSONL ledger at path, in
// file order. A ledger that does not exist yet is empty, not an error — the
// first run creates it. An unparseable line IS an error naming its line
// number (ADR-027): a loader that skipped what it could not read would
// report a shorter, healthier history than the truth.
func LoadScenarioLedger(path string) ([]ScenarioLedgerRow, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("opening scenario ledger %s: %w", path, err)
	}
	defer f.Close()

	var rows []ScenarioLedgerRow
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		var r ScenarioLedgerRow
		if err := json.Unmarshal([]byte(text), &r); err != nil {
			return nil, fmt.Errorf("scenario ledger %s line %d: %w", path, lineNo, err)
		}
		rows = append(rows, r)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading scenario ledger %s: %w", path, err)
	}
	return rows, nil
}

// notMeasuredCell is the render placeholder for a column this row's own
// category never made meaningful (e.g. "violation" on a public-corpus
// row) — dash, never a blank cell that could be misread as a zero.
const notMeasuredCell = "–"

// renderTrialsTrueOf formats a row-level "count of trials true" reading
// (ViolationTrials, RuleServedBeforeEditTrials) as "<true>/<trials>", or
// notMeasuredCell when no trial in the row ever measured it.
func renderTrialsTrueOf(trueCount *int, trials int) string {
	if trueCount == nil {
		return notMeasuredCell
	}
	return fmt.Sprintf("%d/%d", *trueCount, trials)
}

// renderHooksFiredTotal sums a row's own HooksFired map (every hook event,
// every trial) to one number — the per-event breakdown still lives in the
// map itself for a caller that wants it.
func renderHooksFiredTotal(hooks map[string]int) string {
	if len(hooks) == 0 {
		return notMeasuredCell
	}
	total := 0
	for _, n := range hooks {
		total += n
	}
	return fmt.Sprintf("%d", total)
}

// renderOverlapRatio formats the row's OverlapRatioMean/N (reuse corpora,
// ModePlannerWorkers tasks), or notMeasuredCell when no trial measured it.
func renderOverlapRatio(r Readings) string {
	if r.OverlapRatioN == 0 {
		return notMeasuredCell
	}
	return fmt.Sprintf("%.2f (n=%d)", r.OverlapRatioMean, r.OverlapRatioN)
}

// renderFactsReused formats the row's own FactsReused sum, or
// notMeasuredCell when no trial ever measured it.
func renderFactsReused(n *int) string {
	if n == nil {
		return notMeasuredCell
	}
	return fmt.Sprintf("%d", *n)
}

// renderResumption formats the row's ResumptionTokensToFirstEditMean/N, or
// notMeasuredCell when no trial measured it.
func renderResumption(r Readings) string {
	if r.ResumptionTokensToFirstEditN == 0 {
		return notMeasuredCell
	}
	return fmt.Sprintf("%.0f (n=%d)", r.ResumptionTokensToFirstEditMean, r.ResumptionTokensToFirstEditN)
}

// renderFalseDoneSummary sums Claimed/Refuted across every per-trial
// FalseDoneReading that was actually measurable, from the row's own
// PerTrial detail — public-corpus rows are the ones that carry this.
func renderFalseDoneSummary(trials []TrialReading) string {
	var claimed, refuted, n int
	for _, t := range trials {
		if t.FalseDone == nil || t.FalseDone.NotMeasurable != "" {
			continue
		}
		claimed += t.FalseDone.Claimed
		refuted += t.FalseDone.Refuted
		n++
	}
	if n == 0 {
		return notMeasuredCell
	}
	return fmt.Sprintf("%d/%d refuted", refuted, claimed)
}

// renderPlanCoverageSummary averages Fraction across every per-trial
// PlanCoverageReading that was actually measurable, from the row's own
// PerTrial detail.
func renderPlanCoverageSummary(trials []TrialReading) string {
	var sum float64
	var n int
	for _, t := range trials {
		if t.PlanCoverage == nil || t.PlanCoverage.NotMeasurable != "" {
			continue
		}
		sum += t.PlanCoverage.Fraction
		n++
	}
	if n == 0 {
		return notMeasuredCell
	}
	return fmt.Sprintf("%.2f", sum/float64(n))
}

// RenderScenarioLedger renders rows (in the order given — the caller's
// LoadScenarioLedger order, oldest first) as a markdown table, one row per
// (run, task, arm). Columns beyond the always-present run/task/trials
// header carry a reading only where the corpus category that produced the
// row makes it meaningful — a governance (guards) row fills hooks
// fired/rule served/violation, a reuse row fills overlap ratio/facts
// reused, a public row fills resumption/false done/plan coverage — every
// other row's cell for that column renders notMeasuredCell rather than a
// misleading zero (docs/bench/scenarios.md, the row-aggregation defect).
func RenderScenarioLedger(rows []ScenarioLedgerRow) string {
	var b strings.Builder
	b.WriteString("| run | commit | task | category | arm | model | trials | successes | prediction | surface calls | hooks fired | rule served | violation | overlap ratio | facts reused | resumption | false done | plan coverage | cost | notes |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, r := range rows {
		surfaceCalls := fmt.Sprintf("%d", r.Readings.SurfaceCalls)
		if r.SurfaceUnused {
			surfaceCalls += " (unused)"
		}
		notes := r.Notes
		// RC-36: a voided row stays IN the table (deleting history is not
		// the fix) but is named as voided, never blends silently into a
		// reader's eye as a normal reading.
		if r.InstrumentDefect != "" {
			notes = strings.TrimSpace("VOIDED (" + r.InstrumentDefect + ") " + notes)
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %d | %d | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %.4f | %s |\n",
			r.RunID, r.Commit, r.TaskID, r.Category, r.Arm, r.ModelID,
			r.Trials, r.Successes, r.Prediction, surfaceCalls,
			renderHooksFiredTotal(r.Readings.HooksFired),
			renderTrialsTrueOf(r.Readings.RuleServedBeforeEditTrials, r.Trials),
			renderTrialsTrueOf(r.Readings.ViolationTrials, r.Trials),
			renderOverlapRatio(r.Readings),
			renderFactsReused(r.Readings.FactsReused),
			renderResumption(r.Readings),
			renderFalseDoneSummary(r.PerTrial),
			renderPlanCoverageSummary(r.PerTrial),
			r.CostUSD, notes)
	}
	return b.String()
}

// GoldSourceRateByCategory aggregates every row's own GoldSourceRate by
// task Category (VB-4): a per-question-shape breakdown of which tool
// earned the gold source, so "obligations wins" is not one number hiding
// a lookup/change/guards split. Weighted by each row's own Trials so the
// aggregate reconstructs exact counts rather than averaging fractions of
// different sizes; a row with no GoldSourceRate (scored before VB-4)
// contributes nothing.
func GoldSourceRateByCategory(rows []ScenarioLedgerRow) map[Category]map[string]float64 {
	type acc struct {
		counts map[string]int
		total  int
	}
	byCat := map[Category]*acc{}
	for _, r := range rows {
		if len(r.GoldSourceRate) == 0 || r.Trials == 0 {
			continue
		}
		a, ok := byCat[r.Category]
		if !ok {
			a = &acc{counts: map[string]int{}}
			byCat[r.Category] = a
		}
		for tool, rate := range r.GoldSourceRate {
			a.counts[tool] += int(rate*float64(r.Trials) + 0.5)
		}
		a.total += r.Trials
	}
	if len(byCat) == 0 {
		return nil
	}
	out := make(map[Category]map[string]float64, len(byCat))
	for cat, a := range byCat {
		rates := make(map[string]float64, len(a.counts))
		for tool, n := range a.counts {
			rates[tool] = float64(n) / float64(a.total)
		}
		out[cat] = rates
	}
	return out
}

// RenderGoldSourceByCategory renders GoldSourceRateByCategory as a
// markdown table: one row per category, one column per tool seen in ANY
// category — so a shape where a tool never won still shows it at 0.00,
// never a silent absence (VB-4). Empty string when byCat is empty.
func RenderGoldSourceByCategory(byCat map[Category]map[string]float64) string {
	if len(byCat) == 0 {
		return ""
	}
	cats := make([]string, 0, len(byCat))
	toolSet := map[string]bool{}
	for cat, rates := range byCat {
		cats = append(cats, string(cat))
		for tool := range rates {
			toolSet[tool] = true
		}
	}
	sort.Strings(cats)
	tools := make([]string, 0, len(toolSet))
	for t := range toolSet {
		tools = append(tools, t)
	}
	sort.Strings(tools)

	var b strings.Builder
	b.WriteString("### Gold source by question shape\n\n")
	b.WriteString("| category | " + strings.Join(tools, " | ") + " |\n")
	b.WriteString("|---|" + strings.Repeat("---|", len(tools)) + "\n")
	for _, cat := range cats {
		row := byCat[Category(cat)]
		fmt.Fprintf(&b, "| %s |", cat)
		for _, tool := range tools {
			fmt.Fprintf(&b, " %.2f |", row[tool])
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// ScenarioEvaluation is one corpus criterion's verdict over a set of rows.
type ScenarioEvaluation struct {
	CriterionID string `json:"criterion_id"`
	Verdict     string `json:"verdict"` // pass | fail | indeterminate
	Reason      string `json:"reason"`
}

const (
	ScenarioVerdictPass          = "pass"
	ScenarioVerdictFail          = "fail"
	ScenarioVerdictIndeterminate = "indeterminate"
)

// Evaluate applies c to rows (one run's ScenarioLedgerRow per task/arm):
// fewer than c.MinTrials trials on the treatment arm is indeterminate (the
// run did not measure enough to decide); otherwise a treatment success
// rate below the control's, when c.RequireSuccessAtLeastEqual, is a fail,
// and so is a token-per-success ratio above c.MaxTokensPerSuccessRatio when
// one is registered. Anything else is a pass. Rows for arms other than
// c.Treatment/c.Control are ignored.
func (c Criterion) Evaluate(rows []ScenarioLedgerRow) ScenarioEvaluation {
	ev := ScenarioEvaluation{CriterionID: c.ID}
	var tTrials, tSuccesses, cTrials, cSuccesses int
	var tTokens, cTokens int64
	for _, r := range rows {
		// ADR-034 decision 2 / RC-36: a row carrying instrument_defect is
		// voided — it measured something other than what it claims — and
		// is excluded from every verdict, never silently counted in.
		if r.InstrumentDefect != "" {
			continue
		}
		switch r.Arm {
		case c.Treatment:
			tTrials += r.Trials
			tSuccesses += r.Successes
			tTokens += r.Readings.TotalTokens
		case c.Control:
			cTrials += r.Trials
			cSuccesses += r.Successes
			cTokens += r.Readings.TotalTokens
		}
	}
	if tTrials < c.MinTrials {
		ev.Verdict = ScenarioVerdictIndeterminate
		ev.Reason = fmt.Sprintf("treatment %q ran %d trial(s), below the registered floor of %d", c.Treatment, tTrials, c.MinTrials)
		return ev
	}
	tRate := rateOf(tSuccesses, tTrials)
	cRate := rateOf(cSuccesses, cTrials)
	if c.RequireSuccessAtLeastEqual && tRate < cRate {
		ev.Verdict = ScenarioVerdictFail
		ev.Reason = fmt.Sprintf("treatment success rate %.3f is below control's %.3f", tRate, cRate)
		return ev
	}
	if c.MaxTokensPerSuccessRatio > 0 && tSuccesses > 0 && cSuccesses > 0 {
		tPerSuccess := float64(tTokens) / float64(tSuccesses)
		cPerSuccess := float64(cTokens) / float64(cSuccesses)
		if cPerSuccess > 0 {
			ratio := tPerSuccess / cPerSuccess
			if ratio > c.MaxTokensPerSuccessRatio {
				ev.Verdict = ScenarioVerdictFail
				ev.Reason = fmt.Sprintf("tokens-per-success ratio %.3f× exceeds the registered bar of %.3f×", ratio, c.MaxTokensPerSuccessRatio)
				return ev
			}
		}
	}
	ev.Verdict = ScenarioVerdictPass
	ev.Reason = "treatment met the registered bar"
	return ev
}

func rateOf(successes, trials int) float64 {
	if trials == 0 {
		return 0
	}
	return float64(successes) / float64(trials)
}

// EscalationCell is one task RC-14 (vision 28 §2.6: "screen at n=2,
// escalate to n≥3 only where the delta sits within the criterion's
// margin") flags for a third trial.
type EscalationCell struct {
	TaskID string `json:"task_id"`
	// Trials is the treatment arm's own trial count at the point this cell
	// was flagged — always 2 (the screening floor); EscalationPlan does
	// not consider a task that already escalated.
	Trials int `json:"trials"`
	// SuccessDelta is the treatment success rate minus the control's — the
	// quantity RequireSuccessAtLeastEqual's zero line judges.
	SuccessDelta float64 `json:"success_delta"`
	// TokensRatio is the treatment/control tokens-per-success ratio
	// MaxTokensPerSuccessRatio judges; 0 when neither arm had a success to
	// compute one from.
	TokensRatio float64 `json:"tokens_ratio,omitempty"`
	Reason      string  `json:"reason"`
}

// EscalationPlan is RC-14: among ledgerRows (any run's ScenarioLedgerRow
// set — the caller filters by run-id first when that matters), the tasks
// whose TREATMENT arm ran EXACTLY criterion.MinTrials-independent n=2 (the
// registered screening floor) and whose delta against the criterion sits
// within margin of the line that decides pass/fail: the success rate
// against the zero line RequireSuccessAtLeastEqual judges, and the
// tokens-per-success ratio against MaxTokensPerSuccessRatio. A task with
// fewer than 2 trials, or more than 2 (already escalated, or run at a
// higher floor), is not a candidate. Rows for arms other than
// criterion.Treatment/.Control are ignored. A row carrying
// InstrumentDefect (ADR-034 decision 2, RC-36: it measured something
// other than what it claims) is also skipped, exactly as Evaluate already
// excludes it from every verdict — counted and returned as skipped, so a
// caller can report that an escalation recommendation was not built from
// voided evidence. Returned in task-id order.
func EscalationPlan(ledgerRows []ScenarioLedgerRow, criterion Criterion, margin float64) (cells []EscalationCell, skipped int) {
	type acc struct {
		tTrials, tSuccesses, cTrials, cSuccesses int
		tTokens, cTokens                         int64
	}
	byTask := map[string]*acc{}
	var order []string
	for _, r := range ledgerRows {
		if r.InstrumentDefect != "" {
			skipped++
			continue
		}
		if r.Arm != criterion.Treatment && r.Arm != criterion.Control {
			continue
		}
		a, ok := byTask[r.TaskID]
		if !ok {
			a = &acc{}
			byTask[r.TaskID] = a
			order = append(order, r.TaskID)
		}
		switch r.Arm {
		case criterion.Treatment:
			a.tTrials += r.Trials
			a.tSuccesses += r.Successes
			a.tTokens += r.Readings.TotalTokens
		case criterion.Control:
			a.cTrials += r.Trials
			a.cSuccesses += r.Successes
			a.cTokens += r.Readings.TotalTokens
		}
	}
	sort.Strings(order)

	for _, taskID := range order {
		a := byTask[taskID]
		if a.tTrials != 2 {
			continue // only the n=2 screen is a candidate for escalation
		}
		successDelta := rateOf(a.tSuccesses, a.tTrials) - rateOf(a.cSuccesses, a.cTrials)

		var reasons []string
		if math.Abs(successDelta) <= margin {
			reasons = append(reasons, fmt.Sprintf("success delta %.3f is within %.2f of the pass/fail line", successDelta, margin))
		}

		var ratio float64
		if criterion.MaxTokensPerSuccessRatio > 0 && a.tSuccesses > 0 && a.cSuccesses > 0 {
			tPerSuccess := float64(a.tTokens) / float64(a.tSuccesses)
			cPerSuccess := float64(a.cTokens) / float64(a.cSuccesses)
			if cPerSuccess > 0 {
				ratio = tPerSuccess / cPerSuccess
				if math.Abs(ratio-criterion.MaxTokensPerSuccessRatio) <= margin {
					reasons = append(reasons, fmt.Sprintf("tokens-per-success ratio %.3f× is within %.2f of the registered bar %.3f×", ratio, margin, criterion.MaxTokensPerSuccessRatio))
				}
			}
		}

		if len(reasons) == 0 {
			continue
		}
		cells = append(cells, EscalationCell{
			TaskID:       taskID,
			Trials:       a.tTrials,
			SuccessDelta: successDelta,
			TokensRatio:  ratio,
			Reason:       strings.Join(reasons, "; "),
		})
	}
	return cells, skipped
}

// ScenarioLedgerBeginMarker / ScenarioLedgerEndMarker fence the rendered
// table inside docs/bench/README.md's "## Scenario runs" section, so the
// prose around it survives every re-render (mirrors
// scripts/bench-ledger-render.sh's ledger table markers).
const (
	ScenarioLedgerBeginMarker = "<!-- BEGIN scenario ledger (task bench-scenario-ledger) -->"
	ScenarioLedgerEndMarker   = "<!-- END scenario ledger -->"
)

// RewriteScenarioLedgerSection replaces the content between
// ScenarioLedgerBeginMarker and ScenarioLedgerEndMarker in the file at
// readmePath with body, leaving everything else untouched. Both markers
// must already be present (this package does not invent the section
// heading) — an error names which is missing.
func RewriteScenarioLedgerSection(readmePath, body string) error {
	raw, err := os.ReadFile(readmePath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", readmePath, err)
	}
	text := string(raw)
	i := strings.Index(text, ScenarioLedgerBeginMarker)
	if i < 0 {
		return fmt.Errorf("%s: marker %q not found", readmePath, ScenarioLedgerBeginMarker)
	}
	j := strings.Index(text, ScenarioLedgerEndMarker)
	if j < 0 {
		return fmt.Errorf("%s: marker %q not found", readmePath, ScenarioLedgerEndMarker)
	}
	if j < i {
		return fmt.Errorf("%s: end marker precedes begin marker", readmePath)
	}
	begin := i + len(ScenarioLedgerBeginMarker)
	rewritten := text[:begin] + "\n\n" + strings.TrimRight(body, "\n") + "\n\n" + text[j:]
	return os.WriteFile(readmePath, []byte(rewritten), 0o644)
}
