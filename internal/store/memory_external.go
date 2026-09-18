package store

// memory_external.go — MemoryStore's counterpart to sqlite_external.go
// (plan FL-7): the findings-loop's per-claim cursor table, in-process.

import (
	"context"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// PutExternalFinding upserts the cursor row for (f.ServiceID, f.ClaimID).
func (m *MemoryStore) PutExternalFinding(_ context.Context, f core.ExternalFinding) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.externalFindings == nil {
		m.externalFindings = make(map[claimKey]core.ExternalFinding)
	}
	m.externalFindings[claimKey{serviceID: f.ServiceID, claimID: f.ClaimID}] = f
	return nil
}

// ExternalFinding returns the cursor row for (serviceID, claimID). found is
// false when no such row exists.
func (m *MemoryStore) ExternalFinding(_ context.Context, serviceID, claimID string) (core.ExternalFinding, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	f, ok := m.externalFindings[claimKey{serviceID: serviceID, claimID: claimID}]
	return f, ok, nil
}

// ExternalFindingsForService returns every cursor row for serviceID, sorted
// by claim_id.
func (m *MemoryStore) ExternalFindingsForService(_ context.Context, serviceID string) ([]core.ExternalFinding, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []core.ExternalFinding
	for k, f := range m.externalFindings {
		if k.serviceID != serviceID {
			continue
		}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ClaimID < out[j].ClaimID })
	return out, nil
}
