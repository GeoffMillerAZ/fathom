package store

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// symbolRow is one (symbol, source_ref) pair SymbolNamesForSource reads.
type symbolRow struct {
	symbol string
	ref    string
}

// refNamesFile reports whether an observation's source_ref names file: the
// path itself or the path with a ":<line>" suffix.
func refNamesFile(ref, file string) bool {
	if ref == file {
		return true
	}
	if !strings.HasPrefix(ref, file+":") {
		return false
	}
	_, err := strconv.Atoi(ref[len(file)+1:])
	return err == nil
}

// distinctSymbolNames returns rows' distinct non-empty symbols, sorted.
func distinctSymbolNames(rows []symbolRow) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range rows {
		if r.symbol == "" || seen[r.symbol] {
			continue
		}
		seen[r.symbol] = true
		out = append(out, r.symbol)
	}
	sort.Strings(out)
	return out
}

// symbolRowsForSourceQuery reads file's candidate (symbol, source_ref) rows
// as ONE range [file, file+";") over the (service_id, source_ref) index —
// ';' is the byte after ':', so the range holds file itself and every
// "file:<line>" ref. refNamesFile then drops the other refs the range can
// hold (a sibling "file.bak", "file/x"). An OR'd equality-plus-range
// predicate plans on idx_observations_service_id alone when the store has
// no sqlite_stat tables (it never runs ANALYZE): a lookup of every
// observation in the service per obligations call
// (TestSymbolNamesForSource_QueryPlanRangesSourceRef).
const symbolRowsForSourceQuery = `
SELECT symbol, source_ref FROM observations
WHERE service_id = ? AND source_ref >= ? AND source_ref < ?
`

func symbolRowsForSourceArgs(serviceID, file string) []any {
	return []any{serviceID, file, file + ";"}
}

// symbolRowsForSourceRaw reads file's (symbol, source_ref) rows.
func (s *SQLiteStore) symbolRowsForSourceRaw(ctx context.Context, serviceID, file string) ([]symbolRow, error) {
	rows, err := s.db.QueryContext(ctx, symbolRowsForSourceQuery, symbolRowsForSourceArgs(serviceID, file)...)
	if err != nil {
		return nil, fmt.Errorf("SymbolNamesForSource query: %w", err)
	}
	defer rows.Close()
	var out []symbolRow
	for rows.Next() {
		var r symbolRow
		if err := rows.Scan(&r.symbol, &r.ref); err != nil {
			return nil, fmt.Errorf("SymbolNamesForSource scan: %w", err)
		}
		if refNamesFile(r.ref, file) {
			out = append(out, r)
		}
	}
	return out, rows.Err()
}

func (m *MemoryStore) symbolRowsForSourceRaw(_ context.Context, serviceID, file string) ([]symbolRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []symbolRow
	for _, o := range m.observations {
		if o.ServiceID == serviceID && refNamesFile(o.SourceRef, file) {
			out = append(out, symbolRow{symbol: o.Symbol, ref: o.SourceRef})
		}
	}
	return out, nil
}
