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

// SymbolSource is one indexed declaration of a symbol: its package and the
// source_ref (file[:line]) it was declared at.
type SymbolSource struct {
	Package   string
	Symbol    string
	SourceRef string
}

// symbolSourcesChunk bounds one IN (...) list (SQLite's default host
// parameter limit is far above this; the chunk keeps statements small).
const symbolSourcesChunk = 200

// sortSymbolSources orders rows by (symbol, package, source_ref) so every
// consumer sees the same order whatever the store returned.
func sortSymbolSources(rows []SymbolSource) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Symbol != rows[j].Symbol {
			return rows[i].Symbol < rows[j].Symbol
		}
		if rows[i].Package != rows[j].Package {
			return rows[i].Package < rows[j].Package
		}
		return rows[i].SourceRef < rows[j].SourceRef
	})
}

func distinctNonEmpty(names []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, n := range names {
		if n != "" && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// symbolSourcesForNamesRaw reads the (package, symbol, source_ref) rows
// declaring any of names — only those columns, only those rows. Until the
// (service_id, symbol) index lands (design §6, wave C) SQLite walks the
// service's rows to find them, but no other row is materialized.
func (s *SQLiteStore) symbolSourcesForNamesRaw(ctx context.Context, serviceID string, names []string) ([]SymbolSource, error) {
	names = distinctNonEmpty(names)
	var out []SymbolSource
	for start := 0; start < len(names); start += symbolSourcesChunk {
		end := min(start+symbolSourcesChunk, len(names))
		chunk := names[start:end]
		args := make([]any, 0, len(chunk)+1)
		args = append(args, serviceID)
		for _, n := range chunk {
			args = append(args, n)
		}
		q := `SELECT package, symbol, source_ref FROM observations
WHERE service_id = ? AND source_ref != '' AND symbol IN (?` + strings.Repeat(",?", len(chunk)-1) + `)`
		rows, err := s.db.QueryContext(ctx, q, args...)
		if err != nil {
			return nil, fmt.Errorf("SymbolSourcesForNames query: %w", err)
		}
		for rows.Next() {
			var r SymbolSource
			if err := rows.Scan(&r.Package, &r.Symbol, &r.SourceRef); err != nil {
				rows.Close()
				return nil, fmt.Errorf("SymbolSourcesForNames scan: %w", err)
			}
			out = append(out, r)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	sortSymbolSources(out)
	return out, nil
}

func (m *MemoryStore) symbolSourcesForNamesRaw(_ context.Context, serviceID string, names []string) ([]SymbolSource, error) {
	want := map[string]bool{}
	for _, n := range distinctNonEmpty(names) {
		want[n] = true
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []SymbolSource
	for _, o := range m.observations {
		if o.ServiceID == serviceID && o.SourceRef != "" && want[o.Symbol] {
			out = append(out, SymbolSource{Package: o.Package, Symbol: o.Symbol, SourceRef: o.SourceRef})
		}
	}
	sortSymbolSources(out)
	return out, nil
}
