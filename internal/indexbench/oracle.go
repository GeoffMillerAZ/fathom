// Package indexbench is the bench pyramid's two free layers (ADR-032,
// docs/design/bench-pyramid.md §3.1–3.2): L0, the index oracle, which scores
// fathom's own answers against an independent go/types truth, and L1,
// payload replay, which pins what each tool's answer COSTS.
//
// Both run in `task validate` on every commit, spend no tokens, and fail as
// tests rather than as benchmark numbers a reader might not notice. The
// division of labour is deliberate: L0 asks "is the index RIGHT", L1 asks
// "is the answer SMALL AND STABLE", and neither re-asks a question the other
// already answered.
//
// The truth comes from internal/contextbench/publicloc's go/types walker —
// the same walker cmd/loctruth prints for the public corpus. It never opens
// a fathom store and never asks a daemon: a benchmark whose truth comes from
// the instrument under test measures nothing.
package indexbench

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/analysis/references"
	"github.com/geoffmilleraz/fathom/internal/contextbench/publicloc"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// Tool names scored by the oracle. They are the MCP tool names on purpose:
// a red number in this report points at the tool an agent actually calls.
const (
	ToolFindCallers = "find_callers"
	ToolReferences  = "references"
	ToolBlastRadius = "blast_radius"
)

// Backend is the slice of mcp.QueryInterface the oracle needs. *pkg/client.
// Client satisfies it, so the oracle scores the same code path the MCP tool
// handlers call — never the HTTP surface, which would make the daemon's
// availability part of the measurement.
type Backend interface {
	FindCallers(ctx context.Context, serviceID, symbol string) ([]core.SymbolNode, error)
	ListAllSymbols(ctx context.Context, serviceID string) ([]core.SymbolNode, error)
	TypedEdges(ctx context.Context, serviceID string) ([]core.Edge, error)
	Sources(ctx context.Context, serviceID string) ([]core.SourceFile, error)
}

// Subject is one scored unit: a service as the index knows it, and the Go
// module tree the go/types oracle reads for the same code.
type Subject struct {
	// Service is the service id in the index.
	Service string
	// Root is the module root on disk — it must contain a go.mod, because
	// go/types loads a module, not a directory.
	Root string
	// Prefix is prepended to a truth path to get the path the index uses
	// (e.g. "payment-service/" for the fauxtown fixture fleet, whose
	// connector reports paths relative to the fleet root, not the service
	// root). Empty when the two agree.
	Prefix string
	// MaxQueries bounds how many questions each tool is asked. Zero means
	// every one — the fixture fleet's setting, and the only setting a
	// committed golden may use.
	//
	// A whole module is quadratic to score exhaustively (every declared name
	// against every source file), which on this repository is minutes, not
	// seconds. A positive MaxQueries takes a deterministic STRIDE sample of
	// the sorted candidate list — same subjects, same questions, every run —
	// and Population records what it was sampled from, so a rate is never
	// read as covering more than it did.
	MaxQueries int
	// BlastDepth is the reverse-walk bound blast_radius is scored at, and
	// the bound the truth closure is computed to. Zero means the oracle's
	// default of 1 (direct callers), the bound BP-27 gates at >=0.95;
	// negative means unbounded, the pre-BP-27 walk.
	BlastDepth int
	// RecordMisses collects every truth caller the resolved edges did NOT
	// predict, with the file and line of each side, so a recall number can
	// be turned into a list of defects instead of an argument. Off for the
	// committed goldens (the list is long and churns with the fixture); on
	// for the self run, whose whole job is to find the next fix.
	RecordMisses bool
	// MaxBlastQueries bounds blast_radius separately, because it is two
	// orders of magnitude more expensive per question than the other two:
	// impact.BlastRadiusWithTypedEdges rebuilds the whole call graph on
	// every call, which on a ten-thousand-symbol service is ~10s a query.
	// That cost is a FINDING, not a measurement artifact — it is what the
	// blast_radius tool charges a real agent — so the oracle pays it
	// honestly on a smaller sample rather than hiding it by caching a graph
	// the tool does not cache. Zero falls back to MaxQueries.
	MaxBlastQueries int
	// ReferencesExact (SB-45, docs/plans/strengths-bench.md) scores the
	// references handler's exact-mode answer instead of its pre-SB-45 one:
	// a text occurrence is only counted when the symbol enclosing it has a
	// typed call edge to the query's definition (callgraph.MentionConfidence
	// — the same confirmation the handler renders as edge_confidence).
	// False (the zero value) keeps every committed golden's own number
	// unchanged; the self run and `compare` set it true to measure the
	// improvement SB-45 was built for.
	ReferencesExact bool
}

