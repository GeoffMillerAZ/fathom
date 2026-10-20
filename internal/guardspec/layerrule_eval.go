// layerrule_eval.go — EvaluateLayerRule (VB-13): checks one LayerRule
// against a LayerRuleIndex and reports every Violation.
//
// The package-level edges a LayerRuleIndex carries are declaring-package
// directories ("internal/store", not the bare package name "store" —
// avoiding the name-collision risk a bare-name graph would carry), built
// by one of two producers that must agree on that convention so the SAME
// rule text means the same thing to both:
//
//   - BuildIndexFromSymbols (this file): call-derived, from the index's
//     own []core.SymbolNode — what backs live obligations serving. GAP:
//     a package dependency that exists ONLY through a type reference,
//     embed, or unused import — never a call — is invisible to it. See
//     internal/guardspec/layerrule_eval_test.go's go/packages cross-check
//     against the fauxtown fixture for the honest measurement of that gap.
//   - internal/guardspec/layerrules.BuildIndex: the TRUE import graph via
//     go/packages, at `fathom rules gen-test`'s generated-test time —
//     no gap, but too slow for the hot edit path.
package guardspec

import (
	"path"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/archconformance"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/depgraph"
	"github.com/geoffmilleraz/fathom/internal/pathglob"
)

// Violation is one layer-rule failure.
type Violation struct {
	RuleID   string
	Kind     LayerRuleKind
	Severity Severity
	// From/To are the two ends of the offending edge: two package
	// directories for import_direction/no_cycles/boundary, two service
	// names for cross_service_direction.
	From    string
	To      string
	Message string
}

// CrossServiceEdge is one fleet-level service dependency (FromService
// depends on ToService) — what cross_service_direction checks. Building
// this list from the fleet's typed edges is the caller's job; this
// package only evaluates it.
type CrossServiceEdge struct {
	FromService string
	ToService   string
}

// LayerRuleIndex is what EvaluateLayerRule reads. See the package doc
// comment for the two producers and the convention they share.
type LayerRuleIndex struct {
	// PackageEdges is a directed graph of package-directory dependencies
	// ("internal/store" -> "internal/extractor"). Nil is treated as empty.
	PackageEdges *depgraph.Graph
	// CrossService are the fleet's service-level typed edges.
	CrossService []CrossServiceEdge
	// CallEdges are the index's package-normalized call edges, what
	// call_direction checks. Nil is treated as empty.
	CallEdges []core.Edge
}

// EvaluateLayerRule checks rule against idx and returns every violation,
// sorted deterministically by (From, To).
func EvaluateLayerRule(rule LayerRule, idx LayerRuleIndex) []Violation {
	var out []Violation
	switch rule.Kind {
	case ImportDirection:
		out = evalImportDirection(rule, idx)
	case NoCycles:
		out = evalNoCycles(rule, idx)
	case Boundary:
		out = evalBoundary(rule, idx)
	case CrossServiceDirection:
		out = evalCrossServiceDirection(rule, idx)
	case CallDirection:
		out = evalCallDirection(rule, idx)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		return out[i].To < out[j].To
	})
	return out
}

func evalImportDirection(rule LayerRule, idx LayerRuleIndex) []Violation {
	if idx.PackageEdges == nil {
		return nil
	}
	var out []Violation
	for _, from := range idx.PackageEdges.Nodes() {
		if !pathglob.MatchAny(rule.From, from) {
			continue
		}
		for _, to := range idx.PackageEdges.Deps(from) {
			if !pathglob.MatchAny(rule.To, to) {
				continue
			}
			out = append(out, Violation{
				RuleID: rule.ID, Kind: rule.Kind, Severity: rule.Severity,
				From: from, To: to,
				Message: from + " imports " + to + ": " + rule.Sentence,
			})
		}
	}
	return out
}

func evalBoundary(rule LayerRule, idx LayerRuleIndex) []Violation {
	if idx.PackageEdges == nil {
		return nil
	}
	var out []Violation
	for _, from := range idx.PackageEdges.Nodes() {
		if pathglob.MatchAny(rule.Allow, from) {
			continue
		}
		for _, to := range idx.PackageEdges.Deps(from) {
			if !pathglob.MatchAny(rule.Guarded, to) {
				continue
			}
			out = append(out, Violation{
				RuleID: rule.ID, Kind: rule.Kind, Severity: rule.Severity,
				From: from, To: to,
				Message: from + " imports guarded package " + to + " without being on the allow list: " + rule.Sentence,
			})
		}
	}
	return out
}

