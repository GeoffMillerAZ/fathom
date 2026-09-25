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
	"github.com/geoffmilleraz/fathom/internal/store"
)

// DefaultResidueMax is Residue's own cap (design §3.4: "capped (default
// 200, --residue-max)") when a caller passes maxItems <= 0.
const DefaultResidueMax = 200

// maxItemTests caps how many reaching test names ONE ResidueItem carries
// (a bound the counts/list-only rule, I43, does not itself set — this is
// an output-size guard, not a threshold on evidence: every reaching test
// still participates in classifying attested/unattested/unreached, only
// the rendered NAME LIST is capped). Without it a widely-called helper's
// item can carry thousands of test names, growing a single report from
// kilobytes to megabytes.
const maxItemTests = 20

// ChangedFunction is one function added or modified in base..head, found by
// ChangedFunctions — Residue's own raw material before classification.
type ChangedFunction struct {
	// Symbol is the funcDeclName convention (resurrection.go): the bare
	// function name, or "Receiver.Method" for a method.
	Symbol string
	// File/Line are the function's own HEAD location. EndLine is its
	// closing brace's line (CV-69, design §3.5): the span execution
	// evidence's block lookup matches coverage blocks against.
	File    string
	Line    int
	EndLine int
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
// "modified" when one does and the EXACT body text (funcSite.exactHash — a
// plain sha256 of the unnormalized source slice, never
// clone.FuncBodyHash's identifier-insensitive hash, which exists for
// resurrection.go's own, deliberately different question) differs. A
// function whose body is unchanged — whether it sits in the same file or
// moved to another changed file in the same directory, including one
// deleted outright at head (its base functions are still pooled here, so a
// same-directory rename never misreads as "added") — is neither, and never
// appears. A function present at base and absent from every changed file
// in its directory at head is "deleted": never reported here (the compiler
// proves it gone; the resurrection/absence planes cover what the compiler
// cannot).
//
// Matching within a name is a MULTISET over exact-text hashes, never a
// last-write-wins map: Go allows several same-named functions in one
// directory (multiple init(), or GOOS-suffixed build-tag twins), and a
// name-only match would compare an unrelated twin's body, reporting a
// stable function "modified" or hiding a real edit that happens to land on
// another twin's old text. Each head function consumes (removes) the
// first still-available base candidate whose exactHash matches; a head
// function that finds no match is "modified" when the name existed at base
// at all, "added" otherwise. An unconsumed leftover base candidate is
// simply "deleted" here (never reported, per the paragraph above).
//
// unanalyzed names a changed, non-test, non-generated .go file whose HEAD
// content could be read but not parsed as Go at all (residue.go's own
// Unanalyzed convention, extended from "not Go" to "Go fathom could not
// actually read as Go" — I44/design §3.4's honesty rule: a file fathom
// never analyzed must be named, never silently vanish as if it had zero
// changed functions).
//
// missing names a changed file whose base or head content could not be
// READ — an actual error, distinct from "did not exist" (resurrection.go's
// own Missing convention) — never silently dropped.
func ChangedFunctions(in Input) (funcs []ChangedFunction, unanalyzed []string, missing []string) {
	// pool[dir][name] holds every base funcSite found across this
	// directory's own changed files sharing that name — the multiset a
	// same-named head function's exact text is matched, and consumed,
	// against (design §3.4: "the same package directory", never only the
	// same file; this doc comment's own "Matching within a name" above).
	pool := map[string]map[string][]funcSite{}
	poolFor := func(dir string) map[string][]funcSite {
		p, ok := pool[dir]
		if !ok {
			p = map[string][]funcSite{}
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
		baseBody, baseOK, err := in.BaseFile(filePath)
		if err != nil {
			missing = append(missing, fmt.Sprintf("%s: reading base content: %v", filePath, err))
			continue
		}

		dir := path.Dir(filePath)
		if baseOK {
			p := poolFor(dir)
			if baseSites, parsed := funcSitesIn(filePath, baseBody); parsed {
				for _, f := range baseSites {
					p[f.name] = append(p[f.name], f)
				}
			}
			// An unparseable BASE is left as "nothing pooled from this
			// file" — the same honest-empty shape as a file that was new
			// at base — rather than failing the whole directory; a head
			// function in this exact file may still resolve to "modified"
			// via another changed file's base pool in the same directory.
		} else {
			poolFor(dir) // ensure the directory exists even with no base content
		}

		if !headOK {
			continue // deleted at head: its base was still pooled above
		}
		if connector.IsGenerated(filePath, headBody) {
			continue // generated files: the generated plane's territory
		}
		headSites, parsed := funcSitesIn(filePath, headBody)
		if !parsed {
			// Readable but not valid Go (e.g. a WIP syntax error, or a
			// build-ignored fixture under a directory named "_..."):
			// honestly unanalyzed, never silently zero (I44).
			unanalyzed = append(unanalyzed, filePath)
			continue
		}
		files = append(files, headFile{dir: dir, head: headSites})
	}

	for _, fe := range files {
		p := pool[fe.dir]
		for _, f := range fe.head {
			candidates, existed := p[f.name]
			matched := -1
			for i, bf := range candidates {
				if bf.exactHash == f.exactHash {
					matched = i
					break
				}
			}
			switch {
			case matched >= 0:
				// Consume the matched candidate so a second, genuinely
				// different same-named head function cannot also claim
				// it (the multiset rule above).
				p[f.name] = append(candidates[:matched:matched], candidates[matched+1:]...)
				// Identical body: an ordinary unchanged function, or one
				// that simply moved to another changed file in the same
				// package directory — design §3.4's own "neither".
			case existed:
				funcs = append(funcs, ChangedFunction{Symbol: f.name, File: f.file, Line: f.line, EndLine: f.endLine, Change: "modified"})
			default:
				funcs = append(funcs, ChangedFunction{Symbol: f.name, File: f.file, Line: f.line, EndLine: f.endLine, Change: "added"})
			}
		}
	}

	sort.Strings(unanalyzed)
	sort.Strings(missing)
	return funcs, unanalyzed, missing
}

// ResidueItem is one changed function the reading set could not clear —
// fathom.conserve/v1's own residue.items[] shape (design §3.4/§3.5).
type ResidueItem struct {
	Symbol  string   `json:"symbol"`
	File    string   `json:"file"`
	Line    int      `json:"line"`
	Change  string   `json:"change"`
	Class   string   `json:"class"`
	Tests   []string `json:"tests"`
	Callers int      `json:"callers"`

	// Evidence names which evidence classed this item (CV-69, design
	// §3.5): evidenceExecuted when a usable, commit-bound coverage
	// profile's own block data decided the class ("attested"/"partial"/
	// "unexecuted" purely from execution, never call-graph reach);
	// evidenceStaticReach when static resolved-edge reach plus a
	// commit-bound passing run decided it instead (design §3.4's
	// original rule) — because no usable profile existed at all, or this
	// SPECIFIC function's (or its reaching tests') file changed since the
	// profile's own bound commit.
	Evidence string `json:"evidence"`

	// UnexecutedLines names the changed line ranges ("start-end", HEAD
	// line numbers, ascending) that never ran, for a "partial" item only
	// (design §3.5's own "the item lists those line ranges
	// (unexecuted_lines)"). Empty for every other class.
	UnexecutedLines []string `json:"unexecuted_lines,omitempty"`
}

// evidenceExecuted/evidenceStaticReach are ResidueItem.Evidence's only two
// values (CV-69).
const (
	evidenceExecuted    = "executed"
	evidenceStaticReach = "static-reach"
)

// ResidueReport is fathom.conserve/v1's additive "residue" object (design
// §3.4/§3.5): counts and lists only (I43) — never a percentage, ratio or
// threshold. Never a plane: it carries no Verdict and never changes one.
type ResidueReport struct {
	Changed    int `json:"changed"`
	Attested   int `json:"attested"`
	Unattested int `json:"unattested"`
	Unreached  int `json:"unreached"`
	// Partial/Unexecuted (CV-69, design §3.5) count items an execution-
	// evidence class placed: Partial when SOME of a function's changed
	// blocks ran and some did not, Unexecuted when NONE did. Both stay 0
	// when no usable coverage classed anything — the pre-CV-67 shape.
	Partial    int `json:"partial"`
	Unexecuted int `json:"unexecuted"`
	// UnanalyzedFunctions counts a changed function whose own FILE the
	// index never analyzed at all (vendor/, an ignore rule, a build-tag
	// exclusion for this host) — CV-71's own reconciliation: Changed
	// always equals Attested+Unattested+Unreached+Partial+Unexecuted+
	// UnanalyzedFunctions, so a reader can see where every changed
	// function went. Distinct from Unanalyzed below, which names FILES,
	// not a per-function count.
	UnanalyzedFunctions int `json:"unanalyzed_functions"`

	Items      []ResidueItem `json:"items"`
	Truncated  int           `json:"truncated"`
	Unanalyzed []string      `json:"unanalyzed"`
	Missing    []string      `json:"missing"`

	// CoverageUnusable names why NO coverage profile could be used at all
	// (CV-70, design §3.5's "An unusable profile is named in the
	// report"): unbound (no commit), dirty, from a failing run, or an
	// operational failure reading/parsing it. "" when a usable profile
	// classed at least one item by execution, or when no profile was
	// ever configured at all (itself not a reportable failure — most
	// runs simply have none).
	CoverageUnusable string `json:"coverage_unusable,omitempty"`
}

// Residue computes the reading set (design §3.4, CV-61/CV-62/CV-63):
// ChangedFunctions' own output, each classed attested/unattested/unreached
// from callgraph.TestImpactDetail over the store's RESOLVED call edges
// ONLY (callgraph.BuildResolved — never BuildBoth's syntax-derived,
// tolerant-name-matched edges, which a heuristic caller could satisfy with
// no typed-pass confirmation at all) and the reaching tests' commit-bound
// runs. maxItems <= 0 uses DefaultResidueMax.
//
// Honesty (I44, CV-62): the reading set classes NOTHING — Items stays
// empty and Attested/Unattested/Unreached stay zero — when in.IndexAtHead
// is false, in.FailedTypedPackages is non-empty, in.Store is nil, a
// changed file's base/head content could not be read (ChangedFunctions'
// own missing), or the store's typed edges could not be READ (a store
// error is never silently treated as "no typed edges"). Every such reason
// is named in Missing. Changed and Unanalyzed are reported regardless:
// both are pure facts of the diff itself (ChangedFunctions' own count;
// every non-.go changed file plus every unparseable .go one, by path),
// computed without touching the store, so a stale or partial index never
// taints them — exactly how a Plane with a missing input still reports the
// Findings it safely computed (conserve.go's own Plane contract) alongside
// an overridden verdict. A changed .go file whose functions are wholly
// absent from the index at HEAD (every symbol, not just the one changed —
// an ignore rule, a build-tag exclusion for this host, or a whole-file
// extraction gap) is likewise named in Unanalyzed rather than classed
// "unreached": fathom never analyzed it at all, so "no test reaches it" —
// resolved-edge reach's own claim — would assert something never checked.
func Residue(ctx context.Context, in Input, maxItems int) ResidueReport {
	if maxItems <= 0 {
		maxItems = DefaultResidueMax
	}

	changed, cfUnanalyzed, cfMissing := ChangedFunctions(in)
	rep := ResidueReport{
		Changed:    len(changed),
		Items:      []ResidueItem{},
		Unanalyzed: mergedSortedUnique(unanalyzedFiles(in.ChangedFiles), cfUnanalyzed),
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
	edges, err := in.Store.TypedEdges(ctx, in.Service)
	if err != nil {
		rep.Missing = []string{fmt.Sprintf("querying typed edges: %v", err)}
		return rep
	}
	// A resolved-only graph (never BuildBoth): design §3.4/CV-61 define
	// both a class ("unreached: no test reaches it over resolved edges")
	// and the ordering key (resolvedCallerCount) in terms of resolved
	// edges alone. Walking BuildBoth's syntax-derived edges too would let
	// a tolerant, unconfirmed name match "attest" a function no typed pass
	// ever verified reaches it.
	g := callgraph.BuildResolved(nodes, edges)

	byFileName := make(map[string]core.SymbolNode, len(nodes))
	byKey := make(map[string]core.SymbolNode, len(nodes))
	filesIndexed := make(map[string]bool, len(nodes))
	keyFiles := make(map[string]map[string]bool, len(nodes))
	for _, n := range nodes {
		byKey[n.Key()] = n
		filesIndexed[n.FilePath] = true
		if n.Kind == core.KindGoFunc || n.Kind == core.KindGoMethod {
			byFileName[n.FilePath+"\x00"+n.Name] = n
			if keyFiles[n.Key()] == nil {
				keyFiles[n.Key()] = map[string]bool{}
			}
			keyFiles[n.Key()][n.FilePath] = true
		}
	}

	// globalCovOK/covUnusableReason (CV-69/CV-70, design §3.5) gate
	// execution evidence for the WHOLE run: an unbound, dirty or
	// failing-run profile is never used, and every item below falls back
	// to static reach — the pre-CV-67 classification, unchanged. A
	// profile that DOES clear the bar can still be refused per function
	// (coverageUsableForFunction) when that function's or its reaching
	// tests' own file moved since the profile's bound commit.
	globalCovOK, covUnusableReason := coverageUsableGlobally(in)
	rep.CoverageUnusable = covUnusableReason

	var excludedFiles []string
	runsCache := map[string]testRunsCacheEntry{}
	items := make([]ResidueItem, 0, len(changed))
	unanalyzedFuncs := 0
	for _, cf := range changed {
		item := ResidueItem{Symbol: cf.Symbol, File: cf.File, Line: cf.Line, Change: cf.Change, Tests: []string{}, Evidence: evidenceStaticReach}

		node, ok := byFileName[cf.File+"\x00"+cf.Symbol]
		if !ok && !filesIndexed[cf.File] {
			// The WHOLE file carries no node at all — fathom never
			// analyzed it (an ignore rule, a build-tag exclusion for
			// this host, or a whole-file extraction gap), so "no test
			// reaches it" would be a claim with no evidence behind it
			// (I44/design §3.4 honesty). Named, not misclassed — and
			// counted so Changed still reconciles (CV-71).
			excludedFiles = append(excludedFiles, cf.File)
			unanalyzedFuncs++
			continue
		}

		var (
			ambiguousKey      bool
			reached           bool
			attested          bool
			reachingTestFiles []string
		)
		if ok {
			key := node.Key()
			// keyFiles[key] carries every FilePath sharing this exact
			// index key — GOOS/build-tag twins with the same
			// package-qualified name collapse to one key
			// (core.SymbolNode.Key() carries no file), so a test that
			// reaches "the key" cannot be trusted to have reached THIS
			// twin's specific compiled body. Reach/callers still walk
			// the shared key (the graph has no finer identity to
			// offer), but such an item can never read attested: at most
			// unattested, honestly short of proof rather than a false
			// positive (CV-M12's own "genuinely... in the right class").
			ambiguousKey = len(keyFiles[key]) > 1
			item.Callers = resolvedCallerCount(g, key)
			reach := callgraph.TestImpactDetail(g, key)
			if len(reach) > 0 {
				reached = true
				tests := make([]string, 0, len(reach))
				for _, r := range reach {
					tn, tok := byKey[r.Key]
					if !tok {
						continue
					}
					tests = append(tests, tn.Name)
					reachingTestFiles = append(reachingTestFiles, tn.FilePath)
					if attested || ambiguousKey {
						continue // one attesting test is enough; still list every reaching test's name
					}
					if isAttestedBy(ctx, in, tn, node, runsCache) {
						attested = true
					}
				}
				sort.Strings(tests)
				if len(tests) > maxItemTests {
					tests = tests[:maxItemTests]
				}
				item.Tests = tests
				reachingTestFiles = sortedUniqueSlice(reachingTestFiles)
			}
		}
		// !ok (file indexed, symbol not): reached stays false — a
		// build-tag-excluded function sharing a file with others that
		// aren't, or a genuine extraction gap for just this function;
		// honest default, never guessed. Coverage execution evidence
		// below does not depend on this symbol being indexed at all
		// (cf.File/Line/EndLine come from the AST parse, not the index),
		// so it still gets a chance to class this item correctly.

		staticClass := "unreached"
		if reached {
			if attested {
				staticClass = "attested"
			} else {
				staticClass = "unattested"
			}
		}

		if blocks, usable := coverageUsableForFunction(in, cf, reachingTestFiles, globalCovOK); usable {
			item.Evidence = evidenceExecuted
			classifyByExecution(&item, changedBlocksFor(in, cf, blocks))
		} else {
			item.Class = staticClass
		}
		items = append(items, item)
	}
	if len(excludedFiles) > 0 {
		rep.Unanalyzed = mergedSortedUnique(rep.Unanalyzed, excludedFiles)
	}
	rep.UnanalyzedFunctions = unanalyzedFuncs

	classRank := map[string]int{"unexecuted": 0, "unreached": 1, "partial": 2, "unattested": 3}
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
		case "partial":
			rep.Partial++
			listed = append(listed, it)
		case "unexecuted":
			rep.Unexecuted++
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

// testRunsCacheEntry memoizes one TestCaseRunsForSymbol call's outcome.
type testRunsCacheEntry struct {
	runs []store.TestCaseRun
	err  error
}

// cachedTestCaseRuns is in.Store.TestCaseRunsForSymbol(ctx, in.Service,
// testKey, 1), memoized in cache for the lifetime of one Residue call.
// Without it, a widely-reused Test* function (the common case: a shared
// helper, or a table test many changed functions all route through) pays
// for the identical store query once per changed function that reaches
// it — O(changed x reach) round trips against a query whose answer never
// changes mid-call. A CPU profile on a 55-commit range measured 94% of
// Residue's own time in exactly this repeated query (docs/bench).
func cachedTestCaseRuns(ctx context.Context, in Input, cache map[string]testRunsCacheEntry, testKey string) ([]store.TestCaseRun, error) {
	if e, ok := cache[testKey]; ok {
		return e.runs, e.err
	}
	runs, err := in.Store.TestCaseRunsForSymbol(ctx, in.Service, testKey, 1)
	cache[testKey] = testRunsCacheEntry{runs: runs, err: err}
	return runs, err
}

// isAttestedBy reports whether test's latest attested run vouches for
// changed's behaviour (design §3.4's "attested"): the newest run for
// test's symbol passed, is bound to a commit, was made on a clean tree,
// and neither test's own file nor changed's file changed since that
// commit (certify.ReachUnchangedSince, O114's rule, applied without a
// declared target). Any failure to establish this — no run, a failing or
// unbound newest run, or in.ReachUnchangedSince itself unavailable — is
// honestly "not attested through this test," never a guess. runsCache
// memoizes the store lookup per test key across one Residue call
// (cachedTestCaseRuns's own doc comment).
func isAttestedBy(ctx context.Context, in Input, test, changed core.SymbolNode, runsCache map[string]testRunsCacheEntry) bool {
	if in.ReachUnchangedSince == nil {
		return false
	}
	runs, err := cachedTestCaseRuns(ctx, in, runsCache, test.Key())
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

// coverageUsableGlobally reports whether in.Coverage clears CV-70/I44's
// bound/dirty/passed bar for the WHOLE run, and names why not otherwise.
// in.Coverage == nil with no in.CoverageLoadError reads as "nothing was
// configured at all" — a benign, non-reportable state (most runs simply
// have no profile), distinct from a profile that WAS found and failed the
// bar.
func coverageUsableGlobally(in Input) (usable bool, reason string) {
	if in.CoverageLoadError != "" {
		return false, in.CoverageLoadError
	}
	if in.Coverage == nil {
		return false, ""
	}
	if in.CoverageMeta.Commit == "" {
		return false, "coverage meta names no bound commit"
	}
	if in.CoverageMeta.Dirty {
		return false, "coverage meta was captured on a dirty working tree"
	}
	if !in.CoverageMeta.Passed {
		return false, "coverage meta's run did not pass"
	}
	return true, ""
}

// coverageUsableForFunction reports whether cf's own execution can be
// trusted from in.Coverage (CV-69/CV-70's per-function half of "Usable
// only when bound"): the global bar (globalOK) cleared, at least one
// coverage block falls inside cf's own line span, and neither cf's file
// nor any of testFiles (its reaching tests') changed since
// in.CoverageMeta.Commit (O114's rule, applied exactly like
// isAttestedBy already applies it for the static path — this package's
// own "one rule, never a second implementation that could drift"). On
// success, returns every coverage block inside cf's span (not yet
// filtered to the CHANGED ones — changedBlocksFor does that).
func coverageUsableForFunction(in Input, cf ChangedFunction, testFiles []string, globalOK bool) ([]CoverageBlock, bool) {
	if !globalOK {
		return nil, false
	}
	blocks := blocksInSpan(in.Coverage, cf.File, cf.Line, cf.EndLine)
	if len(blocks) == 0 {
		return nil, false
	}
	if in.ReachUnchangedSince == nil {
		return nil, false
	}
	files := append([]string{cf.File}, testFiles...)
	unchanged, err := in.ReachUnchangedSince(in.CoverageMeta.Commit, sortedUniqueSlice(files))
	if err != nil || !unchanged {
		return nil, false
	}
	return blocks, true
}

// blocksInSpan returns cov's blocks for file whose own line range
// overlaps [startLine, endLine] (a changed function's HEAD span),
// preserving LoadCoverage's own per-file (StartLine, StartCol) order.
func blocksInSpan(cov *CoverageProfile, file string, startLine, endLine int) []CoverageBlock {
	if cov == nil {
		return nil
	}
	var out []CoverageBlock
	for _, b := range cov.Blocks[file] {
		if b.EndLine < startLine || b.StartLine > endLine {
			continue
		}
		out = append(out, b)
	}
	return out
}

// changedBlocksFor narrows fnBlocks (every coverage block inside cf's own
// span) to the ones design §3.5 calls "changed blocks": every block, for
// an "added" function (design: "for an added function, that is every
// block") — or when in.ChangedLines is nil or errors, since fathom then
// has no way to know WHICH lines changed and treating every block as
// changed is the honest, conservative default (never silently clearing a
// function fathom simply could not diff); otherwise, only the blocks
// overlapping a line in.ChangedLines(cf.File) reports as added or
// changed inside cf's own file.
func changedBlocksFor(in Input, cf ChangedFunction, fnBlocks []CoverageBlock) []CoverageBlock {
	if cf.Change != "modified" || in.ChangedLines == nil {
		return fnBlocks
	}
	ranges, err := in.ChangedLines(cf.File)
	if err != nil {
		return fnBlocks
	}
	var out []CoverageBlock
	for _, b := range fnBlocks {
		for _, r := range ranges {
			if b.EndLine >= r.Start && b.StartLine <= r.End {
				out = append(out, b)
				break
			}
		}
	}
	return out
}

// classifyByExecution fills item's Class (and UnexecutedLines, for
// "partial") from changedBlocks' own per-block execution counts (design
// §3.5's "Classes with execution evidence"): "attested" when every
// changed block executed — including the vacuous case of NO changed
// block at all, a comment-only edit, design's own "cleared"; "unexecuted"
// when none did; "partial" otherwise, naming the unexecuted ranges
// ("start-end", ascending).
func classifyByExecution(item *ResidueItem, changedBlocks []CoverageBlock) {
	if len(changedBlocks) == 0 {
		item.Class = "attested"
		return
	}
	executed := 0
	var unexecuted []string
	for _, b := range changedBlocks {
		if b.Count > 0 {
			executed++
		} else {
			unexecuted = append(unexecuted, fmt.Sprintf("%d-%d", b.StartLine, b.EndLine))
		}
	}
	switch {
	case executed == len(changedBlocks):
		item.Class = "attested"
	case executed == 0:
		item.Class = "unexecuted"
	default:
		sort.Strings(unexecuted)
		item.Class = "partial"
		item.UnexecutedLines = unexecuted
	}
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

// mergedSortedUnique returns the sorted, deduplicated union of a and b —
// Residue's own Unanalyzed convention, merging ChangedFunctions' own
// (non-.go plus unparseable-.go) list with the index-exclusion list
// Residue itself discovers once the store is available.
func mergedSortedUnique(a, b []string) []string {
	set := make(map[string]bool, len(a)+len(b))
	for _, s := range a {
		set[s] = true
	}
	for _, s := range b {
		set[s] = true
	}
	return sortedSet(set)
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

// sortedUniqueSlice dedups and sorts ss (dropping any "" entries) —
// coverageUsableForFunction's own version of sortedUniqueStrings' rule,
// for an arbitrary-length file list (a function's own file plus every
// reaching test's file) rather than a fixed pair.
func sortedUniqueSlice(ss []string) []string {
	set := make(map[string]bool, len(ss))
	for _, s := range ss {
		if s != "" {
			set[s] = true
		}
	}
	return sortedSet(set)
}
