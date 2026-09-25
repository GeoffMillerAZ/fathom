package conserve

// residue.go — CV-60/CV-61/CV-62/CV-63 (docs/design/refactor-conservation.md
// §3.4, ADR-044 amended 2026-09-25, Q22): the reading set. Not a plane —
// ChangedFunctions/Residue never register with Register, never contribute a
// Verdict, and never change --strict's exit (design §3.4: "It is not a
// plane"). It answers the residue decision 7 hands to Review Kit: the
// changed functions in base..head whose behaviour no commit-bound passing
// test vouches for.
//
// ChangedFunctions reuses funcSitesIn (resurrection.go) — the same
// go/ast-over-already-read-content parse the resurrection plane already
// pays for, never a second AST walk of its own invention. Residue's
// "attested" rule reuses certify.ReachUnchangedSince (exported for exactly
// this caller, internal/certify/git.go) — O114's own "reach unchanged since
// a bound commit" check, applied here without a declared target, rather
// than a second implementation that could quietly drift from the
// certificate ladder's.
import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// DefaultResidueMax is Residue's own cap (design §3.4: "capped (default
// 200, --residue-max)") when a caller passes maxItems <= 0.
const DefaultResidueMax = 200

// ChangedFunction is one function added or modified in base..head, found by
// ChangedFunctions — Residue's own raw material before classification.
type ChangedFunction struct {
	// Symbol is the funcDeclName convention (resurrection.go): the bare
	// function name, or "Receiver.Method" for a method.
	Symbol string
	// File/Line are the function's own HEAD location.
	File string
	Line int
	// Change is "added" (no function of this name existed at base,
	// anywhere in the same changed package directory) or "modified" (one
	// did, and the exact body text differs).
	Change string
}

// ChangedFunctions returns every function ChangedFunction found across
// in.ChangedFiles' non-test, non-generated .go files (design §3.4's
// "Changed functions"): a head function is "added" when no function of the
// same receiver-qualified name exists at base ANYWHERE among the SAME
// changed files' base content in the SAME package directory (path.Dir), and
// "modified" when one does and the exact body text (clone.FuncBodyHash's
// own hash) differs. A function whose body is unchanged — whether it sits
// in the same file or moved to another changed file in the same directory
// — is neither, and never appears. A function present at base and absent
// from every changed file in its directory at head is "deleted": never
// reported here (the compiler proves it gone; the resurrection/absence
// planes cover what the compiler cannot).
//
// missing names a changed file whose base or head content could not be
// READ — an actual error, distinct from "did not exist" (resurrection.go's
// own Missing convention) — never silently dropped.
func ChangedFunctions(in Input) (funcs []ChangedFunction, missing []string) {
	// pool[dir] indexes every base funcSite found across this directory's
	// own changed files, by name — the matching pool "added"/"modified"
	// resolve against (design §3.4: "the same package directory", never
	// only the same file).
	pool := map[string]map[string]funcSite{}
	poolFor := func(dir string) map[string]funcSite {
		p, ok := pool[dir]
		if !ok {
			p = map[string]funcSite{}
			pool[dir] = p
		}
		return p
	}

	type headFile struct {
		dir  string
		head []funcSite
	}
	var files []headFile

	for _, filePath := range in.ChangedFiles {
		if !strings.HasSuffix(filePath, ".go") || strings.HasSuffix(filePath, "_test.go") {
			continue // test files: the integrity plane's territory
		}
		headBody, headOK, err := in.HeadFile(filePath)
		if err != nil {
			missing = append(missing, fmt.Sprintf("%s: reading head content: %v", filePath, err))
			continue
		}
		if !headOK {
			continue // deleted at head: nothing to add here
		}
		if connector.IsGenerated(filePath, headBody) {
			continue // generated files: the generated plane's territory
		}
		baseBody, baseOK, err := in.BaseFile(filePath)
		if err != nil {
			missing = append(missing, fmt.Sprintf("%s: reading base content: %v", filePath, err))
			continue
		}

		dir := path.Dir(filePath)
		if baseOK {
			p := poolFor(dir)
			for _, f := range funcSitesIn(filePath, baseBody) {
				p[f.name] = f
			}
		} else {
			poolFor(dir) // ensure the directory exists even with no base content
		}
		files = append(files, headFile{dir: dir, head: funcSitesIn(filePath, headBody)})
	}

	for _, fe := range files {
		p := pool[fe.dir]
		for _, f := range fe.head {
			bf, existed := p[f.name]
			switch {
			case !existed:
				funcs = append(funcs, ChangedFunction{Symbol: f.name, File: f.file, Line: f.line, Change: "added"})
			case bf.hash != f.hash:
				funcs = append(funcs, ChangedFunction{Symbol: f.name, File: f.file, Line: f.line, Change: "modified"})
			default:
				// Identical body: an ordinary unchanged function, or one
				// that simply moved to another changed file in the same
				// package directory — design §3.4's own "neither".
			}
		}
	}

	sort.Strings(missing)
	return funcs, missing
}

