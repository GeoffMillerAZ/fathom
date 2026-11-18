package conformance

import (
	"context"
	"crypto/sha256"
	"errors"
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/geoffmilleraz/fathom/pkg/depgraph"
	"github.com/geoffmilleraz/fathom/pkg/depgraph/mockresolver"
)

func digestOf(s string) []byte {
	sum := sha256.Sum256([]byte(s))
	return sum[:]
}

func fixtureUnits() []depgraph.Unit {
	return []depgraph.Unit{
		{
			Ecosystem: "mock",
			Name:      "acme/lib",
			Sources:   []depgraph.SourceRef{{Path: "lib/lib.go", Digest: digestOf("package lib\n")}},
			LockDigests: map[string]string{
				"go.sum": "sum-digest-lib",
			},
		},
		{
			Ecosystem: "mock",
			Name:      "acme/app",
			Sources: []depgraph.SourceRef{
				{Path: "app/main.go", Digest: digestOf("package main\n")},
				{Path: "app/run.sh", Mode: depgraph.ModeExec, Digest: digestOf("#!/bin/sh\n")},
			},
			Deps: []string{"acme/lib"},
		},
	}
}

// TestMockResolverConformance holds the mock adapter to the same
// contract every real resolver must satisfy.
func TestMockResolverConformance(t *testing.T) {
	r := mockresolver.New("mock", depgraph.Exact, fixtureUnits()...)
	ResolverConformance(t, "mockresolver", r, fstest.MapFS{}, depgraph.Profile{})
}

// TestResolverSuiteRejectsBrokenResolvers is the suite's own proof:
// each adapter below violates exactly one clause, and the suite must
// name that violation — a suite nobody has watched fail is
// indistinguishable from one that passes everything.
func TestResolverSuiteRejectsBrokenResolvers(t *testing.T) {
	cases := []struct {
		name      string
		r         depgraph.Resolver
		wantCheck string
	}{
		{"an empty ecosystem name", mockresolver.New("", depgraph.Exact), "ecosystem is named"},
		{"units that fail to resolve", mockresolver.NewFailing("mock", errors.New("acme backend down")), "units resolve"},
		{"nondeterministic unit order", &flipper{}, "units are deterministic"},
		{"a backslash-separated path", brokenUnits(depgraph.Unit{
			Ecosystem: "mock", Name: "acme/win",
			Sources: []depgraph.SourceRef{{Path: `acme\main.go`, Digest: digestOf("x")}},
		}), "source paths are slash-separated and relative"},
		{"an absolute path", brokenUnits(depgraph.Unit{
			Ecosystem: "mock", Name: "acme/abs",
			Sources: []depgraph.SourceRef{{Path: "/etc/acme.conf", Digest: digestOf("x")}},
		}), "source paths are slash-separated and relative"},
		{"an empty digest", brokenUnits(depgraph.Unit{
			Ecosystem: "mock", Name: "acme/hollow",
			Sources: []depgraph.SourceRef{{Path: "acme/main.go"}},
		}), "source digests are non-empty"},
		{"an unstable fidelity mark", &fidelityWobble{}, "fidelity is stable"},
		{"a dep naming no unit", brokenUnits(depgraph.Unit{
			Ecosystem: "mock", Name: "acme/orphan",
			Sources: []depgraph.SourceRef{{Path: "acme/o.go", Digest: digestOf("x")}},
			Deps:    []string{"acme/ghost"},
		}), "unit deps name existing units"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CheckResolver(context.Background(), tc.r, fstest.MapFS{}, depgraph.Profile{})
			if len(got) == 0 {
				t.Fatalf("the suite passed %s — it does not bite", tc.name)
			}
			var names []string
			for _, v := range got {
				names = append(names, v.Check)
			}
			if !slices.Contains(names, tc.wantCheck) {
				t.Errorf("violations %v do not include %q", names, tc.wantCheck)
			}
		})
	}
}

func brokenUnits(units ...depgraph.Unit) depgraph.Resolver {
	return mockresolver.New("mock", depgraph.Exact, units...)
}

// flipper alternates unit order across calls — the determinism breach
// most likely in a real resolver (map iteration order leaking out).
type flipper struct{ calls int }

func (f *flipper) Ecosystem() string           { return "mock" }
func (f *flipper) Fidelity() depgraph.Fidelity { return depgraph.Exact }
func (f *flipper) Units(ctx context.Context, root fs.FS, p depgraph.Profile) ([]depgraph.Unit, error) {
	f.calls++
	units := fixtureUnits()
	if f.calls%2 == 0 {
		slices.Reverse(units)
	}
	return units, nil
}

// fidelityWobble degrades its honesty mark between calls; a wobbling
// mark would let approximate evidence masquerade as exact mid-run.
type fidelityWobble struct{ calls int }

func (f *fidelityWobble) Ecosystem() string { return "mock" }
func (f *fidelityWobble) Fidelity() depgraph.Fidelity {
	f.calls++
	if f.calls%2 == 0 {
		return depgraph.Approximate
	}
	return depgraph.Exact
}

func (f *fidelityWobble) Units(ctx context.Context, root fs.FS, p depgraph.Profile) ([]depgraph.Unit, error) {
	return fixtureUnits(), nil
}
