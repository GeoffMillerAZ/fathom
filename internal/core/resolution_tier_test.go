package core

import "testing"

func TestResolutionTierOf(t *testing.T) {
	cases := []struct {
		name string
		edge Edge
		want ResolutionTierKind
	}{
		{"empty kind is heuristic", Edge{Kind: ""}, TierHeuristic},
		{"unresolved kind is heuristic", Edge{Kind: EdgeUnresolved}, TierHeuristic},
		{"direct kind is typed", Edge{Kind: EdgeDirect}, TierTyped},
		{"iface-dispatch kind is typed", Edge{Kind: EdgeIfaceDispatch}, TierTyped},
		{"func-value kind is typed", Edge{Kind: EdgeFuncValue}, TierTyped},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolutionTierOf(tc.edge); got.Kind != tc.want {
				t.Errorf("resolutionTierOf(%+v) = %v, want %v", tc.edge, got.Kind, tc.want)
			}
			// The exported wrapper must agree exactly — no drift between
			// the anchor VB-19 names and what external packages call.
			if got := ResolutionTierOf(tc.edge); got.Kind != tc.want {
				t.Errorf("ResolutionTierOf(%+v) = %v, want %v", tc.edge, got.Kind, tc.want)
			}
		})
	}
}

func TestResolutionTierString(t *testing.T) {
	cases := []struct {
		tier ResolutionTier
		want string
	}{
		{ResolutionTier{Kind: TierTyped}, "typed"},
		{ResolutionTier{Kind: TierHeuristic}, "heuristic"},
		{ExternalResolutionTier("scip-go", "1.2.3"), "external(scip-go@1.2.3)"},
	}
	for _, tc := range cases {
		if got := tc.tier.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
	}
}

// TestResolutionTierNothingProducesExternal is a documentation-as-test
// guard (ADR-021 §R6, ADR-035 decision 4): resolutionTierOf never returns
// TierExternal — that tier is reserved for a future imported index, and
// nothing this repository's extractors or typed pass emit today should
// ever be classified as one by silent default.
func TestResolutionTierNothingProducesExternal(t *testing.T) {
	kinds := []EdgeKind{"", EdgeDirect, EdgeIfaceDispatch, EdgeUnresolved, EdgeFuncValue}
	for _, k := range kinds {
		if got := resolutionTierOf(Edge{Kind: k}).Kind; got == TierExternal {
			t.Errorf("resolutionTierOf(Kind:%q) = external, want typed or heuristic", k)
		}
	}
}
