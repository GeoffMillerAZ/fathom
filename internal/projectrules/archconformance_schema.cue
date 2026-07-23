// Package archconformance defines the CUE schema for project-level
// architecture-conformance rule packs (Phase C, docs/plans/review-verb.md
// C4-1). A service's own .fathom/rules/archconformance.cue file is plain
// data — no package clause, no "#RuleSet &" wrapper (the schema/semantic
// layer.cue precedent, internal/semantic/manifest.go's decodePack) — unified
// against #RuleSet at load time by internal/projectrules.Load. Unlike
// schema/config (TOML with a CUE validation step), there is no format
// indirection here: a rule pack is authored in CUE directly, so a malformed
// or out-of-shape file fails loudly at load, never silently empty
// (C4-5's honesty audit: "no rules configured" and "zero rules, all
// checked" must never look the same).
//
// This file is authoritative AND live: internal/projectrules embeds a
// byte-identical copy (internal/projectrules/archconformance_schema.cue,
// drift-tested against this file — see internal/config/schema_test.go for
// the precedent) and validates every loaded rule pack against it.
package archconformance

// #Rule mirrors internal/analysis/archconformance.Rule: a single layering
// constraint between two package-prefix patterns. "*" matches any package
// on that side of the rule.
//
// What from_pkg/to_pkg actually compare against: the package prefix
// archconformance.Check derives from each edge's symbol-key text
// (everything before the LAST dot; see pkgOf). For a top-level FUNCTION
// symbol that is the bare Go package-clause name recorded on
// core.SymbolNode.Package (e.g. "connector", "query") — never an import
// path.
//
// CLOSED (P1-4, docs/plans/pulled-tools-excellence.md): for a METHOD
// symbol the Go extractor records the RECEIVER TYPE as SymbolNode.Package
// (internal/extractor goextract.go funcNode: pkg = receiver type name),
// which would leave a method-origin edge's from-side prefix as e.g.
// "LocalConnector", not "connector" — and a raw call INTO a method is
// often receiver-VARIABLE-qualified ("h.Method", never package-qualified
// at all), which pkgOf can't turn into a real package prefix either. The
// MCP arch_conformance handler (internal/mcp/extra_tools.go
// archConformanceEdges) now normalizes BOTH sides before matching: a
// method-origin edge's FromSymbol is rebuilt as "declaringPkg.MethodName"
// (the receiver type's own package, resolved from its GoType/GoInterface
// node — see archTypePackages/archDeclPkg), and a method-target edge's
// raw callee is resolved to the node it targets (archResolver, mirroring
// callgraph.Graph.resolve's matching order) and rebuilt the same way. A
// callee that doesn't resolve to a node in the service (external/stdlib
// calls, or genuine ambiguity) keeps its raw, unresolved form — exactly as
// before. So package-named rules now guard method-origin and
// method-target edges too, not only function-to-function ones; naming a
// receiver type directly in from_pkg/to_pkg still works as a narrower,
// receiver-scoped alternative.
#Rule: {
	from_pkg: string
	to_pkg:   string
	allow:    bool
}

// #RuleSet is the top-level shape of one archconformance.cue file: an
// ordered list of rules plus the evaluation mode. deny_by_default, when
// true, makes an edge with no matching rule a violation (allowlist mode);
// it defaults to false so a rule pack that only lists deny rules keeps its
// natural denylist meaning (internal/analysis/archconformance.Check's
// default).
#RuleSet: {
	rules: [...#Rule]
	deny_by_default: bool | *false
}
