// Package embeddingprovenance tracks where embeddings came from: which model
// produced them, from which source document, and when.
// The Go types mirror the CUE definitions in schema.cue.
// CUE is authoritative for data shape; Go is authoritative for behaviour.
//
// Concurrency contract: Tracker is safe for concurrent use; all methods are
// guarded by a sync.RWMutex. EmbeddingProvenance is a plain value type safe
// to read concurrently without additional synchronization.
package embeddingprovenance

import (
	"fmt"
	"sync"
	"time"
)

// EmbeddingProvenance captures the origin of a stored embedding vector.
// It mirrors #EmbeddingProvenance in schema.cue.
type EmbeddingProvenance struct {
	// VectorID is the unique identifier of the stored vector.
	VectorID string

	// Model is the name/version of the model that produced the embedding
	// (e.g. "text-embedding-ada-002", "bge-base-en-v1.5").
	Model string

	// SourceID is the identifier of the source document or chunk that was
	// embedded (e.g. a document UUID or chunk key).
	SourceID string

	// CreatedAt is the Unix timestamp (seconds) when the embedding was created.
	CreatedAt int64

	// Dims is the dimensionality of the embedding vector (e.g. 768, 1536).
	Dims int
}

// Validate checks that the provenance record satisfies all invariants:
//   - VectorID, Model, SourceID must be non-empty.
//   - CreatedAt must be positive (> 0).
//   - Dims must be positive (> 0).
func (ep EmbeddingProvenance) Validate() error {
	if ep.VectorID == "" {
		return fmt.Errorf("embeddingprovenance: VectorID must not be empty")
	}
	if ep.Model == "" {
		return fmt.Errorf("embeddingprovenance: Model must not be empty")
	}
	if ep.SourceID == "" {
		return fmt.Errorf("embeddingprovenance: SourceID must not be empty")
	}
	if ep.CreatedAt <= 0 {
		return fmt.Errorf("embeddingprovenance: CreatedAt must be positive, got %d", ep.CreatedAt)
	}
	if ep.Dims <= 0 {
		return fmt.Errorf("embeddingprovenance: Dims must be positive, got %d", ep.Dims)
	}
	return nil
}

// Tracker records and retrieves EmbeddingProvenance entries.
// The zero value is not usable; create one with NewTracker.
type Tracker struct {
	mu      sync.RWMutex
	byVecID map[string]EmbeddingProvenance
}

// NewTracker returns an empty, ready-to-use Tracker.
func NewTracker() *Tracker {
	return &Tracker{
		byVecID: make(map[string]EmbeddingProvenance),
	}
}

// Record validates ep and stores it. Returns an error if ep is invalid or if
// a different provenance record is already stored for ep.VectorID.
func (t *Tracker) Record(ep EmbeddingProvenance) error {
	if err := ep.Validate(); err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if existing, found := t.byVecID[ep.VectorID]; found {
		if existing != ep {
			return fmt.Errorf("embeddingprovenance: conflicting record for VectorID %q", ep.VectorID)
		}
		// Idempotent re-record of identical entry is allowed.
		return nil
	}
	t.byVecID[ep.VectorID] = ep
	return nil
}

// Lookup returns the provenance record for vectorID, and whether it was found.
func (t *Tracker) Lookup(vectorID string) (EmbeddingProvenance, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	ep, ok := t.byVecID[vectorID]
	return ep, ok
}

// All returns a snapshot of all recorded provenance entries.
func (t *Tracker) All() []EmbeddingProvenance {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]EmbeddingProvenance, 0, len(t.byVecID))
	for _, ep := range t.byVecID {
		out = append(out, ep)
	}
	return out
}

// NowUnix is a replaceable clock for testing.
// It returns the current time as a Unix timestamp in seconds.
var NowUnix = func() int64 { return time.Now().Unix() }
