package explmap

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func eph(id string, includes ...string) Map {
	return Map{ID: id, Kind: KindEphemeral, Includes: includes}
}

func nodeIDs(r Resolution) []string {
	out := make([]string, len(r.Nodes))
	for i, n := range r.Nodes {
		out[i] = n.ID
	}
	return out
}

// TestResolveMapDAGRefusesCyclesAndCapsDepth pins XM-2: includes form a DAG.
// A shared base is visited once, at its nearest depth; a cycle is refused and
// named; a chain deeper than MaxDepth is refused and named; a missing include
// is reported, never silently skipped.
func TestResolveMapDAGRefusesCyclesAndCapsDepth(t *testing.T) {
	t.Run("shared base allowed once at nearest depth", func(t *testing.T) {
		maps := Maps{
			"root":  eph("root", "a", "b"),
			"a":     eph("a", "base"),
			"b":     eph("b", "c"),
			"c":     eph("c", "base"),
			"base":  eph("base"),
			"other": eph("other"),
		}
		r, err := ResolveMap("root", maps)
		if err != nil {
			t.Fatalf("ResolveMap: %v", err)
		}
		if got, want := nodeIDs(r), []string{"root", "a", "b", "base", "c"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("order = %v, want %v (breadth-first, each map once)", got, want)
		}
		for _, n := range r.Nodes {
			if n.ID == "base" && n.Depth != 2 {
				t.Errorf("base depth = %d, want 2 (its nearest route, via a)", n.Depth)
			}
		}
		if r.Height != 3 {
			t.Errorf("height = %d, want 3 (root>b>c>base)", r.Height)
		}
	})

	t.Run("cycle refused and named", func(t *testing.T) {
		maps := Maps{"a": eph("a", "b"), "b": eph("b", "c"), "c": eph("c", "a")}
		_, err := ResolveMap("a", maps)
		if !errors.Is(err, ErrCycle) {
			t.Fatalf("err = %v, want ErrCycle", err)
		}
		if !strings.Contains(err.Error(), "a -> b -> c -> a") {
			t.Errorf("cycle not named: %v", err)
		}
		// a self-include is the smallest cycle
		if _, err := ResolveMap("s", Maps{"s": eph("s", "s")}); !errors.Is(err, ErrCycle) {
			t.Errorf("self include: err = %v, want ErrCycle", err)
		}
	})

	t.Run("depth capped at 4", func(t *testing.T) {
		ok := Maps{"d0": eph("d0", "d1"), "d1": eph("d1", "d2"), "d2": eph("d2", "d3"), "d3": eph("d3", "d4"), "d4": eph("d4")}
		r, err := ResolveMap("d0", ok)
		if err != nil {
			t.Fatalf("four levels must resolve: %v", err)
		}
		if r.Height != MaxDepth {
			t.Errorf("height = %d, want %d", r.Height, MaxDepth)
		}
		deep := Maps{"d0": eph("d0", "d1"), "d1": eph("d1", "d2"), "d2": eph("d2", "d3"), "d3": eph("d3", "d4"), "d4": eph("d4", "d5"), "d5": eph("d5")}
		_, err = ResolveMap("d0", deep)
		if !errors.Is(err, ErrTooDeep) {
			t.Fatalf("five levels: err = %v, want ErrTooDeep", err)
		}
		if !strings.Contains(err.Error(), "d0 -> d1 -> d2 -> d3 -> d4 -> d5") {
			t.Errorf("the too-deep chain is not named: %v", err)
		}
		// the longest route counts even when a shorter one reaches the same base
		mixed := Maps{"r": eph("r", "d5", "d1"), "d1": eph("d1", "d2"), "d2": eph("d2", "d3"), "d3": eph("d3", "d4"), "d4": eph("d4", "d5"), "d5": eph("d5")}
		if _, err := ResolveMap("r", mixed); !errors.Is(err, ErrTooDeep) {
			t.Errorf("a long route beside a short one: err = %v, want ErrTooDeep", err)
		}
	})

	t.Run("missing include reported; unknown root refused", func(t *testing.T) {
		r, err := ResolveMap("root", Maps{"root": eph("root", "gone", "a"), "a": eph("a")})
		if err != nil {
			t.Fatalf("ResolveMap: %v", err)
		}
		if !reflect.DeepEqual(r.Missing, []string{"gone"}) {
			t.Errorf("missing = %v, want [gone]", r.Missing)
		}
		if _, err := ResolveMap("nope", Maps{}); !errors.Is(err, ErrUnknownMap) {
			t.Errorf("unknown root: err = %v, want ErrUnknownMap", err)
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		maps := Maps{"root": eph("root", "b", "a"), "a": eph("a", "z"), "b": eph("b", "y"), "y": eph("y"), "z": eph("z")}
		first, _ := ResolveMap("root", maps)
		for i := 0; i < 20; i++ {
			again, _ := ResolveMap("root", maps)
			if !reflect.DeepEqual(first, again) {
				t.Fatalf("resolution changed between runs: %v vs %v", nodeIDs(first), nodeIDs(again))
			}
		}
		if got, want := nodeIDs(first), []string{"root", "b", "a", "y", "z"}; !reflect.DeepEqual(got, want) {
			t.Errorf("order = %v, want %v (include order is priority)", got, want)
		}
	})
}