// ToolScore is one tool's precision and recall over one subject, with the
// denominators attached. A rate with no denominator is a claim, not a
// measurement (ADR-031): 1.0000 over two questions and 1.0000 over nine
// hundred are different facts, and the golden records both.
type ToolScore struct {
	Tool string `json:"tool"`
	// Queries is how many oracle questions were asked (symbols, or files
	// for blast_radius).
	Queries int `json:"queries"`
	// Population is how many questions COULD have been asked. It differs
	// from Queries only under Subject.MaxQueries, and its presence is what
	// stops a sampled rate from being read as an exhaustive one.
	Population int `json:"population"`
	// Predicted, Truth and Correct are micro-averaged: summed over every
	// query, then divided once. A macro average would let a symbol with one
	// caller outvote a symbol with forty.
	Predicted int     `json:"predicted"`
	Truth     int     `json:"truth"`
	Correct   int     `json:"correct"`
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	// Unadjudicated counts predicted answers the truth is not entitled to
	// rule on (BP-30) — they are NOT in Predicted and so never move a rate.
	// A tool that returns a doc hit the go/types truth cannot contain has
	// not made an error; reporting it as one would be the error.
	Unadjudicated int `json:"unadjudicated,omitempty"`
	// UnadjudicatedQueries counts questions dropped whole because no
	// honest comparison exists for them (BP-30: an ambiguously-named
	// function). They are not in Queries either.
	UnadjudicatedQueries int `json:"unadjudicated_queries,omitempty"`
	// EdgeFilter names which evidence the scored answer used: "resolved"
	// (typed pass only, the shipped default since BP-26/BP-27) or "all".
	EdgeFilter string `json:"edge_filter,omitempty"`
	// Depth is the reverse-walk bound the scored answer used, and the bound
	// the truth closure was computed to. 0 = unbounded. Prediction and
	// truth always share it: comparing a depth-1 answer against an
	// unbounded closure would report a recall failure that is a bound, not
	// a miss.
	Depth int `json:"depth,omitempty"`
	// ElapsedMS is wall-clock time for this tool's whole scored pass. It is
	// the one field in the report that is NOT deterministic, so the
	// committed goldens zero it (see zeroTimings): a golden that carried a
	// timing would fail on a loaded laptop, and a gate that fails for
	// reasons unrelated to the index teaches people to rerun it until it
	// passes. It is here for the on-demand self run, whose whole question
	// is "what does this cost".
	ElapsedMS int64 `json:"elapsed_ms,omitempty"`
}

// ZeroTimings clears every non-deterministic field in rep so it can be
// compared against, or written as, a committed golden.
func ZeroTimings(rep *Report) {
	for i := range rep.Services {
		for j := range rep.Services[i].Tools {
			rep.Services[i].Tools[j].ElapsedMS = 0
		}
	}
}

// Miss is one truth answer the index did not predict — the raw material of
// the next fix. A recall rate says how many; this says which.
type Miss struct {
	Tool string `json:"tool"`
	// Query is the question asked (a callee's qualified name).
	Query string `json:"query"`
	// Want is the answer go/types holds that the index did not give.
	Want string `json:"want"`
	// WantFile/WantLine locate the missing caller's declaration, when the
	// index knows the symbol at all.
	WantFile string `json:"want_file,omitempty"`
	WantLine int    `json:"want_line,omitempty"`
	// Indexed is false when the index has no symbol by that name at all —
	// a different failure from "indexed but no edge", and the one that
	// points at ingest rather than at the typed pass.
	Indexed bool `json:"indexed"`
	// HeuristicEdge is true when the SYNTAX pass did find this caller. A
	// true here is a typed-pass defect by definition: the edge is visible
	// to the weaker pass and not the stronger one.
	HeuristicEdge bool `json:"heuristic_edge"`
}