// ResidueItem is one changed function the reading set could not clear —
// fathom.conserve/v1's own residue.items[] shape (design §3.4).
type ResidueItem struct {
	Symbol  string   `json:"symbol"`
	File    string   `json:"file"`
	Line    int      `json:"line"`
	Change  string   `json:"change"`
	Class   string   `json:"class"`
	Tests   []string `json:"tests"`
	Callers int      `json:"callers"`
}

// ResidueReport is fathom.conserve/v1's additive "residue" object (design
// §3.4): counts and lists only (I43) — never a percentage, ratio or
// threshold. Never a plane: it carries no Verdict and never changes one.
type ResidueReport struct {
	Changed    int           `json:"changed"`
	Attested   int           `json:"attested"`
	Unattested int           `json:"unattested"`
	Unreached  int           `json:"unreached"`
	Items      []ResidueItem `json:"items"`
	Truncated  int           `json:"truncated"`
	Unanalyzed []string      `json:"unanalyzed"`
	Missing    []string      `json:"missing"`
}

// Residue computes the reading set (design §3.4, CV-61/CV-62/CV-63):
// ChangedFunctions' own output, each classed attested/unattested/unreached
// from callgraph.TestImpactDetail over the store's resolved edges and the
// reaching tests' commit-bound runs. maxItems <= 0 uses DefaultResidueMax.
//
// Honesty (I44, CV-62): the reading set classes NOTHING — Items stays
// empty and Attested/Unattested/Unreached stay zero — when in.IndexAtHead
// is false, in.FailedTypedPackages is non-empty, in.Store is nil, or a
// changed file's base/head content could not be read (ChangedFunctions'
// own missing). Every such reason is named in Missing. Changed and
// Unanalyzed are reported regardless: both are pure facts of the diff
// itself (ChangedFunctions' own count; every non-.go changed file, by
// path), computed without touching the store, so a stale or partial index
// never taints them — exactly how a Plane with a missing input still
// reports the Findings it safely computed (conserve.go's own Plane
// contract) alongside an overridden verdict.
func Residue(ctx context.Context, in Input, maxItems int) ResidueReport {
	if maxItems <= 0 {
		maxItems = DefaultResidueMax
	}

	changed, cfMissing := ChangedFunctions(in)
	rep := ResidueReport{
		Changed:    len(changed),
		Items:      []ResidueItem{},
		Unanalyzed: unanalyzedFiles(in.ChangedFiles),
		Missing:    []string{},
	}

	missingSet := map[string]bool{}
	for _, m := range cfMissing {
		missingSet[m] = true
	}
	if !in.IndexAtHead {
		reason := in.IndexAtHeadReason
		if reason == "" {
			reason = "the index is not at HEAD"
		}
		missingSet[reason] = true
	}
	for _, r := range in.FailedTypedPackages {
		missingSet["typed pass failed: "+r] = true
	}
	if in.Store == nil {
		missingSet["no store"] = true
	}
	if len(missingSet) > 0 {
		rep.Missing = sortedSet(missingSet)
		return rep
	}

	obs, err := in.Store.QueryObservations(ctx, in.Service, "")
	if err != nil {
		rep.Missing = []string{fmt.Sprintf("querying the index: %v", err)}
		return rep
	}
	nodes := make([]core.SymbolNode, 0, len(obs))
	for _, o := range obs {
		nodes = append(nodes, o.ToSymbolNode())
	}
	edges, _ := in.Store.TypedEdges(ctx, in.Service)
	g := callgraph.BuildBoth(nodes, edges)

	byFileName := make(map[string]core.SymbolNode, len(nodes))
	byKey := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byKey[n.Key()] = n
		if n.Kind == core.KindGoFunc || n.Kind == core.KindGoMethod {
			byFileName[n.FilePath+"\x00"+n.Name] = n
		}
	}

	items := make([]ResidueItem, 0, len(changed))
	for _, cf := range changed {
		item := ResidueItem{Symbol: cf.Symbol, File: cf.File, Line: cf.Line, Change: cf.Change, Tests: []string{}}

		node, ok := byFileName[cf.File+"\x00"+cf.Symbol]
		if !ok {
			// Not found in the index at HEAD (a build-tag exclusion, or an
			// extraction gap): honest default — nothing PROVES a test
			// reaches it, so it is unreached, never guessed otherwise.
			item.Class = "unreached"
			items = append(items, item)
			continue
		}

		key := node.Key()
		item.Callers = resolvedCallerCount(g, key)
		reach := callgraph.TestImpactDetail(g, key)
		if len(reach) == 0 {
			item.Class = "unreached"
			items = append(items, item)
			continue
		}

		tests := make([]string, 0, len(reach))
		attested := false
		for _, r := range reach {
			tn, ok := byKey[r.Key]
			if !ok {
				continue
			}
			tests = append(tests, tn.Name)
			if attested {
				continue // one attesting test is enough; still list every reaching test's name
			}
			if isAttestedBy(ctx, in, tn, node) {
				attested = true
			}
		}
		sort.Strings(tests)
		item.Tests = tests
		if attested {
			item.Class = "attested"
		} else {
			item.Class = "unattested"
		}
		items = append(items, item)
	}

	classRank := map[string]int{"unreached": 0, "unattested": 1}
	var listed []ResidueItem
	for _, it := range items {
		switch it.Class {
		case "attested":
			rep.Attested++
		case "unattested":
			rep.Unattested++
			listed = append(listed, it)
		case "unreached":
			rep.Unreached++
			listed = append(listed, it)
		}
	}
	sort.SliceStable(listed, func(i, j int) bool {
		if classRank[listed[i].Class] != classRank[listed[j].Class] {
			return classRank[listed[i].Class] < classRank[listed[j].Class]
		}
		if listed[i].Callers != listed[j].Callers {
			return listed[i].Callers > listed[j].Callers
		}
		return listed[i].Symbol < listed[j].Symbol
	})
	if len(listed) > maxItems {
		rep.Truncated = len(listed) - maxItems
		listed = listed[:maxItems]
	}
	rep.Items = listed
	if rep.Items == nil {
		rep.Items = []ResidueItem{}
	}
	return rep
}

