package catalogkit

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/knowledge/catalog"
)

func fixture() Spec {
	return Spec{Version: 1, Layers: []Layer{{Name: "general", Nodes: []catalog.AuthoredNode{{Slug: "review", Title: "Review", StableKey: "review", Source: "general.cue", Children: []catalog.AuthoredNode{{Slug: "evidence", Title: "Evidence", StableKey: "review.evidence", Source: "general.cue", Order: 7, Hidden: true, Facets: map[string][]string{"language": {"go"}}}}}}}}}
}

func TestResolveExplicitOverrideAndZeroValues(t *testing.T) {
	s := fixture()
	base, err := Resolve(s, Policy{})
	if err != nil {
		t.Fatal(err)
	}
	n, _ := base.Catalog.Get("review/evidence")
	zero, visible := 0, false
	s.Layers = append(s.Layers, Layer{Name: "project", Requires: []string{"general"}, Overrides: []Override{{ID: n.ID(), ExpectedDigest: NodeDigest(n), Reason: "Use project ordering and expose this entry", Set: Patch{Order: &zero, Hidden: &visible}}}})
	if _, err = Resolve(s, Policy{}); err == nil {
		t.Fatal("ungranted override accepted")
	}
	p := Policy{Grants: []Grant{{Layer: "project", ID: n.ID(), Fields: []string{"order", "hidden"}}}}
	b, err := Resolve(s, p)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := b.Catalog.Get(n.ID())
	if got.Order != 0 || got.Hidden || len(b.Changes) != 1 {
		t.Fatalf("zero-valued patch lost: %+v", b)
	}
	if got.Provenance.Source != "general.cue" || got.Provenance.OverriddenBy != "project" {
		t.Fatalf("provenance lost: %+v", got)
	}
	if b.Digest == base.Digest {
		t.Fatal("semantic override did not change bundle digest")
	}
	s.Layers[1].Overrides[0].ExpectedDigest = strings.Repeat("0", 64)
	if _, err := Resolve(s, p); err == nil {
		t.Fatal("stale patch accepted")
	}
}

func TestResolveRefusesAmbiguityAndInvalidStructure(t *testing.T) {
	tests := map[string]func(*Spec){
		"cycle":              func(s *Spec) { s.Layers[0].Requires = []string{"general"} },
		"missing dependency": func(s *Spec) { s.Layers[0].Requires = []string{"missing"} },
		"duplicate layer":    func(s *Spec) { s.Layers = append(s.Layers, s.Layers[0]) },
		"duplicate identity": func(s *Spec) { s.Layers[0].Nodes[0].Children[0].StableKey = "review" },
		"missing source":     func(s *Spec) { s.Layers[0].Nodes[0].Source = "" },
		"implicit scalar override": func(s *Spec) {
			s.Layers = append(s.Layers, Layer{Name: "project", Requires: []string{"general"}, Nodes: []catalog.AuthoredNode{{Slug: "review", Title: "Different", Source: "general.cue", StableKey: "review"}}})
		},
		"unrelated collision": func(s *Spec) { s.Layers = append(s.Layers, Layer{Name: "other", Nodes: s.Layers[0].Nodes}) },
		"bad layer pin":       func(s *Spec) { s.Layers[0].Pin = strings.Repeat("0", 64) },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			s := fixture()
			mutate(&s)
			if _, err := Resolve(s, Policy{}); err == nil {
				t.Fatal("invalid spec accepted")
			}
		})
	}
}

func TestResolveDeterministicDetachedAndBounded(t *testing.T) {
	s := fixture()
	s.Layers = append(s.Layers, Layer{Name: "team", Requires: []string{"general"}, Nodes: []catalog.AuthoredNode{{Slug: "review", Source: "general.cue", StableKey: "review", Children: []catalog.AuthoredNode{{Slug: "evidence", Source: "general.cue", StableKey: "review.evidence", Facets: map[string][]string{"language": {"rust", "go"}}}}}}})
	before, _ := json.Marshal(s)
	a, err := Resolve(s, Policy{})
	if err != nil {
		t.Fatal(err)
	}
	s.Layers[0], s.Layers[1] = s.Layers[1], s.Layers[0]
	b, err := Resolve(s, Policy{})
	if err != nil {
		t.Fatal(err)
	}
	if a.Digest != b.Digest || !reflect.DeepEqual(a.Order, b.Order) {
		t.Fatal("input order changed resolution")
	}
	s.Layers[0], s.Layers[1] = s.Layers[1], s.Layers[0]
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("resolve mutated input")
	}
	a.Catalog.Nodes[1].Facets["language"][0] = "mutated"
	if s.Layers[0].Nodes[0].Children[0].Facets["language"][0] != "go" {
		t.Fatal("output aliases caller input")
	}
	for _, p := range []Policy{{Limits: Limits{MaxNodes: 1}}, {Limits: Limits{MaxDepth: 1}}, {Limits: Limits{MaxLayers: 1}}, {Limits: Limits{MaxBytes: 1}}} {
		if _, err := Resolve(s, p); err == nil {
			t.Fatal("resource bound ignored")
		}
	}
}

func TestNodeDigestCoversSemanticFieldsWithoutDelimiterCollisions(t *testing.T) {
	b, _ := Resolve(fixture(), Policy{})
	n, _ := b.Catalog.Get("review/evidence")
	a := NodeDigest(n)
	n.Hidden = false
	if NodeDigest(n) == a {
		t.Fatal("hidden omitted from digest")
	}
	n.Facets = map[string][]string{"x": {"a,b"}}
	a = NodeDigest(n)
	n.Facets = map[string][]string{"x": {"a", "b"}}
	if NodeDigest(n) == a {
		t.Fatal("ambiguous facet encoding")
	}
}

func TestDecodeStrictly(t *testing.T) {
	for _, input := range []string{`{"version":1,"layers":[],"typo":1}`, `{"version":1,"version":2,"layers":[]}`, `{"version":1,"layers":[]} {}`, `{"version":null,"layers":[]}`, `{"version":1,"layers":[{"name":"\ud800"}]}`, `{"version":1,"layers":[{"name":"\udc00"}]}`} {
		if _, err := DecodeSpec([]byte(input)); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
}

func TestResolveBoundsPendingTreeBeforeStackGrowth(t *testing.T) {
	for _, children := range []bool{false, true} {
		s := fixture()
		if children {
			s.Layers[0].Nodes[0].Children = make([]catalog.AuthoredNode, 1000)
		} else {
			s.Layers[0].Nodes = make([]catalog.AuthoredNode, 1000)
		}
		if _, err := Resolve(s, Policy{Limits: Limits{MaxNodes: 2}}); err == nil || !strings.Contains(err.Error(), "limit") {
			t.Fatalf("expected early tree bound: %v", err)
		}
	}
}