// ServiceScore is every tool's score for one subject.
type ServiceScore struct {
	Service            string      `json:"service"`
	Module             string      `json:"module"`
	GoPackages         int         `json:"go_packages"`
	PackagesWithErrors int         `json:"packages_with_errors"`
	IndexedSymbols     int         `json:"indexed_symbols"`
	Tools              []ToolScore `json:"tools"`
	// Misses is populated only under Subject.RecordMisses.
	Misses []Miss `json:"misses,omitempty"`
}

// Report is the whole L0 run — the committed golden's shape.
type Report struct {
	Oracle   string         `json:"oracle"`
	Corpus   string         `json:"corpus"`
	Services []ServiceScore `json:"services"`
}

// IndexOracleReport scores fathom's index for every subject against the
// go/types truth for the same trees (bench-pyramid BP-12).
//
// What each tool is scored on, stated once so a number means one thing:
//
//   - find_callers: for every function declared in the module, the set of
//     function names that directly call it. Truth from go/types; prediction
//     from Backend.FindCallers, the function the find_callers handler calls.
//     Interface dispatch is not a static call and is not in the truth.
//   - references: for every RECEIVERLESS function (a method's mention in
//     source text is the bare method name, so scoring "Type.Method" as a
//     text query would measure the query spelling, not the index), the set
//     of files that mention it. Truth is every identifier go/types resolves
//     to the function, plus its declaring file; prediction is the
//     references handler's own two halves — definitions from
//     ListAllSymbols, occurrences from references.Find.
//   - blast_radius: for every file declaring at least one function, the
//     transitive closure of callers over the go/types call graph, seeded
//     with the file's own functions. Prediction is
//     impact.BlastRadiusWithTypedEdges — the function the blast_radius
//     handler calls — filtered to function and method symbols, because the
//     truth is a call graph and types/consts/vars are not in it.
//
// Every subject is scored even when a denominator is zero (fauxtown's
// worktree-service declares no functions at all): a rate over nothing is
// reported as 1.0 with Queries and Truth at 0, so the reader can see that it
// measured nothing rather than mistaking it for a pass.
func IndexOracleReport(ctx context.Context, be Backend, subjects []Subject) (Report, error) {
	rep := Report{Oracle: "internal/indexbench", Services: []ServiceScore{}}
	for _, sub := range subjects {
		truth, err := publicloc.LoadGoTruth(sub.Root)
		if err != nil {
			return Report{}, fmt.Errorf("indexbench: truth for %s: %w", sub.Service, err)
		}
		nodes, err := be.ListAllSymbols(ctx, sub.Service)
		if err != nil {
			return Report{}, fmt.Errorf("indexbench: symbols for %s: %w", sub.Service, err)
		}
		sc := ServiceScore{
			Service:            sub.Service,
			Module:             truth.Module,
			GoPackages:         truth.Packages,
			PackagesWithErrors: truth.PackagesWithErrors,
			IndexedSymbols:     len(nodes),
		}
		typed, err := be.TypedEdges(ctx, sub.Service)
		if err != nil {
			return Report{}, fmt.Errorf("indexbench: typed edges %s: %w", sub.Service, err)
		}
		// BP-28 made the graph memoizable per (service, watermark) below
		// the MCP layer; the oracle holds the same graph for the whole of
		// one subject for the same reason the daemon now does — nothing
		// about building it is per-query. Before BP-28 this was rebuilt on
		// every blast_radius call, at ~10.9s each.
		resolvedGraph := callgraph.BuildResolved(nodes, typed)
		// The go/types truth records STATIC CALL SITES. An
		// interface-dispatch edge (one per in-scope implementer) and a
		// func-value edge (a handler REGISTERED, not called here) are real,
		// useful, honestly-labeled evidence that the truth cannot contain
		// by construction — publicloc's walker resolves a selector to one
		// *types.Func or records nothing. Scoring them against it would
		// report as false positives the two things the typed pass was
		// built to add (ADR-021 §3).
		//
		// So the oracle scores the DIRECT subgraph and counts what the
		// other kinds add as Unadjudicated — the same rule BP-30 applies to
		// references, applied to the call graph.
		directGraph := callgraph.BuildResolved(nodes, directCallEdges(typed))
		nameOf := funcNames(nodes)

		t0 := time.Now()
		// The syntax graph exists only to CLASSIFY a miss (did the weaker
		// pass see the edge the typed pass lost?). Building it over ten
		// thousand symbols costs ~9.5s, so it is built on the first miss
		// and never when there are none — a diagnostic must not dominate
		// the measurement it annotates.
		var syntaxGraph *callgraph.Graph
		syntaxFn := func() *callgraph.Graph {
			if syntaxGraph == nil {
				syntaxGraph = callgraph.Build(nodes)
			}
			return syntaxGraph
		}
		callers, misses, err := scoreFindCallers(ctx, be, sub, truth, directGraph, resolvedGraph, syntaxFn, nameOf, nodes)
		if err != nil {
			return Report{}, err
		}
		sc.Misses = misses
		callers.ElapsedMS = time.Since(t0).Milliseconds()
		t0 = time.Now()
		refs, err := scoreReferences(ctx, be, sub, truth, nodes, typed)
		if err != nil {
			return Report{}, err
		}
		refs.ElapsedMS = time.Since(t0).Milliseconds()
		t0 = time.Now()
		blast, err := scoreBlastRadius(sub, truth, nodes, directGraph, resolvedGraph)
		if err != nil {
			return Report{}, err
		}
		blast.ElapsedMS = time.Since(t0).Milliseconds()
		sc.Tools = []ToolScore{callers, refs, blast}
		rep.Services = append(rep.Services, sc)
	}
	sort.Slice(rep.Services, func(i, j int) bool { return rep.Services[i].Service < rep.Services[j].Service })
	return rep, nil
}