// isAttestedBy reports whether test's latest attested run vouches for
// changed's behaviour (design §3.4's "attested"): the newest run for
// test's symbol passed, is bound to a commit, was made on a clean tree,
// and neither test's own file nor changed's file changed since that
// commit (certify.ReachUnchangedSince, O114's rule, applied without a
// declared target). Any failure to establish this — no run, a failing or
// unbound newest run, or in.ReachUnchangedSince itself unavailable — is
// honestly "not attested through this test," never a guess.
func isAttestedBy(ctx context.Context, in Input, test, changed core.SymbolNode) bool {
	if in.ReachUnchangedSince == nil {
		return false
	}
	runs, err := in.Store.TestCaseRunsForSymbol(ctx, in.Service, test.Key(), 1)
	if err != nil || len(runs) == 0 {
		return false
	}
	latest := runs[0]
	if latest.Status != "pass" || latest.Commit == "" || latest.Dirty {
		return false
	}
	unchanged, err := in.ReachUnchangedSince(latest.Commit, sortedUniqueStrings(test.FilePath, changed.FilePath))
	if err != nil {
		return false
	}
	return unchanged
}

// resolvedCallerCount counts symbol's callers whose edge is
// core.EdgeConfidenceResolved (a real typed-pass confirmation, never a
// syntax-only guess) — the reading set's own ordering key (design §3.4:
// "resolved caller count descending").
func resolvedCallerCount(g *callgraph.Graph, key string) int {
	n := 0
	for _, ci := range callgraph.CallersDetail(g, key) {
		if core.EdgeConfidenceOf(core.Edge{Kind: ci.Kind}) == core.EdgeConfidenceResolved {
			n++
		}
	}
	return n
}

// unanalyzedFiles returns every changed file that is not a .go file,
// sorted (design §3.4: "changed files in a language without typed edges
// are listed as unanalyzed by path" — the reading set only ever extracts
// functions from Go source, so every other changed file is, honestly,
// never checked at all).
func unanalyzedFiles(changedFiles []string) []string {
	out := make([]string, 0, len(changedFiles))
	for _, f := range changedFiles {
		if !strings.HasSuffix(f, ".go") {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

// sortedSet renders a string set as a sorted slice — Residue's own Missing
// convention (deduplicated, deterministic), matching Run's fs/ms pattern
// in conserve.go.
func sortedSet(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// sortedUniqueStrings returns a and b, deduplicated (a == b is common: a
// test in the same file as what it tests) and sorted — the scope
// isAttestedBy hands to in.ReachUnchangedSince.
func sortedUniqueStrings(a, b string) []string {
	if a == b {
		return []string{a}
	}
	out := []string{a, b}
	sort.Strings(out)
	return out
}
