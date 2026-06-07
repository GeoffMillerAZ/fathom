package store

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// MemoryStore is an in-process Store implementation backed by plain Go maps.
// It is intended for unit tests and session-mode operation where persistence is
// not required. All operations are goroutine-safe.
type MemoryStore struct {
	mu             sync.RWMutex
	observations   []core.Observation
	fingerprints   map[fpKey]string
	knowledgeCards map[kcKey]KnowledgeCard
	docSummaries   map[fpKey]DocSummary
}

type fpKey struct {
	serviceID string
	sourceRef string
}

type kcKey struct {
	serviceID string
	symbol    string
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		fingerprints:   make(map[fpKey]string),
		knowledgeCards: make(map[kcKey]KnowledgeCard),
		docSummaries:   make(map[fpKey]DocSummary),
	}
}

// PutObservations appends the supplied observations. IngestedAt is set to now
// when the caller leaves it as the zero value.
func (m *MemoryStore) PutObservations(_ context.Context, obs []core.Observation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	for _, o := range obs {
		if o.IngestedAt.IsZero() {
			o.IngestedAt = now
		}
		m.observations = append(m.observations, o)
	}
	return nil
}

// QueryObservations returns observations matching serviceID and, when
// concernID is non-empty, also matching concernID.
func (m *MemoryStore) QueryObservations(_ context.Context, serviceID, concernID string) ([]core.Observation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []core.Observation
	for _, o := range m.observations {
		if o.ServiceID != serviceID {
			continue
		}
		if concernID != "" && o.ConcernID != concernID {
			continue
		}
		result = append(result, o)
	}
	return result, nil
}

// Fingerprint returns the stored hash for (serviceID, sourceRef).
func (m *MemoryStore) Fingerprint(_ context.Context, serviceID, sourceRef string) (string, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h, ok := m.fingerprints[fpKey{serviceID, sourceRef}]
	return h, ok, nil
}

// SetFingerprint records or replaces the hash for (serviceID, sourceRef).
func (m *MemoryStore) SetFingerprint(_ context.Context, serviceID, sourceRef, hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fingerprints[fpKey{serviceID, sourceRef}] = hash
	return nil
}

// PurgeSource removes all observations for (serviceID, sourceRef).
func (m *MemoryStore) PurgeSource(_ context.Context, serviceID, sourceRef string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	kept := m.observations[:0]
	for _, o := range m.observations {
		if o.ServiceID == serviceID && o.SourceRef == sourceRef {
			continue
		}
		kept = append(kept, o)
	}
	m.observations = kept
	return nil
}

// PutKnowledgeCards upserts knowledge-card rows keyed by (service_id, symbol).
func (m *MemoryStore) PutKnowledgeCards(_ context.Context, cards []KnowledgeCard) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range cards {
		// Copy the Tags slice so the stored value is independent of the caller's slice.
		var tags []string
		if c.Tags != nil {
			tags = make([]string, len(c.Tags))
			copy(tags, c.Tags)
		}
		m.knowledgeCards[kcKey{c.ServiceID, c.Symbol}] = KnowledgeCard{
			ServiceID: c.ServiceID,
			Symbol:    c.Symbol,
			File:      c.File,
			Summary:   c.Summary,
			Tags:      tags,
		}
	}
	return nil
}

// KnowledgeCards returns all knowledge cards for serviceID ordered by symbol.
func (m *MemoryStore) KnowledgeCards(_ context.Context, serviceID string) ([]KnowledgeCard, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []KnowledgeCard
	for k, c := range m.knowledgeCards {
		if k.serviceID != serviceID {
			continue
		}
		// Return a copy so callers cannot mutate stored state.
		var tags []string
		if c.Tags != nil {
			tags = make([]string, len(c.Tags))
			copy(tags, c.Tags)
		}
		result = append(result, KnowledgeCard{
			ServiceID: c.ServiceID,
			Symbol:    c.Symbol,
			File:      c.File,
			Summary:   c.Summary,
			Tags:      tags,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Symbol < result[j].Symbol })
	return result, nil
}

// PutDocSummaries upserts doc-summary rows keyed by (service_id, source_ref).
func (m *MemoryStore) PutDocSummaries(_ context.Context, summaries []DocSummary) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range summaries {
		m.docSummaries[fpKey{d.ServiceID, d.SourceRef}] = d
	}
	return nil
}

// DocSummaries returns all doc summaries for serviceID ordered by source_ref.
func (m *MemoryStore) DocSummaries(_ context.Context, serviceID string) ([]DocSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []DocSummary
	for k, d := range m.docSummaries {
		if k.serviceID != serviceID {
			continue
		}
		result = append(result, d)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].SourceRef < result[j].SourceRef })
	return result, nil
}

// ListServiceIDs returns the sorted distinct set of service IDs present in the
// store.
func (m *MemoryStore) ListServiceIDs(_ context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	seen := make(map[string]struct{}, 4)
	for _, o := range m.observations {
		seen[o.ServiceID] = struct{}{}
	}
	if len(seen) == 0 {
		return nil, nil
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

// Close is a no-op for the in-memory store.
func (m *MemoryStore) Close() error { return nil }