// IndexOracleGoldens is VB-6's `task validate` entry point (docs/plans/
// verbs.md, ADR-035 decision 5): it stages the fauxtown fixture fleet's Go
// services under root exactly the way FauxtownSubjects/TestIndexOracleMatchesGolden
// already do, builds a fresh fixture index, and scores it with
// IndexOracleReport. root is the caller's scratch directory for the staged
// modules (a test passes t.TempDir()) — go/types loads a MODULE, and the
// fixture services on disk are deliberately not modules (see
// StageFauxtownModule).
//
// The returned Report is the same rich per-service, per-tool shape
// IndexOracleReport and the self-run (`task bench-l0-self`) use.
// TestIndexOracleGoldens derives the compact, micro-averaged per-tool
// summary that internal/indexbench/testdata/l0-goldens.json commits — the
// literal {tool: {precision, recall, n}} golden VB-6 gates on — via
// aggregateL0Goldens, so the two committed goldens (this package's richer
// per-service fauxtown.golden.json, and l0-goldens.json's compact
// regression floor) are two views of one scored run, never two divergent
// computations.
func IndexOracleGoldens(root string) (Report, error) {
	ctx := context.Background()
	subjects, err := FauxtownSubjects(root)
	if err != nil {
		return Report{}, err
	}
	c, _, err := NewFauxtownIndex(ctx)
	if err != nil {
		return Report{}, err
	}
	defer func() { _ = c.Close() }()
	rep, err := IndexOracleReport(ctx, c, subjects)
	if err != nil {
		return Report{}, err
	}
	rep.Corpus = "fauxtown"
	return rep, nil
}

