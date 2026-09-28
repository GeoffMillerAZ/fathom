package support

// ledger.go — AppendLedger (SM-8, design §3.2): every reading write appends
// one line to docs/bench/readings/support-ledger.jsonl. The file is
// append-only in the sense that a row, once written, is never dropped or
// mutated by a later call — but its ON-DISK order is kept sorted (by
// reading_date, then kind/id/capability) on every write, so a reader never
// has to sort a growing file itself (internal/CLAUDE.md: "sort every output
// slice").

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// AppendLedger validates row against the embedded #LedgerRow definition,
// reads path's existing rows (if any), adds row, and rewrites path with
// every row sorted by (reading_date, kind, id, capability) — the two
// callers design §3.2 names, `indexbench lang-oracle` and `fathom harness
// verify`, both call this rather than opening the file themselves.
func AppendLedger(path string, row LedgerRow) error {
	if err := validateLedgerRow(row); err != nil {
		return err
	}

	rows, err := readLedger(path)
	if err != nil {
		return err
	}
	rows = append(rows, row)
	sortLedger(rows)

	var buf strings.Builder
	for _, r := range rows {
		data, err := json.Marshal(r)
		if err != nil {
			return fmt.Errorf("support: marshal ledger row: %w", err)
		}
		buf.Write(data)
		buf.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(buf.String()), 0o644); err != nil {
		return fmt.Errorf("support: write %s: %w", path, err)
	}
	return nil
}

// LoadLedgerRows reads path's committed rows without appending anything —
// the read-only half of AppendLedger (SM-10, design §4), for a caller (the
// `fathom support` CLI) that only needs Project's regression history, never
// a write. A missing file is an empty ledger, never an error.
func LoadLedgerRows(path string) ([]LedgerRow, error) { return readLedger(path) }

// readLedger reads path's existing JSONL rows; a missing file is an empty
// ledger, never an error (the first AppendLedger call creates it).
func readLedger(path string) ([]LedgerRow, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("support: read %s: %w", path, err)
	}
	defer f.Close()

	var rows []LedgerRow
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var r LedgerRow
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return nil, fmt.Errorf("support: %s line %d: %w", path, lineNo, err)
		}
		rows = append(rows, r)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("support: scan %s: %w", path, err)
	}
	return rows, nil
}

// sortLedger sorts rows by (reading_date, kind, id, capability) in place —
// design §3.2's "sorted on write by date then key".
func sortLedger(rows []LedgerRow) {
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.ReadingDate != b.ReadingDate {
			return a.ReadingDate < b.ReadingDate
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		return a.Capability < b.Capability
	})
}

// validateLedgerRow unifies row (marshaled to JSON) against the embedded
// #LedgerRow definition.
func validateLedgerRow(row LedgerRow) error {
	data, err := json.Marshal(row)
	if err != nil {
		return fmt.Errorf("support: marshal ledger row: %w", err)
	}
	cctx := cuecontext.New()
	def, err := compileSupportDef(cctx, "#LedgerRow")
	if err != nil {
		return err
	}
	val := cctx.CompileBytes(data)
	if err := val.Err(); err != nil {
		return fmt.Errorf("support: compile ledger row: %w", err)
	}
	unified := def.Unify(val)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return fmt.Errorf("support: ledger row does not validate against #LedgerRow: %w", err)
	}
	return nil
}
