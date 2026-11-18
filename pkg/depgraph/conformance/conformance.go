// Package conformance checks the depgraph Resolver contract against
// caller-supplied filesystem fixtures. The checks return violations and
// are shared by mock and real resolvers.
package conformance

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/depgraph"
)

// Violation is one contract breach, named by the check that caught it.
type Violation struct {
	Check string
	Err   error
}

func (v Violation) String() string { return v.Check + ": " + v.Err.Error() }

// ResolverConformance runs the Resolver contract against one adapter
// over its recorded fixture and reports every violation through t.
func ResolverConformance(t *testing.T, name string, r depgraph.Resolver, fixture fs.FS, p depgraph.Profile) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		for _, v := range CheckResolver(context.Background(), r, fixture, p) {
			t.Errorf("%s", v)
		}
	})
}

// CheckResolver runs every assertion and returns the violations in a
// stable order. An empty result means the adapter conforms.
func CheckResolver(ctx context.Context, r depgraph.Resolver, fixture fs.FS, p depgraph.Profile) []Violation {
	var out []Violation
	fail := func(check string, format string, args ...any) {
		out = append(out, Violation{Check: check, Err: fmt.Errorf(format, args...)})
	}

	if r.Ecosystem() == "" {
		fail("ecosystem is named", "Ecosystem() is empty: an unnamed adapter cannot be selected, and its lock-digest keys would collide")
	}

	units, err := r.Units(ctx, fixture, p)
	if err != nil {
		fail("units resolve", "Units: %v", err)
		return out
	}

	// Determinism is the port's core promise: units feed keys, and a
	// resolver that orders differently per call cold-starts fleets.
	again, err := r.Units(ctx, fixture, p)
	if err != nil {
		fail("units are deterministic", "second Units call failed where the first succeeded: %v", err)
	} else if !reflect.DeepEqual(units, again) {
		fail("units are deterministic", "two Units calls over one tree differ (order, digests, or deps)")
	}

	if f1, f2 := r.Fidelity(), r.Fidelity(); f1 != f2 {
		fail("fidelity is stable", "Fidelity() wobbled %v → %v: the honesty mark must not depend on call history", f1, f2)
	}

	names := make(map[string]bool, len(units))
	for _, u := range units {
		names[u.Name] = true
	}
	for _, u := range units {
		for _, s := range u.Sources {
			if s.Path == "" || strings.Contains(s.Path, `\`) ||
				path.IsAbs(s.Path) || s.Path != path.Clean(s.Path) {
				fail("source paths are slash-separated and relative", "unit %s: path %q is not a clean repo-relative slash path", u.Name, s.Path)
			}
			if len(s.Digest) == 0 {
				fail("source digests are non-empty", "unit %s: %s has no digest — an undigested source cannot enter a key", u.Name, s.Path)
			}
		}
		for _, d := range u.Deps {
			if !names[d] {
				fail("unit deps name existing units", "unit %s depends on %q, which no unit carries — a dangling dep is a missing key input", u.Name, d)
			}
		}
	}
	return out
}
