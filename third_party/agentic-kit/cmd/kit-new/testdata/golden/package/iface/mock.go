package widgetgizmo

import (
	"sync"
	"time"
)

// MockStamper is a scriptable Stamper for testing code that consumes the
// interface. Each Stamp consumes the next scripted time; once the script is
// exhausted every further call returns Fallback. It is safe for concurrent
// use.
type MockStamper struct {
	mu     sync.Mutex
	script []time.Time
	idx    int
	calls  int

	// Fallback is returned once the script is exhausted (zero value: the
	// zero time).
	Fallback time.Time
}

// NewMockStamper returns a MockStamper that yields the given times in order.
func NewMockStamper(script ...time.Time) *MockStamper {
	return &MockStamper{script: script}
}

// Stamp records the call and returns the next scripted time.
func (m *MockStamper) Stamp() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.idx < len(m.script) {
		t := m.script[m.idx]
		m.idx++
		return t
	}
	return m.Fallback
}

// Calls returns how many times Stamp was invoked.
func (m *MockStamper) Calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// compile-time interface check.
var _ Stamper = (*MockStamper)(nil)
