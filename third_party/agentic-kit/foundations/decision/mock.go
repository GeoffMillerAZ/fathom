package decision

import (
	"context"
	"sync"
)

// MockEntry is one scripted response or error for MockDecider.
type MockEntry struct {
	// Response is returned if Err is nil.
	Response Response
	// Err is returned instead of Response when non-nil.
	Err error
}

// MockDecider is a deterministic, scriptable Decider for use in tests that
// depend on the Decider interface without exercising a real provider.
//
// Concurrency: MockDecider is safe for concurrent use.
type MockDecider struct {
	mu      sync.Mutex
	entries []MockEntry
	calls   []Request
	cursor  int
	name    string
}

// NewMockDecider creates a MockDecider pre-loaded with entries, reporting
// name from Name() (e.g. "mock").
func NewMockDecider(name string, entries ...MockEntry) *MockDecider {
	return &MockDecider{name: name, entries: append([]MockEntry(nil), entries...)}
}

// Name returns the name given to NewMockDecider.
func (m *MockDecider) Name() string { return m.name }

// AddEntry appends a scripted response or error.
func (m *MockDecider) AddEntry(e MockEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, e)
}

// Calls returns a snapshot of every Request passed to Decide so far, in
// call order.
func (m *MockDecider) Calls() []Request {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]Request, len(m.calls))
	copy(cp, m.calls)
	return cp
}

// Decide records req and returns the next scripted entry. When entries are
// exhausted, it returns an empty Response naming Provider m.name and a nil
// error (an empty, valid batch — every question simply unanswered).
func (m *MockDecider) Decide(_ context.Context, req Request) (Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls = append(m.calls, req)
	if m.cursor >= len(m.entries) {
		return Response{Provider: m.name}, nil
	}
	e := m.entries[m.cursor]
	m.cursor++
	return e.Response, e.Err
}
