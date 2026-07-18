package proofofwork

import (
	"sync"
	"time"
)

// MockSeenSetCall records one TryAdd invocation on a MockSeenSet.
type MockSeenSetCall struct {
	// Key is the key passed to TryAdd.
	Key string

	// ExpiresAt is the expiry passed to TryAdd.
	ExpiresAt time.Time
}

// MockSeenSet is a scriptable SeenSet for testing code that consumes the
// interface (e.g. exercising the fail-closed AddAtCapacity path without
// filling a real set). It is concurrency-safe.
//
// Each TryAdd consumes the next scripted result; once the script is
// exhausted, every further call returns Fallback.
type MockSeenSet struct {
	mu     sync.Mutex
	script []AddResult
	idx    int
	calls  []MockSeenSetCall

	// Fallback is returned once the script is exhausted (zero value AddOK).
	Fallback AddResult
}

// NewMockSeenSet returns a MockSeenSet that yields the given results in order.
func NewMockSeenSet(script ...AddResult) *MockSeenSet {
	return &MockSeenSet{script: script}
}

// TryAdd records the call and returns the next scripted result.
func (m *MockSeenSet) TryAdd(key string, expiresAt time.Time) AddResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, MockSeenSetCall{Key: key, ExpiresAt: expiresAt})
	if m.idx < len(m.script) {
		r := m.script[m.idx]
		m.idx++
		return r
	}
	return m.Fallback
}

// Calls returns a copy of the recorded TryAdd invocations.
func (m *MockSeenSet) Calls() []MockSeenSetCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]MockSeenSetCall, len(m.calls))
	copy(out, m.calls)
	return out
}

// compile-time interface check.
var _ SeenSet = (*MockSeenSet)(nil)