// L0Golden is one tool's committed regression floor at
// internal/indexbench/testdata/l0-goldens.json: precision and recall
// MICRO-averaged across every fixture service (summed correct/predicted/
// truth, then divided once — the same accumulation ToolScore.finish uses
// per service, extended across services), with N the total queries asked.
// A rate with N at 0 asked nothing and is not a claim (ADR-031's rule,
// same as ToolScore.Queries/Population).
type L0Golden struct {
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	N         int     `json:"n"`
}

// aggregateL0Goldens collapses a Report's per-service ToolScores into the
// compact per-tool L0Golden map l0-goldens.json commits.
func aggregateL0Goldens(rep Report) map[string]L0Golden {
	type accum struct{ correct, predicted, truth, n int }
	acc := map[string]*accum{}
	for _, svc := range rep.Services {
		for _, t := range svc.Tools {
			a, ok := acc[t.Tool]
			if !ok {
				a = &accum{}
				acc[t.Tool] = a
			}
			a.correct += t.Correct
			a.predicted += t.Predicted
			a.truth += t.Truth
			a.n += t.Queries
		}
	}
	out := make(map[string]L0Golden, len(acc))
	for tool, a := range acc {
		out[tool] = L0Golden{
			Precision: round4(rate(a.correct, a.predicted)),
			Recall:    round4(rate(a.correct, a.truth)),
			N:         a.n,
		}
	}
	return out
}

