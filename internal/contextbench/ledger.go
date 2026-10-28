package contextbench

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

// The bench ledger is the committed history of every context-bench run: one
// JSON object per line, appended, never rewritten. Raw transcripts and
// per-task artifacts stay under workspace/artifacts/ per the repo's
// workspace convention; the ledger is the small, greppable, reviewable part
// that belongs in git, because a benchmark nobody outside the run can see is
// a claim, not evidence.
//
// One row per (run, criterion) — not per task — keeps it readable for years
// and makes "has this regressed?" a diff rather than an archaeology exercise.
// A run may be scored under several criteria (README "Re-scoring"), so the
// key is (run_id, evaluation.criterion_id), never the run id alone (NP-B9):
// CheckLedgerKeys names a key that is empty or repeated, and AppendLedger
// refuses a row whose key the ledger already holds.

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
	// ModelID is the PRIMARY model the envelopes reported (e.g. a dated
	// id), as opposed to Model, the alias the runner was asked for — the
	// model that generated the run's answers, never a joined blend
	// (PrimaryModelID, BP-34). The cell key uses this id alone.
	ModelID string `json:"model_id,omitempty"`
	// AuxModels lists every other model the run's envelopes reported,
	// sorted — the harness's own small-model traffic for tool search and
	// summaries. Additive, reported, never part of any key.
	AuxModels []string `json:"aux_models,omitempty"`
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
	// BP-4/BP-5/BP-8 additions (ADR-032 §2). Scope says whether the run
	// covered the corpus (ScopeFull) or only the tasks a diff selected
	// (ScopeTargeted, which never carries a verdict). Borrowed lists every
	// reading this run did not pay for, with the key that admitted it and
	// the run that did pay — so "what did this row cost" and "what did it
	// measure" stay separable years later.
	Scope    string         `json:"scope,omitempty"`
	Borrowed []BorrowedCell `json:"borrowed_cells,omitempty"`

	// BP-9/BP-11 additions (H2-3). PayloadBytes/CostUSD ratios are the
	// cost half the criterion reads; ToolSearchTurns is the harness tax,
	// reported and never gated (O24); GoldSourceRates says, per arm, how
	// often each tool was the one whose result first carried a gold token
	// — the number that turns "the task passed" into "this tool passed".
	AggregatePayloadRatio  float64                       `json:"aggregate_payload_ratio,omitempty"`
	AggregateCostRatio     float64                       `json:"aggregate_cost_ratio,omitempty"`
	TreatmentToolSearchAvg float64                       `json:"treatment_toolsearch_turns_mean,omitempty"`
	ControlToolSearchAvg   float64                       `json:"control_toolsearch_turns_mean,omitempty"`
	GoldSourceRates        map[string]map[string]float64 `json:"gold_source_rate,omitempty"`
	// GoldSourceByShape is VB-5: the same reading, broken down by each
	// task's own Category (question shape) and then by arm — see
	// GoldSourceByShape.
	GoldSourceByShape map[string]map[string]map[string]float64 `json:"gold_source_by_shape,omitempty"`

	// TrustedWithoutRecheck is the RC-17 reading (F8, vision 28 §2.4): how
	// often a fathom tool call in this run stood without a Grep/Read/Bash
	// re-check within the next few turns, per tool. Nil when no cell in the
	// run recorded a fathom call (see AggregateTrustedWithoutRecheck).
	TrustedWithoutRecheck *TrustedWithoutRecheckStats `json:"trusted_without_recheck,omitempty"`

	// VB-36 additions (O47): the scratch-daemon-default safety checks,
	// summed/listed across every cell in the run (SumOutOfRootReads,
	// FlaggedCells). OutOfRootReads is the total count of Bash/Read tool
	// inputs that named an absolute path outside a session's root;
	// FlaggedCells names every task/arm/trial cell that carries a
	// path_leak or an out_of_root_reads > 0, so a leak or an escape is
	// visible on the row rather than averaged away by the verdict.
	OutOfRootReads int      `json:"out_of_root_reads,omitempty"`
	FlaggedCells   []string `json:"flagged_cells,omitempty"`

	// Reliability carries the PB-10 figures (ci_success_diff, wilcoxon_p,
	// a12, pass_at_1, pass_pow_3, and the seed that produced them),
	// flattened into the row. Reported, never gating: Evaluation is computed
	// from the summary alone. Nil for rows written before PB-10.
	*Reliability
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
		Scope:               p.Scope,

		AggregatePayloadRatio:  p.AggregatePayloadRatio,
		AggregateCostRatio:     p.AggregateCostRatio,
		TreatmentToolSearchAvg: p.TreatmentToolSearchAvg,
		ControlToolSearchAvg:   p.ControlToolSearchAvg,
	}
}

