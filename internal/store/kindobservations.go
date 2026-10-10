package store

// kindobservations.go — EW-57 (wave h3): the doc-xref pass reads code for a
// Lambda handler only where the service declares a Lambda function, and it
// asks the store which of a few declaring kinds' symbols the service holds
// (a CloudFormation resource, a Terraform resource or module call) instead
// of re-reading every template. An optional capability, like
// FileImportsStore: an ingest-time read, never suppression-filtered. No
// table, index or cache of its own.

import (
	"context"
	"fmt"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// KindObservationsStore is the optional store capability behind the
// doc-xref pass's Lambda gate. Both SQLiteStore and MemoryStore implement
// it.
type KindObservationsStore interface {
	// ObservationsOfKinds returns serviceID's observations whose source
	// kind is one of kinds, ordered by source_ref, line, symbol: the rows
	// of those kinds only, never the service's whole observation set.
	ObservationsOfKinds(ctx context.Context, serviceID string, kinds []core.SourceKind) ([]core.Observation, error)
}

// ObservationsOfKinds implements KindObservationsStore: one read over the
// service's rows (idx_observations_service_id), only those of kinds decoded.
func (s *SQLiteStore) ObservationsOfKinds(ctx context.Context, serviceID string, kinds []core.SourceKind) ([]core.Observation, error) {
	if len(kinds) == 0 {
		return nil, nil
	}
	args := make([]any, 0, len(kinds)+1)
	args = append(args, serviceID)
	for _, k := range kinds {
		args = append(args, string(k))
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT `+observationCols+`
FROM observations
WHERE service_id = ? AND source_kind IN (`+strings.TrimSuffix(strings.Repeat("?,", len(kinds)), ",")+`)
`, args...)
	if err != nil {
		return nil, fmt.Errorf("ObservationsOfKinds: %w", err)
	}
	defer rows.Close()
	out, err := scanObservations(ctx, rows)
	if err != nil {
		return nil, err
	}
	sortObservationsBySource(out)
	return out, nil
}

// ObservationsOfKinds implements KindObservationsStore.
func (m *MemoryStore) ObservationsOfKinds(_ context.Context, serviceID string, kinds []core.SourceKind) ([]core.Observation, error) {
	want := make(map[core.SourceKind]bool, len(kinds))
	for _, k := range kinds {
		want[k] = true
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []core.Observation
	for _, o := range m.observations {
		if o.ServiceID == serviceID && want[o.SourceKind] {
			out = append(out, o)
		}
	}
	sortObservationsBySource(out)
	return out, nil
}
