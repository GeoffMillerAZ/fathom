package main

// langoracle_react.go — LB-F379 (wave c3, item react): `indexbench
// lang-react`, the React reading. React is not a dialect of its own: its
// components, class components, hooks and props are kinds the TypeScript and
// JavaScript dialects emit (JSComponent, JSClassComponent, JSHook, JSProp), so
// this measures one subject's TS/JS files through ingest's own dispatch
// (fusion's ExtractForBench, the "javascript" tags extractor) and scores the
// React kinds alone against calibration/oracles/react-docgen's oracle:
//
//   - L1: the oracle's components and props (react-docgen-typescript), matched
//     as lang-oracle matches definitions (langbench.ScoreL1: same file, bare
//     name, |Δline| <= 2) under reactManifest, whose kinds are the React ones
//     only, so every other definition is excluded and counted, and a hook (no
//     docgen truth) is excluded too;
//   - L2: the oracle's render and hook pairs (the TypeScript checker's: a JSX
//     element or a hook call, from the enclosing definition), against fathom's
//     call pairs whose callee names one of the oracle's components or hooks;
//   - L3: the oracle's file pairs against the file resolver's edges
//     (fusion.ResolveFileEdges) to those definitions, and the oracle's imports
//     that bind one against the resolver's own module resolution
//     (fusion.ResolveImportSpecs).
//
// The level is langbench's computation from the counts; nobody types it. The
// oracle is tier C (docgen's type test), which caps every level at good.

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// reactDialect is the reading's dialect id and queue id.
const reactDialect = "react"

// reactExts are the files the TS/JS dialects own, so React's.
var reactExts = map[string]bool{".js": true, ".jsx": true, ".mjs": true, ".cjs": true, ".ts": true, ".tsx": true, ".mts": true, ".cts": true}

// reactManifest is the manifest the React reading is scored under: the
// React kinds of the TS/JS dialects, with a hook left out of L1 (the oracle
// grades no hook) and the oracle's kinds mapped onto the same canonical kinds.
var reactManifest = langbench.Manifest{
	ID: reactDialect, Display: "React", Kind: "language", Engine: "tags", LangTag: "ts",
	Match: langbench.ManifestMatch{Extensions: []string{".cjs", ".cts", ".js", ".jsx", ".mjs", ".mts", ".ts", ".tsx"}},
	Kinds: map[string]langbench.KindSpec{
		"component":       {SourceKind: string(core.KindJSComponent), Canonical: "function", Callable: true},
		"class_component": {SourceKind: string(core.KindJSClassComponent), Canonical: "class", Container: true},
		"prop":            {SourceKind: string(core.KindJSProp), Canonical: "field"},
	},
	RequiredKinds: []string{"component", "prop"},
	OracleKinds: map[string]map[string]string{
		"tool": {"component": "function", "class_component": "class", "prop": "field"},
	},
	TargetLevel: "L3",
	QueueID:     reactDialect,
}

// reactBaselineManifest scores what the TS/JS dialects emitted for React's
// definitions before LB-F377: a function component as a JSFunc, a class
// component as a JSClass, a prop as nothing (step 0 of wave c3's item react).
func reactBaselineManifest() langbench.Manifest {
	m := reactManifest
	m.ID = reactDialect + "-baseline"
	m.Kinds = map[string]langbench.KindSpec{
		"component":       {SourceKind: string(core.KindJSFunc), Canonical: "function", Callable: true},
		"class_component": {SourceKind: string(core.KindJSClass), Canonical: "class", Container: true},
		"prop":            {SourceKind: string(core.KindJSProp), Canonical: "field"},
	}
	return m
}

// reactKinds are the SourceKinds the reading's notes count.
var reactKinds = []core.SourceKind{core.KindJSComponent, core.KindJSClassComponent, core.KindJSHook, core.KindJSProp}

// reactOracleFile is calibration/oracles/react-docgen/react_oracle.mjs's output.
type reactOracleFile struct {
	Tool        string                `json:"tool"`
	Version     string                `json:"version"`
	Tier        string                `json:"tier"`
	Definitions []langbench.OracleDef `json:"definitions"`
	Hooks       []langbench.OracleDef `json:"hooks"`
	Calls       []oraclePair          `json:"calls"`
	FileEdges   [][2]string           `json:"file_edges"`
	// Names are the (file, name as written) pairs at which a reference
	// resolves to one of the components or hooks.
	Names    [][2]string         `json:"names"`
	Imports  []importOracleEntry `json:"imports"`
	Excluded map[string]int      `json:"excluded"`
	Added    map[string]int      `json:"added"`
	Files    int                 `json:"files"`
}

