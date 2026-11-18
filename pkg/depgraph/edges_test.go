package depgraph

import (
	"crypto/sha256"
	"strings"
	"testing"
)

func sumOf(s string) []byte {
	sum := sha256.Sum256([]byte(s))
	return sum[:]
}

// unitKey is how a consumer folds an edge target into its own key:
// the target's KEY enters Deps — keys, not content, so a leaf edit
// rekeys each downstream exactly once.
func unitKey(u Unit) Key {
	return InputKey(KeyInputs{
		Domain:   "cikit/nodekey/v1",
		Epoch:    1,
		Task:     "build",
		Platform: "linux/amd64",
		Sources:  u.Sources,
	})
}

// TestDeclaredEdgeRekeysConsumer proves the key propagation chain: a
// declared cross-ecosystem edge resolves against real units, the edge
// target's key enters the consumer's KeyInputs.Deps, and a change in
// the embedded unit therefore rekeys the consumer. A declaration naming
// a unit that does not exist is refused loudly — a dangling declaration
// would be a missing edge, and a missing edge is under-keying.
func TestDeclaredEdgeRekeysConsumer(t *testing.T) {
	goUnit := Unit{
		Ecosystem: "go",
		Name:      "acme/embedder",
		Sources:   []SourceRef{{Path: "embedder/main.go", Digest: sumOf("package main // embeds config\n")}},
	}
	cueUnit := Unit{
		Ecosystem: "cue",
		Name:      "acme/config",
		Sources:   []SourceRef{{Path: "config/config.cue", Digest: sumOf("acme: replicas: 3\n")}},
	}
	units := []Unit{goUnit, cueUnit}
	decls := []EdgeDecl{{
		From: UnitRef{Ecosystem: "go", Name: "acme/embedder"},
		To:   UnitRef{Ecosystem: "cue", Name: "acme/config"},
	}}

	edges, err := ResolveDeclaredEdges(decls, units)
	if err != nil {
		t.Fatalf("ResolveDeclaredEdges: %v", err)
	}
	want := Edge{From: decls[0].From, To: decls[0].To}
	if len(edges) != 1 || edges[0] != want {
		t.Fatalf("edges = %+v, want [%+v]", edges, want)
	}

	// Determinism: resolving twice yields identical edges.
	again, err := ResolveDeclaredEdges(decls, units)
	if err != nil || len(again) != 1 || again[0] != edges[0] {
		t.Errorf("second resolution diverged: %+v, %v", again, err)
	}

	consumerKey := func(target Unit) Key {
		return InputKey(KeyInputs{
			Domain:   "cikit/nodekey/v1",
			Epoch:    1,
			Task:     "build",
			Platform: "linux/amd64",
			Sources:  goUnit.Sources,
			Deps:     []string{string(unitKey(target))},
		})
	}

	before := consumerKey(cueUnit)

	// The embedded CUE instance changes: its key changes, and because
	// that key sits in the consumer's Deps, the consumer rekeys — with
	// the consumer's own sources untouched.
	changed := cueUnit
	changed.Sources = []SourceRef{{Path: "config/config.cue", Digest: sumOf("acme: replicas: 5\n")}}
	if unitKey(changed) == unitKey(cueUnit) {
		t.Fatal("the embedded unit's key did not change with its content")
	}
	if after := consumerKey(changed); after == before {
		t.Error("the consumer's key survived a change in its declared edge target")
	}

	// Without the edge the consumer's key is blind to the change —
	// the under-keying the declaration exists to prevent.
	blind := InputKey(KeyInputs{
		Domain: "cikit/nodekey/v1", Epoch: 1, Task: "build", Platform: "linux/amd64",
		Sources: goUnit.Sources,
	})
	blindAfter := InputKey(KeyInputs{
		Domain: "cikit/nodekey/v1", Epoch: 1, Task: "build", Platform: "linux/amd64",
		Sources: goUnit.Sources,
	})
	if blind != blindAfter {
		t.Error("control: an edge-less consumer rekeyed without cause")
	}

	// A dangling declaration refuses, naming the missing unit.
	dangling := []EdgeDecl{{
		From: UnitRef{Ecosystem: "go", Name: "acme/embedder"},
		To:   UnitRef{Ecosystem: "cue", Name: "acme/ghost"},
	}}
	if _, err := ResolveDeclaredEdges(dangling, units); err == nil {
		t.Error("a declaration naming a nonexistent unit resolved")
	} else if !strings.Contains(err.Error(), "acme/ghost") {
		t.Errorf("refusal does not name the missing unit: %v", err)
	}
	danglingFrom := []EdgeDecl{{
		From: UnitRef{Ecosystem: "go", Name: "acme/ghost"},
		To:   UnitRef{Ecosystem: "cue", Name: "acme/config"},
	}}
	if _, err := ResolveDeclaredEdges(danglingFrom, units); err == nil {
		t.Error("a declaration from a nonexistent unit resolved")
	}
}
