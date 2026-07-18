package eventstore

import (
	"fmt"
	"sync"
	"time"
)

// Store is an in-memory, append-only, hash-chained event store. Every Append
// call links the new entry to its predecessor via a SHA-256 hash chain;
// Verify re-derives every hash from stored payloads and detects any tampering.
//
// Store is safe for concurrent use: Append, Entries, Head, Len, Verify, and
// Replay all acquire the appropriate mutex lock.
//
// Persistence is opt-in: call AttachSink (defined in persistence.go) once at
// construction time to wire a durable backend (e.g. JSONLSink) before the
// first Append. The sink is called synchronously for every entry; a sink error
// aborts the Append and leaves the chain unchanged.
type Store struct {
	mu      sync.RWMutex
	entries []Entry
	now     func() time.Time
	// sink, when non-nil, receives a deep copy of each entry synchronously
	// before it is added to entries. A sink error aborts the Append.
	// Set via AttachSink in persistence.go.
	sink func(Entry) error
}

// NewStore returns a new, empty Store that uses time.Now().UTC() as its clock.
func NewStore() *Store {
	return &Store{now: func() time.Time { return time.Now().UTC() }}
}

// NewStoreWithClock returns a new, empty Store that uses the supplied clock
// function. Inject a deterministic clock in tests to produce reproducible
// entry hashes.
func NewStoreWithClock(clock func() time.Time) *Store {
	if clock == nil {
		panic("eventstore: NewStoreWithClock: clock must not be nil")
	}
	return &Store{now: clock}
}

// Append adds a new entry to the chain and returns it.
//
// The entry's fields are computed as follows:
//   - Seq = len(entries) before append (0-based).
//   - PayloadHash = SHA-256(payload).
//   - PrevHash = previous entry's Hash, or GenesisPrevHash if this is the first entry.
//   - RecordedAt = now() from the store's clock.
//   - Hash = canonical hash over (Seq, Kind, PayloadHash, PrevHash, RecordedAt).
//
// Append never mutates past entries.
func (s *Store) Append(kind string, payload []byte) (Entry, error) {
	if kind == "" {
		return Entry{}, fmt.Errorf("eventstore: Append: kind must not be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	seq := uint64(len(s.entries))
	prevHash := GenesisPrevHash
	if seq > 0 {
		prevHash = s.entries[seq-1].Hash
	}

	payloadHash := hashPayload(payload)
	recordedAt := s.now()
	entryHash := hashEntry(seq, kind, payloadHash, prevHash, recordedAt.UnixNano())

	// Copy payload to avoid retaining a reference to the caller's buffer.
	var payloadCopy []byte
	if len(payload) > 0 {
		payloadCopy = make([]byte, len(payload))
		copy(payloadCopy, payload)
	}

	e := Entry{
		Seq:         seq,
		Kind:        kind,
		Payload:     payloadCopy,
		PayloadHash: payloadHash,
		PrevHash:    prevHash,
		Hash:        entryHash,
		RecordedAt:  recordedAt,
	}

	// Deliver to sink (if any) while still holding the write lock. A deep copy
	// is sent so the sink cannot corrupt internal state; a sink error aborts
	// the append (the chain is unchanged).
	if s.sink != nil {
		sinkCopy := e
		if len(e.Payload) > 0 {
			pc := make([]byte, len(e.Payload))
			copy(pc, e.Payload)
			sinkCopy.Payload = pc
		}
		if err := s.sink(sinkCopy); err != nil {
			return Entry{}, fmt.Errorf("eventstore: Append: sink: %w", err)
		}
	}

	s.entries = append(s.entries, e)
	return e, nil
}

// Entries returns a defensive copy of every entry in the chain, in order.
// The caller may freely modify the returned slice, including Payload bytes,
// without affecting the Store.
func (s *Store) Entries() []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Entry, len(s.entries))
	for i, e := range s.entries {
		out[i] = e
		if len(e.Payload) > 0 {
			out[i].Payload = make([]byte, len(e.Payload))
			copy(out[i].Payload, e.Payload)
		}
	}
	return out
}

// Head returns a deep copy of the most recent entry and true, or a zero Entry
// and false if the store is empty. The caller may freely modify the returned
// Entry, including Payload bytes, without affecting the Store.
func (s *Store) Head() (Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.entries) == 0 {
		return Entry{}, false
	}
	e := s.entries[len(s.entries)-1]
	if len(e.Payload) > 0 {
		pc := make([]byte, len(e.Payload))
		copy(pc, e.Payload)
		e.Payload = pc
	}
	return e, true
}

