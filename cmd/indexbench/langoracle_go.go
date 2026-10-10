package main

// langoracle_go.go — LB-F58: Go's L3. Go is native (goextract.go): it has no
// tags dialect and no file resolver (extractor.FileResolveLangs skips the
// go-package strategy), so fusion.ResolveFileEdges cannot run for it. What
// ingest stores for Go across files is the typed go/packages pass's edges
// (fusion.EnrichTypedEdges): the harness runs that pass in process into an
// in-memory store and scores its edges against the SCIP index's cross-file
// edges, and resolves each import spec with Go's own loader to the measured
// files of the package it names, scored against the index's references on
// the import lines (import_resolution).

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/langbench"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// goDialect is the native Go dialect's id.
const goDialect = "go"

// noteGoTypedFileEdges discloses what Go's L3 metrics measure.
const noteGoTypedFileEdges = "go: file_edge_precision and file_edge_recall score the typed go/packages pass's edges (fusion.EnrichTypedEdges, the cross-file edges ingest stores for Go, run in process into an in-memory store), and import_resolution each import spec resolved by Go's own loader (go/packages, the typed pass's environment) to the measured files of the package it names; Go has no tags dialect and no file resolver (LB-F58)"

// goTypedEdges runs the typed pass over root, scoped to the measured files
// (the connector-scope filter ingest passes), and returns the edges it
// stored.
func goTypedEdges(ctx context.Context, service, root string, inScope map[string]bool) ([]core.Edge, fusion.TypedEdgeReport, error) {
	st := store.NewMemoryStore()
	defer st.Close()
	rep, err := fusion.EnrichTypedEdges(ctx, st, service, root, inScope)
	if err != nil {
		return nil, rep, err
	}
	edges, err := st.TypedEdges(ctx, service)
	return edges, rep, err
}

// goImportSites is Go's imports: every import spec of the measured .go
// files, at the lines of the spec itself, resolved by go/packages (loaded
// from root under fusion.TypedEdgeEnv, as the typed pass loads) to the
// measured files of the package its path names; a path no package of the
// module has (the standard library, a dependency) resolves to nothing.
// faults names the files whose imports could not be parsed.
func goImportSites(ctx context.Context, root string, files []core.SourceFile, inScope map[string]bool) (sites []langbench.ImportSite, faults []string, err error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, err
	}
	pkgs, err := packages.Load(&packages.Config{Context: ctx, Dir: abs, Env: fusion.TypedEdgeEnv(), Mode: packages.NeedName | packages.NeedFiles}, "./...")
	if err != nil {
		return nil, nil, fmt.Errorf("go/packages load: %w", err)
	}
	rel := goRelPath(abs)
	filesOf := map[string][]string{} // import path -> the package's measured files
	for _, p := range pkgs {
		for _, f := range p.GoFiles {
			if r, ok := rel(f); ok && inScope[r] {
				filesOf[p.PkgPath] = append(filesOf[p.PkgPath], r)
			}
		}
	}
	for _, fs := range filesOf {
		sort.Strings(fs)
	}
	for _, f := range files {
		fset := token.NewFileSet()
		af, perr := parser.ParseFile(fset, f.Path, f.Content, parser.ImportsOnly)
		if perr != nil && af == nil {
			faults = append(faults, f.Path)
			continue
		}
		for _, spec := range af.Imports {
			ip, uerr := strconv.Unquote(spec.Path.Value)
			if uerr != nil {
				continue
			}
			local := path.Base(ip)
			if spec.Name != nil {
				local = spec.Name.Name
			}
			var targets []string
			for _, t := range filesOf[ip] {
				if t != f.Path {
					targets = append(targets, t)
				}
			}
			sites = append(sites, langbench.ImportSite{Path: f.Path, StartLine: fset.Position(spec.Pos()).Line,
				EndLine: fset.Position(spec.End()).Line, Local: local, Targets: targets})
		}
	}
	return sites, faults, nil
}

