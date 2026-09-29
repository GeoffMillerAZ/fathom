// Package archconformance checks a set of dependency edges against declared
// architectural layering rules and reports violations.
//
// Rules are expressed as (FromPkg, ToPkg, Allow bool) triples.  The package
// name embedded in an edge symbol is extracted as the longest prefix before the
// last dot; if the symbol contains no dot the whole symbol string is used as
// the package.
//
// Wildcard: a FromPkg or ToPkg value of "*" matches any package string.
//
// Evaluation order for a single edge:
//  1. Collect all rules whose (FromPkg, ToPkg) pattern matches the edge.
//  2. If any matching rule has Allow=true the edge is permitted (allow wins).
//  3. If any matching rule has Allow=false and no allow rule matched, the edge
//     is a violation.
//  4. When DenyByDefault is active, an edge with no matching rules is also a
//     violation.
//
// All exported functions are pure, deterministic, and safe for concurrent use.
package archconformance

import (
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// Rule expresses one layering constraint.
type Rule struct {
	// FromPkg is the source package prefix (or "*" for any).
	FromPkg string
	// ToPkg is the target package prefix (or "*" for any).
	ToPkg string
	// Allow, when true, permits the edge; when false, forbids it.
	Allow bool
}

// Violation describes a single edge that breaks the declared architecture.
type Violation struct {
	// From is the source symbol (as recorded on the edge).
	From string
	// To is the target symbol (as recorded on the edge).
	To string
	// Reason is a human-readable explanation of why the edge is forbidden.
	Reason string
}

// option is an internal functional option for Check.
type option struct {
	denyByDefault bool
}

// Option is a functional option for Check.
type Option func(*option)

// DenyByDefault returns an Option that causes edges with no matching rule to be
// treated as violations (deny-by-default / allowlist mode).
func DenyByDefault() Option {
	return func(o *option) { o.denyByDefault = true }
}

// pkgOf extracts the package name from a symbol key.  The convention is
// "<pkg>.<Symbol>"; if there is no dot the whole string is the package.
func pkgOf(symbol string) string {
	idx := strings.LastIndex(symbol, ".")
	if idx < 0 {
		return symbol
	}
	return symbol[:idx]
}

// matches reports whether the rule's (FromPkg, ToPkg) pattern covers (from, to).
func (r Rule) matches(from, to string) bool {
	fromOK := r.FromPkg == "*" || r.FromPkg == from
	toOK := r.ToPkg == "*" || r.ToPkg == to
	return fromOK && toOK
}

// Check evaluates edges against rules and returns all Violations.
//
// opts may include DenyByDefault() to enable allowlist mode.  By default only
// explicit deny rules generate violations.
//
// The returned slice is sorted by (From, To) for determinism.
func Check(edges []core.Edge, rules []Rule, opts ...Option) []Violation {
	cfg := &option{}
	for _, o := range opts {
		o(cfg)
	}

	var violations []Violation

	for _, e := range edges {
		fromPkg := pkgOf(e.FromSymbol)
		toPkg := pkgOf(e.ToSymbol)

		var hasDeny, hasAllow bool
		for _, r := range rules {
			if !r.matches(fromPkg, toPkg) {
				continue
			}
			if r.Allow {
				hasAllow = true
			} else {
				hasDeny = true
			}
		}

		if hasAllow {
			// An explicit allow always wins — edge is permitted.
			continue
		}

		if hasDeny {
			violations = append(violations, Violation{
				From:   e.FromSymbol,
				To:     e.ToSymbol,
				Reason: fmt.Sprintf("package %q must not import %q (deny rule)", fromPkg, toPkg),
			})
			continue
		}

		if cfg.denyByDefault {
			violations = append(violations, Violation{
				From:   e.FromSymbol,
				To:     e.ToSymbol,
				Reason: fmt.Sprintf("package %q importing %q is not permitted by any allow rule (deny-by-default)", fromPkg, toPkg),
			})
		}
	}

	// Sort for determinism.
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].From != violations[j].From {
			return violations[i].From < violations[j].From
		}
		return violations[i].To < violations[j].To
	})

	return violations
}

// DenyByDefaultRule is the RuleCount key for violations no deny rule names
// that DenyByDefault() makes violations.
const DenyByDefaultRule = "(deny by default)"

// RuleKey names a rule the way a violation-count reading keys it.
func RuleKey(r Rule) string { return r.FromPkg + " -> " + r.ToPkg }

// RuleCount is one deny rule's violation count over an edge set.
type RuleCount struct {
	Rule  string // RuleKey of a deny rule, or DenyByDefaultRule
	Count int
}

// RuleCounts attributes Check's violations to the deny rules that forbid
// them (CX-20, the drift radar's per-rule trend): one row per distinct deny
// rule, zero counts included, so a reading distinguishes "checked, none"
// from "not a rule"; plus a DenyByDefaultRule row when DenyByDefault() is
// given. An edge two deny rules both match counts once under each, so the
// per-rule sum can exceed len(Check(...)) — the rows answer "how many
// edges break this rule", not a total. Sorted by rule.
func RuleCounts(edges []core.Edge, rules []Rule, opts ...Option) []RuleCount {
	cfg := &option{}
	for _, o := range opts {
		o(cfg)
	}
	counts := map[string]int{}
	var denies []Rule
	for _, r := range rules {
		if r.Allow {
			continue
		}
		if _, ok := counts[RuleKey(r)]; !ok {
			counts[RuleKey(r)] = 0
			denies = append(denies, r)
		}
	}
	if cfg.denyByDefault {
		counts[DenyByDefaultRule] = 0
	}
	for _, e := range edges {
		fromPkg, toPkg := pkgOf(e.FromSymbol), pkgOf(e.ToSymbol)
		allowed := false
		for _, r := range rules {
			if r.Allow && r.matches(fromPkg, toPkg) {
				allowed = true
				break
			}
		}
		if allowed {
			continue
		}
		denied := false
		for _, r := range denies {
			if r.matches(fromPkg, toPkg) {
				counts[RuleKey(r)]++
				denied = true
			}
		}
		if !denied && cfg.denyByDefault {
			counts[DenyByDefaultRule]++
		}
	}
	out := make([]RuleCount, 0, len(counts))
	for k, n := range counts {
		out = append(out, RuleCount{Rule: k, Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rule < out[j].Rule })
	return out
}
