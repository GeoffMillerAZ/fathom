package indexbench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// CompetitorLedgerRow is one docs/bench/competitors-ledger.jsonl line
// (SB-31): fathom's own row and a competitor's row are the same shape so a
// reader compares them without translation. Adjudicable is the number of
// queries the truth was entitled to rule on (Comparison.Queries /
// ToolScore.Queries) — the denominator a bare precision/recall pair needs
// context from.
type CompetitorLedgerRow struct {
	Arm          string         `json:"arm"`
	Version      string         `json:"version"`
	Repo         string         `json:"repo"`
	Commit       string         `json:"commit"`
	Precision    float64        `json:"precision"`
	Recall       float64        `json:"recall"`
	Adjudicable  int            `json:"adjudicable"`
	DroppedKinds map[string]int `json:"dropped_kinds,omitempty"`
	Date         string         `json:"date"`
}

// AppendCompetitorLedger appends rows to path as JSON Lines (one row per
// line), creating the file and any missing parent directory. It never
// rewrites an existing line — the ledger is a log, like docs/bench/
// ledger.jsonl.
func AppendCompetitorLedger(path string, rows []CompetitorLedgerRow) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("indexbench: create ledger dir for %s: %w", path, err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("indexbench: open ledger %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	for _, r := range rows {
		data, err := json.Marshal(r)
		if err != nil {
			return fmt.Errorf("indexbench: marshal ledger row for arm %q: %w", r.Arm, err)
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("indexbench: write ledger row for arm %q: %w", r.Arm, err)
		}
	}
	return nil
}
