package metrics

// discovery.go — Q-8 (docs/plans/expert-suite-q.md, docs/design/
// observability-surfaces.md §2): the design's own instruction is "reusing
// phase P's discoveryIndexTools set — import it, don't redefine, a second
// definition WILL drift." DiscoveryIndexTools moves HERE (from
// cmd/fathom/research_report.go, which cannot be imported back — it is
// `package main`) so both phase P's research_report.go and phase Q's new
// traffic aggregator (traffic.go) import the SAME set instead of each
// keeping their own copy.

// DiscoveryIndexTools is the companion surface's GENUINE index/navigation
// tools — deliberately excludes plumbing (capabilities, ingest,
// list_services, obligations, staleness_report, verify_plan) per
// docs/design/production-tokenomics.md §4's own instruction, so a
// hook-identity round trip or a governance/proof call never inflates the
// "agent chose to look something up" numerator.
var DiscoveryIndexTools = map[string]bool{
	"find_symbol":    true,
	"query_symbols":  true,
	"references":     true,
	"find_callers":   true,
	"symbol_context": true,
	"outline":        true,
	"repo_map":       true,
	"pack_context":   true,
	"read_source":    true,
	"entrypoints":    true,
}

// IsDiscoveryIndexTool reports whether tool is one of fathom's own
// index/navigation tools (as opposed to plumbing or a harness-native
// discovery action like Grep/Glob/Read).
func IsDiscoveryIndexTool(tool string) bool {
	return DiscoveryIndexTools[tool]
}

// PlumbingTools is the explicit set docs/design/production-tokenomics.md §4
// names as excluded from discovery-share accounting — identity/governance
// round trips, not "the agent chose to look something up." traffic.go's
// AggregateTraffic uses this (alongside IsDiscoveryIndexTool) so every call
// lands in exactly one of discovery/plumbing/other, never double-counted.
var PlumbingTools = map[string]bool{
	"capabilities":     true,
	"ingest":           true,
	"list_services":    true,
	"obligations":      true,
	"staleness_report": true,
	"verify_plan":      true,
}

// IsPlumbingTool reports whether tool is one of the excluded plumbing
// calls (identity resolution, governance, proof) rather than a genuine
// discovery action.
func IsPlumbingTool(tool string) bool {
	return PlumbingTools[tool]
}
