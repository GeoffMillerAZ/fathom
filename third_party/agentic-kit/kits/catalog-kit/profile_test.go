package catalogkit

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/knowledge/catalog"
)

func profileFixture() Spec {
	return Spec{Version: 1, Layers: []Layer{
		{Name: "base", Nodes: []catalog.AuthoredNode{{Slug: "base", Title: "Base", StableKey: "base", Source: "base.cue"}}},
		{Name: "middle", Requires: []string{"base"}, Nodes: []catalog.AuthoredNode{{Slug: "middle", Title: "Middle", StableKey: "middle", Source: "middle.cue"}}},
		{Name: "leaf", Requires: []string{"middle"}, Nodes: []catalog.AuthoredNode{{Slug: "leaf", Title: "Leaf", StableKey: "leaf", Source: "leaf.cue"}}},
		{Name: "other", Nodes: []catalog.AuthoredNode{{Slug: "other", Title: "Other", StableKey: "other", Source: "other.cue"}}},
	}}
}

func TestResolveProfileSelectsTransitiveClosure(t *testing.T) {
	b, err := ResolveProfile(profileFixture(), Policy{}, Profile{Name: "project", Roots: []string{"leaf"}})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"base", "middle", "leaf"}; !reflect.DeepEqual(b.Order, want) {
		t.Fatalf("order = %v, want %v", b.Order, want)
	}
	if _, ok := b.Catalog.Get("other"); ok {
		t.Fatal("unselected layer contributed a node")
	}
	for _, id := range []string{"base", "middle", "leaf"} {
		if _, ok := b.Catalog.Get(id); !ok {
			t.Fatalf("missing selected or required node %s", id)
		}
	}
	if err := VerifyBundle(b); err != nil {
		t.Fatal(err)
	}
}

func TestResolveProfileDeterministicAcrossNamesAndPermutations(t *testing.T) {
	s := profileFixture()
	a, err := ResolveProfile(s, Policy{}, Profile{Name: "one", Roots: []string{"other", "leaf"}})
	if err != nil {
		t.Fatal(err)
	}
	s.Layers[0], s.Layers[3] = s.Layers[3], s.Layers[0]
	s.Layers[1], s.Layers[2] = s.Layers[2], s.Layers[1]
	b, err := ResolveProfile(s, Policy{}, Profile{Name: "two", Roots: []string{"leaf", "other"}})
	if err != nil {
		t.Fatal(err)
	}
	if a.Digest != b.Digest || !reflect.DeepEqual(a.Order, b.Order) {
		t.Fatalf("selection changed with name or order: %s %v; %s %v", a.Digest, a.Order, b.Digest, b.Order)
	}
}

