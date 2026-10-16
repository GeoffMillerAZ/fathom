// Package evidenceresolver provides a deterministic registry that maps span
// reference strings to their Span values. Callers register spans by ID and
// later resolve them by that same ID.
//
// All outputs are deterministic: ResolveAll always returns results in the same
// order as the input refs slice.
//
// Concurrency contract: Resolver is not safe for concurrent use; callers that
// share a Resolver across goroutines must guard access with their own
// synchronization.
package evidenceresolver

import (
	es "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-span"
)

// Resolver maps span reference strings (IDs) to Span values.
// The zero value is ready to use.
type Resolver struct {
	m map[string]es.Span
}

// Register adds span to the resolver, keyed by span.ID. If a span with the
// same ID has already been registered, it is silently overwritten.
func (r *Resolver) Register(span es.Span) {
	if r.m == nil {
		r.m = make(map[string]es.Span)
	}
	r.m[span.ID] = span
}

// Resolve looks up the span identified by ref. It returns the span and true
// when found, or the zero Span and false otherwise.
func (r *Resolver) Resolve(ref string) (es.Span, bool) {
	if r.m == nil {
		return es.Span{}, false
	}
	s, ok := r.m[ref]
	return s, ok
}

// ResolveAll resolves each ref in order and returns a slice of (Span, found)
// pairs in the same order as the input. The output length always equals the
// input length, preserving positional correspondence.
func (r *Resolver) ResolveAll(refs []string) []Result {
	out := make([]Result, len(refs))
	for i, ref := range refs {
		s, ok := r.Resolve(ref)
		out[i] = Result{Span: s, Found: ok}
	}
	return out
}

// Result is a single entry in the slice returned by ResolveAll.
type Result struct {
	// Span is the resolved span (zero value when Found is false).
	Span es.Span
	// Found reports whether the ref was present in the registry.
	Found bool
}
