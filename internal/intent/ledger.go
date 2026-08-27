package intent

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
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
}

// LedgerSchemaVersion is bumped when a row's meaning changes, never when a
// field is added (readers tolerate unknown fields by construction).
const LedgerSchemaVersion = 1

// LedgerPath is the intent ledger's location. FATHOM_INTENT_LEDGER
// overrides it, which is how tests stay off a developer's real ledger.
func LedgerPath() string {
	if p := os.Getenv("FATHOM_INTENT_LEDGER"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".fathom", "intent-ledger.jsonl")
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
