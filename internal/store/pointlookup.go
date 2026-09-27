package store

// pointlookup.go — find_symbol and find_callers as SQL point lookups
// (RT-85, docs/design/responsible-tenant.md §4.5): the candidates for a
// name come from the (service_id, symbol) index and the observation_calls
// index, never from decoding the service's whole observation set.

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// SymbolCandidates is every name a stored symbol may carry to match name
// under query.FindSymbol's rule: name itself, and each dotted suffix of it
// ("a.b.C" → "b.C", "C") — a stored bare symbol a qualified query names.
// The third reading (a stored "T.M" matching the bare query "M") is the
// ".name" suffix, which the lookup tests on the index column.
func SymbolCandidates(name string) []string {
	out := []string{name}
	for i := 0; i < len(name); i++ {
		if name[i] == '.' && i < len(name)-1 {
			out = append(out, name[i+1:])
		}
	}
	return out
}

// globEscape quotes GLOB's metacharacters so s matches literally.
func globEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '*', '?', '[':
			b.WriteByte('[')
			b.WriteRune(r)
			b.WriteByte(']')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

const observationCols = `service_id, concern_id, source_ref, source_kind, evidence_tier,
       symbol, package, claim, content_hash, calls, complexity, line, end_line, ingested_at,
       tool_kind, generated`

// observationsForSymbolRaw reads the observations whose symbol equals one
// of SymbolCandidates(name) or ends in "."+name: the candidate rowids come
// from a covering scan of the (service_id, symbol) index (GLOB is
// case-sensitive, unlike LIKE), then only those rows are decoded.
func (s *SQLiteStore) observationsForSymbolRaw(ctx context.Context, serviceID, name string) ([]core.Observation, error) {
	if name == "" {
		return nil, nil
	}
	cands := SymbolCandidates(name)
	args := make([]any, 0, len(cands)+2)
	args = append(args, serviceID)
	for _, c := range cands {
		args = append(args, c)
	}
	args = append(args, "*."+globEscape(name))
	rows, err := s.db.QueryContext(ctx, `SELECT `+observationCols+` FROM observations WHERE rowid IN (
SELECT rowid FROM observations INDEXED BY idx_observations_service_symbol
WHERE service_id = ? AND (symbol IN (`+placeholders(len(cands))+`) OR symbol GLOB ?))
ORDER BY rowid`, args...)
	if err != nil {
		return nil, fmt.Errorf("ObservationsForSymbol: %w", err)
	}
	defer rows.Close()
	return sortIngested(scanObservations(ctx, rows))
}

// observationsCallingRaw reads the observations whose calls carry symbol
// or a key whose final name segment is symbol's — a candidate superset the
// engine narrows — through observation_calls. indexed is false when the
// service's call index is not complete (the caller keeps its scan).
func (s *SQLiteStore) observationsCallingRaw(ctx context.Context, serviceID, symbol string) ([]core.Observation, bool, error) {
	var indexed bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM observation_calls_state WHERE service_id = ?)`, serviceID).Scan(&indexed)
	if isNoSuchTable(err) || (err == nil && !indexed) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("ObservationsCalling state: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+observationCols+` FROM observations WHERE service_id = ? AND rowid IN (
SELECT caller_rowid FROM observation_calls WHERE service_id = ? AND (callee_key = ? OR callee_name = ?))
ORDER BY rowid`, serviceID, serviceID, symbol, CalleeName(symbol))
	if isNoSuchTable(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("ObservationsCalling: %w", err)
	}
	defer rows.Close()
	obs, err := sortIngested(scanObservations(ctx, rows))
	return obs, err == nil, err
}

// sortIngested is queryObservationsRaw's order: ingested_at, stable over
// rowid (insertion) order.
func sortIngested(obs []core.Observation, err error) ([]core.Observation, error) {
	if err != nil {
		return nil, err
	}
	sort.SliceStable(obs, func(i, j int) bool { return obs[i].IngestedAt.Before(obs[j].IngestedAt) })
	return obs, nil
}

func (m *MemoryStore) observationsForSymbolRaw(_ context.Context, serviceID, name string) ([]core.Observation, error) {
	if name == "" {
		return nil, nil
	}
	want := map[string]bool{}
	for _, c := range SymbolCandidates(name) {
		want[c] = true
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []core.Observation
	for _, o := range m.observations {
		if o.ServiceID == serviceID && (want[o.Symbol] || strings.HasSuffix(o.Symbol, "."+name)) {
			out = append(out, o)
		}
	}
	return out, nil
}

func (m *MemoryStore) observationsCallingRaw(_ context.Context, serviceID, symbol string) ([]core.Observation, bool, error) {
	name := CalleeName(symbol)
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []core.Observation
	for _, o := range m.observations {
		if o.ServiceID != serviceID {
			continue
		}
		for _, c := range o.Calls {
			if c == symbol || CalleeName(c) == name {
				out = append(out, o)
				break
			}
		}
	}
	return out, true, nil
}

// ObservationsForSymbol filters by observations.source_ref.
func (s *SQLiteStore) ObservationsForSymbol(ctx context.Context, serviceID, name string) ([]core.Observation, error) {
	obs, err := s.observationsForSymbolRaw(ctx, serviceID, name)
	return filterScoped(ctx, s.gate, serviceID, obs, err, func(o core.Observation) string { return o.SourceRef })
}

// ObservationsCalling filters by observations.source_ref.
func (s *SQLiteStore) ObservationsCalling(ctx context.Context, serviceID, symbol string) ([]core.Observation, bool, error) {
	obs, indexed, err := s.observationsCallingRaw(ctx, serviceID, symbol)
	obs, err = filterScoped(ctx, s.gate, serviceID, obs, err, func(o core.Observation) string { return o.SourceRef })
	return obs, indexed, err
}

// ObservationsForSymbol filters by observations.source_ref.
func (m *MemoryStore) ObservationsForSymbol(ctx context.Context, serviceID, name string) ([]core.Observation, error) {
	obs, err := m.observationsForSymbolRaw(ctx, serviceID, name)
	return filterScoped(ctx, m.gate, serviceID, obs, err, func(o core.Observation) string { return o.SourceRef })
}

// ObservationsCalling filters by observations.source_ref.
func (m *MemoryStore) ObservationsCalling(ctx context.Context, serviceID, symbol string) ([]core.Observation, bool, error) {
	obs, indexed, err := m.observationsCallingRaw(ctx, serviceID, symbol)
	obs, err = filterScoped(ctx, m.gate, serviceID, obs, err, func(o core.Observation) string { return o.SourceRef })
	return obs, indexed, err
}
