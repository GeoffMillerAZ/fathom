// Package mockresolver provides an in-memory depgraph Resolver that
// satisfies the same conformance suite as other resolvers.
package mockresolver

import (
	"context"
	"io/fs"

	"github.com/geoffmilleraz/fathom/pkg/depgraph"
)

// Resolver serves a fixed unit set from memory. The filesystem and
// profile handed to Units are ignored — the fixture IS the resolution.
type Resolver struct {
	ecosystem string
	fidelity  depgraph.Fidelity
	units     []depgraph.Unit
	err       error
}

// New returns a Resolver serving exactly the given units, in the given
// order, under the given ecosystem name and fidelity mark.
func New(ecosystem string, fidelity depgraph.Fidelity, units ...depgraph.Unit) *Resolver {
	return &Resolver{ecosystem: ecosystem, fidelity: fidelity, units: units}
}

// NewFailing returns a Resolver whose Units always fails — the error
// path is part of the port and callers must be testable against it.
func NewFailing(ecosystem string, err error) *Resolver {
	return &Resolver{ecosystem: ecosystem, err: err}
}

func (r *Resolver) Ecosystem() string { return r.ecosystem }

func (r *Resolver) Fidelity() depgraph.Fidelity { return r.fidelity }

// Units returns deep copies: a caller mutating the result must not be
// able to change what the next call resolves — the port promises
// determinism, and shared backing arrays would break it silently.
func (r *Resolver) Units(ctx context.Context, root fs.FS, p depgraph.Profile) ([]depgraph.Unit, error) {
	if r.err != nil {
		return nil, r.err
	}
	out := make([]depgraph.Unit, len(r.units))
	for i, u := range r.units {
		out[i] = copyUnit(u)
	}
	return out, nil
}

func copyUnit(u depgraph.Unit) depgraph.Unit {
	c := u
	c.Sources = make([]depgraph.SourceRef, len(u.Sources))
	for i, s := range u.Sources {
		c.Sources[i] = s
		c.Sources[i].Digest = append([]byte(nil), s.Digest...)
	}
	c.Deps = append([]string(nil), u.Deps...)
	if u.LockDigests != nil {
		c.LockDigests = make(map[string]string, len(u.LockDigests))
		for k, v := range u.LockDigests {
			c.LockDigests[k] = v
		}
	}
	return c
}
