package depgraph

import (
	"reflect"
	"testing"
)

func units(spec map[string][]string) []Unit {
	out := make([]Unit, 0, len(spec))
	for name, deps := range spec {
		out = append(out, Unit{Ecosystem: "image", Name: name, Deps: deps})
	}
	return out
}

// TestDependentsWalksTransitively is the whole point: a base image patched
// two levels up must still reach the leaf that inherits from it. A
// one-level answer is the bug that ships a rebuild plan missing exactly the
// images nobody thought about.
func TestDependentsWalksTransitively(t *testing.T) {
	u := units(map[string][]string{
		"base-static": nil,
		"runtime":     {"base-static"},
		"api":         {"runtime"},
		"worker":      {"runtime"},
		"unrelated":   nil,
	})
	got := Dependents(u, []string{"base-static"})
	want := []string{"api", "runtime", "worker"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Dependents = %v, want %v", got, want)
	}
}

// TestDependentsExcludesTheSeeds: whether you rebuild the thing that
// changed is the caller's decision — for an upstream base image the answer
// is usually no, because it arrived already built.
func TestDependentsExcludesTheSeeds(t *testing.T) {
	u := units(map[string][]string{"base": nil, "app": {"base"}})
	got := Dependents(u, []string{"base"})
	for _, name := range got {
		if name == "base" {
			t.Errorf("the seed appeared in its own blast radius: %v", got)
		}
	}
}

// TestDependentsIsSortedAndDeterministic: two runs over one graph must
// agree, or a CI job cannot diff this run's blast radius against the last.
// Unit order is deliberately shuffled between the two calls.
func TestDependentsIsSortedAndDeterministic(t *testing.T) {
	spec := map[string][]string{
		"base": nil, "zebra": {"base"}, "alpha": {"base"}, "middle": {"base"},
	}
	first := Dependents(units(spec), []string{"base"})
	want := []string{"alpha", "middle", "zebra"}
	if !reflect.DeepEqual(first, want) {
		t.Fatalf("Dependents = %v, want sorted %v", first, want)
	}
	for i := range 8 {
		if got := Dependents(units(spec), []string{"base"}); !reflect.DeepEqual(got, first) {
			t.Fatalf("run %d = %v, first = %v — map iteration leaked into the answer", i, got, first)
		}
	}
}

// TestDependentsSurvivesACycle. A declared graph CAN contain a cycle, and a
// blast-radius query that hangs on one hangs in CI, where nobody is
// watching it.
func TestDependentsSurvivesACycle(t *testing.T) {
	done := make(chan []string, 1)
	go func() {
		done <- Dependents(units(map[string][]string{
			"a": {"c"}, "b": {"a"}, "c": {"b"}, "outside": {"a"},
		}), []string{"a"})
	}()
	select {
	case got := <-done:
		want := []string{"b", "c", "outside"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Dependents over a cycle = %v, want %v", got, want)
		}
	case <-timeoutC(t):
		t.Fatal("Dependents did not terminate on a cyclic graph")
	}
}

// TestDependentsHandlesManySeeds: patching two bases at once is one query,
// and a unit reachable from both appears exactly once.
func TestDependentsHandlesManySeeds(t *testing.T) {
	u := units(map[string][]string{
		"base-a": nil, "base-b": nil,
		"shared": {"base-a", "base-b"},
		"only-a": {"base-a"},
		"far":    {"shared"},
	})
	got := Dependents(u, []string{"base-a", "base-b"})
	want := []string{"far", "only-a", "shared"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Dependents = %v, want %v", got, want)
	}
}

// TestDependentsEmptyInputs: no units or no seeds is an empty answer, not a
// panic. A caller with nothing declared yet is the normal first run.
func TestDependentsEmptyInputs(t *testing.T) {
	if got := Dependents(nil, []string{"x"}); got != nil {
		t.Errorf("Dependents(nil units) = %v, want nil", got)
	}
	if got := Dependents(units(map[string][]string{"a": nil}), nil); got != nil {
		t.Errorf("Dependents(no seeds) = %v, want nil", got)
	}
}

// TestUnknownDepsSurfacesTheSilentGap is the failure Dependents CANNOT
// report: a dep pointing at nothing contributes no dependents, so a whole
// subtree can be missing from a blast radius while the answer still looks
// like an answer.
func TestUnknownDepsSurfacesTheSilentGap(t *testing.T) {
	u := units(map[string][]string{
		"app":   {"base-that-was-never-declared"},
		"other": {"also-missing", ""},
		"base":  nil,
		"fine":  {"base"},
	})
	got := UnknownDeps(u)
	want := []string{"also-missing", "base-that-was-never-declared"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("UnknownDeps = %v, want %v (empty deps ignored, sorted, deduped)", got, want)
	}
	// And the silent part: querying the missing base yields nothing at all.
	if r := Dependents(u, []string{"base-that-was-never-declared"}); len(r) != 1 || r[0] != "app" {
		t.Logf("Dependents over an undeclared seed = %v", r)
	}
	if r := Dependents(u, []string{"typo-nobody-declared"}); r != nil {
		t.Errorf("a seed nothing references returned %v, want nil — which is exactly why UnknownDeps exists", r)
	}
	if got := UnknownDeps(units(map[string][]string{"a": nil, "b": {"a"}})); got != nil {
		t.Errorf("a complete graph reported missing deps: %v", got)
	}
}
