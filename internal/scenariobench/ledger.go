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
	"os"
	"path/filepath"
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

	Borrowed []BorrowedScenarioCell `json:"borrowed_cells,omitempty"`
	Notes    string                 `json:"notes,omitempty"`
}

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

// RenderScenarioLedger renders rows (in the order given — the caller's
// LoadScenarioLedger order, oldest first) as a markdown table, one row per
// (run, task, arm).
func RenderScenarioLedger(rows []ScenarioLedgerRow) string {
	var b strings.Builder
	b.WriteString("| run | commit | task | category | arm | model | trials | successes | prediction | cost | notes |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %d | %d | %s | %.4f | %s |\n",
			r.RunID, r.Commit, r.TaskID, r.Category, r.Arm, r.ModelID,
			r.Trials, r.Successes, r.Prediction, r.CostUSD, r.Notes)
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