// goRelPath maps an absolute file go/packages reports to its path relative
// to root (forward slashes), trying root as given and with its symlinks
// resolved (go list may report either).
func goRelPath(root string) func(string) (string, bool) {
	roots := []string{root}
	if r, err := filepath.EvalSymlinks(root); err == nil && r != root {
		roots = append(roots, r)
	}
	return func(f string) (string, bool) {
		for _, r := range roots {
			if rel, err := filepath.Rel(r, f); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return filepath.ToSlash(rel), true
			}
		}
		return "", false
	}
}

// goCallSet names one candidate for the pairs Go's L2 grades (language-
// breadth §1.6). Every option is computed on every run so a reading carries
// all of them; goL2Set is the one scored.
type goCallSet string

const (
	// goCallsSyntactic (A) is goextract's collectCalls, every callee written
	// in the source (the L2 of every reading before wave le).
	goCallsSyntactic goCallSet = "syntactic"
	// goCallsTyped (B) is the typed pass's edges alone, read through the
	// call graph the product builds (callgraph.AddTypedEdges, in-service
	// targets only).
	goCallsTyped goCallSet = "typed"
	// goCallsServed (C) is callgraph.BuildBoth read through its callers and
	// callees views: the syntactic Calls that resolve to a node of the
	// service plus the typed edges, less (since wave lg) the syntax-only
	// edges into a method from a file the typed pass resolved (E,
	// LB-F199) and the references to a Go type (D, LB-F200) — what
	// symbol_context, blast_radius with include_heuristic and verify read.
	goCallsServed goCallSet = "served"
	// goCallsServedPreLG is C as the product served it before wave lg
	// (BuildBoth at 90f0a01f): every syntactic edge that reaches a node,
	// references to a type included, plus the typed edges. Built here
	// explicitly from the syntax graph's dependencies and the typed graph.
	goCallsServedPreLG goCallSet = "served-pre-lg"
	// goCallsServedNoRefs (D) is the pre-lg C without the syntactic edges
	// to a type: what LB-F200 moved out of the callers views.
	goCallsServedNoRefs goCallSet = "served-no-type-refs"
	// goCallsServedReach (E) is the pre-lg C without the syntax-only edges
	// into a method from a file the typed pass resolved: the rule
	// ProductReach.walk applied, which LB-F199 moved into the graph.
	goCallsServedReach goCallSet = "served-typed-methods"
	// goCallsServedBoth (F) is D and E together: the set C now serves.
	goCallsServedBoth goCallSet = "served-no-type-refs-typed-methods"
)

// goL2Set is the pair set Go's L2 scores. §1.6 records why.
const goL2Set = goCallsServed

// goTypedCallStats counts what the typed pass's edges were, and what reading
// them as call pairs left out, so no class hides (§1.6).
type goTypedCallStats struct {
	Direct, Dispatch, FuncValue, Unresolved, Instantiates int
	DispatchPairs                                         int // distinct (caller, callee name) pairs the dispatch edges collapse to
	NoCallerNode                                          int // an edge whose caller has no node in the measured files (a package initializer)
}

// goCallSets are the candidate graded sets over one subject.
type goCallSets struct {
	Syntactic    map[langbench.CallPair]bool // A: nodes' Calls, as FathomCallPairs reads them
	SyntacticSrv map[langbench.CallPair]bool // A restricted to callees that resolve to a node (callgraph.Build)
	Typed        map[langbench.CallPair]bool // B
	Served       map[langbench.CallPair]bool // C: the served graph's callers/callees views (since wave lg, E and D applied in the graph)
	ServedPreLG  map[langbench.CallPair]bool // C before wave lg, built explicitly
	ServedNoRefs map[langbench.CallPair]bool // D: the pre-lg C less the syntactic-only edges to a type (LB-F200)
	ServedReach  map[langbench.CallPair]bool // E: the pre-lg C less the syntactic-only edges into a method from a file the typed pass resolved (LB-F199)
	ServedBoth   map[langbench.CallPair]bool // F: D and E together, which C must equal
	Stats        goTypedCallStats
	// InIndex marks the truth pairs whose callee the index defines in the
	// subject (langbench.SCIPResult.CallInIndex): the others call the
	// standard library or a dependency, which the product cannot serve.
	InIndex map[langbench.CallPair]bool
}