// LedgerLine is one ledger row with the 1-based file line it was read from,
// so a key check can name where an offending row lives.
type LedgerLine struct {
	Line int
	Row  LedgerRow
}

// CheckLedgerKeys returns an error naming, in line order, every row whose
// (run_id, evaluation.criterion_id) key is empty or repeats an earlier
// row's key, with the line of the earlier row. Nil means every key is
// present and unique (NP-B9).
func CheckLedgerKeys(lines []LedgerLine) error {
	sorted := append([]LedgerLine(nil), lines...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Line < sorted[j].Line })
	type key struct{ run, criterion string }
	seen := map[key]int{}
	var problems []string
	for _, l := range sorted {
		// Keys compare trimmed, so "r1 " cannot pass as a second key for r1.
		k := key{strings.TrimSpace(l.Row.RunID), strings.TrimSpace(l.Row.Evaluation.CriterionID)}
		switch {
		case k.run == "":
			problems = append(problems, fmt.Sprintf("line %d: empty run_id", l.Line))
		case k.criterion == "":
			problems = append(problems, fmt.Sprintf("line %d: run %s has an empty evaluation.criterion_id", l.Line, k.run))
		default:
			if first, ok := seen[k]; ok {
				problems = append(problems, fmt.Sprintf("line %d: key (%s, %s) repeats line %d", l.Line, k.run, k.criterion, first))
				continue
			}
			seen[k] = l.Line
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("ledger keys (run_id, evaluation.criterion_id): %s", strings.Join(problems, "; "))
	}
	return nil
}