// Len returns the number of entries in the store.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// Verify walks the entire chain and re-derives each entry's PayloadHash and
// Hash from stored data. It also checks:
//   - entry[0].PrevHash == GenesisPrevHash
//   - entry[i].PrevHash == entry[i-1].Hash for i > 0
//   - Seq values are contiguous from 0
//
// Returns a descriptive error identifying the first tampered or inconsistent
// entry, or nil if the chain is intact.
//
// Threat model: Verify detects any edit by a party who cannot recompute
// SHA-256, and any reordering, middle-drop, or Seq-gap. It does NOT detect
// authorized rewrites by a holder who recomputes the chain (e.g. backdating
// RecordedAt then recomputing all hashes), nor tail truncation (dropping the
// last N entries yields a valid shorter chain). Detecting those attacks
// requires anchoring Head().Hash and the entry count in an external trusted
// store (e.g. a transparency log or notary service).
func (s *Store) Verify() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i, e := range s.entries {
		// Check Seq is contiguous.
		if e.Seq != uint64(i) {
			return fmt.Errorf("eventstore: Verify: entry index %d has Seq %d (expected %d)", i, e.Seq, i)
		}

		// Check PrevHash linkage.
		if i == 0 {
			if e.PrevHash != GenesisPrevHash {
				return fmt.Errorf("eventstore: Verify: entry seq=0 PrevHash %q != GenesisPrevHash", e.PrevHash)
			}
		} else {
			if e.PrevHash != s.entries[i-1].Hash {
				return fmt.Errorf("eventstore: Verify: entry seq=%d PrevHash %q != previous entry Hash %q",
					e.Seq, e.PrevHash, s.entries[i-1].Hash)
			}
		}

		// Re-derive PayloadHash.
		wantPayloadHash := hashPayload(e.Payload)
		if e.PayloadHash != wantPayloadHash {
			return fmt.Errorf("eventstore: Verify: entry seq=%d PayloadHash mismatch: stored %q, computed %q",
				e.Seq, e.PayloadHash, wantPayloadHash)
		}

		// Re-derive entry Hash.
		wantHash := hashEntry(e.Seq, e.Kind, e.PayloadHash, e.PrevHash, e.RecordedAt.UnixNano())
		if e.Hash != wantHash {
			return fmt.Errorf("eventstore: Verify: entry seq=%d Hash mismatch: stored %q, computed %q",
				e.Seq, e.Hash, wantHash)
		}

		// Validate structural invariants. This ensures adversarially crafted
		// entries with valid hash chains but invalid fields (e.g. zero
		// RecordedAt, empty Kind) are rejected. Hash correctness is necessary
		// but not sufficient for chain integrity in a legal chain-of-custody
		// context.
		if err := e.Validate(); err != nil {
			return fmt.Errorf("eventstore: Verify: entry seq=%d failed validation: %w", e.Seq, err)
		}
	}
	return nil
}

// NewStoreFromEntries constructs a Store whose internal chain is pre-populated
// with the provided entries. Each entry is deep-copied — including its Payload
// bytes — so mutations to the caller's input slice after this call cannot
// corrupt the store's internal state. Verify can then be used to detect any
// tampering introduced before or after construction.
//
// This constructor is primarily intended for testing tamper-detection: build a
// valid chain via Append, retrieve Entries(), mutate specific fields, and pass
// the mutated slice here to confirm Verify catches the change.
func NewStoreFromEntries(entries []Entry) *Store {
	s := &Store{now: func() time.Time { return time.Now().UTC() }}
	s.entries = make([]Entry, len(entries))
	for i, e := range entries {
		s.entries[i] = e
		if len(e.Payload) > 0 {
			pc := make([]byte, len(e.Payload))
			copy(pc, e.Payload)
			s.entries[i].Payload = pc
		}
	}
	return s
}

// Replay invokes fn for each entry in chain order. If fn returns a non-nil
// error, Replay stops immediately and returns that error. fn is called with
// a deep copy of each entry — including Payload bytes — so fn cannot mutate
// the stored chain.
func (s *Store) Replay(fn func(Entry) error) error {
	s.mu.RLock()
	// Deep-copy all entries while holding the lock.
	entries := make([]Entry, len(s.entries))
	for i, e := range s.entries {
		entries[i] = e
		if len(e.Payload) > 0 {
			pc := make([]byte, len(e.Payload))
			copy(pc, e.Payload)
			entries[i].Payload = pc
		}
	}
	s.mu.RUnlock()

	for _, e := range entries {
		if err := fn(e); err != nil {
			return err
		}
	}
	return nil
}