// goCallPairSets reads the candidate sets from the extracted nodes and the
// typed pass's edges. A dispatch edge to each of N implementers is N edges
// but, written as pairs (caller, callee bare name), one pair per distinct
// method name: scip-go's single interface-method term (O261) and the N
// implementers meet at that name. An EdgeUnresolved edge names no callee and
// gives no pair.
func goCallPairSets(nodes []core.SymbolNode, typed []core.Edge, files map[string]bool, inIndex map[langbench.CallPair]bool) goCallSets {
	sets := goCallSets{InIndex: inIndex}
	byKey := make(map[string]core.SymbolNode, len(nodes))
	bare := make(map[string]string, len(nodes))
	for _, n := range nodes {
		byKey[n.Key()] = n
		bare[n.Key()] = langbench.BareName(n.Name)
	}
	bareOf := func(key string) string {
		if b, ok := bare[key]; ok {
			return b
		}
		return ""
	}
	seenDispatch := map[[2]string]bool{}
	for _, e := range typed {
		switch e.Kind {
		case core.EdgeDirect:
			sets.Stats.Direct++
		case core.EdgeIfaceDispatch:
			sets.Stats.Dispatch++
			if from, to := bareOf(e.FromSymbol), bareOf(e.ToSymbol); from != "" && to != "" {
				seenDispatch[[2]string{e.FromSymbol, to}] = true
			}
		case core.EdgeFuncValue:
			sets.Stats.FuncValue++
		case core.EdgeUnresolved:
			sets.Stats.Unresolved++
		case core.EdgeInstantiates:
			sets.Stats.Instantiates++
		default:
			continue
		}
		if _, ok := byKey[e.FromSymbol]; !ok && e.Kind != core.EdgeUnresolved && e.Kind != core.EdgeInstantiates {
			sets.Stats.NoCallerNode++
		}
	}
	sets.Stats.DispatchPairs = len(seenDispatch)

	noCalls := make([]core.SymbolNode, len(nodes))
	for i, n := range nodes {
		n.Calls = nil
		noCalls[i] = n
	}
	kindOf := make(map[string]core.SourceKind, len(nodes))
	for _, n := range nodes {
		kindOf[n.Key()] = n.Kind
	}
	// pairsOf writes each node's edges, of the measured files, as pairs.
	pairsOf := func(edges func(n core.SymbolNode) []callgraph.EdgeInfo) map[langbench.CallPair]bool {
		out := map[langbench.CallPair]bool{}
		for _, n := range nodes {
			if files != nil && !files[n.FilePath] {
				continue
			}
			for _, c := range edges(n) {
				if callee := bareOf(c.Key); callee != "" {
					out[langbench.CallPair{Path: n.FilePath, Caller: langbench.BareName(n.Name), Callee: callee}] = true
				}
			}
		}
		return out
	}
	callees := func(g *callgraph.Graph) func(core.SymbolNode) []callgraph.EdgeInfo {
		return func(n core.SymbolNode) []callgraph.EdgeInfo { return callgraph.CalleesDetail(g, n.Key()) }
	}
	sets.Syntactic = langbench.FathomCallPairs(nodes, files)
	// A resolved keeps its definition (every syntactic Calls entry that
	// reaches a node), references to a type included.
	syntax := callgraph.Build(nodes)
	sets.SyntacticSrv = pairsOf(func(n core.SymbolNode) []callgraph.EdgeInfo { return callgraph.DependenciesDetail(syntax, n.Key()) })
	typedG := callgraph.BuildBoth(noCalls, typed)
	sets.Typed = pairsOf(callees(typedG))
	sets.Served = pairsOf(callees(callgraph.BuildBoth(nodes, typed)))

	// The pre-lg served graph, built explicitly: each node's syntactic
	// edges (references included) and its typed edges, an edge labelled
	// with its typed kind when the typed pass has the pair, as BuildBoth
	// labelled it before wave lg.
	preLG := func(n core.SymbolNode) []callgraph.EdgeInfo {
		kind := map[string]core.EdgeKind{}
		for _, c := range callgraph.CalleesDetail(typedG, n.Key()) {
			kind[c.Key] = c.Kind
		}
		var out []callgraph.EdgeInfo
		for _, c := range callgraph.DependenciesDetail(syntax, n.Key()) {
			out = append(out, callgraph.EdgeInfo{Key: c.Key, Kind: kind[c.Key]})
			delete(kind, c.Key)
		}
		for k, kd := range kind {
			out = append(out, callgraph.EdgeInfo{Key: k, Kind: kd})
		}
		return out
	}
	typedFile := map[string]bool{}
	for _, e := range typed {
		if e.FromFile != "" {
			typedFile[e.FromFile] = true
		}
	}
	isType := func(k string) bool { return kindOf[k] == core.KindGoType || kindOf[k] == core.KindGoInterface }
	preIf := func(keep func(from core.SymbolNode, c callgraph.EdgeInfo) bool) map[langbench.CallPair]bool {
		return pairsOf(func(n core.SymbolNode) []callgraph.EdgeInfo {
			var out []callgraph.EdgeInfo
			for _, c := range preLG(n) {
				if keep(n, c) {
					out = append(out, c)
				}
			}
			return out
		})
	}
	noRefs := func(_ core.SymbolNode, c callgraph.EdgeInfo) bool { return c.Kind != "" || !isType(c.Key) }
	noGuess := func(from core.SymbolNode, c callgraph.EdgeInfo) bool {
		return c.Kind != "" || kindOf[c.Key] != core.KindGoMethod || !typedFile[from.FilePath]
	}
	sets.ServedPreLG = preIf(func(core.SymbolNode, callgraph.EdgeInfo) bool { return true })
	sets.ServedNoRefs = preIf(noRefs)
	sets.ServedReach = preIf(noGuess)
	sets.ServedBoth = preIf(func(f core.SymbolNode, c callgraph.EdgeInfo) bool { return noRefs(f, c) && noGuess(f, c) })
	return sets
}

