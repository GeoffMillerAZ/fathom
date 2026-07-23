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
// constraint between two package-prefix patterns. from_pkg/to_pkg compare
// against the bare Go package-clause name recorded on core.SymbolNode.Package
// (e.g. "connector", "query", "mcp") — not an import path. "*" matches any
// package on that side of the rule.
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
