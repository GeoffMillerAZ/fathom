package contextbench

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
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

	// PB-3 additions (ADR-031). All additive and omitempty: every row
	// written before them still parses, and a row that lacks them reads
	// as "not recorded", never as zero.

	// PerTask carries the paired per-task outcomes the verdict was computed
	// over, so a later run can be compared task by task (the regression
	// gate, cmdRegress) instead of only on totals.
	PerTask []PairedTask `json:"per_task,omitempty"`
	// CorpusSHA256 identifies the exact corpus bytes the run scored; two
	// rows are comparable only when it matches (CorpusSHA256).
	CorpusSHA256 string `json:"corpus_sha256,omitempty"`
	// ModelID is the concrete model the envelopes reported (e.g. a dated
	// id), as opposed to Model, the alias the runner was asked for.
	ModelID string `json:"model_id,omitempty"`
	// CostUSD is the sum of every cell's reported total_cost_usd.
	CostUSD float64 `json:"cost_usd,omitempty"`
	// DirtyTree records that the working tree had uncommitted changes when
	// the run measured it, so Commit alone does not identify the subject.
	DirtyTree bool `json:"dirty_tree,omitempty"`
	// CellsLost counts (task, arm, trial) cells the runner intended but
	// could not score — a watchdog kill or an empty envelope — so a thinner
	// run says so in the row instead of only in the summary line.
	CellsLost int `json:"cells_lost,omitempty"`
	// LeakedTasks lists tasks excluded from the verdict because the
	// no-tools arm answered them from memory (LeakedTasks, PB-9).
	LeakedTasks []string `json:"leaked_tasks,omitempty"`
}

// CorpusSHA256 returns the hex SHA-256 of the corpus file's bytes — the
// identity a ledger row records so that two rows are known to have scored
// the same questions against the same ground truth. It is the whole file,
// not a canonical subset, so `shasum -a 256 <corpus>` reproduces it with
// no tooling; the price is that a provenance-note edit yields a new corpus
// identity, which is the honest reading — the corpus changed.
func CorpusSHA256(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("hashing corpus %s: %w", path, err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// BackfillLedgerRow builds a row for a run that predates the ledger from
// the numbers its plan recorded (PB-2). It exists so the ledger's history
// is not silently empty for runs that were real, and it is honest about
// what it is: the row is evaluated against the corpus's CURRENT criterion
// (so an n=1 run reads indeterminate under the replication floor, whatever
// verdict was recorded at the time), it carries no per-task data and no
// corpus hash (nothing can pair against it), and notes must say it was
// backfilled. Tasks is the number of paired tasks; successes are summed
// across the single trial.
func BackfillLedgerRow(c Criterion, runID, generatedAt, commit, corpus, model string, tasks, treatmentSuccesses, controlSuccesses int, aggregateTokenRatio float64, notes string) LedgerRow {
	p := PairedSummary{
		Treatment:           "fathom",
		Control:             "grep",
		Tasks:               tasks,
		MinTrials:           1,
		TreatmentSuccesses:  treatmentSuccesses,
		ControlSuccesses:    controlSuccesses,
		TreatmentTrials:     tasks,
		ControlTrials:       tasks,
		AggregateTokenRatio: aggregateTokenRatio,
	}
	row := NewLedgerRow(runID, generatedAt, commit, corpus, model, p, c.Evaluate(p))
	row.Notes = notes
	return row
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
		if r.CellsLost > 0 {
			caveat += fmt.Sprintf(" cells_lost=%d", r.CellsLost)
		}
		if len(r.LeakedTasks) > 0 {
			caveat += fmt.Sprintf(" leaked=%d", len(r.LeakedTasks))
		}
		// A backfilled row has no per-task ratios, so its median is not
		// recorded — shown as such, never as 0.000×.
		median := "n/r"
		if r.MedianTokenRatio > 0 {
			median = fmt.Sprintf("%.3f×", r.MedianTokenRatio)
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %d | %d | %d/%d | %.3f× | %s | %s | %s%s |\n",
			r.RunID, r.Commit, corpus, r.Model, r.Tasks, r.MinTrials,
			r.TreatmentSuccesses, r.ControlSuccesses,
			r.AggregateTokenRatio, median,
			r.Evaluation.Verdict, r.Evaluation.Reason, caveat)
	}
	return b.String()
}