// pick returns the candidate named.
func (s goCallSets) pick(which goCallSet) map[langbench.CallPair]bool {
	switch which {
	case goCallsTyped:
		return s.Typed
	case goCallsServed:
		return s.Served
	case goCallsServedPreLG:
		return s.ServedPreLG
	case goCallsServedNoRefs:
		return s.ServedNoRefs
	case goCallsServedReach:
		return s.ServedReach
	case goCallsServedBoth:
		return s.ServedBoth
	}
	return s.Syntactic
}

// servableTruth keeps the truth pairs whose callee the index defines in the
// subject, and counts the rest: a call into the standard library or a
// dependency, which the typed pass and the call graph drop by design.
func (s goCallSets) servableTruth(truth map[langbench.CallPair]bool) (kept map[langbench.CallPair]bool, external int) {
	kept = map[langbench.CallPair]bool{}
	for p := range truth {
		if s.InIndex[p] {
			kept[p] = true
		} else {
			external++
		}
	}
	return kept, external
}

// goPR is a set's precision and recall against truth, as counts.
func goPR(truth, found map[langbench.CallPair]bool) (agree int) {
	for p := range found {
		if truth[p] {
			agree++
		}
	}
	return agree
}

// goCallNotes discloses every candidate's numbers against the full truth and
// the servable truth, the graded set, and each excluded class.
func goCallNotes(name string, sets goCallSets, truth map[langbench.CallPair]bool) []string {
	servable, external := sets.servableTruth(truth)
	row := func(label string, found map[langbench.CallPair]bool) string {
		a, as := goPR(truth, found), goPR(servable, found)
		pr := func(n, d int) string {
			if d == 0 {
				return "n/a"
			}
			return fmt.Sprintf("%.4f", float64(n)/float64(d))
		}
		return fmt.Sprintf("%s %d pairs: %d in the truth, precision %s, recall %s of %d; against the servable truth %d in it, precision %s, recall %s of %d",
			label, len(found), a, pr(a, len(found)), pr(a, len(truth)), len(truth), as, pr(as, len(found)), pr(as, len(servable)), len(servable))
	}
	st := sets.Stats
	return []string{
		fmt.Sprintf("%s: Go L2 option numbers (§1.6): A %s; A with callees that resolve to a node %s; B typed edges %s; C served (callgraph.BuildBoth's callers and callees, E and D in the graph since wave lg) %s; C before wave lg (BuildBoth at 90f0a01f, built here explicitly) %s; D served without the syntactic edges to a type (of C before wave lg) %s; E served without the syntax-only edges into a method from a file the typed pass resolved (of C before wave lg) %s; F D and E together %s", name,
			row("syntactic", sets.Syntactic), row("syntactic-served", sets.SyntacticSrv), row("typed", sets.Typed), row("served", sets.Served), row("served-pre-lg", sets.ServedPreLG), row("served-no-type-refs", sets.ServedNoRefs), row("served-typed-methods", sets.ServedReach), row("served-no-type-refs-typed-methods", sets.ServedBoth)),
		fmt.Sprintf("%s: typed edges read as pairs: %d direct, %d interface dispatch (%d edges, one per implementer, collapse to %d pairs by method name, where scip-go writes one interface-method term), %d function values, %d unresolved (a function value names no callee: no pair), %d instantiations (a fact, not a call); %d edges have a caller with no node in the measured files; %d of %d truth pairs name a callee the subject does not define (the standard library or a dependency) and are left out of the servable truth, counted here", name,
			st.Direct, st.Dispatch, st.Dispatch, st.DispatchPairs, st.FuncValue, st.Unresolved, st.Instantiates, st.NoCallerNode, external, len(truth)),
	}
}

