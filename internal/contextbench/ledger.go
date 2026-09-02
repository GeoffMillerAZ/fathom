package contextbench

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
)

// The bench ledger is the committed history of every context-bench run: one
// JSON object per line, appended, never rewritten. Raw transcripts and
// per-task artifacts stay under workspace/artifacts/ per the repo's
// workspace convention; the ledger is the small, greppable, reviewable part
// that belongs in git, because a benchmark nobody outside the run can see is
// a claim, not evidence.
//
// One row per run — not per task — keeps it readable for years and makes
// "has this regressed?" a diff rather than an archaeology exercise.

// LedgerRow is one committed context-bench run.
type LedgerRow struct {
	RunID       string `json:"run_id"`
	GeneratedAt string `json:"generated_at"`
	// Commit is the repo HEAD the run measured. Without it a row is
	// uninterpretable later, so RenderLedgerMarkdown shows it in every row.
	Commit string `json:"commit"`
	Corpus string `json:"corpus"`
	Model  string `json:"model"`

	Treatment string `json:"treatment"`
	Control   string `json:"control"`
	Tasks     int    `json:"tasks"`
	MinTrials int    `json:"min_trials"`

	TreatmentSuccesses  int     `json:"treatment_successes"`
	ControlSuccesses    int     `json:"control_successes"`
	TreatmentTrials     int     `json:"treatment_trials"`
	ControlTrials       int     `json:"control_trials"`
	AggregateTokenRatio float64 `json:"aggregate_token_ratio"`
	MedianTokenRatio    float64 `json:"median_token_ratio"`

	// Absent and FlakyTasks travel with the verdict deliberately: they are
	// the two facts that decide how much the verdict is worth, and a row
	// that carried only the verdict would read cleaner than the run was.
	Absent     []string `json:"absent,omitempty"`
	FlakyTasks []string `json:"flaky_tasks,omitempty"`

	Evaluation Evaluation `json:"evaluation"`
	Notes      string     `json:"notes,omitempty"`
}

// NewLedgerRow builds a row from a paired summary and its criterion's
// evaluation, so the numbers in the ledger cannot drift from the numbers
// the verdict was computed over.
func NewLedgerRow(runID, generatedAt, commit, corpus, model string, p PairedSummary, ev Evaluation) LedgerRow {
	absent := append([]string(nil), p.Absent...)
	flaky := append([]string(nil), p.FlakyTasks...)
	sort.Strings(absent)
	sort.Strings(flaky)
	return LedgerRow{
		RunID:               runID,
		GeneratedAt:         generatedAt,
		Commit:              commit,
		Corpus:              corpus,
		Model:               model,
		Treatment:           p.Treatment,
		Control:             p.Control,
		Tasks:               p.Tasks,
		MinTrials:           p.MinTrials,
		TreatmentSuccesses:  p.TreatmentSuccesses,
		ControlSuccesses:    p.ControlSuccesses,
		TreatmentTrials:     p.TreatmentTrials,
		ControlTrials:       p.ControlTrials,
		AggregateTokenRatio: p.AggregateTokenRatio,
		MedianTokenRatio:    p.MedianTokenRatio,
		Absent:              absent,
		FlakyTasks:          flaky,
		Evaluation:          ev,
	}
}

// AppendLedger appends one row to the JSONL ledger at path, creating the
// file if it does not exist. The parent directory must already exist.
func AppendLedger(path string, row LedgerRow) error {
	line, err := json.Marshal(row)
	if err != nil {
		return fmt.Errorf("marshaling ledger row %s: %w", row.RunID, err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening ledger %s: %w", path, err)
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("appending to ledger %s: %w", path, err)
	}
	return nil
}

// LoadLedger reads every row from the JSONL ledger at path, in file order.
// A ledger that does not exist yet is empty, not an error — the first run
// creates it. An unparseable line IS an error naming its line number: a
// loader that skipped what it could not read would report a shorter,
// healthier history than the truth (ADR-027).
func LoadLedger(path string) ([]LedgerRow, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("opening ledger %s: %w", path, err)
	}
	defer f.Close()

	var rows []LedgerRow
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		var r LedgerRow
		if err := json.Unmarshal([]byte(text), &r); err != nil {
			return nil, fmt.Errorf("ledger %s line %d: %w", path, lineNo, err)
		}
		rows = append(rows, r)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading ledger %s: %w", path, err)
	}
	return rows, nil
}

// RenderLedgerMarkdown renders the ledger as a table in file order (oldest
// first), so the trend reads top to bottom and a regression is a visible
// step rather than a number to recompute.
func RenderLedgerMarkdown(rows []LedgerRow) string {
	var b strings.Builder
	b.WriteString("| run | commit | corpus | model | tasks | n | success (t/c) | tokens agg | tokens median | verdict | why |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, r := range rows {
		corpus := r.Corpus
		if i := strings.LastIndex(corpus, "/"); i >= 0 {
			corpus = corpus[i+1:]
		}
		caveat := ""
		if len(r.Absent) > 0 {
			caveat += fmt.Sprintf(" absent=%d", len(r.Absent))
		}
		if len(r.FlakyTasks) > 0 {
			caveat += fmt.Sprintf(" flaky=%d", len(r.FlakyTasks))
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %d | %d | %d/%d | %.3f× | %.3f× | %s | %s%s |\n",
			r.RunID, r.Commit, corpus, r.Model, r.Tasks, r.MinTrials,
			r.TreatmentSuccesses, r.ControlSuccesses,
			r.AggregateTokenRatio, r.MedianTokenRatio,
			r.Evaluation.Verdict, r.Evaluation.Reason, caveat)
	}
	return b.String()
}
