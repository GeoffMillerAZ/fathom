package core

import "testing"

// BD-1 (docs/plans/blast-depth.md): an edge's confidence is read from the
// provenance the fusion pass already records on it — Kind is "" for the
// syntax-only producers and set by the Go typed pass — never guessed.
func TestEdgeConfidenceOf(t *testing.T) {
	cases := []struct {
		kind EdgeKind
		want EdgeConfidence
	}{
		{"", EdgeConfidenceHeuristic},
		{EdgeUnresolved, EdgeConfidenceHeuristic},
		{EdgeDirect, EdgeConfidenceResolved},
		{EdgeIfaceDispatch, EdgeConfidenceResolved},
		{EdgeFuncValue, EdgeConfidenceResolved},
		// A syntax-only reference to a Go type (wave lg, LB-F200) is
		// never promoted by the non-empty default.
		{EdgeTypeRef, EdgeConfidenceHeuristic},
	}
	for _, c := range cases {
		if got := EdgeConfidenceOf(Edge{FromSymbol: "a", ToSymbol: "b", Kind: c.kind}); got != c.want {
			t.Errorf("EdgeConfidenceOf(kind=%q) = %q, want %q", c.kind, got, c.want)
		}
	}
	if EdgeConfidenceResolved != "resolved" || EdgeConfidenceHeuristic != "heuristic" {
		t.Fatalf("wire strings drifted: %q / %q", EdgeConfidenceResolved, EdgeConfidenceHeuristic)
	}
}