// goGraded is Go's graded L2: the diff -unmatched writes and the notes.
type goGraded struct {
	diff  *callDiff
	notes []string
}

// scoreGoCalls re-scores Go's L2 over goL2Set (§1.6) in place of goextract's
// syntactic Calls: counts already hold the syntactic scoring of sr, which it
// replaces, and the notes carry every candidate's numbers. A set that is not
// the syntactic one is graded against the servable truth, the calls whose
// callee the service has a node for.
func scoreGoCalls(name string, nodes []core.SymbolNode, typed []core.Edge, sr langbench.SCIPResult, counts map[string]int) *goGraded {
	sets := goCallPairSets(nodes, typed, sr.Docs, sr.CallInIndex)
	notes := goCallNotes(name, sets, sr.CallTruth)
	found, truth := sets.pick(goL2Set), sr.CallTruth
	if goL2Set != goCallsSyntactic {
		var external int
		truth, external = sets.servableTruth(truth)
		notes = append(notes, fmt.Sprintf("%s: Go L2 grades the %s pairs (§1.6) against the %d truth pairs whose callee the subject defines; %d truth pairs on a callee outside the service are left out, and goextract's syntactic pairs (%d) are option A", name, goL2Set, len(truth), external, len(sets.Syntactic)))
	}
	notes = append(notes, goMissClasses(name, nodes, truth, found, sr.CallTruth)...)
	for _, k := range []string{langbench.CntCallTruth, langbench.CntCallTruthFound, langbench.CntCallFathom, langbench.CntCallFathomInTrue} {
		counts[k] -= sr.Counts[k]
	}
	langbench.ScorePairs(truth, found, counts, langbench.CntCallTruth, langbench.CntCallTruthFound, langbench.CntCallFathom, langbench.CntCallFathomInTrue)
	return &goGraded{diff: &callDiff{oracle: orDefault(sr.Tool, "scip"), truth: truth, mine: found}, notes: notes}
}

