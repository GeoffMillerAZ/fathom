package decision

import (
	"context"
	"slices"
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
	mu       sync.Mutex
	entries  []MockEntry
	calls    []Request
	cursor   int
	name     string
	caps     Capabilities
	validate bool
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
//
// With SetValidate(true), a req that fails Validate is refused first, as
// every shipped adapter refuses it: Decide returns Validate's error and an
// empty Response, and neither records req nor consumes an entry.
func (m *MockDecider) Decide(_ context.Context, req Request) (Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.validate {
		if err := Validate(req); err != nil {
			return Response{}, err
		}
	}
	m.calls = append(m.calls, req)
	if m.cursor >= len(m.entries) {
		return Response{Provider: m.name}, nil
	}
	e := m.entries[m.cursor]
	m.cursor++
	return e.Response, e.Err
}

// SetValidate turns request validation on or off; it is off by default, so
// an existing script sees every Request unchanged. On, Decide refuses a
// Request that fails Validate before the script, the way LLM and TypeSafe
// refuse it before their provider: the refusal is not recorded in Calls
// and consumes no entry. The decisiontest conformance suite runs a
// MockDecider with it on.
func (m *MockDecider) SetValidate(on bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.validate = on
}

// SetCapabilities sets the declaration Capabilities returns, so a test can
// exercise code that reads CapabilitiesOf. c is copied: mutating c.Kinds
// afterwards does not change the declaration. A MockDecider never given
// one is undeclared, and c.Declared false leaves it undeclared.
func (m *MockDecider) SetCapabilities(c Capabilities) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c.Kinds = slices.Clone(c.Kinds)
	m.caps = c
}

// Capabilities returns a copy of the declaration SetCapabilities set, or the
// zero Capabilities (Declared false) when it was never called. Read it
// through CapabilitiesOf, which canonicalizes it.
func (m *MockDecider) Capabilities() Capabilities {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := m.caps
	c.Kinds = slices.Clone(c.Kinds)
	return c
}
