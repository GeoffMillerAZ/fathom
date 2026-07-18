// Package memorymanifest provides an append-only manifest of stored knowledge
// units. Each entry in the manifest records a card ID, a content hash, and
// the creation timestamp. The manifest never modifies or deletes past entries.
//
// The Manifest type is safe to use from a single goroutine; it is not
// concurrency-safe on its own. Callers that share a Manifest across goroutines
// must supply their own synchronisation.
package memorymanifest

import "time"

// ManifestEntry records a single knowledge card that has been stored.
//
// CUE mirror: #ManifestEntry in schema.cue.
type ManifestEntry struct {
	// CardID is the unique, stable identifier of the knowledge card.
	// Must be non-empty.
	CardID string

	// Hash is the deterministic content fingerprint of the card at
	// storage time (e.g. a hex-encoded SHA-256 of the canonical serialisation).
	// Must be non-empty.
	Hash string

	// CreatedAt is the wall-clock instant at which this entry was appended
	// to the manifest.
	CreatedAt time.Time
}

// Manifest is an append-only collection of ManifestEntry values.
// Once an entry is added it is never mutated or removed.
type Manifest struct {
	entries []ManifestEntry

	// Now is the clock used to stamp entries whose CreatedAt is zero.
	// nil means time.Now (documented default); inject a fixed clock for
	// deterministic tests.
	Now func() time.Time
}

// Add appends e to the manifest. If e.CreatedAt is the zero value it is set
// to the current UTC time from the manifest's clock before appending.
//
// Add never modifies previously appended entries.
func (m *Manifest) Add(e ManifestEntry) {
	if e.CreatedAt.IsZero() {
		now := m.Now
		if now == nil {
			now = time.Now
		}
		e.CreatedAt = now().UTC()
	}
	m.entries = append(m.entries, e)
}

// List returns a read-only snapshot of all entries in the manifest in the
// order they were added. The returned slice must not be modified by the caller.
func (m *Manifest) List() []ManifestEntry {
	out := make([]ManifestEntry, len(m.entries))
	copy(out, m.entries)
	return out
}

// Contains reports whether a ManifestEntry with the given cardID is already
// present in the manifest. The search is linear; manifests are expected to be
// small relative to workload latency requirements.
func (m *Manifest) Contains(cardID string) bool {
	for _, e := range m.entries {
		if e.CardID == cardID {
			return true
		}
	}
	return false
}