// goMissClasses counts, never hides, what the graded set gets wrong: its
// pairs the truth lacks (precision) and the truth's pairs it lacks (recall),
// each by the class of the callee. A callee is read by its bare name: a type
// when every node of that name is a type; fullTruth is the truth before the
// servable restriction.
func goMissClasses(name string, nodes []core.SymbolNode, truth, found, fullTruth map[langbench.CallPair]bool) []string {
	callable, typed := map[string]bool{}, map[string]bool{}
	for _, n := range nodes {
		b := langbench.BareName(n.Name)
		if n.Kind == core.KindGoType || n.Kind == core.KindGoInterface {
			typed[b] = true
		} else {
			callable[b] = true
		}
	}
	var typeRef, depCall, otherFound int
	for p := range found {
		if truth[p] {
			continue
		}
		switch {
		case typed[p.Callee] && !callable[p.Callee]:
			typeRef++
		case fullTruth[p]:
			depCall++
		default:
			otherFound++
		}
	}
	var noNode, hasNode int
	for p := range truth {
		if found[p] {
			continue
		}
		if callable[p.Callee] || typed[p.Callee] {
			hasNode++
		} else {
			noNode++
		}
	}
	return []string{
		fmt.Sprintf("%s: Go L2 graded pairs the truth lacks: %d name a type (a type reference in a declaration, a literal or an assertion, which since wave lg the served graph keeps as a reference, not a call), %d are a call to a dependency symbol bound to the one in-service node of that name (a syntax-only edge: assert.Equal to a service's Equal), %d other", name, typeRef, depCall, otherFound),
		fmt.Sprintf("%s: Go L2 truth pairs the graded set lacks: %d name no node of the service (a call through a func-typed field, variable or parameter, which the typed pass records as unresolved), %d name a node and no edge reaches it", name, noNode, hasNode),
	}
}

// fileEdgeKinds says, for each file edge the edges draw (the reduction
// fileEdgePairs makes), which kinds of edge drew it: the distinct kinds,
// sorted and joined by "+" (LB-F297). -unmatched writes it beside each
// fathom-only file edge so Go's false pairs classify by the typed edge that
// drew them.
func fileEdgeKinds(edges []core.Edge, nodes []core.SymbolNode) map[[2]string]string {
	fileOf := map[string]string{}
	for _, n := range nodes {
		fileOf[n.Key()] = n.FilePath
	}
	set := map[[2]string]map[string]bool{}
	for _, e := range edges {
		to, ok := fileOf[e.ToSymbol]
		if !ok || to == e.FromFile {
			continue
		}
		k := [2]string{e.FromFile, to}
		if set[k] == nil {
			set[k] = map[string]bool{}
		}
		set[k][string(e.Kind)] = true
	}
	out := make(map[[2]string]string, len(set))
	for k, ks := range set {
		var names []string
		for n := range ks {
			names = append(names, n)
		}
		sort.Strings(names)
		out[k] = strings.Join(names, "+")
	}
	return out
}