// reactUnmatched is -unmatched's JSON for one subject (diagnosis only).
type reactUnmatched struct {
	DialectOnly []langbench.OracleDef `json:"dialect_only"`
	OracleOnly  []langbench.OracleDef `json:"oracle_only"`
	CallsFathom []langbench.CallPair  `json:"calls_fathom_only"`
	CallsOracle []langbench.CallPair  `json:"calls_oracle_only"`
	EdgesFathom [][2]string           `json:"file_edges_fathom_only"`
	EdgesOracle [][2]string           `json:"file_edges_oracle_only"`
	Imports     []unmatchedImport     `json:"imports_unmatched"`
}

// cmdLangReact is `indexbench lang-react` (LB-F379).
func cmdLangReact(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lang-react", flag.ContinueOnError)
	fs.SetOutput(stderr)
	subject := fs.String("subject", "", "subject checkout (the oracle's --repo) (required)")
	subjectName := fs.String("subject-name", "", "subject name recorded in the reading (default: the directory name)")
	oraclePath := fs.String("oracle", "", "calibration/oracles/react-docgen/react_oracle.mjs output for the subject (required)")
	only := fs.String("only", "", "comma-separated globs (subject-relative) restricting the files measured")
	out := fs.String("out", "", "reading to write; an existing react reading is merged (required)")
	unmatched := fs.String("unmatched", "", "write the subject's misses both ways as JSON (diagnosis; never part of the reading)")
	queuePath := fs.String("queue", "docs/plans/language-breadth.queue.json", "the queue (for the target level)")
	date := fs.String("date", time.Now().UTC().Format("2006-01-02"), "measured_at")
	commit := fs.String("commit", "", "fathom commit measured (default: git rev-parse --short HEAD)")
	baseline := fs.Bool("baseline-kinds", false, "score the kinds the TS/JS dialects gave React's definitions before LB-F377 (a function as a component, a class as a class component, no prop): the step-0 reading, written to -out under dialect react-baseline")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *subject == "" || *oraclePath == "" || *out == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: indexbench lang-react -subject DIR -oracle O.json [-only GLOBS] -out R.json")
		return 2
	}
	var globs []string
	for _, g := range strings.Split(*only, ",") {
		if g = strings.TrimSpace(g); g != "" {
			globs = append(globs, g)
		}
	}
	ctx := context.Background()
	if *commit == "" {
		*commit = gitOut(ctx, ".", "rev-parse", "--short", "HEAD")
	}
	name := *subjectName
	if name == "" {
		name = filepath.Base(filepath.Clean(*subject))
	}
	target := reactManifest.TargetLevel
	if q, err := langbench.LoadQueue(*queuePath); err == nil {
		for _, e := range q {
			if e.ID == reactDialect {
				target = e.TargetLevel
			}
		}
	}
	of, err := readReactOracle(*oraclePath)
	if err != nil {
		fmt.Fprintln(stderr, "lang-react:", err)
		return 1
	}
	m := reactManifest
	if *baseline {
		m = reactBaselineManifest()
	}
	sub, run, notes, um, err := measureReact(ctx, m, *subject, name, globs, of)
	if err != nil {
		fmt.Fprintln(stderr, "lang-react:", err)
		return 1
	}
	r, err := loadOrNewReading(*out, m, target)
	if err != nil {
		fmt.Fprintln(stderr, "lang-react:", err)
		return 1
	}
	r.TargetLevel = target
	r.MeasuredAt = *date
	r.FathomCommit = *commit
	r.Extractor = "dialect"
	r.Oracles = mergeOracles(r.Oracles, []langbench.OracleRun{run})
	r.Notes = mergeNotes(r.Notes, append([]string{noteReactScope}, notes...))
	langbench.MergeSubject(&r, sub)
	if err := r.Validate(); err != nil {
		fmt.Fprintln(stderr, "lang-react: reading refused:", err)
		return 1
	}
	b, err := langbench.MarshalReading(r)
	if err != nil {
		fmt.Fprintln(stderr, "lang-react:", err)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintln(stderr, "lang-react:", err)
		return 1
	}
	if err := os.WriteFile(*out, b, 0o644); err != nil {
		fmt.Fprintln(stderr, "lang-react:", err)
		return 1
	}
	if *unmatched != "" {
		ub, err := json.MarshalIndent(um, "", " ")
		if err == nil {
			err = os.WriteFile(*unmatched, append(ub, '\n'), 0o644)
		}
		if err != nil {
			fmt.Fprintln(stderr, "lang-react:", err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "%s: level_met %s grade %+v (%d subjects) -> %s\n", r.Dialect, r.LevelMet, r.Grade, len(r.Subjects), *out)
	return 0
}

// noteReactScope says what the React reading measures.
const noteReactScope = "React's kinds of the TypeScript and JavaScript dialects (JSComponent, JSClassComponent, JSHook, JSProp), extracted by ingest's own dispatch: L1 scores components and props against react-docgen-typescript, a hook is outside L1 (no docgen truth) and counted; L2 scores the render and hook pairs (a JSX element or a hook call, from its enclosing definition) the TypeScript checker resolves to one of the oracle's components or hooks, against fathom's call pairs whose callee is a name the file binds to one; L3 scores the file pairs any reference to one makes across files and the imports that bind one. Every other definition and pair is out of the reading (excluded_by_kind)"

func readReactOracle(p string) (reactOracleFile, error) {
	var of reactOracleFile
	b, err := os.ReadFile(p)
	if err != nil {
		return of, err
	}
	if err := json.Unmarshal(b, &of); err != nil {
		return of, fmt.Errorf("%s: %w", p, err)
	}
	of.Tier = orDefault(of.Tier, "C")
	if of.Tool == "" || !strings.Contains("ABCD", of.Tier) || len(of.Tier) != 1 {
		return of, fmt.Errorf("%s: tool and a tier A-D are required", p)
	}
	return of, nil
}

// measureReact extracts the subject's TS/JS files as ingest does and scores
// the React kinds against the oracle.
func measureReact(ctx context.Context, m langbench.Manifest, root, name string, globs []string, of reactOracleFile) (langbench.Subject, langbench.OracleRun, []string, reactUnmatched, error) {
	var um reactUnmatched
	run := langbench.OracleRun{Tier: of.Tier, Tool: of.Tool, Version: of.Version, Licence: "MIT"}
	const service = "langbench"
	refs, err := connector.NewLocalConnector().Discover(ctx, core.ServiceRef{ID: service, Path: root})
	if err != nil {
		return langbench.Subject{}, run, nil, um, fmt.Errorf("discover %s: %w", root, err)
	}
	pipe := fusion.New(nil, nil, fusion.Config{})
	defer pipe.Close()
	tsengine.ResetPeakModulePages()
	sha, _ := subjectSHA(ctx, root, "") // round 3 A1: never an enclosing repository's HEAD
	sub := langbench.Subject{Name: name, SHA: sha, Counts: map[string]int{},
		Families: map[string]string{}, ExcludedByKind: map[string]int{}, Tenancy: &langbench.SubjectTenancy{FaultingFiles: []string{}}}
	var owned []core.SourceRef
	for _, ref := range refs {
		if reactExts[strings.ToLower(path.Ext(ref.Path))] && onlyMatch(globs, ref.Path) {
			owned = append(owned, ref)
		}
	}
	sort.Slice(owned, func(i, j int) bool { return owned[i].Path < owned[j].Path })
	var notes []string
	if len(globs) > 0 {
		notes = append(notes, fmt.Sprintf("%s: restricted to %s (-only; the files outside are a disclosed gap, not measured)", name, strings.Join(globs, ", ")))
	}
	// Warm-up: one untimed extraction per extension compiles each grammar
	// variant (js, ts, tsx) and its queries, so per-file times measure
	// extraction, not a one-time compile (lang-oracle's rule, design §9).
	warmed := map[string]bool{}
	for _, ref := range owned {
		ext := strings.ToLower(path.Ext(ref.Path))
		if warmed[ext] {
			continue
		}
		if content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ref.Path))); err == nil {
			if ex, _ := pipe.ExtractForBench(service, refs, ref.Path, content); ex.Skipped == "" {
				warmed[ext] = true
			}
		}
	}
	if len(warmed) > 0 {
		notes = append(notes, "per-file times exclude the one-time grammar compile (an untimed warm-up extraction of one file per extension precedes timing; a file that still instantiates a grammar module is timed again, warm)")
	}
	var nodes []core.SymbolNode
	var srcFiles []core.SourceFile
	inScope := map[string]bool{}
	kinds := map[core.SourceKind]int{}
	for _, ref := range owned {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ref.Path)))
		if err != nil {
			return sub, run, nil, um, err
		}
		inst := tsengine.Instantiations()
		start, cpu0 := fileClocks.wall(), fileClocks.cpu()
		ex, err := pipe.ExtractForBench(service, refs, ref.Path, content)
		elapsed, cpu := fileClocks.wall().Sub(start), fileClocks.cpu()-cpu0
		if tsengine.Instantiations() != inst && ex.Skipped == "" {
			start, cpu0 = fileClocks.wall(), fileClocks.cpu()
			ex, err = pipe.ExtractForBench(service, refs, ref.Path, content)
			elapsed, cpu = fileClocks.wall().Sub(start), fileClocks.cpu()-cpu0
		}
		if ex.Skipped != "" {
			continue
		}
		sub.Files++
		sub.Bytes += int64(len(content))
		sub.Tenancy.ExtractMS += float64(elapsed.Microseconds()) / 1000
		sub.Tenancy.ExtractCPUMS += float64(cpu.Microseconds()) / 1000
		sub.Tenancy.MaxFileMS = max(sub.Tenancy.MaxFileMS, fileCostMS(elapsed, cpu, cpu0 > 0))
		if err != nil {
			sub.Tenancy.Faults++
			sub.Tenancy.FaultingFiles = append(sub.Tenancy.FaultingFiles, ref.Path)
			continue
		}
		for _, n := range ex.Nodes {
			if n.FilePath == ref.Path {
				nodes = append(nodes, n)
				kinds[n.Kind]++
			}
		}
		inScope[ref.Path] = true
		srcFiles = append(srcFiles, core.SourceFile{Path: ref.Path, Content: content})
	}
	if sub.Files == 0 {
		return sub, run, nil, um, fmt.Errorf("subject %s has no TypeScript or JavaScript file fathom would extract", root)
	}
	sub.Tenancy.PeakModulePages = int(tsengine.PeakModulePages())
	var kn []string
	for _, k := range reactKinds {
		kn = append(kn, fmt.Sprintf("%s %d", k, kinds[k]))
	}
	notes = append(notes, fmt.Sprintf("%s: fathom's React definitions: %s", name, strings.Join(kn, ", ")))
	if len(of.Excluded) > 0 {
		var ex []string
		for _, k := range sortedKeys(of.Excluded) {
			ex = append(ex, fmt.Sprintf("%s %d", k, of.Excluded[k]))
		}
		notes = append(notes, fmt.Sprintf("%s: the oracle left out of its truth: %s", name, strings.Join(ex, "; ")))
	}
	if len(of.Added) > 0 {
		var ad []string
		for _, k := range sortedKeys(of.Added) {
			ad = append(ad, fmt.Sprintf("%s %d", k, of.Added[k]))
		}
		notes = append(notes, fmt.Sprintf("%s: the oracle added to docgen's components: %s", name, strings.Join(ad, "; ")))
	}

	// L1: components and props.
	var defs []langbench.OracleDef
	dropped := 0
	for _, d := range of.Definitions {
		if !inScope[d.Path] {
			dropped++
			continue
		}
		defs = append(defs, d)
	}
	l1 := langbench.ScoreL1(m, "tool", nodes, defs)
	addCounts(sub.Counts, l1.Counts)
	addCounts(sub.ExcludedByKind, l1.Excluded)
	sub.KindIntersection = l1.Intersection
	sub.Families[langbench.FamilyL1] = of.Tier
	um.DialectOnly, um.OracleOnly = nonNil(l1.DialectOnly), nonNil(l1.OracleOnly)

	// The React definitions the edges are graded over (the oracle's
	// components and hooks, by file and bare name), and the names each file
	// binds to one of them.
	reactDefs := map[[2]string]bool{}
	for _, d := range append(append([]langbench.OracleDef(nil), of.Definitions...), of.Hooks...) {
		if d.Kind != "prop" {
			reactDefs[[2]string{d.Path, langbench.BareName(d.Name)}] = true
		}
	}
	bound := map[[2]string]bool{}
	for _, n := range of.Names {
		bound[n] = true
	}

	// L2: render and hook pairs.
	truth := map[langbench.CallPair]bool{}
	files := map[string]bool{}
	for _, c := range of.Calls {
		if !inScope[c.Path] {
			dropped++
			continue
		}
		truth[langbench.CallPair{Path: c.Path, Caller: m.MatchName(c.From), Callee: m.MatchName(c.To)}] = true
		files[c.Path] = true
	}
	found := map[langbench.CallPair]bool{}
	for p := range m.CallPairs(nodes, inScope) {
		if bound[[2]string{p.Path, p.Callee}] {
			found[p] = true
		}
	}
	langbench.ScorePairs(truth, found, sub.Counts, langbench.CntCallTruth, langbench.CntCallTruthFound, langbench.CntCallFathom, langbench.CntCallFathomInTrue)
	sub.Families[langbench.FamilyCalls] = of.Tier
	um.CallsFathom, um.CallsOracle = pairsOnly(found, truth), pairsOnly(truth, found)

	// L3: file pairs to the React definitions, and the imports binding one.
	edgeTruth := map[[2]string]bool{}
	for _, e := range of.FileEdges {
		if inScope[e[0]] && inScope[e[1]] && e[0] != e[1] {
			edgeTruth[e] = true
		}
	}
	edges, rep := fusion.ResolveFileEdges(ctx, "javascript", root, srcFiles, nodes)
	if rep.Skipped {
		notes = append(notes, name+": file-resolved edges not measured: "+rep.SkipReason)
	} else {
		toReact := map[string]bool{}
		for _, n := range nodes {
			if reactDefs[[2]string{n.FilePath, langbench.BareName(n.Name)}] {
				toReact[n.Key()] = true
			}
		}
		var kept []core.Edge
		for _, e := range edges {
			if toReact[e.ToSymbol] {
				kept = append(kept, e)
			}
		}
		langbench.ScoreFileEdges(edgeTruth, kept, nodes, sub.Counts)
		sub.Families[langbench.FamilyFileEdges] = of.Tier
		mine := fileEdgePairs(kept, nodes)
		um.EdgesFathom, um.EdgesOracle = nonNilPairs(edgesOnlyIn(mine, edgeTruth)), nonNilPairs(edgesOnlyIn(edgeTruth, mine))
	}
	si := &subjectImports{ctx: ctx, dialect: "typescript", root: root, name: name, files: srcFiles, nodes: nodes, inScope: inScope}
	irefs := make([]langbench.OracleImport, 0, len(of.Imports))
	for _, e := range of.Imports {
		irefs = append(irefs, langbench.OracleImport{Path: e.Path, Line: e.Line, ToPath: e.ToPath, Explicit: true})
	}
	if err := si.score(irefs, of.Tier, of.Tool, &sub); err != nil {
		return sub, run, nil, um, err
	}
	notes = append(notes, si.notes...)
	if si.ok {
		um.Imports = importMisses(of.Imports, si.sites, inScope)
	}
	if dropped > 0 {
		notes = append(notes, fmt.Sprintf("%s: %d oracle entries name a file outside the measured files and are not scored", name, dropped))
	}
	return sub, run, notes, um, nil
}

func nonNil(ds []langbench.OracleDef) []langbench.OracleDef {
	if ds == nil {
		return []langbench.OracleDef{}
	}
	return ds
}

func nonNilPairs(ps [][2]string) [][2]string {
	if ps == nil {
		return [][2]string{}
	}
	return ps
}

// pairsOnly are a's pairs not in b, sorted.
func pairsOnly(a, b map[langbench.CallPair]bool) []langbench.CallPair {
	out := []langbench.CallPair{}
	for p := range a {
		if !b[p] {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		x, y := out[i], out[j]
		if x.Path != y.Path {
			return x.Path < y.Path
		}
		if x.Caller != y.Caller {
			return x.Caller < y.Caller
		}
		return x.Callee < y.Callee
	})
	return out
}
