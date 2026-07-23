// fathom's own architecture-conformance rule pack (C4-4,
// docs/plans/review-verb.md) — dogfoods the three-layer boundary this repo's
// own CLAUDE.md and ADR.md describe (see "## Architecture (three layers —
// see ADR.md)"):
//
//   - internal/connector must not import internal/query.
//   - internal/extractor must not import internal/mcp.
//   - internal/analysis/* packages are pure: they import neither
//     internal/mcp nor internal/store (CLAUDE.md's "Adding things ▸
//     Analyzer" convention: "a pure internal/analysis/<name> package over
//     []core.SymbolNode/[]core.Observation").
//
// This file is plain data (no package clause, no "#RuleSet &" wrapper — the
// schema/semantic layer.cue precedent): internal/projectrules.Load unifies
// it against schema/rules/archconformance/schema.cue's #RuleSet directly.
// from_pkg/to_pkg are bare Go package-clause names (core.SymbolNode.Package
// — e.g. "connector", "mcp"), not import paths.
//
// analysisPkgs is every internal/analysis/* subpackage as of this file's
// authoring (verified zero-violation against the real repo — see
// internal/projectrules's C4-4 fixture test, which proves the rule shape
// catches a violation using a synthetic edge, never a live one here).
// Adding a new internal/analysis/<name> package should add its name here so
// the boundary keeps covering it.
let analysisPkgs = [
	"antipattern",
	"apichangelog",
	"apisurface",
	"archconformance",
	"callgraph",
	"churn",
	"cloc",
	"clone",
	"cognitive",
	"complexity",
	"configdrift",
	"crossrepocycles",
	"dataflow",
	"deprecation",
	"deps",
	"doccoverage",
	"docdrift",
	"endpoints",
	"envkeys",
	"erraudit",
	"fieldusage",
	"godobject",
	"impact",
	"ineffassign",
	"license",
	"markers",
	"migrationtargets",
	"naming",
	"packagedeps",
	"proto",
	"references",
	"routes",
	"schemaexport",
	"security",
	"sqlmap",
	"structtags",
	"symbolnav",
]

rules: [
	{from_pkg: "connector", to_pkg: "query", allow: false},
	{from_pkg: "extractor", to_pkg: "mcp", allow: false},
	for p in analysisPkgs {
		{from_pkg: p, to_pkg: "mcp", allow: false}
	},
	for p in analysisPkgs {
		{from_pkg: p, to_pkg: "store", allow: false}
	},
]

// denylist mode: only the edges named above are checked; an edge with no
// matching rule is silently permitted (deny_by_default would require
// allowlisting every legitimate edge in the repo, which this rule pack does
// not attempt — it targets the three named boundaries only).
deny_by_default: false
