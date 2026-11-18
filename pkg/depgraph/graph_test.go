package depgraph_test

import (
	"context"
	"crypto/sha256"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/geoffmilleraz/fathom/pkg/depgraph"
	"github.com/geoffmilleraz/fathom/pkg/depgraph/mockresolver"
)

func digestOf(s string) []byte {
	sum := sha256.Sum256([]byte(s))
	return sum[:]
}

// TestDepGraphReturnsNodesEdgesLockDigests exercises resolver selection
// by ecosystem against the mock adapter,
// unit pass-through, lock-digest aggregation under the documented
// "<ecosystem>/<unit>/<name>" key shape, the named-known-set refusal
// for an unknown ecosystem, and determinism across two calls.
func TestDepGraphReturnsNodesEdgesLockDigests(t *testing.T) {
	units := []depgraph.Unit{
		{
			Ecosystem: "mock",
			Name:      "acme/app",
			Sources:   []depgraph.SourceRef{{Path: "app/main.go", Digest: digestOf("package main\n")}},
			Deps:      []string{"acme/lib"},
			LockDigests: map[string]string{
				"go.sum": "sum-digest-app",
			},
		},
		{
			Ecosystem: "mock",
			Name:      "acme/lib",
			Sources:   []depgraph.SourceRef{{Path: "lib/lib.go", Digest: digestOf("package lib\n")}},
			LockDigests: map[string]string{
				"go.sum": "sum-digest-lib",
			},
		},
	}
	mock := mockresolver.New("mock", depgraph.Exact, units...)
	other := mockresolver.New("othereco", depgraph.Exact)
	root := fstest.MapFS{}

	g, err := depgraph.DepGraph(context.Background(), "mock", root, depgraph.Profile{}, other, mock)
	if err != nil {
		t.Fatalf("DepGraph: %v", err)
	}
	if !reflect.DeepEqual(g.Units, units) {
		t.Errorf("units: got %+v, want %+v", g.Units, units)
	}
	if len(g.Edges) != 0 {
		t.Errorf("single-ecosystem graph grew edges: %+v", g.Edges)
	}
	wantLocks := map[string]string{
		"mock/acme/app/go.sum": "sum-digest-app",
		"mock/acme/lib/go.sum": "sum-digest-lib",
	}
	if !reflect.DeepEqual(g.LockDigests, wantLocks) {
		t.Errorf("lock digests: got %v, want %v", g.LockDigests, wantLocks)
	}

	// Determinism: the same call twice is deep-equal.
	again, err := depgraph.DepGraph(context.Background(), "mock", root, depgraph.Profile{}, other, mock)
	if err != nil {
		t.Fatalf("DepGraph again: %v", err)
	}
	if !reflect.DeepEqual(g, again) {
		t.Errorf("two identical calls diverged:\n%+v\n%+v", g, again)
	}

	// An unknown ecosystem refuses loudly and names the known set —
	// a typo'd ecosystem silently resolving to nothing would be
	// under-keying's quietest entrance.
	_, err = depgraph.DepGraph(context.Background(), "npm", root, depgraph.Profile{}, other, mock)
	if err == nil {
		t.Fatal("unknown ecosystem did not error")
	}
	for _, want := range []string{"npm", "mock", "othereco"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("unknown-ecosystem error %q does not name %q", err, want)
		}
	}
}