func evalNoCycles(rule LayerRule, idx LayerRuleIndex) []Violation {
	if idx.PackageEdges == nil {
		return nil
	}
	sub := depgraph.NewGraph()
	for _, from := range idx.PackageEdges.Nodes() {
		if !pathglob.MatchAny(rule.Scope, from) {
			continue
		}
		for _, to := range idx.PackageEdges.Deps(from) {
			if pathglob.MatchAny(rule.Scope, to) {
				sub.AddEdge(from, to)
			}
		}
	}
	var out []Violation
	for _, cycle := range sub.Cycles() {
		out = append(out, Violation{
			RuleID: rule.ID, Kind: rule.Kind, Severity: rule.Severity,
			From: cycle[0], To: cycle[len(cycle)-1],
			Message: "import cycle among " + strings.Join(cycle, " -> ") + ": " + rule.Sentence,
		})
	}
	return out
}

// evalCallDirection reports every call edge a forbidding call_direction
// rule matches. An allowing rule reports nothing. Cross-rule precedence
// (an allow overriding a deny) is applied where the whole set is checked,
// CallDirectionRules.
func evalCallDirection(rule LayerRule, idx LayerRuleIndex) []Violation {
	if rule.AllowCall {
		return nil
	}
	var out []Violation
	for _, e := range idx.CallEdges {
		from, to := pkgOf(e.FromSymbol), pkgOf(e.ToSymbol)
		if !pathglob.MatchAny(rule.From, from) || !pathglob.MatchAny(rule.To, to) {
			continue
		}
		out = append(out, Violation{
			RuleID: rule.ID, Kind: rule.Kind, Severity: rule.Severity,
			From: e.FromSymbol, To: e.ToSymbol,
			Message: e.FromSymbol + " calls " + e.ToSymbol + ": " + rule.Sentence,
		})
	}
	return out
}

// CallDirectionRules compiles the call_direction rules of set into
// archconformance rules, in file order, so review and arch_conformance
// check them with the one engine the rules/archconformance.cue form uses.
func CallDirectionRules(set LayerRuleSet) []archconformance.Rule {
	var out []archconformance.Rule
	for _, r := range set.Rules {
		if r.Kind != CallDirection {
			continue
		}
		out = append(out, archconformance.Rule{FromGlobs: r.From, ToGlobs: r.To, Allow: r.AllowCall})
	}
	return out
}

func evalCrossServiceDirection(rule LayerRule, idx LayerRuleIndex) []Violation {
	var out []Violation
	for _, e := range idx.CrossService {
		if e.FromService == rule.FromService && e.ToService == rule.ToService {
			out = append(out, Violation{
				RuleID: rule.ID, Kind: rule.Kind, Severity: rule.Severity,
				From: e.FromService, To: e.ToService,
				Message: e.FromService + " depends on " + e.ToService + ": " + rule.Sentence,
			})
		}
	}
	return out
}

// BuildIndexFromSymbols builds a LayerRuleIndex from a service's indexed
// []core.SymbolNode — the live-obligations-serving producer (see the
// package doc comment for the call-derived gap). cross is the fleet's
// cross-service edges (nil when not evaluating any cross_service_direction
// rule, or when no fleet store is wired).
func BuildIndexFromSymbols(nodes []core.SymbolNode, cross []CrossServiceEdge) LayerRuleIndex {
	dirs := packageDirs(nodes)
	g := depgraph.NewGraph()
	callEdges := archconformance.EdgesFromNodes(nodes)
	for _, e := range callEdges {
		fromPkg := pkgOf(e.FromSymbol)
		toPkg := pkgOf(e.ToSymbol)
		fromDir, ok := dirs[fromPkg]
		if !ok {
			continue
		}
		toDir, ok := dirs[toPkg]
		if !ok || toDir == fromDir {
			continue
		}
		g.AddEdge(fromDir, toDir)
	}
	return LayerRuleIndex{PackageEdges: g, CrossService: cross, CallEdges: callEdges}
}

// packageDirs maps a Go file's DECLARED package name to its source
// directory, first-seen wins. GoMethod nodes are skipped: the Go extractor
// overrides SymbolNode.Package to the receiver TYPE name for methods
// (internal/extractor goextract.go funcNode — the same override
// archconformance's declPkg works around), which is never the file's real
// package. Every other kind (func, type, interface, var, const) carries
// the real declaring package unchanged.
func packageDirs(nodes []core.SymbolNode) map[string]string {
	m := map[string]string{}
	for _, n := range nodes {
		if n.Package == "" || n.FilePath == "" || n.Kind == core.KindGoMethod {
			continue
		}
		if _, ok := m[n.Package]; !ok {
			m[n.Package] = path.Dir(n.FilePath)
		}
	}
	return m
}

// pkgOf extracts the package segment from an archconformance.EdgesFromNodes
// edge endpoint ("pkg.Name" — the declaring-package-normalized form; see
// archconformance.symbolKey). Mirrors archconformance's own unexported
// pkgOf exactly (longest prefix before the last dot).
func pkgOf(symbol string) string {
	idx := strings.LastIndex(symbol, ".")
	if idx < 0 {
		return symbol
	}
	return symbol[:idx]
}
