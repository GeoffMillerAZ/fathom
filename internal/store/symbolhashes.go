package store

// symbolhashes.go — the KR-6 doc-link pass's one read of a service's
// symbols (EW-49, wave ic; O362). The link pass resolved each distinct
// span name with ObservationsForSymbol, whose `symbol GLOB '*.name'`
// cannot range idx_observations_service_symbol, so every name scanned
// every observation of the service: names times symbols, the cost that
// made a whole-repository ingest 1.93 times its subjects' sum. The pass
// now reads the service's (symbol, source_ref, content_hash) rows once and
// resolves every name from them (internal/docindex's symbolIndex).

import (
	"context"
	"fmt"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// SymbolHash is one observation's symbol, source ref and content hash.
type SymbolHash struct {
	Symbol      string
	SourceRef   string
	ContentHash string
}

// SymbolHashes is every observation of serviceID as a SymbolHash, in the
// order ObservationsForSymbol returns a name's candidates (ingested_at,
// stable over rowid), through the same read chokepoint: a suppressed
// file's rows are not returned. ObservationsForSymbol(name) is exactly the
// rows of this list whose symbol is a candidate for name, in this order.
func (s *SQLiteStore) SymbolHashes(ctx context.Context, serviceID string) ([]SymbolHash, error) {
	rows, err := s.symbolHashesRaw(ctx, serviceID)
	return filterScoped(ctx, s.gate, serviceID, rows, err, func(r SymbolHash) string { return r.SourceRef })
}

func (s *SQLiteStore) symbolHashesRaw(ctx context.Context, serviceID string) ([]SymbolHash, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT symbol, source_ref, content_hash, ingested_at FROM observations
WHERE service_id = ? ORDER BY rowid`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("SymbolHashes: %w", err)
	}
	defer rows.Close()
	type row struct {
		SymbolHash
		ingestedAt int64
	}
	var all []row
	gov := tenancy.NewRowGovernor(ctx, tenancy.ClassIndex)
	for rows.Next() {
		if err := gov.Row(); err != nil {
			return nil, err
		}
		var r row
		if err := rows.Scan(&r.Symbol, &r.SourceRef, &r.ContentHash, &r.ingestedAt); err != nil {
			return nil, fmt.Errorf("SymbolHashes scan: %w", err)
		}
		all = append(all, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("SymbolHashes rows: %w", err)
	}
	// sortIngested's order: ingested_at, stable over rowid.
	sort.SliceStable(all, func(i, j int) bool { return all[i].ingestedAt < all[j].ingestedAt })
	out := make([]SymbolHash, len(all))
	for i, r := range all {
		out[i] = r.SymbolHash
	}
	return out, nil
}