func TestResolveProfileRejectsInvalidSelectionAndLoadedGraph(t *testing.T) {
	base := profileFixture()
	for _, tc := range []struct {
		name    string
		spec    Spec
		profile Profile
	}{
		{"empty name", base, Profile{Roots: []string{"leaf"}}},
		{"invalid name", base, Profile{Name: "bad name", Roots: []string{"leaf"}}},
		{"empty roots", base, Profile{Name: "project"}},
		{"missing root", base, Profile{Name: "project", Roots: []string{"missing"}}},
		{"duplicate roots", base, Profile{Name: "project", Roots: []string{"leaf", "leaf"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ResolveProfile(tc.spec, Policy{}, tc.profile); err == nil {
				t.Fatal("invalid profile accepted")
			}
		})
	}
	for _, tc := range []struct {
		name   string
		mutate func(*Spec)
	}{
		{"unselected missing dependency", func(s *Spec) { s.Layers[3].Requires = []string{"absent"} }},
		{"unselected cycle", func(s *Spec) { s.Layers[3].Requires = []string{"other"} }},
		{"unselected duplicate layer", func(s *Spec) { s.Layers = append(s.Layers, s.Layers[3]) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := profileFixture()
			tc.mutate(&s)
			if _, err := ResolveProfile(s, Policy{}, Profile{Name: "project", Roots: []string{"leaf"}}); err == nil {
				t.Fatal("malformed loaded graph accepted")
			}
		})
	}
}

func TestResolveProfileEnforcesLimitsAndUnchangedGrants(t *testing.T) {
	s := profileFixture()
	p := Profile{Name: "project", Roots: []string{"leaf"}}
	for _, policy := range []Policy{
		{Limits: Limits{MaxLayers: 3}}, // Bound all loaded layers before graph traversal.
		{Limits: Limits{MaxNodes: 2}},  // Bound selected resolution.
		{Grants: []Grant{{Layer: "other", ID: "other", Fields: []string{"title"}}}},
	} {
		if _, err := ResolveProfile(s, policy, p); err == nil {
			t.Fatalf("profile accepted restrictive policy %+v", policy)
		} else if len(policy.Grants) != 0 && !strings.Contains(err.Error(), "grant") {
			t.Fatalf("unused grant refused for unexpected reason: %v", err)
		}
	}
}

func TestResolveProfileValidatesUnselectedDeclarations(t *testing.T) {
	p := Profile{Name: "project", Roots: []string{"leaf"}}
	for _, tc := range []struct {
		name   string
		mutate func(*Spec) Policy
		want   string
	}{
		{"invalid node slug", func(s *Spec) Policy { s.Layers[3].Nodes[0].Slug = "bad slug"; return Policy{} }, "slug"},
		{"stale pin", func(s *Spec) Policy { s.Layers[3].Pin = strings.Repeat("0", 64); return Policy{} }, "pin mismatch"},
		{"node limit", func(*Spec) Policy { return Policy{Limits: Limits{MaxNodes: 3}} }, "node/depth limit"},
		{"depth limit", func(s *Spec) Policy {
			s.Layers[3].Nodes[0].Children = []catalog.AuthoredNode{{Slug: "child"}}
			return Policy{Limits: Limits{MaxDepth: 1}}
		}, "node/depth limit"},
		{"authored byte limit", func(s *Spec) Policy {
			full, _ := json.Marshal(s)
			return Policy{Limits: Limits{MaxBytes: len(full) - 1}}
		}, "authored byte limit"},
		{"receiver policy byte limit", func(s *Spec) Policy {
			full, _ := json.Marshal(s)
			return Policy{Grants: []Grant{{Layer: strings.Repeat("x", len(full)), ID: "other", Fields: []string{"title"}}}, Limits: Limits{MaxBytes: len(full)}}
		}, "receiver policy byte limit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := profileFixture()
			policy := tc.mutate(&s)
			if _, err := ResolveProfile(s, policy, p); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestResolveProfileAllowsUnselectedAlternativeConflict(t *testing.T) {
	s := profileFixture()
	s.Layers[3].Nodes[0] = catalog.AuthoredNode{Slug: "base", Title: "Alternative", StableKey: "base", Source: "base.cue"}
	if _, err := ResolveProfile(s, Policy{}, Profile{Name: "project", Roots: []string{"leaf"}}); err != nil {
		t.Fatalf("valid unselected alternative refused: %v", err)
	}
}

func TestResolveProfileBoundsRootCardinality(t *testing.T) {
	spec := profileFixture()
	profile := Profile{Name: "bounded", Roots: make([]string, 10000)}
	if _, err := ResolveProfile(spec, Policy{}, profile); err == nil || !strings.Contains(err.Error(), "root count") {
		t.Fatalf("oversized root collection not refused before allocation: %v", err)
	}
}

func TestResolveProfileBoundsReceiverStrings(t *testing.T) {
	for _, field := range []string{"name", "root"} {
		t.Run(field, func(t *testing.T) {
			profile := Profile{Name: "bounded", Roots: []string{"leaf"}}
			if field == "name" {
				profile.Name = strings.Repeat("a", 10000)
			} else {
				profile.Roots[0] = strings.Repeat("a", 10000)
			}
			if _, err := ResolveProfile(profileFixture(), Policy{Limits: Limits{MaxBytes: 4096}}, profile); err == nil || !strings.Contains(err.Error(), "profile byte limit") {
				t.Fatalf("oversized profile string not refused: %v", err)
			}
		})
	}
}
