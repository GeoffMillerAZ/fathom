// Package investigationmemo provides a keyed store of investigation findings.
// Findings are persisted as Memo values behind the Store interface. The
// in-memory implementation (MemoryStore) is the default backend; a SQLite
// backend is a natural future addition — simply implement Store against the
// database/sql package without altering callers.
//
// SQLite future backend note:
//
//	A SQLite-backed Store would implement the same Store interface using
//	database/sql with a driver registered at init time (e.g.
//	modernc.org/sqlite). The schema would be a single table:
//	  CREATE TABLE memos (key TEXT PRIMARY KEY, summary TEXT NOT NULL,
//	                      detail TEXT NOT NULL);
//	No driver is imported here — stdlib only.
//
// Concurrency contract: MemoryStore is safe for concurrent use; all methods
// are guarded by a sync.RWMutex. Store implementations must also be safe for
// concurrent use.
package investigationmemo

import (
	"fmt"
	"sync"
)

// Memo is a single investigation finding stored under a unique key.
type Memo struct {
	// Key uniquely identifies this memo within the store. Must be non-empty.
	Key string

	// Summary is a short, human-readable title for the finding.
	// Must be non-empty.
	Summary string

	// Detail contains the full narrative or structured content of the finding.
	Detail string
}

// Store is the interface that every memo backend must satisfy.
type Store interface {
	// Put stores memo under memo.Key. If a memo with the same key already
	// exists it is overwritten. Returns an error if memo.Key or memo.Summary
	// is empty.
	Put(memo Memo) error

	// Get returns the memo stored under key and true, or the zero Memo and
	// false when no memo with that key exists.
	Get(key string) (Memo, bool)

	// List returns all stored memos in deterministic (lexicographic by Key)
	// order.
	List() []Memo
}

// MemoryStore is a thread-safe, in-memory implementation of Store. The zero
// value is ready to use.
type MemoryStore struct {
	mu    sync.RWMutex
	memos map[string]Memo
}

// Put implements Store. It returns an error if memo.Key or memo.Summary is
// empty.
func (s *MemoryStore) Put(memo Memo) error {
	if memo.Key == "" {
		return fmt.Errorf("investigationmemo: Key must not be empty")
	}
	if memo.Summary == "" {
		return fmt.Errorf("investigationmemo: Summary must not be empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.memos == nil {
		s.memos = make(map[string]Memo)
	}
	s.memos[memo.Key] = memo
	return nil
}

// Get implements Store.
func (s *MemoryStore) Get(key string) (Memo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.memos[key]
	return m, ok
}

// List implements Store. Returns memos in lexicographic Key order.
func (s *MemoryStore) List() []Memo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.memos) == 0 {
		return nil
	}
	keys := make([]string, 0, len(s.memos))
	for k := range s.memos {
		keys = append(keys, k)
	}
	sortStrings(keys)
	out := make([]Memo, len(keys))
	for i, k := range keys {
		out[i] = s.memos[k]
	}
	return out
}

// sortStrings sorts a slice of strings in-place using insertion sort.
func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		key := ss[i]
		j := i - 1
		for j >= 0 && ss[j] > key {
			ss[j+1] = ss[j]
			j--
		}
		ss[j+1] = key
	}
}
