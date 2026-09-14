// Package mcp — promotedVerbs (ADR-035 decision 1, docs/vision/
// 29-verbs-not-lookups.md §3 wave A, plan VB-1; wave G, plan VB-26).
//
// Three independent readings (the home bench, the lookup pilot, and
// arXiv 2608.13568) agree: an agent greps first, and fathom's lookups win
// no accuracy over grep on their own shape. What fathom is actually ahead
// on is the answer grep cannot give at all — blast radius, a composed
// review verdict, which tests reach a symbol, every call site a rename
// must touch, and (wave G) where a bug lives before anyone knows which
// file. ADR-035 promotes those onto the agent surface instead of chasing
// another lookup-cost claim: "never token savings versus grep —
// precision, completeness, governance or falsifiability only."
package mcp

// promotedVerb names one tool ADR-035 promoted onto the agent surface and
// the one-clause reason it earned the spot: the answer grep cannot give.
type promotedVerb struct {
	Name   string
	Reason string
}

// promotedVerbs returns the impact verbs waves A and G promote onto the
// agent/companion surface, in a stable order. companionTools consults it
// for surface membership; docs generation (docs/mcp-tools.md's "Also on
// the agent/companion surface" annotations) is meant to read the same
// list rather than a second, hand-maintained one.
func promotedVerbs() []promotedVerb {
	return []promotedVerb{
		{Name: "blast_radius", Reason: "who transitively breaks — grep has no call graph to walk"},
		{Name: "review_diff", Reason: "one cited verdict over a diff — grep cannot compose obligations, coverage and conformance"},
		{Name: "test_impact", Reason: "which tests reach a symbol — grep cannot walk the call graph to a Test* root"},
		{Name: "rename_impact", Reason: "every call site a rename must touch — grep's text match cannot resolve identity"},
		{Name: "localize", Reason: "where a bug lives before anyone knows the file — grep's literal match has no graph proximity, recency or governance ranking to fall back on"},
	}
}
