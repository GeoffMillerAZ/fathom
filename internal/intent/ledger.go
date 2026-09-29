package intent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/fathomhome"
)

// ledger.go — EP-6/EP-12, the intent ledger.
//
// A SEPARATE file from ~/.fathom/verify-ledger.jsonl, deliberately. That
// ledger's readers (the flywheel's verify-gate loop, instrument --adoption,
// the SDD trend) decode every line as a gate run and COUNT it; an
// intent-oracle row landing there would decode as a zero-task verify and
// quietly corrupt the program's headline instrument ("the verify gate is
// PROVEN, 180 of 436 runs"). Two record kinds, two files.
//
// Everything here is append-only evidence, never state: a verdict is always
// recomputed, and the ledger exists only to answer "when was this FIRST
// seen?" — a question no pure re-derivation can answer, because the index is
// not in git and a lexical condition's first-true moment needs a bisect.

// LedgerKind names what a row records.
type LedgerKind string

const (
	// KindOracle is one verdict TRANSITION for one condition: the first time
	// (id, verdict) was seen at a commit. Dedup is on that triple, so a
	// verb run every hour does not fill the file.
	KindOracle LedgerKind = "oracle"
	// KindDebt is one audit's counts by kind — the only source for a
	// "+k since" delta, which is why the design forbids the brief from
	// inventing one.
	KindDebt LedgerKind = "debt"
	// KindClock is the law-6 stamp: the moment a kill clock started, with
	// the consumer population present at that moment.
	KindClock LedgerKind = "clock"
	// KindLearning (GL-14, docs/plans/governance-learning.md, design doc
	// §6) is one promotion or retirement applyLearning (internal/semantic)
	// made: ID is the claim id, Verdict is the lifecycle it moved TO
	// ("active" | "retired"), Evidence carries the signals that justified
	// it (LearningDecision.Reason, verbatim — GL-X1: every state change is
	// announced with its signals, never silent). Written by cmd/fathom's
	// learningAnnouncements, the SAME ledger the intent debt half already
	// uses — "extended, not doubled" (intentattention.go's own doctrine),
	// not a second learning-specific file.
	KindLearning LedgerKind = "learning"
)

// LedgerRow is one append-only record.
type LedgerRow struct {
	SchemaVersion int            `json:"schema_version"`
	FathomVersion string         `json:"fathom_version,omitempty"`
	Kind          LedgerKind     `json:"kind"`
	At            string         `json:"at"`
	RepoRoot      string         `json:"repo_root,omitempty"`
	Commit        string         `json:"commit,omitempty"`
	ID            string         `json:"id,omitempty"`      // condition id, for oracle rows
	Verdict       string         `json:"verdict,omitempty"` // fired | holds | …
	Evidence      string         `json:"evidence,omitempty"`
	ByKind        map[string]int `json:"by_kind,omitempty"`    // debt rows
	Annotated     int            `json:"annotated,omitempty"`  // debt rows: K0's signal
	Population    []string       `json:"population,omitempty"` // clock rows
	MS            int64          `json:"ms,omitempty"`

	// K1's PRE-clock baseline (EP-24), carried on a clock row: how long a
	// decision's condition sat between its last text edit and the edit that
	// answered it, BEFORE any of this shipped. Without it "the plane made
	// decisions faster" has nothing to be faster than — law 12.
	BaselineDays   []int  `json:"baseline_days,omitempty"`
	BaselineMedian int    `json:"baseline_median,omitempty"`
	BaselineNote   string `json:"baseline_note,omitempty"`
}

// LedgerSchemaVersion is bumped when a row's meaning changes, never when a
// field is added (readers tolerate unknown fields by construction).
const LedgerSchemaVersion = 1

// LedgerPath is the intent ledger's location. FATHOM_INTENT_LEDGER
// overrides it, which is how tests stay off a developer's real ledger.
func LedgerPath() string {
	return fathomhome.PerFile("FATHOM_INTENT_LEDGER", "intent-ledger.jsonl")
}

// AppendLedger writes one row. Failure is silent by design: this is
// telemetry, and a hook or verb must never fail because a log could not be
// written (I9's fail-open contract). The caller that needs to know asks.
func AppendLedger(row LedgerRow) error {
	path := LedgerPath()
	if path == "" {
		return nil
	}
	row.SchemaVersion = LedgerSchemaVersion
	if row.At == "" {
		row.At = time.Now().UTC().Format(time.RFC3339)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(row)
}

// ReadLedger returns every row, oldest first. A malformed line is skipped
// rather than failing the read: a truncated write must not blind the
// instrument (and a partial line is exactly what a killed process leaves).
func ReadLedger() []LedgerRow {
	path := LedgerPath()
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []LedgerRow
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var row LedgerRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			continue
		}
		out = append(out, row)
	}
	return out
}

// FirstSeen returns the earliest row recording (id, verdict), or false. This
// is the timestamp "decided" is measured from: a record that changed BEFORE
// its condition fired did not answer it.
func FirstSeen(rows []LedgerRow, id, verdict string) (LedgerRow, bool) {
	var best LedgerRow
	found := false
	for _, r := range rows {
		if r.Kind != KindOracle || r.ID != id || r.Verdict != verdict {
			continue
		}
		if !found || r.At < best.At {
			best, found = r, true
		}
	}
	return best, found
}

