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
	// goCallsServed (C) is callgraph.BuildBoth: the syntactic Calls that
	// resolve to a node of the service plus the typed edges, which is what
	// callers and blast_radius read.
	goCallsServed goCallSet = "served"
	// goCallsServedNoRefs (D) is C without the syntactic edges to a type:
	// LB-Q-go-rest's trigger, which would move type references out of Calls.
	goCallsServedNoRefs goCallSet = "served-no-type-refs"
	// goCallsServedReach (E) is C without the syntax-only edges into a method
	// from a file the typed pass resolved: the rule ProductReach.walk applies.
	goCallsServedReach goCallSet = "served-typed-methods"
	// goCallsServedBoth (F) is D and E together.
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
	Served       map[langbench.CallPair]bool // C
	ServedNoRefs map[langbench.CallPair]bool // D: C less the syntactic-only edges to a type (LB-Q-go-rest's trigger, evaluated not shipped)
	ServedReach  map[langbench.CallPair]bool // E: C less the syntactic-only edges into a method from a file the typed pass resolved (what ProductReach.walk already discounts)
	ServedBoth   map[langbench.CallPair]bool // F: D and E together
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
	readIf := func(g *callgraph.Graph, keep func(callgraph.EdgeInfo) bool) map[langbench.CallPair]bool {
		out := map[langbench.CallPair]bool{}
		for _, n := range nodes {
			if files != nil && !files[n.FilePath] {
				continue
			}
			for _, c := range callgraph.CalleesDetail(g, n.Key()) {
				if !keep(c) {
					continue
				}
				if callee := bareOf(c.Key); callee != "" {
					out[langbench.CallPair{Path: n.FilePath, Caller: langbench.BareName(n.Name), Callee: callee}] = true
				}
			}
		}
		return out
	}
	read := func(g *callgraph.Graph) map[langbench.CallPair]bool {
		return readIf(g, func(callgraph.EdgeInfo) bool { return true })
	}
	sets.Syntactic = langbench.FathomCallPairs(nodes, files)
	sets.SyntacticSrv = read(callgraph.Build(nodes))
	sets.Typed = read(callgraph.BuildBoth(noCalls, typed))
	gc := callgraph.BuildBoth(nodes, typed)
	sets.Served = read(gc)
	typedFile := map[string]bool{}
	for _, e := range typed {
		if e.FromFile != "" {
			typedFile[e.FromFile] = true
		}
	}
	isType := func(k string) bool { return kindOf[k] == core.KindGoType || kindOf[k] == core.KindGoInterface }
	// readIfFrom is readIf with the caller's file in view.
	readIfFrom := func(keep func(from core.SymbolNode, c callgraph.EdgeInfo) bool) map[langbench.CallPair]bool {
		out := map[langbench.CallPair]bool{}
		for _, n := range nodes {
			if files != nil && !files[n.FilePath] {
				continue
			}
			for _, c := range callgraph.CalleesDetail(gc, n.Key()) {
				if callee := bareOf(c.Key); callee != "" && keep(n, c) {
					out[langbench.CallPair{Path: n.FilePath, Caller: langbench.BareName(n.Name), Callee: callee}] = true
				}
			}
		}
		return out
	}
	noRefs := func(_ core.SymbolNode, c callgraph.EdgeInfo) bool { return c.Kind != "" || !isType(c.Key) }
	noGuess := func(from core.SymbolNode, c callgraph.EdgeInfo) bool {
		return c.Kind != "" || kindOf[c.Key] != core.KindGoMethod || !typedFile[from.FilePath]
	}
	sets.ServedNoRefs = readIfFrom(noRefs)
	sets.ServedReach = readIfFrom(noGuess)
	sets.ServedBoth = readIfFrom(func(f core.SymbolNode, c callgraph.EdgeInfo) bool { return noRefs(f, c) && noGuess(f, c) })
	return sets
}

// pick returns the candidate named.
func (s goCallSets) pick(which goCallSet) map[langbench.CallPair]bool {
	switch which {
	case goCallsTyped:
		return s.Typed
	case goCallsServed:
		return s.Served
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
		fmt.Sprintf("%s: Go L2 option numbers (§1.6): A %s; A with callees that resolve to a node %s; B typed edges %s; C served (callgraph.BuildBoth) %s; D served without the syntactic edges to a type %s; E served without the syntax-only edges into a method from a file the typed pass resolved %s; F D and E together %s", name,
			row("syntactic", sets.Syntactic), row("syntactic-served", sets.SyntacticSrv), row("typed", sets.Typed), row("served", sets.Served), row("served-no-type-refs", sets.ServedNoRefs), row("served-typed-methods", sets.ServedReach), row("served-no-type-refs-typed-methods", sets.ServedBoth)),
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
		fmt.Sprintf("%s: Go L2 graded pairs the truth lacks: %d name a type (a type reference in a declaration, a literal or an assertion, kept in Calls for test_reaches), %d are a call to a dependency symbol bound to the one in-service node of that name (a syntax-only edge: assert.Equal to a service's Equal), %d other", name, typeRef, depCall, otherFound),
		fmt.Sprintf("%s: Go L2 truth pairs the graded set lacks: %d name no node of the service (a call through a func-typed field, variable or parameter, which the typed pass records as unresolved), %d name a node and no edge reaches it", name, noNode, hasNode),
	}
}