// AppendLedger appends one row to the JSONL ledger at path, creating the
// file if it does not exist. The parent directory must already exist.
// Under an exclusive lock (the labs.AppendLabResult pattern) it parses every
// existing line, refusing over a corrupt one by its line number, and runs
// CheckLedgerKeys over the existing rows plus the new one; only a clean
// check appends, so a refused row leaves the file byte for byte unchanged
// (a row with an empty key is refused before the file is opened, so a
// refusal never creates one). A key collision already in the file blocks
// every append until an operator resolves it: the ledger is never written
// past a state it cannot vouch for.
func AppendLedger(path string, row LedgerRow) error {
	line, err := json.Marshal(row)
	if err != nil {
		return fmt.Errorf("marshaling ledger row %s: %w", row.RunID, err)
	}
	if err := CheckLedgerKeys([]LedgerLine{{Line: 1, Row: row}}); err != nil {
		return fmt.Errorf("refusing to append run %s under %q to ledger %s: %w", row.RunID, row.Evaluation.CriterionID, path, err)
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("opening ledger %s: %w", path, err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("locking ledger %s: %w", path, err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	existing, lastLine, unterminated, err := readLedgerLines(f, path)
	if err != nil {
		return fmt.Errorf("refusing to append to ledger: %w", err)
	}
	if err := CheckLedgerKeys(append(existing, LedgerLine{Line: lastLine + 1, Row: row})); err != nil {
		return fmt.Errorf("refusing to append run %s under %q to ledger %s: %w", row.RunID, row.Evaluation.CriterionID, path, err)
	}
	if unterminated {
		line = append([]byte{'\n'}, line...)
	}
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
	lines, err := LoadLedgerLines(path)
	if err != nil {
		return nil, err
	}
	var rows []LedgerRow
	for _, l := range lines {
		rows = append(rows, l.Row)
	}
	return rows, nil
}

// LoadLedgerLines is LoadLedger keeping each row's file line, the input
// CheckLedgerKeys names offenders by.
func LoadLedgerLines(path string) ([]LedgerLine, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("opening ledger %s: %w", path, err)
	}
	defer f.Close()
	lines, _, _, err := readLedgerLines(f, path)
	return lines, err
}

// readLedgerLines parses every non-blank line of r. It also returns the last
// line number read and whether the file ends without a newline, so an
// append can terminate that line before adding its own.
func readLedgerLines(r io.Reader, path string) (lines []LedgerLine, lastLine int, unterminated bool, err error) {
	br := bufio.NewReader(r)
	for {
		b, rerr := br.ReadBytes('\n')
		if rerr != nil && rerr != io.EOF {
			return nil, 0, false, fmt.Errorf("reading ledger %s: %w", path, rerr)
		}
		if len(b) > 0 {
			lastLine++
			unterminated = b[len(b)-1] != '\n'
			if text := bytes.TrimSpace(b); len(text) > 0 {
				var row LedgerRow
				if e := json.Unmarshal(text, &row); e != nil {
					return nil, 0, false, fmt.Errorf("ledger %s line %d: %w", path, lastLine, e)
				}
				lines = append(lines, LedgerLine{Line: lastLine, Row: row})
			}
		}
		if rerr == io.EOF {
			return lines, lastLine, unterminated, nil
		}
	}
}

// RenderLedgerMarkdown renders the ledger as a table in file order (oldest
// first), so the trend reads top to bottom and a regression is a visible
// step rather than a number to recompute.
func RenderLedgerMarkdown(rows []LedgerRow) string {
	var b strings.Builder
	b.WriteString("| run | commit | corpus | model | tasks | n | success (t/c) | tokens agg | tokens median | Δsuccess 95% CI | criterion | verdict | why |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
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
		if len(r.Borrowed) > 0 {
			caveat += fmt.Sprintf(" borrowed=%d", len(r.Borrowed))
		}
		if r.Scope != "" && r.Scope != ScopeFull {
			caveat += " scope=" + r.Scope
		}
		if r.AggregatePayloadRatio > 0 {
			caveat += fmt.Sprintf(" payload=%.3f×", r.AggregatePayloadRatio)
		}
		if r.TrustedWithoutRecheck != nil && r.TrustedWithoutRecheck.Calls > 0 {
			caveat += fmt.Sprintf(" trusted=%.0f%%(n=%d)", r.TrustedWithoutRecheck.Rate*100, r.TrustedWithoutRecheck.Calls)
		}
		if r.OutOfRootReads > 0 {
			caveat += fmt.Sprintf(" out_of_root=%d", r.OutOfRootReads)
		}
		if len(r.FlaggedCells) > 0 {
			caveat += fmt.Sprintf(" flagged=%d", len(r.FlaggedCells))
		}
		// A backfilled row has no per-task ratios, so its median is not
		// recorded — shown as such, never as 0.000×.
		median := "n/r"
		if r.MedianTokenRatio > 0 {
			median = fmt.Sprintf("%.3f×", r.MedianTokenRatio)
		}
		// One run may carry two rows — the criterion it was scored under
		// as history and the newer one (BP-38) — so the column that tells
		// them apart is on the table.
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %d | %d | %d/%d | %.3f× | %s | %s | %s | %s | %s%s |\n",
			r.RunID, r.Commit, corpus, r.Model, r.Tasks, r.MinTrials,
			r.TreatmentSuccesses, r.ControlSuccesses,
			r.AggregateTokenRatio, median, renderReliability(r.Reliability),
			r.Evaluation.CriterionID, r.Evaluation.Verdict, r.Evaluation.Reason, caveat)
	}
	return b.String()
}

// ─── BP-33: where a targeted run's row belongs ─────────────────────────────

// TargetedLedgerPath returns the ledger file a run's row must be appended to.
//
// docs/bench/ledger.jsonl is COMMITTED history: every row in it is supposed
// to be a corpus-scoped reading someone can compare against the next one. A
// `--scope targeted` / `--only` run is a diff-selected subset that carries no
// verdict by construction, so appending it to that file grows the committed
// history with rows that can never be compared — noise that a reader has to
// learn to skip. Such a run writes to
// <artifactsDir>/ledger-targeted.jsonl instead, under the gitignored
// workspace/ tree, unless the operator named a ledger explicitly (`--ledger`
// or CONTEXT_BENCH_LEDGER) — an explicit path is always obeyed, because
// overriding the default is how an operator says "yes, this one counts".
//
// Pure and deterministic: it decides from the run's own shape, never from the
// filesystem.
func TargetedLedgerPath(requested string, explicit bool, scope string, onlySelected bool, artifactsDir string) string {
	if explicit {
		return requested
	}
	if scope != ScopeTargeted && !onlySelected {
		return requested
	}
	if artifactsDir == "" {
		artifactsDir = filepath.Join("workspace", "artifacts", "context-bench")
	}
	return filepath.Join(artifactsDir, "ledger-targeted.jsonl")
}
