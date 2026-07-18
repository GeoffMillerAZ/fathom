// Package operatorelicitation tracks elicitation requests directed at a human
// operator and their answers.
//
// # Composition status
//
// Composition unproven — no consumer has wired this package into a kit or
// application workflow. The known real-world consumer (throughline) built its
// own elicitation layer (app/elicitation) with typed card kinds, form field
// validation, criticality ordering, and a ledger-backed status FSM. That shape
// is domain-specific and imports kit foundations directly, so absorbing it here
// would require org-specific types. See D2 (no org names in kit). If a
// second consumer emerges that needs a generic ask/answer primitive, this
// package is the natural home; until then treat it as provisional.
//
// # Usage
//
//	e := NewElicitor()
//	id := e.Ask("What is the preferred output format?")
//	// ... operator answers later ...
//	e.Answer(id, "Markdown")
//	pending := e.Pending() // []Request with Resolved==false
//
// Determinism: Pending() returns requests in the order they were asked.
// Ask/Answer are deterministic given the same call sequence.
//
// An Elicitor is not safe for concurrent use; guard it with your own
// synchronization when asking or answering from multiple goroutines.
package operatorelicitation

import (
	"errors"
	"fmt"
)

// ErrNotFound is returned when Answer references an ID that was never asked.
var ErrNotFound = errors.New("operatorelicitation: request not found")

// ErrAlreadyResolved is returned when Answer is called on an already-resolved
// request.
var ErrAlreadyResolved = errors.New("operatorelicitation: request already resolved")

// Request is a single elicitation request directed at a human operator.
type Request struct {
	// ID is the stable, unique identifier assigned by Ask.
	ID string

	// Prompt is the question or instruction presented to the operator.
	Prompt string

	// Answer is the operator's response, or "" if not yet answered.
	Answer string

	// Resolved is true once the operator has provided an answer.
	Resolved bool
}

// Elicitor manages a set of operator elicitation requests.
// It is not safe for concurrent use from multiple goroutines.
type Elicitor struct {
	order   []string
	items   map[string]*Request
	counter int
}

// NewElicitor returns an empty Elicitor.
func NewElicitor() *Elicitor {
	return &Elicitor{items: make(map[string]*Request)}
}

// Ask registers a new elicitation request with the given prompt and returns
// its assigned ID. IDs are stable, monotonically increasing strings of the
// form "req-1", "req-2", etc.
func (e *Elicitor) Ask(prompt string) string {
	e.counter++
	id := fmt.Sprintf("req-%d", e.counter)
	req := &Request{ID: id, Prompt: prompt}
	e.items[id] = req
	e.order = append(e.order, id)
	return id
}

// Answer records the operator's answer for the request with the given ID.
// Returns ErrNotFound if the ID was never asked.
// Returns ErrAlreadyResolved if the request is already resolved.
func (e *Elicitor) Answer(id, answer string) error {
	req, ok := e.items[id]
	if !ok {
		return ErrNotFound
	}
	if req.Resolved {
		return ErrAlreadyResolved
	}
	req.Answer = answer
	req.Resolved = true
	return nil
}

// Pending returns all requests that have not yet been resolved, in the order
// they were asked.
func (e *Elicitor) Pending() []Request {
	var out []Request
	for _, id := range e.order {
		req, ok := e.items[id]
		if ok && !req.Resolved {
			out = append(out, *req)
		}
	}
	if out == nil {
		out = []Request{}
	}
	return out
}

// Get returns a copy of the Request with the given ID and whether it was found.
func (e *Elicitor) Get(id string) (Request, bool) {
	req, ok := e.items[id]
	if !ok {
		return Request{}, false
	}
	return *req, true
}
