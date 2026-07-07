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