// goFileEdgeClasses discloses, per kind-set of the typed edges that draw a
// file edge, how many file edges there are and how many the truth has (LB-F297):
// so the classes a rule excludes, and what they cost, stay counted.
func goFileEdgeClasses(name string, edges []core.Edge, nodes []core.SymbolNode, truth map[[2]string]bool) string {
	kinds := fileEdgeKinds(edges, nodes)
	type tally struct{ pairs, agree int }
	by := map[string]*tally{}
	for k, ks := range kinds {
		t := by[ks]
		if t == nil {
			t = &tally{}
			by[ks] = t
		}
		t.pairs++
		if truth[k] {
			t.agree++
		}
	}
	names := make([]string, 0, len(by))
	for ks := range by {
		names = append(names, ks)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, ks := range names {
		parts = append(parts, fmt.Sprintf("%s %d (%d in the truth)", ks, by[ks].pairs, by[ks].agree))
	}
	return fmt.Sprintf("%s: Go file edges by the kinds of typed edge that draw them: %s", name, strings.Join(parts, "; "))
}

// goNamedEdges keeps the typed edges whose target the source names at a site:
// a direct call, a function value and, since K17 (wave lf2, LB-F343), a field
// reference (core.EdgeFieldRead/EdgeFieldWrite: `c.Timeout` names the field,
// declared in its struct's file, as scip-go's own reference does). The typed pass also stores facts no
// source line writes: an interface dispatch edge to EACH implementer (a call
// through an interface names the interface's method, in the interface's
// file) and an instantiation fact (a value the caller holds, written or
// inferred). A file edge is a reference the source writes to a definition in
// another file (design §1.2), so the file edges those kinds alone draw are
// left, returned as the pairs they draw (LB-F297); the edges stay in the
// store and in every served graph.
func goNamedEdges(edges []core.Edge, nodes []core.SymbolNode) (named []core.Edge, leftOut map[[2]string]bool) {
	for _, e := range edges {
		if e.Kind == core.EdgeIfaceDispatch || e.Kind == core.EdgeInstantiates {
			continue
		}
		named = append(named, e)
	}
	leftOut = map[[2]string]bool{}
	kept := fileEdgePairs(named, nodes)
	for k := range fileEdgePairs(edges, nodes) {
		if !kept[k] {
			leftOut[k] = true
		}
	}
	return named, leftOut
}

// goLeftOutNote discloses the file edges goNamedEdges leaves out: how many,
// and how many of them the truth has (what the rule costs recall).
func goLeftOutNote(name string, left, truth map[[2]string]bool, all int) string {
	inTruth := 0
	for k := range left {
		if truth[k] {
			inTruth++
		}
	}
	return fmt.Sprintf("%s: Go file edges drawn only by an interface-dispatch edge (one per implementer; the call names the interface method, in the interface's file) or an instantiation fact (a held value's type, written or inferred) are not references the source writes to a definition in another file (design §1.2, amendment F.34) and are left out of file_edge_precision: %d of the typed pass's %d file edges, %d of them in the truth (cost to recall, counted); the edges stay in the store and in every served graph", name, len(left), all, inTruth)
}

// goNativeCoverage discloses how much of the SCIP import's typed edges the
// native go/types pass also draws (LB-F300): Go's typed_recall is graded on
// the import (design §1.2, the SCIP edges present as typed edges), while the
// product serves the native pass, a call graph that stores no reference to a
// type or a variable but as an instantiation fact. Counted over the unique
// (from, to) pairs of the import's edges between mapped definitions.
func goNativeCoverage(name string, imported, native []core.Edge) string {
	nat := map[[2]string]map[string]bool{}
	for _, e := range native {
		k := [2]string{e.FromSymbol, e.ToSymbol}
		if nat[k] == nil {
			nat[k] = map[string]bool{}
		}
		nat[k][string(e.Kind)] = true
	}
	pairs := map[[2]string]bool{}
	for _, e := range imported {
		pairs[[2]string{e.FromSymbol, e.ToSymbol}] = true
	}
	var covered, nonSelf, nonSelfCovered int
	byKind := map[string]int{}
	for k := range pairs {
		ks, ok := nat[k]
		if ok {
			covered++
			var names []string
			for n := range ks {
				names = append(names, n)
			}
			sort.Strings(names)
			byKind[strings.Join(names, "+")]++
		}
		if k[0] != k[1] {
			nonSelf++
			if ok {
				nonSelfCovered++
			}
		}
	}
	kinds := make([]string, 0, len(byKind))
	for k := range byKind {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	parts := make([]string, 0, len(kinds))
	for _, k := range kinds {
		parts = append(parts, fmt.Sprintf("%s %d", k, byKind[k]))
	}
	ratio := func(n, d int) string {
		if d == 0 {
			return "n/a"
		}
		return fmt.Sprintf("%.4f", float64(n)/float64(d))[1:]
	}
	return fmt.Sprintf("%s: Go typed_recall is graded on the SCIP import (design §1.2); the native go/types typed pass covers its edges between mapped definitions so: of %d unique (from, to) pairs, %d are edges of the native typed pass (%s; by kind: %s), and of %d non-self pairs, %d of them native (%s); the native pass is a call graph and stores no reference to a type or a variable but an instantiation fact (a held type), so this is a coverage figure, not a graded one (LB-F300)",
		name, len(pairs), covered, ratio(covered, len(pairs)), strings.Join(parts, ", "), nonSelf, nonSelfCovered, ratio(nonSelfCovered, nonSelf))
}

// orNone is s, or "none" when s is empty.
func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

// goSCIPNotes discloses what Go's L3 truth and L4 counts leave out or miss,
// from the scip-go counts of one subject (LB-F298, LB-F299): the documents
// outside the subject, the file edges only a package reference draws, the
// definitions and references no fathom symbol answers by descriptor class,
// and the namespace rule's counts (K18, wave lf2: a Go package is a
// namespace, never a fathom symbol; its package-clause definitions and the
// references to it leave def_mapping and typed_recall, counted).
func goSCIPNotes(name string, c map[string]int) []string {
	var notes []string
	if n := c[langbench.CntScipGoDocsOutside]; n > 0 {
		notes = append(notes, fmt.Sprintf("%s: %d documents of the scip-go index lie outside the subject (`scip-go index ./...` indexes the testmain `go test` generates into the build cache for each test package: an init(), a main() and the calls into the package under test) and are left out of every count (design §1.4 amendment, F.34, LB-F299)", name, n))
	}
	if n := c[langbench.CntScipFileEdgesMultiDef]; n > 0 {
		notes = append(notes, fmt.Sprintf("%s: %d file edges only a reference to a package symbol would draw are left out of the file-edge truth: scip-go defines a package at the package clause of every file, so a reference to the package (an import spec, the `b` of `b.Foo`) names no one file and the index's first document of the package stands in for it, a guess (design §1.2 amendment, F.34, LB-F298; the same rule leaves a symbol defined in two documents out of an import truth)", name, n))
	}
	list := func(prefix string, c map[string]int) string {
		var ks []string
		for k := range c {
			if strings.HasPrefix(k, prefix) {
				ks = append(ks, k)
			}
		}
		sort.Strings(ks)
		parts := make([]string, 0, len(ks))
		for _, k := range ks {
			parts = append(parts, fmt.Sprintf("%s %d", strings.TrimPrefix(k, prefix), c[k]))
		}
		return strings.Join(parts, ", ")
	}
	if defs := c[langbench.CntScipDefs]; defs > 0 {
		notes = append(notes, fmt.Sprintf("%s: scip-go's %d named definitions, %d mapped to a fathom symbol; unmapped definitions by class: %s. Struct fields are Owner.name symbols and an interface's methods Iface.Method since K17 (wave lf2, LB-F342)", name, defs, c[langbench.CntScipDefsMapped], orNone(list("scip_go_defs_unmapped_", c))))
	}
	if n := c[langbench.CntScipDefsNamespace]; n > 0 {
		notes = append(notes, fmt.Sprintf("%s: %d package-clause definitions name a Go package, a namespace (K18): out of def_mapping, counted (scip_defs_namespace); a package is file granularity, graded at L3 by file edges, and never a fathom symbol (LB-F344)", name, n))
	}
	if edges := c[langbench.CntScipEdges]; edges > 0 {
		notes = append(notes, fmt.Sprintf("%s: of scip-go's %d edges between definitions, %d import as typed edges; unmapped endpoints by class: %s; scip-go writes most parameters and locals as `local N` (the local rule, LB-F55: out of typed_recall) and a function literal's parameters in a package initializer as a term of a meta descriptor (`Options:c.`), a parameter (%d references, scip_edges_param_target, LB-F299)", name, edges, c[langbench.CntScipEdgesTyped], orNone(list("scip_go_unmapped_", c)), c[langbench.CntScipEdgesParamTarget]))
	}
	if n := c[langbench.CntScipEdgesNamespaceTarget]; n > 0 {
		notes = append(notes, fmt.Sprintf("%s: %d references to a Go package are a namespace's (K18): out of typed_recall, counted (scip_edges_namespace_target); a reference whose enclosing definition is itself unmapped stays an unmapped endpoint of its from class (LB-F344)", name, n))
	}
	return notes
}