// RecordTransition appends an oracle row only when (id, verdict, commit) is
// new — the dedup that keeps an hourly verb from filling the file while
// still capturing every real transition.
func RecordTransition(rows []LedgerRow, row LedgerRow) (bool, error) {
	for _, r := range rows {
		if r.Kind == KindOracle && r.ID == row.ID && r.Verdict == row.Verdict && r.Commit == row.Commit {
			return false, nil
		}
	}
	row.Kind = KindOracle
	return true, AppendLedger(row)
}

// Clock returns the kill-clock stamp for root, or false when none was ever
// written. Law 6: an activation step that leaves no stamp cannot be
// measured later, and "when did this start?" becomes a memory question.
func Clock(rows []LedgerRow, root string) (LedgerRow, bool) {
	for _, r := range rows {
		if r.Kind == KindClock && r.RepoRoot == root {
			return r, true
		}
	}
	return LedgerRow{}, false
}

// StartClock writes the stamp once. population is the computed consumer set
// (router ∧ .fathom present), never a hand-kept list.
func StartClock(root, commit string, population []string) error {
	if _, ok := Clock(ReadLedger(), root); ok {
		return nil
	}
	sort.Strings(population)
	return AppendLedger(LedgerRow{
		Kind:       KindClock,
		RepoRoot:   root,
		Commit:     commit,
		Population: population,
	})
}

// NewestDebt returns the most recent debt row for root, or false.
func NewestDebt(rows []LedgerRow, root string) (LedgerRow, bool) {
	var best LedgerRow
	found := false
	for _, r := range rows {
		if r.Kind != KindDebt || r.RepoRoot != root {
			continue
		}
		if !found || r.At > best.At {
			best, found = r, true
		}
	}
	return best, found
}

// DebtBefore returns the newest debt row older than cutoff — the baseline a
// "+k since" delta is measured against. Two back-to-back audits must not
// produce a triumphant "+0"; the delta is only meaningful against a row
// from a previous session.
func DebtBefore(rows []LedgerRow, root string, cutoff time.Time) (LedgerRow, bool) {
	stamp := cutoff.UTC().Format(time.RFC3339)
	var best LedgerRow
	found := false
	for _, r := range rows {
		if r.Kind != KindDebt || r.RepoRoot != root || r.At >= stamp {
			continue
		}
		if !found || r.At > best.At {
			best, found = r, true
		}
	}
	return best, found
}

// LearningRecorded reports whether a (root, claim id, verdict) learning
// transition is already in rows — the dedup a caller checks before
// appending, so a re-run over the same already-persisted lifecycle state
// (e.g. a caller that computed decisions again before writing them back to
// the store) does not fill the ledger with repeats of a transition that,
// by applyLearning's own state machine, can only ever happen once per
// claim (candidate→active, or active→retired — never revisited).
func LearningRecorded(rows []LedgerRow, root, claimID, verdict string) bool {
	for _, r := range rows {
		if r.Kind == KindLearning && r.RepoRoot == root && r.ID == claimID && r.Verdict == verdict {
			return true
		}
	}
	return false
}

// RecentLearning returns every learning row for root at or after since,
// newest first — the attention line's fragment and `fathom intent audit`'s
// own report both read this rather than re-deriving a window filter.
func RecentLearning(rows []LedgerRow, root string, since time.Time) []LedgerRow {
	stamp := since.UTC().Format(time.RFC3339)
	var out []LedgerRow
	for _, r := range rows {
		if r.Kind == KindLearning && r.RepoRoot == root && r.At >= stamp {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At > out[j].At })
	return out
}

// Baseline returns the newest clock row for root that actually carries a K1
// baseline (measured or explained), and whether one exists. A clock row
// without one is not a baseline — it is a clock that started before anyone
// asked what it was measuring against.
func Baseline(rows []LedgerRow, root string) (LedgerRow, bool) {
	var best LedgerRow
	found := false
	for _, r := range rows {
		if r.Kind != KindClock || r.RepoRoot != root {
			continue
		}
		if len(r.BaselineDays) == 0 && r.BaselineNote == "" {
			continue
		}
		if !found || r.At > best.At {
			best, found = r, true
		}
	}
	return best, found
}

// RecordBaseline appends the K1 baseline as a clock row, once. Re-running
// the audit must not append a new baseline every time: the measurement is
// over git history that only moves forward, and a file full of near-identical
// rows makes the one that matters unfindable.
//
// days may be empty when nothing was measurable — then note must say why,
// and the row is still written. "We looked and found nothing" is a result;
// silence is not.
func RecordBaseline(root, commit, at string, days []int, median int, note string) error {
	if _, ok := Baseline(ReadLedger(), root); ok {
		return nil
	}
	if len(days) == 0 && note == "" {
		return fmt.Errorf("intent: refusing a baseline with neither measurements nor a reason")
	}
	return AppendLedger(LedgerRow{
		Kind: KindClock, RepoRoot: root, Commit: commit, At: at,
		BaselineDays: days, BaselineMedian: median, BaselineNote: note,
	})
}