// declaredNames is every qualified function name the module declares, sorted.
func declaredNames(truth publicloc.GoTruth) []string {
	set := map[string]bool{}
	for _, names := range truth.DeclsByFile {
		for _, n := range names {
			set[n] = true
		}
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// sample takes a deterministic stride sample of at most max entries from the
// sorted slice in, or all of in when max is zero or larger. Striding rather
// than truncating keeps the sample spread across the whole alphabet: the
// first two hundred names of a Go module are not a sample of it, they are
// its A section.
func sample(in []string, max int) []string {
	if max <= 0 || len(in) <= max {
		return in
	}
	out := make([]string, 0, max)
	for i := 0; i < max; i++ {
		out = append(out, in[i*len(in)/max])
	}
	return out
}

// scoreFindCallers scores the tool's SHIPPED DEFAULT (BP-26): resolved
// callers — the typed pass's edges — which is what the find_callers handler
// returns unless the caller passes include_heuristic. When a service has no
// typed edges at all the handler falls back to the syntax engine, and so
// does this: scoring a fallback path as if it were the default would report
// a number for code no agent runs.
func scoreFindCallers(ctx context.Context, be Backend, sub Subject, truth publicloc.GoTruth, direct, resolved *callgraph.Graph, syntaxFn func() *callgraph.Graph, nameOf map[string]string, nodes []core.SymbolNode) (ToolScore, []Miss, error) {
	var misses []Miss
	byName := map[string][]core.SymbolNode{}
	if sub.RecordMisses {
		for _, n := range nodes {
			byName[n.Name] = append(byName[n.Name], n)
		}
	}
	all := declaredNames(truth)
	sc := ToolScore{Tool: ToolFindCallers, Population: len(all), EdgeFilter: "resolved"}
	if !resolved.HasEdges() {
		sc.EdgeFilter = "all"
	}
	for _, name := range sample(all, sub.MaxQueries) {
		sc.Queries++
		want := setOf(truth.CallersByName[name])
		got := map[string]bool{}
		if sc.EdgeFilter == "resolved" {
			for _, e := range callgraph.ResolvedCallersAcross(direct, name) {
				if n, ok := nameOf[e.Key]; ok {
					got[n] = true
					continue
				}
				sc.Unadjudicated++ // a test-file caller: outside the truth
			}
			// Rows the shipped default returns that this truth cannot rule
			// on: reached through dispatch or a func value, never a static
			// call site.
			for _, e := range callgraph.ResolvedCallersAcross(resolved, name) {
				if n, ok := nameOf[e.Key]; !ok || !got[n] {
					sc.Unadjudicated++
				}
			}
		} else {
			found, err := be.FindCallers(ctx, sub.Service, name)
			if err != nil {
				return sc, nil, fmt.Errorf("indexbench: find_callers %s/%s: %w", sub.Service, name, err)
			}
			for _, n := range found {
				got[n.Name] = true
			}
		}
		if sub.RecordMisses {
			// Built lazily, and only for a query that actually missed:
			// resolving a bare ambiguous name against the syntax graph
			// costs an O(nodes) scan, and paying it on every query turned a
			// 3ms pass into a 9.5s one for instrumentation nobody reads
			// when recall is 1.0.
			var syntaxSaw map[string]bool
			for w := range want {
				if got[w] {
					continue
				}
				if syntaxSaw == nil {
					syntaxSaw = map[string]bool{}
					for _, e := range callgraph.CallersDetail(syntaxFn(), name) {
						if n, ok := nameOf[e.Key]; ok {
							syntaxSaw[n] = true
						}
					}
				}
				m := Miss{Tool: ToolFindCallers, Query: name, Want: w, HeuristicEdge: syntaxSaw[w]}
				if decls := byName[w]; len(decls) > 0 {
					m.Indexed = true
					m.WantFile = decls[0].FilePath
					m.WantLine = decls[0].Line
				}
				misses = append(misses, m)
			}
		}
		accumulate(&sc, got, want)
	}
	finish(&sc)
	sort.Slice(misses, func(i, j int) bool {
		if misses[i].Query != misses[j].Query {
			return misses[i].Query < misses[j].Query
		}
		return misses[i].Want < misses[j].Want
	})
	return sc, misses, nil
}

// funcNames maps canonical SymbolKey -> the bare qualified name the
// go/types truth speaks (publicloc.QualifiedFuncName: "Name", or
// "Recv.Method"), restricted to functions and methods. The truth is a call
// graph; a type, const or var in the prediction is not a wrong caller, it
// is not a caller at all, so it never enters either set.
// It EXCLUDES symbols declared in _test.go files. publicloc's walker loads
// the module with packages.Config{Tests: false}, so a test function is not
// in the truth at all — neither as a declaration nor as a caller. Charging
// the index a false positive for correctly reporting that TestFoo calls Foo
// would be scoring the oracle's own blind spot against the instrument, so
// test-file callers are counted Unadjudicated instead (BP-30's rule).
func funcNames(nodes []core.SymbolNode) map[string]string {
	out := make(map[string]string, len(nodes))
	for _, n := range nodes {
		if n.Kind != core.KindGoFunc && n.Kind != core.KindGoMethod {
			continue
		}
		if strings.HasSuffix(n.FilePath, "_test.go") {
			continue
		}
		out[n.Key()] = n.Name
	}
	return out
}

// scoreReferences scores ONLY what the go/types truth is entitled to
// adjudicate (BP-30, ReferencesOracleSubset): an unambiguously-named
// receiverless function, and hits in Go files the type checker actually
// walked. Everything else the tool returns — doc and config mentions, hits
// on a same-named function in another package, files in packages that
// failed to type-check — is counted as Unadjudicated and never enters a
// rate, because no honest verdict about it exists. The previous scoring
// folded all of it into Predicted and reported precision 0.158, which
// measured the mismatch between two questions, not the index.
func scoreReferences(ctx context.Context, be Backend, sub Subject, truth publicloc.GoTruth, nodes []core.SymbolNode, typed []core.Edge) (ToolScore, error) {
	subset := NewReferencesOracleSubset(truth, sub.Prefix)
	sc := ToolScore{
		Tool:                 ToolReferences,
		Population:           len(subset.Names),
		UnadjudicatedQueries: len(subset.AmbiguousNames),
	}
	sources, err := be.Sources(ctx, sub.Service)
	if err != nil {
		return sc, fmt.Errorf("indexbench: sources %s: %w", sub.Service, err)
	}
	// SB-45: built ONCE for the whole subject, not per query — the graph
	// construction MentionIndex does is the same cost callgraph.Build pays
	// anywhere else in this file, and this loop asks up to MaxQueries
	// questions over the identical (nodes, typed) pair.
	var mentionIdx *callgraph.MentionIndex
	if sub.ReferencesExact {
		mentionIdx = callgraph.BuildMentionIndex(nodes, typed)
	}
	for _, name := range sample(subset.Names, sub.MaxQueries) {
		sc.Queries++
		want := map[string]bool{}
		for _, f := range truth.UsesByName[name] {
			want[sub.Prefix+f] = true
		}
		got := map[string]bool{}
		unadj := map[string]bool{}
		record := func(path string) {
			if subset.Adjudicable(path) {
				got[path] = true
				return
			}
			unadj[path] = true
		}
		// The references handler's definition half at its shipped default
		// (BP-26): EXACT name matches. The substring-containment matching
		// it used to do is now behind include_heuristic, and is counted
		// here as unadjudicated rather than scored — a hit on "FooBar" for
		// query "Foo" is not a claim that the file references Foo. defKeys
		// doubles as SB-45's exact-mode confirmation set: the query's
		// definitions in this service.
		var defKeys []string
		for _, n := range nodes {
			switch {
			case n.Name == name:
				record(n.FilePath)
				defKeys = append(defKeys, n.Key())
			case strings.Contains(n.Name, name):
				unadj[n.FilePath] = true
			}
		}
		// The occurrence half, at the handler's defaults.
		occs, err := references.Find(name, sources, references.Options{})
		if err != nil {
			return sc, fmt.Errorf("indexbench: references %s/%s: %w", sub.Service, name, err)
		}
		// SB-45: under ReferencesExact, score the same typed-confirmation
		// the handler renders — an occurrence only counts when the symbol
		// enclosing it has a typed call edge to one of defKeys. A query
		// with no typed reference sites (no definitions here, or no typed
		// edges at all) falls back to every occurrence, exactly like the
		// handler's own degrade-and-flag path.
		var mentionConf func(file string, line int) core.EdgeConfidence
		if mentionIdx != nil && len(defKeys) > 0 && len(typed) > 0 {
			mentionConf = mentionIdx.Confidence(defKeys)
		}
		for _, o := range occs {
			if mentionConf != nil && mentionConf(o.File, o.Line) != core.EdgeConfidenceResolved {
				continue
			}
			record(o.File)
		}
		for p := range got {
			delete(unadj, p)
		}
		sc.Unadjudicated += len(unadj)
		accumulate(&sc, got, want)
	}
	finish(&sc)
	return sc, nil
}

// scoreBlastRadius scores the tool's SHIPPED DEFAULTS (BP-27): resolved
// edges, bounded depth. Prediction and truth share the bound — a depth-1
// answer compared against an unbounded closure would report as a recall
// miss what is actually a documented stopping rule.
func scoreBlastRadius(sub Subject, truth publicloc.GoTruth, nodes []core.SymbolNode, direct, resolved *callgraph.Graph) (ToolScore, error) {
	depth := sub.BlastDepth
	if depth == 0 {
		depth = 1
	}
	if depth < 0 {
		depth = 0 // unbounded
	}
	sc := ToolScore{Tool: ToolBlastRadius, EdgeFilter: "resolved", Depth: depth}
	g := direct
	if !resolved.HasEdges() {
		sc.EdgeFilter = "all"
		g = resolved
	}
	funcName := funcNames(nodes)
	files := make([]string, 0, len(truth.DeclsByFile))
	for f := range truth.DeclsByFile {
		if len(truth.DeclsByFile[f]) > 0 {
			files = append(files, f)
		}
	}
	sort.Strings(files)
	sc.Population = len(files)

	maxBlast := sub.MaxBlastQueries
	if maxBlast == 0 {
		maxBlast = sub.MaxQueries
	}
	for _, f := range sample(files, maxBlast) {
		seeds := truth.DeclsByFile[f]
		sc.Queries++
		want := closure(truth.CallersByName, seeds, depth)
		res := impact.BlastRadiusBounded(g, nodes, []string{sub.Prefix + f}, impact.Options{MaxDepth: depth})
		got := map[string]bool{}
		for _, key := range res.Symbols {
			if n, ok := funcName[key]; ok {
				got[n] = true
				continue
			}
			sc.Unadjudicated++ // a test symbol, or a non-function: outside the truth
		}
		// What dispatch and func-value edges add to the same walk: real
		// reach, outside a static-call truth's competence (see above).
		full := impact.BlastRadiusBounded(resolved, nodes, []string{sub.Prefix + f}, impact.Options{MaxDepth: depth})
		for _, key := range full.Symbols {
			if n, ok := funcName[key]; !ok || !got[n] {
				sc.Unadjudicated++
			}
		}
		accumulate(&sc, got, want)
	}
	finish(&sc)
	return sc, nil
}

// closure walks the reverse call graph from seeds, seeds included — the set
// blast_radius is defined to return — stopping maxDepth hops out. maxDepth
// 0 is unbounded. The bound must match the one the prediction was computed
// under; see scoreBlastRadius.
func closure(callersByName map[string][]string, seeds []string, maxDepth int) map[string]bool {
	out := map[string]bool{}
	type item struct {
		name  string
		depth int
	}
	queue := make([]item, 0, len(seeds))
	for _, s := range seeds {
		queue = append(queue, item{s, 0})
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if out[cur.name] {
			continue
		}
		out[cur.name] = true
		if maxDepth > 0 && cur.depth >= maxDepth {
			continue
		}
		for _, c := range callersByName[cur.name] {
			queue = append(queue, item{c, cur.depth + 1})
		}
	}
	return out
}

// accumulate folds one query's predicted/truth sets into the running score.
func accumulate(sc *ToolScore, got, want map[string]bool) {
	sc.Predicted += len(got)
	sc.Truth += len(want)
	for g := range got {
		if want[g] {
			sc.Correct++
		}
	}
}

// finish turns the accumulated counts into rates. A zero denominator scores
// 1.0 — nothing was asked, so nothing was got wrong — and Queries/Truth in
// the same row say so out loud.
func finish(sc *ToolScore) {
	sc.Precision = rate(sc.Correct, sc.Predicted)
	sc.Recall = rate(sc.Correct, sc.Truth)
}

func rate(num, den int) float64 {
	if den == 0 {
		return 1
	}
	return float64(num) / float64(den)
}

// round4 is the comparison precision every committed golden in this package
// rounds to before it is compared or written: four decimals is fine enough
// to catch a single lost edge in a corpus of a few hundred and coarse
// enough that float noise never fails a gate.
func round4(f float64) float64 { return math.Round(f*10000) / 10000 }

// addGoFile records path in the predicted set only when it is Go source.
// The truth is a go/types use set, so a .md or .yaml hit is outside the
// oracle's competence, not a wrong answer: the references tool is a
// fleet-wide occurrence search over code, docs AND config by design, and
// scoring its doc hits against a truth that cannot contain them would
// manufacture a precision failure the tool does not have.
func addGoFile(set map[string]bool, path string) {
	if strings.HasSuffix(path, ".go") {
		set[path] = true
	}
}

// directCallEdges keeps only the typed edges a go/types static-call truth
// can adjudicate: EdgeDirect. EdgeIfaceDispatch and EdgeFuncValue are
// resolved evidence the tools return by default and this oracle cannot
// score — see scoreFindCallers.
func directCallEdges(typed []core.Edge) []core.Edge {
	out := make([]core.Edge, 0, len(typed))
	for _, e := range typed {
		if e.Kind == core.EdgeDirect {
			out = append(out, e)
		}
	}
	return out
}

func setOf(ss []string) map[string]bool {
	out := make(map[string]bool, len(ss))
	for _, s := range ss {
		out[s] = true
	}
	return out
}

// isMethod reports whether a qualified name carries a receiver.
func isMethod(name string) bool { return strings.Contains(name, ".") }
