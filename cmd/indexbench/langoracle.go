package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// scipClangTool is the tool name scip-clang writes in its index's metadata.
const scipClangTool = "scip-clang"

// noteScipClangNoImports discloses why a scip-clang index scores no imports.
// scipClangNoImportsNote writes it at the file edges' own tier (round 3 A4).
const noteScipClangNoImports = "scip-clang records no reference on an #include line and gives no occurrence the Import role, so its index is no imports oracle: import_resolution is an imports dump's (the build-flags preprocessor oracle at tier A, or clang-includes at tier B), and file edges stay the index's cross-file references (tier A)"

// noteScipClangDefsScoped names the files L1 against scip-clang leaves out.
const noteScipClangDefsScoped = "files the scip-clang index does not hold"

// noteScipClangInactive names the lines L1 against scip-clang leaves out.
const noteScipClangInactive = "on lines the build's own preprocessor discards (an inactive #if branch of the one configuration scip-clang indexes)"

// noteFileEdgeTruthScoped names the index edges the file-edge truth leaves out.
const noteFileEdgeTruthScoped = "cross-file edges run from or to a file this run does not measure (-only, discovery's exclusions, an ingest gate) and are left out of the file-edge truth (§1.4)"

// oracleTimeout bounds one external oracle run (design §13.4 step 19).
const oracleTimeout = 20 * time.Minute

// fileResolveLang names the dialects today's file resolver serves
// (fusion.ResolveFileEdges), so file-edge metrics are measured for them:
// against a SCIP index's cross-file edges, or, with none, a neutral file's
// imports (LB-F57). Go, native and without a file resolver, is measured
// from its typed pass instead (LB-F58, langoracle_go.go).
//
// C# and Java resolve through their namespace-dir manifests (LB-F64); C joins
// C++ on the include-path strategy (LB-F80). Rego, native, resolves through
// its own reader's import specs and the rego-package strategy (LB-F131), CUE
// through its own reader's and the cue-module strategy (LB-F583).
// Jinja2 joins C on include-path (LB-Q-jinja2): a template name is looked up in
// the includer's directory, then each ancestor. Kotlin joins them on namespace-dir (O474): its file edges are read from the
// PSI dump's imports, tier B. HLSL joins C on include-path (LB-Q-hlsl): its
// file edges are read from clang-includes' imports, tier B.
var fileResolveLang = map[string]string{"typescript": "javascript", "javascript": "javascript", "python": "python", "c": "c", "cpp": "cpp", "php": "php", "rust": "rust", "ruby": "ruby", "dart": "dart", "csharp": "csharp", "java": "java", "kotlin": "kotlin", "rego": "rego", "scala": "scala", "css": "css", "scss": "scss", "cue": "cue", "jinja2": "jinja2", "hlsl": "hlsl"}

// cmdLangOracle is `indexbench lang-oracle` (LB-F23, design §10): one
// subject's measurement of one dialect, merged into the dialect's reading.
// No daemon, no store: the connector lists the subject's files, the
// dialect's OWN extractor runs them in process (design §10: "runs the
// dialect's extractor on each" — never fusion's live dispatch, which for a
// port whose incumbent hasn't been switched in yet (§11 step 2's merge
// gate) would silently measure the wrong extractor: verified directly, a
// latent gap this fix closes — see resolveBenchSpec's "with" below),
// universal-ctags (tier B), a SCIP index (tier A) and a tool-native oracle
// (tier A) are scored per §1.4, and the reading's level is computed, never
// typed.
func cmdLangOracle(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lang-oracle", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dialect := fs.String("dialect", "", "dialect id (required)")
	manifestPath := fs.String("manifest", "", "the dialect's dialect.json (default: internal/extractor/dialects/<id>/dialect.json when present, else the embedded incumbent spec)")
	subject := fs.String("subject", "", "subject checkout to measure (required)")
	subjectName := fs.String("subject-name", "", "subject name recorded in the reading (default: the directory name)")
	ctagsPath := fs.String("ctags", "", "universal-ctags binary (default: workspace/tools/ctags, then ctags on PATH; a non-universal ctags is reported unavailable)")
	scipPath := fs.String("scip", "", "SCIP index of the subject (tier A; optional)")
	scipTier := fs.String("scip-tier", "", "the -scip index's tier (round 3 A2): A when the subject's own build wrote the compile database it read untranslated, B for a translation (a clang database derived from nvcc's); every metric the index feeds starts at it; required for a scip-clang index (round 4 C4-R2), A for any other when not given")
	scipDefs := fs.Bool("scip-defs", false, "score L1 against the -scip index's definitions (tier A) instead of universal-ctags (owner decision D2: a stronger oracle where ctags is deficient)")
	var toolOracles multiFlag
	fs.Var(&toolOracles, "tool-oracle", "oracle JSON in the neutral shape (definitions, calls, references; tier A unless it says otherwise; repeatable)")
	var baseOracles multiFlag
	fs.Var(&baseOracles, "base-oracle", "a composing dialect's base dialect's oracle JSON (e.g. the cue oracle for timoni): its calls, references and cross_references join those of the -tool-oracle files that grade references or cross-references (langoracle_base.go; LB-Q-timoni-rest; repeatable)")
	callsFrom := fs.String("calls-from", "scip", "which oracle grades L2 beside -scip: scip (the index's call sites, tier A) or tool (the -tool-oracle files' calls at their own tier, the index still grading definitions, typed edges and imports; LB-F204, O264)")
	adjudicate := fs.String("adjudicate", "", "oracle-deficiency disclosure JSON (owner decision D3): dialect-only definitions the L1 oracle is verified to miss, each admitted only by a deterministic file:line, name and module-scope check, listed in the reading and left out of sym_precision's denominator")
	unmatched := fs.String("unmatched", "", "write this subject's L1 misses both ways (dialect_only, oracle_only) and its L2 call pairs both ways (calls.fathom_only, calls.oracle_only) as JSON — the worker's diagnosis and the source of D3 adjudication candidates; never part of the reading")
	subjectSHAFlag := fs.String("subject-sha", "", "the subject's commit (a full 40-hex sha) when -subject is a copy with no .git of its own, as the CUDA driver's scratch subject is; without it the sha is the subject's own HEAD, and a subject that has none (not its repository's top level, no .git) is refused (round 4 C4-R7)")
	var extraNotes multiFlag
	fs.Var(&extraNotes, "note", "a disclosure the measurement driver writes into the reading's notes, prefixed with the subject's name (repeatable; round 3 A4: a driver's own record, never a hand edit)")
	only := fs.String("only", "", "comma-separated globs (subject-relative, * within a segment, ** across): measure only these files — e.g. the SQL families a grammar supports (D2); the reading names the restriction")
	out := fs.String("out", "", "reading to write; an existing reading of the same dialect is merged (required)")
	queuePath := fs.String("queue", "docs/plans/language-breadth.queue.json", "the queue (for the dialect's target level)")
	date := fs.String("date", time.Now().UTC().Format("2006-01-02"), "measured_at")
	commit := fs.String("commit", "", "fathom commit measured (default: git rev-parse --short HEAD)")
	tq := tagsQueryFlags(fs) // a third-party definitions query as a tier-C L1 oracle (LB-F46, langoracle_tagsquery.go)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *scipTier != "" && !slices.Contains([]string{"A", "B", "C", "D"}, *scipTier) {
		fmt.Fprintf(stderr, "lang-oracle: -scip-tier %q: want A, B, C or D\n", *scipTier)
		return 2
	}
	if *scipPath != "" && *scipTier == "" {
		// round 4 C4-R2: a scip-clang index read a compile database the driver
		// chose (the build's own, or a translation), so it names its tier
		raw, err := os.ReadFile(*scipPath)
		if err != nil {
			fmt.Fprintf(stderr, "lang-oracle: %v\n", err)
			return 2
		}
		if _, tool, err := langbench.SCIPDocuments(raw); err == nil && tool == scipClangTool {
			fmt.Fprintf(stderr, "lang-oracle: -scip %s is a scip-clang index: -scip-tier is required (A for the build's own compile database, B for a translation)\n", *scipPath)
			return 2
		}
	}
	if *dialect == "" || *subject == "" || *out == "" || fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: indexbench lang-oracle -dialect <id> -subject <dir> -out <reading.json> [-ctags <bin>] [-scip <index>] [-tool-oracle <json>]")
		return 2
	}
	spec, source, err := resolveBenchSpec(*dialect, *manifestPath)
	if err != nil {
		fmt.Fprintln(stderr, "lang-oracle:", err)
		return 2
	}
	target := spec.Manifest.TargetLevel
	if q, qerr := langbench.LoadQueue(*queuePath); qerr == nil {
		for _, e := range q {
			if e.ID == spec.Manifest.ID {
				target = e.TargetLevel
			}
		}
	}
	if target == "" {
		target = "L1"
	}
	ctx := context.Background()
	if *commit == "" {
		*commit = gitOut(ctx, ".", "rev-parse", "--short", "HEAD")
	}
	name := *subjectName
	if name == "" {
		name = filepath.Base(filepath.Clean(*subject))
	}

	// Measure the dialect's OWN extractor (ExtractForBenchWith, LB-F18),
	// never fusion's live dispatch: a port whose incumbent still occupies
	// register.go's extension map (§11 step 2's merge gate not yet passed)
	// would otherwise be measured as its own incumbent — wrong extractor,
	// wrong reading, and undetectable from the output alone (kept
	// SourceKind strings, §11 step 3, make the incumbent's nodes still
	// "kind-valid" against the new manifest). Verified directly: before
	// this fix, `lang-oracle -dialect c` on the c dialect's own fixtures
	// read signature_coverage 0 (the CGO incumbent never sets Signature at
	// all) despite the dialect itself signing every callable.
	//
	// The doc engine needs the identical treatment (LB-Q-kubernetes found
	// it missing): a doc-engine PORT still dispatched through its
	// incumbent (kubernetes: k8sextract.go, via p.toolExts[ToolK8sManifest])
	// was silently measured as that incumbent instead of the new
	// DocExtractor — dialectExtractor in langparity.go already branches on
	// engine "doc" via NewDocDialectExtractor; this mirrors it here.
	with, closeExtractor, err := langOracleExtractor(spec.Manifest, source)
	if err != nil {
		fmt.Fprintln(stderr, "lang-oracle:", err)
		return 1
	}
	defer closeExtractor()

	if *scipDefs && *scipPath == "" {
		fmt.Fprintln(stderr, "lang-oracle: -scip-defs needs -scip")
		return 2
	}
	if *callsFrom != "scip" && *callsFrom != "tool" {
		fmt.Fprintln(stderr, "lang-oracle: -calls-from is scip or tool")
		return 2
	}
	if *callsFrom == "tool" && (*scipPath == "" || len(toolOracles) == 0) {
		fmt.Fprintln(stderr, "lang-oracle: -calls-from tool needs -scip and a -tool-oracle that carries calls")
		return 2
	}
	var onlyGlobs []string
	for _, g := range strings.Split(*only, ",") {
		if g = strings.TrimSpace(g); g != "" {
			onlyGlobs = append(onlyGlobs, g)
		}
	}
	tools, err := tagsQueryOracle(ctx, *tq, spec, *manifestPath, *subject, name, onlyGlobs, toolOracles, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "lang-oracle:", err)
		return tagsQueryExit(err)
	}
	var baseNotes []string
	if len(baseOracles) > 0 {
		mdir, err := os.MkdirTemp("", "langoracle-base-")
		if err != nil {
			fmt.Fprintln(stderr, "lang-oracle:", err)
			return 1
		}
		defer os.RemoveAll(mdir)
		if tools, baseNotes, err = mergeBaseOracles(tools, baseOracles, mdir); err != nil {
			fmt.Fprintln(stderr, "lang-oracle:", err)
			return 1
		}
	}
	m, err := measureSubject(ctx, spec, *subject, name, oracleOpts{ctags: *ctagsPath, scip: *scipPath, scipTier: *scipTier, scipDefs: *scipDefs, callsFromTool: *callsFrom == "tool", tools: tools, only: onlyGlobs, adjudicate: *adjudicate, unmatched: *unmatched, baseNotes: baseNotes, subjectSHA: *subjectSHAFlag, requireSHA: true}, with)
	if err != nil {
		fmt.Fprintln(stderr, "lang-oracle:", err)
		return 1
	}

	r, err := loadOrNewReading(*out, spec.Manifest, target)
	if err != nil {
		fmt.Fprintln(stderr, "lang-oracle:", err)
		return 1
	}
	r.TargetLevel = target
	r.MeasuredAt = *date
	r.FathomCommit = *commit
	r.Extractor = source
	r.Oracles = mergeOracles(r.Oracles, m.oracles)
	for _, n := range extraNotes {
		m.notes = append(m.notes, name+": "+n)
	}
	r.Notes = mergeNotes(r.Notes, append(append([]string(nil), spec.Notes...), m.notes...))
	langbench.MergeSubject(&r, m.subject)
	if err := r.Validate(); err != nil {
		fmt.Fprintln(stderr, "lang-oracle: reading refused:", err)
		return 1
	}
	b, err := langbench.MarshalReading(r)
	if err != nil {
		fmt.Fprintln(stderr, "lang-oracle:", err)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintln(stderr, "lang-oracle:", err)
		return 1
	}
	if err := os.WriteFile(*out, b, 0o644); err != nil {
		fmt.Fprintln(stderr, "lang-oracle:", err)
		return 1
	}
	fmt.Fprintf(stdout, "%s: level_met %s grade %+v (%d subjects) -> %s\n", r.Dialect, r.LevelMet, r.Grade, len(r.Subjects), *out)
	return 0
}

// langOracleExtractor selects the same extractor for an oracle run and a re-time.
func langOracleExtractor(m langbench.Manifest, source string) (extractor.Extractor, func(), error) {
	closeExtractor := func() {}
	if source != "dialect" {
		return nil, closeExtractor, nil
	}
	var with extractor.Extractor
	var err error
	switch m.Engine {
	case "tags+doc":
		with, err = extractor.NewTagsDocExtractor(m.ID)
	case "tags":
		with, err = extractor.NewTSExtractor(m.HostID())
	case "doc":
		with, err = extractor.NewDocDialectExtractor(m.ID)
	case "native":
		with, _ = extractor.NativeDialectExtractor(m.ID)
	}
	if c, ok := with.(interface{ Close() }); ok {
		closeExtractor = c.Close
	}
	return with, closeExtractor, err
}

// resolveBenchSpec finds what to measure a dialect with: an explicit
// manifest, the dialect's own directory, or the embedded incumbent spec.
func resolveBenchSpec(id, manifestPath string) (langbench.BenchSpec, string, error) {
	if manifestPath == "" {
		p := filepath.Join("internal", "extractor", "dialects", id, "dialect.json")
		if _, err := os.Stat(p); err == nil {
			manifestPath = p
		}
	}
	inc, hasInc := langbench.IncumbentSpec(id)
	if manifestPath != "" {
		b, err := os.ReadFile(manifestPath)
		if err != nil {
			return langbench.BenchSpec{}, "", err
		}
		m, err := langbench.ParseManifest(b)
		if err != nil {
			return langbench.BenchSpec{}, "", fmt.Errorf("%s: %w", manifestPath, err)
		}
		if m.ID != id {
			return langbench.BenchSpec{}, "", fmt.Errorf("%s: id %q, want %q", manifestPath, m.ID, id)
		}
		spec := langbench.BenchSpec{Manifest: m, Notes: m.Notes}
		switch {
		case m.Ctags != nil:
			// The manifest names its own tier-B invocation (design §13.4
			// step 18) — every dialect after the five incumbents.json
			// covers takes this path.
			spec.Ctags = *m.Ctags
		case hasInc:
			spec.Ctags = inc.Ctags
		}
		return spec, "dialect", nil
	}
	if !hasInc {
		return langbench.BenchSpec{}, "", fmt.Errorf("no manifest and no incumbent spec for dialect %q", id)
	}
	return inc, "incumbent", nil
}

// fileClocks are the two clocks one file's extraction is timed with; tests
// replace them (langoracle_cpu_test.go).
var fileClocks = struct {
	wall func() time.Time
	cpu  func() time.Duration
}{wall: time.Now, cpu: processCPU}

// fileCostMS is what one file contributes to the tenancy floor's
// max_file_ms: its CPU time, the basis the throughput floor reads (owner
// decision D1, EW-3), so a machine busy with other work cannot fail a
// reading by making one file wait. cpuOK is whether the platform has a CPU
// reading at all (a running process has always used some, so a zero reading
// before the file means none); without one the wall time stands in and the
// reading's throughput_basis says "wall".
func fileCostMS(wall, cpu time.Duration, cpuOK bool) int64 {
	if cpuOK {
		return cpu.Milliseconds()
	}
	return wall.Milliseconds()
}

type subjectMeasurement struct {
	subject langbench.Subject
	oracles []langbench.OracleRun
	notes   []string
	// kinds counts the SourceKinds extracted from the dialect's files (the
	// parity run's kinds diff, §11 step 3).
	kinds map[string]int
}

// oracleOpts are one subject's oracles and scope.
type oracleOpts struct {
	retimeImports bool     // the reading measured the imports family
	retimeKinds   []string // recorded L1 kind intersection, without consulting an oracle
	noOracles     bool     // return the extraction identity before reading or discovering any oracle
	ctags         string   // universal-ctags binary ("" = workspace/tools/ctags, then PATH)
	scip          string   // SCIP index
	scipTier      string   // the index's tier (-scip-tier; "" is A)
	scipDefs      bool     // the SCIP index's definitions are the L1 oracle (D2)
	// callsFromTool: the -tool-oracle files' calls grade L2 beside -scip
	// (LB-F204, O264); the index's call truth, its call counts and its call
	// notes are not part of the reading.
	callsFromTool bool
	tools         []string // neutral oracle JSON files (definitions, calls, references)
	baseNotes     []string // -base-oracle's disclosures (mergeBaseOracles), one per base oracle
	only          []string // globs restricting the subject's files (D2, SQL)
	adjudicate    string   // oracle-deficiency disclosure file (D3)
	unmatched     string   // where to write the L1 and call-pair misses both ways (diagnosis)
	subjectSHA    string   // the subject's commit when its checkout has no .git (round 3 A1)
	requireSHA    bool     // lang-oracle: a subject with no commit is refused (round 4 C4-R7)
}

// unmatchedFile is -unmatched's JSON: the subject's L1 misses both ways when
// an L1 oracle ran, and its L2 call pairs both ways when a calls oracle ran
// (LB-F64). Diagnosis only; never part of the reading.
type unmatchedFile struct {
	DialectOnly *[]langbench.OracleDef `json:"dialect_only,omitempty"`
	OracleOnly  *[]langbench.OracleDef `json:"oracle_only,omitempty"`
	Calls       *unmatchedCalls        `json:"calls,omitempty"`
	// FileEdges are the file edges on either side of the file-edge scoring
	// (LB-F234): drawn by fathom and not in the truth, and in the truth and not
	// drawn, each as [from, to] sorted.
	FileEdges *unmatchedFileEdges `json:"file_edges,omitempty"`
	// Imports are the imports dump's entries fathom's statements do not bind
	// to the dump's file (LB-F235).
	Imports []unmatchedImport `json:"imports,omitempty"`
}

// unmatchedFileEdges are one subject's (from file, to file) edges on either
// side of the file-edge comparison, from a SCIP index or an imports dump.
type unmatchedFileEdges struct {
	FathomOnly [][2]string `json:"fathom_only"`
	OracleOnly [][2]string `json:"oracle_only"`
	// Via names, for each fathom-only edge in the same order, the smallest
	// (from symbol, to symbol) pair of the edges that drew it.
	Via [][2]string `json:"fathom_only_via,omitempty"`
	// By says, for each fathom-only edge in the same order, how the drawing
	// symbol names the target: "call <text>" when one of its calls ends in the
	// target's name, else "ref" (a type or qualified name the file's references
	// name outside a call).
	By []string `json:"fathom_only_by,omitempty"`
	// Kinds says, for each fathom-only edge in the same order, the kinds of
	// typed edge that drew it ("direct+iface-dispatch"): Go's file edges are
	// the typed pass's (LB-F297).
	Kinds []string `json:"fathom_only_kinds,omitempty"`
	// OracleBlind are the fathom pairs left out of the comparison as ones the
	// index cannot judge (LB-F296), sorted; not in FathomOnly.
	OracleBlind [][2]string `json:"oracle_blind,omitempty"`
}

// fileEdgeDiff is the file-edge truth and fathom's edges over the same files,
// as langbench.ScoreFileEdges compared them.
type fileEdgeDiff struct {
	truth, mine map[[2]string]bool
	via         map[[2]string][2]string // file edge -> its smallest (from symbol, to symbol)
	by          map[[2]string]string    // file edge -> how its drawing symbol names the target
	kinds       map[[2]string]string    // file edge -> the kinds of typed edge that drew it (Go only, LB-F297)
	blind       [][2]string             // pairs left out as oracle-blind (LB-F296)
}

// fileEdgeBy says how each file edge is named by the symbols that draw it: the
// first (in sorted order) call of any drawing symbol whose last segment is the
// target symbol's last name segment ("call <text>"), else "ref". Overloads share
// a key, so a key's calls are the union over its symbols.
func fileEdgeBy(edges []core.Edge, nodes []core.SymbolNode) map[[2]string]string {
	calls := map[string][]string{}
	fileOf := map[string]string{}
	nameOf := map[string]string{}
	for _, n := range nodes {
		calls[n.Key()] = append(calls[n.Key()], n.Calls...)
		fileOf[n.Key()] = n.FilePath
		nameOf[n.Key()] = n.Name
	}
	last := func(s string) string {
		s = strings.ReplaceAll(s, "::", ".")
		return s[strings.LastIndexByte(s, '.')+1:]
	}
	by := map[[2]string]string{}
	for _, e := range edges {
		to, ok := fileOf[e.ToSymbol]
		if !ok || to == e.FromFile {
			continue
		}
		k := [2]string{e.FromFile, to}
		hit := ""
		for _, c := range calls[e.FromSymbol] {
			if last(c) == last(nameOf[e.ToSymbol]) && (hit == "" || c < hit) {
				hit = c
			}
		}
		switch {
		case hit != "":
			if old := by[k]; old == "" || old == "ref" || "call "+hit < old {
				by[k] = "call " + hit
			}
		case by[k] == "":
			by[k] = "ref"
		}
	}
	return by
}

// fileEdgeVia is the smallest (from symbol, to symbol) pair of the edges that
// draw each file edge, the reduction fileEdgePairs makes.
func fileEdgeVia(edges []core.Edge, nodes []core.SymbolNode) map[[2]string][2]string {
	fileOf := map[string]string{}
	for _, n := range nodes {
		fileOf[n.Key()] = n.FilePath
	}
	via := map[[2]string][2]string{}
	for _, e := range edges {
		to, ok := fileOf[e.ToSymbol]
		if !ok || to == e.FromFile {
			continue
		}
		k, v := [2]string{e.FromFile, to}, [2]string{e.FromSymbol, e.ToSymbol}
		if old, seen := via[k]; !seen || v[0]+"\x00"+v[1] < old[0]+"\x00"+old[1] {
			via[k] = v
		}
	}
	return via
}

// fileEdgePairs is the set of (from file, to file) pairs the edges draw, the
// reduction langbench.ScoreFileEdges makes: an edge to a symbol outside the
// nodes, or inside its own file, is no file edge.
func fileEdgePairs(edges []core.Edge, nodes []core.SymbolNode) map[[2]string]bool {
	fileOf := map[string]string{}
	for _, n := range nodes {
		fileOf[n.Key()] = n.FilePath
	}
	mine := map[[2]string]bool{}
	for _, e := range edges {
		if to, ok := fileOf[e.ToSymbol]; ok && to != e.FromFile {
			mine[[2]string{e.FromFile, to}] = true
		}
	}
	return mine
}

// edgesOnlyIn returns the edges of a absent from b, sorted.
func edgesOnlyIn(a, b map[[2]string]bool) [][2]string {
	out := [][2]string{}
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i][0] != out[j][0] {
			return out[i][0] < out[j][0]
		}
		return out[i][1] < out[j][1]
	})
	return out
}

// unmatchedCalls are one subject's call pairs on either side of the L2
// comparison: those fathom found that the oracle does not prove, and those
// the oracle proves that fathom did not find.
type unmatchedCalls struct {
	Oracle     string          `json:"oracle"`
	FathomOnly []unmatchedPair `json:"fathom_only"`
	OracleOnly []unmatchedPair `json:"oracle_only"`
}

type unmatchedPair struct {
	Path   string `json:"path"`
	Caller string `json:"caller"`
	Callee string `json:"callee"`
}

// callDiff is a calls oracle's truth and fathom's pairs over the same files,
// as the L2 scoring compared them.
type callDiff struct {
	oracle      string
	truth, mine map[langbench.CallPair]bool
}

// edgesFromIndexed keeps the edges whose file the SCIP index covers: the
// index proves no edge from a document it does not hold (scip-php skips a
// test file no autoload root maps), so such an edge is out of scope (§1.4),
// counted in a note, never scored as fathom's error (LB-F65).
func edgesFromIndexed(edges []core.Edge, docs map[string]bool, name string, notes *[]string) []core.Edge {
	out := edges[:0:0]
	dropped := 0
	for _, e := range edges {
		if docs[e.FromFile] {
			out = append(out, e)
		} else {
			dropped++
		}
	}
	if dropped > 0 {
		*notes = append(*notes, fmt.Sprintf("%s: %d file-resolved edges from files the SCIP index does not hold are out of the file-edge scope (§1.4)", name, dropped))
	}
	return out
}

// pairsOnlyIn returns the pairs of a absent from b, sorted.
func pairsOnlyIn(a, b map[langbench.CallPair]bool) []unmatchedPair {
	out := []unmatchedPair{}
	for p := range a {
		if !b[p] {
			out = append(out, unmatchedPair{p.Path, p.Caller, p.Callee})
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

// onlyMatch reports whether rel is inside the -only restriction.
func onlyMatch(globs []string, rel string) bool {
	if len(globs) == 0 {
		return true
	}
	for _, g := range globs {
		if langbench.GlobMatch(g, rel) {
			return true
		}
	}
	return false
}

// ownedSourceRefs are the subject's files the dialect is measured on: those
// its manifest owns by basename, glob or extension (inside -only), plus, for
// C++ and C, the polyglot .h headers fusion's header router sends to each
// (the C++ sniff says yes: C++; no: C). No dialect claims .h (design §5 leaves it to that router, which
// sniffs the source), so without this a C++ library's headers — most of fmt —
// would be measured as nothing. contentDialect, when non-nil, names the
// dialect ingest's dispatch reads a file with by its content alone
// (fusion.Pipeline.ContentDialect): a doc-engine dialect that claims no name
// (LB-Q-cloudformation: a template is a .yaml, .yml or .json whose content
// says so) is measured on exactly the files ingest sends it, and so is one
// that claims names too: a file no name claims is still its own when ingest
// claims it by content (LB-Q-prometheus: a rule file named alerts.yaml, read
// through the late-YAML hub). ContentDialect names only content-claimed
// dialects, so a dialect claimed by names alone owns nothing more. A file
// with no extension is the dialect's when ingest reads it with that dialect by
// its shebang (D4, O756: fusion.Pipeline.ShebangDialect, through
// contentDialect), so bats-core's bin/bats counts for bash.
func ownedSourceRefs(m langbench.Manifest, root string, refs []core.SourceRef, only []string, contentDialect func(rel string, content []byte) string) []core.SourceRef {
	byContent := contentDialect != nil && m.Engine == "doc" && m.Match.Sniff == nil
	var variantSniff *regexp.Regexp // a reading of the host's content-selected variant (Tailwind CSS v4 in css)
	if m.HostVariant != "" {
		variantSniff, _ = extractor.VariantSniff(m.HostID(), m.HostVariant)
	}
	var out []core.SourceRef
	for _, ref := range refs {
		if !onlyMatch(only, ref.Path) {
			continue
		}
		if m.Owns(ref.Path) {
			if variantSniff != nil {
				b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ref.Path)))
				if err != nil {
					continue
				}
				if len(b) > extractor.VariantSniffWindow {
					b = b[:extractor.VariantSniffWindow]
				}
				if !variantSniff.Match(b) {
					continue
				}
			}
			out = append(out, ref)
			continue
		}
		if contentDialect != nil && filepath.Ext(ref.Path) == "" {
			if b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ref.Path))); err == nil && contentDialect(ref.Path, b) == m.ID {
				out = append(out, ref)
			}
			continue
		}
		if byContent {
			switch strings.ToLower(filepath.Ext(ref.Path)) {
			case ".yaml", ".yml", ".json":
				if b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ref.Path))); err == nil && contentDialect(ref.Path, b) == m.ID {
					out = append(out, ref)
				}
			}
			continue
		}
		if (m.ID == "cpp" || m.ID == "c" || m.ID == "objective-c") && strings.EqualFold(filepath.Ext(ref.Path), ".h") {
			// The router sends a .h to Objective-C on an @interface,
			// @protocol, @implementation or #import, to C++ on a C++-only
			// signal and to C otherwise (LB-F80: a C include's target is a
			// header, so without them C's import_resolution and file edges
			// have no denominator).
			if b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ref.Path))); err == nil && extractor.HeaderDialect(b) == m.ID {
				out = append(out, ref)
			}
		}
	}
	return out
}

// contentDialectOf is ownedSourceRefs' contentDialect: the doc-engine dialect
// ingest's dispatch reads a file with by its content alone, a CloudFormation
// or OpenAPI document (Pipeline.ContentDialect) or, wave pc, an IAM policy,
// an Azure Policy definition or a Cloud Custodian file
// (Pipeline.CloudPolicyDialect); and the dialect ingest reads a script with no
// extension with by its shebang (Pipeline.ShebangDialect, D4).
func contentDialectOf(pipe *fusion.Pipeline, refs []core.SourceRef) func(rel string, content []byte) string {
	return func(rel string, content []byte) string {
		if d := pipe.ShebangDialect(rel, content); d != "" {
			return d // a script with no extension, by its shebang (D4, O756)
		}
		if d := pipe.ContentDialect(rel, content, refs); d != "" {
			return d
		}
		if d := pipe.CloudPolicyDialect(rel, content, refs); d != "" {
			return d
		}
		if d := fusion.AppFrameworkContentDialect(rel, content); d != "" {
			return d // an ARM template (LB-F146)
		}
		return fusion.JSONSchemaContentDialect(rel, content) // a JSON Schema document (LB-Q-json-schema)
	}
}

// nativeFilesWithError is a native reader's own parse-error count, §14.2's
// line for a dialect no grammar reads: the measured Rego files the reader
// read only by a repair (an unclosed bracket or string, a stray closer;
// extractor.RegoRepairs), which ADR-048's wave-pc amendment holds under 5%.
// ok is false for every other dialect (none counts its own repairs).
func nativeFilesWithError(dialect string, files []core.SourceFile) (n int, ok bool) {
	if dialect != "rego" {
		return 0, false
	}
	for _, f := range files {
		if extractor.RegoRepairs(f.Content) > 0 {
			n++
		}
	}
	return n, true
}

// measureSubject measures one subject. with, when non-nil, extracts every
// file the dialect owns instead of fusion's dispatch (lang-parity's dialect
// side, §11); nil measures what ingest would run.
func measureSubject(ctx context.Context, spec langbench.BenchSpec, root, name string, opts oracleOpts, with extractor.Extractor) (subjectMeasurement, error) {
	res := subjectMeasurement{kinds: map[string]int{}}
	extract := func(pipe *fusion.Pipeline, service string, refs []core.SourceRef, rel string, content []byte) (fusion.BenchExtraction, error) {
		if with != nil {
			return pipe.ExtractForBenchWith(with, service, refs, rel, content)
		}
		return pipe.ExtractForBench(service, refs, rel, content)
	}
	const service = "langbench"
	refs, err := connector.NewLocalConnector().Discover(ctx, core.ServiceRef{ID: service, Path: root})
	if err != nil {
		return res, fmt.Errorf("discover %s: %w", root, err)
	}
	pipe := fusion.New(nil, nil, fusion.Config{})
	defer pipe.Close()
	tsengine.ResetPeakModulePages()

	sha, err := subjectSHA(ctx, root, opts.subjectSHA)
	if err != nil {
		return res, err
	}
	if sha == "" && opts.requireSHA && !allowUnpinnedSubject {
		// round 4 C4-R7: a reading names the commit it measured
		return res, fmt.Errorf("subject %s has no commit of its own (a copy with no .git, or a directory inside another repository): pass -subject-sha", name)
	}
	sub := langbench.Subject{Name: name, SHA: sha, Counts: map[string]int{},
		Families: map[string]string{}, ExcludedByKind: map[string]int{}, Tenancy: &langbench.SubjectTenancy{FaultingFiles: []string{}}}
	var files []string
	var nodes []core.SymbolNode
	var srcFiles []core.SourceFile
	skipped := map[string]int{}
	retimed := 0
	// Warm-up: one untimed extraction compiles the dialect's grammar, so
	// per-file times measure extraction, not the one-time lazy compile
	// (design §9 charges that to the module pool, LB-F15).
	ownedRefs := ownedSourceRefs(spec.Manifest, root, refs, opts.only, contentDialectOf(pipe, refs))
	if len(opts.only) > 0 {
		res.notes = append(res.notes, onlyScopeNote(name, opts.only))
	}
	for _, ref := range ownedRefs {
		if content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ref.Path))); err == nil {
			if ex, _ := extract(pipe, service, refs, ref.Path, content); ex.Skipped == "" {
				res.notes = append(res.notes, "per-file times exclude the one-time grammar compile (an untimed warm-up extraction precedes timing)")
				break
			}
		}
	}
	for _, ref := range ownedRefs {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ref.Path)))
		if err != nil {
			return res, err
		}
		inst := tsengine.Instantiations()
		start, cpu0 := fileClocks.wall(), fileClocks.cpu()
		ex, err := extract(pipe, service, refs, ref.Path, content)
		elapsed, cpu := fileClocks.wall().Sub(start), fileClocks.cpu()-cpu0
		if tsengine.Instantiations() != inst && ex.Skipped == "" {
			// This file instantiated a grammar module the warm-up did not
			// (an injected language's, e.g. HTML's <script>): time it again,
			// warm, so a one-time compile is never charged to throughput.
			start, cpu0 = fileClocks.wall(), fileClocks.cpu()
			ex, err = extract(pipe, service, refs, ref.Path, content)
			elapsed, cpu = fileClocks.wall().Sub(start), fileClocks.cpu()-cpu0
			retimed++
		}
		if ex.Skipped != "" {
			skipped[ex.Skipped]++
			continue
		}
		files = append(files, ref.Path)
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
		if ex.TypeRefs > 0 {
			sub.Counts["type_refs"] += ex.TypeRefs // §3.4: counted, not stored
		}
		for _, n := range ex.Nodes {
			if n.FilePath == ref.Path {
				nodes = append(nodes, n)
				res.kinds[string(n.Kind)]++
			}
		}
		srcFiles = append(srcFiles, core.SourceFile{Path: ref.Path, Content: content})
	}
	if len(files) == 0 && !opts.noOracles {
		return res, fmt.Errorf("subject %s has no %s files fathom would extract", root, spec.Manifest.ID)
	}
	sub.Tenancy.PeakModulePages = int(tsengine.PeakModulePages())
	// LB-F59 (O203): the measured files with a parse error on the dialect's
	// own grammar, counted after timing so throughput never pays for it. A
	// doc engine parses no tree-sitter grammar and reports none; a native
	// reader with a repair count of its own reports that (Rego).
	if n, ok := nativeFilesWithError(spec.Manifest.ID, srcFiles); ok {
		sub.Tenancy.FilesWithError = &n
	} else if spec.Manifest.Engine == "tags" {
		n, unparsed, asWritten, err := filesWithError(ctx, spec.Manifest.HostID(), root, files)
		switch {
		case err != nil:
			res.notes = append(res.notes, fmt.Sprintf("%s: files_with_error not measured: %v", name, err))
		default:
			sub.Tenancy.FilesWithError = &n
			if asWritten > n {
				res.notes = append(res.notes, fmt.Sprintf("%s: %d of %d measured files hold an ERROR or MISSING node as written; the dialect's pre-parse step reads %d of them clean, and files_with_error counts the %d it cannot mend, the files the extractor reads with an error", name, asWritten, len(files), asWritten-n, n))
			}
			if unparsed > 0 {
				res.notes = append(res.notes, fmt.Sprintf("%s: %d files the parser failed on are not counted in files_with_error", name, unparsed))
			}
		}
	}
	yamls, err := subjectYAML(spec.Manifest.ID, root, refs)
	if err != nil {
		return res, err
	}
	res.notes = append(res.notes, iacCountNotes(spec.Manifest.ID, name, srcFiles, yamls)...)
	if retimed > 0 {
		res.notes = append(res.notes, "a file whose extraction instantiated a grammar module after the warm-up (an injected language) is timed on a second, warm extraction")
	}
	for _, k := range sortedKeys(skipped) {
		res.notes = append(res.notes, fmt.Sprintf("%s: %d files skipped by ingest's gates (%s)", name, skipped[k], k))
	}

	if opts.noOracles {
		// Fathom-only count measures: type_refs (above), fathom_defs,
		// callables, callables_signed, and excluded_by_kind's fathom:* keys.
		// L1 uses the recorded kind intersection, never an oracle file.
		// If imports were measured: imports, imports_external, and
		// import_edges_fathom. Other counts need oracle scope, truth, or
		// matching (xref_fathom also needs the oracle's file/entity grain).
		// File/byte totals and parser faults/errors are in Subject/Tenancy above.
		for _, n := range nodes {
			canonical, callable, ok := spec.Manifest.CanonicalForSource(string(n.Kind))
			if !ok || !slices.Contains(opts.retimeKinds, canonical) {
				sub.ExcludedByKind["fathom:"+string(n.Kind)]++
				continue
			}
			sub.Counts[langbench.CntFathomDefs]++
			if callable {
				sub.Counts[langbench.CntCallables]++
				if strings.TrimSpace(n.Signature) != "" {
					sub.Counts[langbench.CntCallablesSigned]++
				}
			}
		}
		if opts.retimeImports {
			inScope := map[string]bool{}
			for _, f := range files {
				inScope[f] = true
			}
			imps := subjectImports{ctx: ctx, dialect: spec.Manifest.HostID(), root: root, name: name, files: srcFiles, nodes: nodes, inScope: inScope}
			if !imps.resolve() {
				return res, fmt.Errorf("%s", strings.Join(imps.notes, "; "))
			}
			counts := map[string]int{}
			langbench.ScoreImports(nil, imps.sites, inScope, counts)
			for _, key := range []string{langbench.CntImports, langbench.CntImportsExternal, langbench.CntImportEdgesFathom} {
				sub.Counts[key] = counts[key]
			}
		}
		res.subject = sub
		return res, nil
	}

	var l1res *langbench.L1Result // the subject's one L1 scoring, for D3
	var calls *callDiff           // the subject's one L2 scoring, for -unmatched
	var fileEdges *fileEdgeDiff   // the subject's one file-edge scoring, for -unmatched
	// Tier B: universal-ctags over exactly the files fathom read.
	ctagsRun := langbench.OracleRun{Tier: "B", Tool: "universal-ctags"}
	switch bin, version, why := findCtags(ctx, opts.ctags); {
	case opts.scipDefs || toolsCarryDefinitions(opts.tools):
		// D2: a stronger oracle (the SCIP index, or an oracle file's
		// definitions) is this subject's one L1 oracle.
		ctagsRun = langbench.OracleRun{}
	case len(spec.Ctags.Languages) == 0:
		ctagsRun.Status = "unavailable: universal-ctags has no parser for this dialect"
	case why != "":
		ctagsRun.Status = "unavailable: " + why
	default:
		ctagsRun.Version = version
		defs, err := runCtags(ctx, bin, root, files, spec.Ctags)
		if err != nil {
			return res, err
		}
		l1 := langbench.ScoreL1(spec.Manifest, "ctags", nodes, defs)
		l1res = &l1
		addCounts(sub.Counts, l1.Counts)
		addCounts(sub.ExcludedByKind, l1.Excluded)
		sub.KindIntersection = l1.Intersection
		sub.Families[langbench.FamilyL1] = "B"
	}
	if ctagsRun.Tool != "" {
		res.oracles = append(res.oracles, ctagsRun)
	}
	inScope := map[string]bool{}
	for _, f := range files {
		inScope[f] = true
	}
	// LB-F40: a language's imports, resolved once and scored against the
	// SCIP index or a neutral file's imports (langoracle_imports.go).
	var imps *subjectImports
	if spec.Manifest.Kind != "tooling" {
		imps = &subjectImports{ctx: ctx, dialect: spec.Manifest.HostID(), root: root, name: name, files: srcFiles, nodes: nodes, inScope: inScope}
	}

	if opts.scip != "" {
		raw, err := os.ReadFile(opts.scip)
		if err != nil {
			return res, err
		}
		// LB-F80: scip-clang indexes one build configuration, so what fathom is
		// scored on (its L1 definitions, the callers of its L2 calls) is its
		// definitions in files the compilation database compiles, on lines that
		// configuration's preprocessor keeps; the rest are disclosed.
		scoped := nodes
		clangIndex := false
		indexTier := orDefault(opts.scipTier, "A") // round 3 A2: the index's tier is an input
		var active map[string][][2]int             // the build's kept lines (LB-F80), when an oracle gives them
		leftOut := "the callers L2 scores"
		if opts.scipDefs {
			leftOut = "L1 and of the callers L2 scores"
		}
		if docs, tool, err := langbench.SCIPDocuments(raw); err != nil {
			return res, err
		} else if tool == scipClangTool {
			clangIndex = true
			scoped = nil
			for _, n := range nodes {
				if docs[n.FilePath] {
					scoped = append(scoped, n)
				}
			}
			if n := len(nodes) - len(scoped); n > 0 {
				res.notes = append(res.notes, fmt.Sprintf("%s: %d of fathom's definitions are in %s (the compilation database compiles no translation unit for them: another platform's tree, an example) and are left out of %s", name, n, noteScipClangDefsScoped, leftOut))
			}
			if active, err = activeLinesOf(opts.tools); err != nil {
				return res, err
			}
			if len(active) > 0 {
				kept := scoped[:0:0]
				for _, n := range scoped {
					if lines, ok := active[n.FilePath]; ok && !lineActive(lines, n.Line) {
						continue
					}
					kept = append(kept, n)
				}
				if n := len(scoped) - len(kept); n > 0 {
					res.notes = append(res.notes, fmt.Sprintf("%s: %d of fathom's definitions are %s and are left out of %s", name, n, noteScipClangInactive, leftOut))
				}
				scoped = kept
			}
		}
		if opts.scipDefs {
			all, _, _, err := langbench.SCIPDefs(raw, func(p string) []byte {
				b, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
				return b
			})
			if err != nil {
				return res, err
			}
			var defs []langbench.OracleDef
			for _, d := range all {
				if inScope[d.Path] {
					defs = append(defs, d)
				}
			}
			l1 := langbench.ScoreL1(spec.Manifest, "scip", scoped, defs)
			l1res = &l1
			if n := l1.Excluded["scip:macro-generated"]; n > 0 && clangIndex {
				res.notes = append(res.notes, fmt.Sprintf("%s: %d scip-clang definitions are written by a macro (libuv's TEST_IMPL(loop_stop) is the function run_test_loop_stop, reported over the macro's name): no extractor of definitions sees them, so they are left out of L1 (scip:macro-generated) and disclosed", name, n))
			}
			addCounts(sub.Counts, l1.Counts)
			addCounts(sub.ExcludedByKind, l1.Excluded)
			sub.KindIntersection = l1.Intersection
			sub.Families[langbench.FamilyL1] = indexTier
		}
		scipOpts := langbench.SCIPOptionsFor(spec.Manifest.ID, func(p string) []byte {
			b, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
			return b
		})
		// Ruling C-R6 (F5): a family the index grades is judged at the weakest
		// tier of the oracle files that also feed it (the call sites and active
		// lines below), not at the index's own tier.
		// Round 3 A2 (F2): each starts at the index's own tier.
		callsTier, defMapTier, fileEdgesTier := indexTier, indexTier, indexTier
		if clangIndex {
			if callsTier, err = feedTierFrom(indexTier, opts.tools, "call_sites", "active_lines"); err != nil {
				return res, err
			}
			if defMapTier, err = feedTierFrom(indexTier, opts.tools, "active_lines"); err != nil {
				return res, err
			}
			if fileEdgesTier, err = feedTierFrom(indexTier, opts.tools, "active_lines"); err != nil { // a dump's imports grade import_resolution, at its own tier
				return res, err
			}
			if err := scipOpts.ClangLinkage.Read(opts.tools, rangeCompanionFile); err != nil { // owner ruling M3 (O863): the oracles' linkage and includes
				return res, err
			}
			// round 4 C4-R3: the ranges and link-unit companions shape the
			// file-edge truth, and the CUDA stand-ins (the ranges') the calls
			fileEdgesTier = scipOpts.ClangLinkage.FeedTier(fileEdgesTier)
			if scipOpts.ClangCUDA {
				callsTier = scipOpts.ClangLinkage.RangesTier(callsTier)
			}
			// LB-F116: the build's own compiler's call sites, when an oracle
			// file carries them, settle what the index cannot.
			if scipOpts.ClangSites, err = clangSitesOf(opts.tools); err != nil {
				return res, err
			}
			if len(active) > 0 {
				scipOpts.ActiveLines = active
			}
			if scipOpts.ClangDefNames {
				scipOpts.ClangScope = inScope // LB-F292: L4 over the files the reading measures
			}
		}
		if scipOpts.DefScoped {
			scipOpts.DefScope = inScope // ruling L5 (LB-F467): def_mapping over the files the reading measures
		}
		if spec.Manifest.ID == "csharp" {
			// LB-F287: the C# compiler's semantic model, when an oracle file
			// carries its call sites (langoracle_csharp.go).
			if scipOpts.CSharp, err = csharpCompilerOf(opts.tools); err != nil {
				return res, err
			}
		}
		sr, err := langbench.ScoreSCIPWith(raw, service, scoped, scipOpts)
		if err != nil {
			return res, err
		}
		if n := langbench.DefScopeNote(name, sr.Counts); n != "" && scipOpts.DefScoped {
			res.notes = append(res.notes, n) // ruling L5 (LB-F467)
		}
		if opts.callsFromTool {
			// LB-F204: the tool oracle's calls are L2's truth; the index's call
			// truth and what its call-site rules counted are not part of this
			// reading (they would grade a different set of pairs).
			for _, k := range []string{langbench.CntCallTruth, langbench.CntCallTruthFound, langbench.CntCallFathom, langbench.CntCallFathomInTrue,
				langbench.CntScipCallsUnwritten, langbench.CntScipCalledNames, langbench.CntCallsOracleBlind} {
				delete(sr.Counts, k)
			}
			res.notes = append(res.notes, name+": L2 is graded by the -tool-oracle's calls (-calls-from tool), not by the index's call sites: the index grades definitions, typed edges and imports")
		}
		if spec.Manifest.ID == "csharp" {
			res.notes = append(res.notes, csharpSiteNotes(name, sr.Counts, scipOpts.CSharp)...) // LB-F286, LB-F287
		} else {
			siteTier, err := feedTierFrom(indexTier, opts.tools, "call_sites")
			if err != nil {
				return res, err
			}
			res.notes = append(res.notes, clangSiteNotes(name, spec.Manifest.ID, siteTier, sr.Counts)...)
		}
		if n := sr.Counts[langbench.CntScipCallsUnwritten]; n > 0 {
			res.notes = append(res.notes, fmt.Sprintf("%s: %d callable references have no call written at their range (%s) and are left out of the call truth (L2 is call sites)", name, n, unwrittenWhy(spec.Manifest.ID)))
		}
		if n := sr.Counts[langbench.CntScipCalledNames]; n > 0 {
			why := "a reference the producer gives no callable descriptor"
			switch spec.Manifest.ID {
			case "python":
				why = "scip-python names a constructed class (`Foo#`), never a constructor, and an attribute or local it calls by its own descriptor"
			case goDialect:
				why = "scip-go writes an interface's method and a func-typed field or variable as a term (`T#m.`)"
			case "typescript", "javascript":
				why = "scip-typescript names a class with no constructor of its own (`new Foo(`), an arrow-function const, a function-typed property, a parameter or a local by its own descriptor, each named by what is written"
			}
			res.notes = append(res.notes, fmt.Sprintf("%s: %d references to a class, a field or variable, an interface method or a local are written as a call (`name(`) and counted as calls to that name (L2 is call sites: %s)", name, n, why))
		}
		if n := sr.Counts[langbench.CntCallsOracleBlind]; n > 0 {
			res.notes = append(res.notes, fmt.Sprintf("%s: %d of fathom's call pairs the truth lacks have no occurrence of the index at any call site written for them (`super(...)`, a member of a receiver the producer types as any): calls the oracle cannot judge, left out of both counts (LB-F175, design §1.4 amendment 2026-10-02)", name, n))
		}
		res.notes = append(res.notes, langbench.SCIPRuleNotes(name, sr.Tool, sr.Counts)...) // K25 ruling 3, K18 (LB-F387, LB-F389)
		// Owner ruling M3 (O863): C's file-local copies, disclosed beside ruling 3.
		res.notes = append(res.notes, scipOpts.ClangLinkage.Finish(name, sr.FileEdges, sr.Counts)...)
		if sr.EnclosingFallback > 0 {
			res.notes = append(res.notes, fmt.Sprintf("%s: %d SCIP definitions carried no enclosing_range; each took its mapped fathom definition's line span so call truth stays measurable (D5)", name, sr.EnclosingFallback))
		}
		addCounts(sub.Counts, sr.Counts)
		if !opts.callsFromTool {
			calls = &callDiff{oracle: orDefault(sr.Tool, "scip"), truth: sr.CallTruth, mine: sr.CallFound}
			sub.Families[langbench.FamilyCalls] = callsTier
		}
		sub.Families[langbench.FamilyDefMap] = defMapTier
		sub.Families[langbench.FamilyTyped] = defMapTier
		if lang, ok := fileResolveLang[spec.Manifest.HostID()]; ok {
			edges, rep := fusion.ResolveFileEdges(ctx, lang, root, srcFiles, nodes)
			if rep.Skipped {
				res.notes = append(res.notes, name+": file-resolved edges not measured: "+rep.SkipReason)
			} else {
				edges = edgesFromIndexed(edges, sr.Docs, name, &res.notes)
				edges = scopeFileEdgesNote(edges, nodes, active, name, &res.notes)
				truth := scopedFileEdgeTruth(sr.FileEdges, inScope)
				edgeNodes := nodes
				if len(rep.Borrowed) > 0 {
					// Round 3 F5: a header borrower's reached headers are measured as
					// the targets of its files' edges (the build's preprocessor says
					// which its includes reach), so its header edges are scored.
					reached := scipOpts.ClangLinkage.IncludeClosure(inScope)
					edgeNodes = append(append([]core.SymbolNode(nil), nodes...), rep.Borrowed...)
					truth = scopedFileEdgeTruthTo(sr.FileEdges, inScope, reached)
					res.notes = append(res.notes, fmt.Sprintf("%s: %d headers the measured files' includes reach (by the build's preprocessor oracle) are measured as file-edge targets, the definitions fathom binds there borrowed as ingest binds them (ruling C-R4); an edge from a header is not measured", name, len(reached)))
				}
				if n := len(sr.FileEdges) - len(truth); n > 0 {
					res.notes = append(res.notes, fmt.Sprintf("%s: %d index %s", name, n, noteFileEdgeTruthScoped))
				}
				var blindPairs [][2]string
				if scipOpts.FileEdgesOracleBlind {
					// LB-F296: the pairs the index cannot judge (langoracle_clang.go).
					if edges, blindPairs, err = fileEdgesOracleBlind(raw, truth, edges, edgeNodes, scipOpts.Source, name, sub.Counts, &res.notes); err != nil {
						return res, err
					}
				}
				langbench.ScoreFileEdges(truth, edges, edgeNodes, sub.Counts)
				fileEdges = &fileEdgeDiff{truth: truth, mine: fileEdgePairs(edges, edgeNodes), via: fileEdgeVia(edges, edgeNodes), blind: blindPairs}
				fileEdges.by = fileEdgeBy(edges, edgeNodes)
				sub.Families[langbench.FamilyFileEdges] = fileEdgesTier
			}
		} else if spec.Manifest.ID == goDialect {
			// LB-F58: Go's cross-file edges are the typed pass's.
			edges, rep, err := goTypedEdges(ctx, service, root, inScope)
			if err != nil {
				res.notes = append(res.notes, name+": file edges not measured: "+err.Error())
			} else {
				// §1.6: Go's L2 grades the calls the product serves.
				if gc := scoreGoCalls(name, nodes, edges, sr, sub.Counts); gc != nil {
					calls = gc.diff
					res.notes = append(res.notes, gc.notes...)
				}
				edges = edgesFromIndexed(edges, sr.Docs, name, &res.notes)
				res.notes = append(res.notes, goFileEdgeClasses(name, edges, nodes, sr.FileEdges), goNativeCoverage(name, sr.ImportEdges, edges))
				res.notes = append(res.notes, goSCIPNotes(name, sr.Counts)...)
				allPairs := len(fileEdgePairs(edges, nodes))
				var left map[[2]string]bool
				edges, left = goNamedEdges(edges, nodes) // LB-F297
				res.notes = append(res.notes, goLeftOutNote(name, left, sr.FileEdges, allPairs))
				langbench.ScoreFileEdges(sr.FileEdges, edges, nodes, sub.Counts)
				fileEdges = &fileEdgeDiff{truth: sr.FileEdges, mine: fileEdgePairs(edges, nodes), via: fileEdgeVia(edges, nodes), kinds: fileEdgeKinds(edges, nodes)}
				fileEdges.by = fileEdgeBy(edges, nodes)
				sub.Families[langbench.FamilyFileEdges] = indexTier
				res.notes = append(res.notes, noteGoTypedFileEdges)
				if rep.PackagesFailed > 0 {
					res.notes = append(res.notes, fmt.Sprintf("%s: %d of %d packages failed the typed pass and give no edge", name, rep.PackagesFailed, rep.PackagesTotal))
				}
			}
		}
		res.oracles = append(res.oracles, langbench.OracleRun{Tier: indexTier, Tool: orDefault(sr.Tool, "scip"), Version: sr.Version})
		if imps != nil {
			if sr.Tool == scipClangTool {
				// LB-F80: scip-clang records no reference on an `#include` line
				// and gives no occurrence the Import role, so the references it
				// does carry coincide with an include's line only by accident:
				// import_resolution is a dump's (clang-includes), never the index's.
				res.notes = append(res.notes, scipClangNoImportsNote(fileEdgesTier))
			} else if err := imps.score(sr.ImportRefs, indexTier, orDefault(sr.Tool, "scip"), &sub); err != nil {
				return res, err
			}
		}
	}

	// LB-F56: a tool format the doc engine does not extract takes its
	// cross-file references from the file resolver (langoracle_xref.go).
	var fx *fileXrefs
	if _, isDoc := with.(extractor.DocResulter); !isDoc && spec.Manifest.Kind == "tooling" {
		fx = &fileXrefs{ctx: ctx, m: spec.Manifest, root: root, name: name, files: srcFiles, nodes: nodes}
	}
	for _, toolOracle := range opts.tools {
		run, dropped, l1, cd, err := scoreToolOracle(toolOracle, spec.Manifest, nodes, &sub, inScope)
		if err != nil {
			return res, err
		}
		if b, err := os.ReadFile(toolOracle); err == nil {
			var tf toolOracleFile
			if json.Unmarshal(b, &tf) == nil {
				res.notes = append(res.notes, toolOracleDisclosures(spec.Manifest, name, tf)...) // wave lf5: the oracle's own counts
				if len(tf.Scope) > 0 {
					in, _ := toolOracleScope(tf.Scope, inScope, nil)
					res.notes = append(res.notes, fmt.Sprintf(noteToolOracleScope, name, tf.Tool, len(in), len(inScope)))
				}
			}
		}
		if l1 != nil {
			l1res = l1
		}
		if cd != nil {
			calls = cd
		}
		if dropped > 0 {
			res.notes = append(res.notes, fmt.Sprintf("%s: %d tool-oracle entries on files fathom does not read (ingest's gates, discovery's exclusions) were left out of scope (§1.4)", name, dropped))
		}
		res.oracles = append(res.oracles, run)
		if spec.Manifest.LangTag == "css" && cd != nil {
			res.notes = append(res.notes, cssNameScopeNotes(name, run.Tool, sub.Counts)...) // design 1.8 W2 (LB-F460)
		}
		if imps != nil {
			if err := scoreImportsOracle(toolOracle, imps, &sub); err != nil {
				return res, err
			}
		}
		// LB-F57: no SCIP index, so a dump's imports are the file-edge truth.
		if lang, ok := fileResolveLang[spec.Manifest.HostID()]; ok && opts.scip == "" && spec.Manifest.Kind != "tooling" {
			notes, diff, err := scoreImportFileEdges(ctx, toolOracle, lang, root, name, srcFiles, nodes, &sub, inScope)
			if err != nil {
				return res, err
			}
			res.notes = append(res.notes, notes...)
			if diff != nil {
				fileEdges = diff
			}
		}
		// ── LB-F30: cross-file references (langoracle_xref.go) ──
		if doc, ok := with.(extractor.DocResulter); ok {
			var resolved []langbench.XrefPair // LB-F583: a native doc dialect the file resolver also serves
			if lang, ok := fileResolveLang[spec.Manifest.ID]; ok && spec.Manifest.Engine == "native" {
				resolved = fileResolvedXrefPairs(ctx, lang, root, srcFiles, nodes)
			}
			// A tags+doc dialect (Conan, LB-Q-conan) has its file resolver's edges beside the doc pass's:
			// ingest stores both, so both score.
			scoreResolved := false
			if lang, ok := importLang(spec.Manifest.ID); ok && spec.Manifest.Engine == "tags+doc" {
				resolved = fileResolvedXrefPairs(ctx, lang, root, srcFiles, nodes)
				scoreResolved = resolved != nil
			}
			n, note, err := scoreXrefOracle(toolOracle, doc, service, srcFiles, lambdaCode(pipe, service, root, refs), resolved, scoreResolved, &sub, inScope)
			if err != nil {
				return res, err
			}
			if note != "" {
				res.notes = append(res.notes, name+": "+note)
			}
			if n > 0 {
				res.notes = append(res.notes, fmt.Sprintf("%s: %d oracle cross-references on files fathom does not read were left out of scope (§1.4)", name, n))
			}
		} else if fx != nil {
			n, err := scoreFileXrefOracle(toolOracle, fx, &sub, inScope)
			if err != nil {
				return res, err
			}
			if n > 0 {
				res.notes = append(res.notes, fmt.Sprintf("%s: %d oracle cross-references on files fathom does not read were left out of scope (§1.4)", name, n))
			}
		}
		// ── end LB-F30 ──
	}
	for _, n := range opts.baseNotes {
		res.notes = append(res.notes, name+": "+n)
	}
	if fx != nil {
		res.notes = append(res.notes, fx.notes...)
	}
	if imps != nil {
		res.notes = append(res.notes, imps.notes...)
	}
	if opts.unmatched != "" && (l1res != nil || calls != nil || fileEdges != nil || (imps != nil && len(imps.misses) > 0)) {
		var u unmatchedFile
		if l1res != nil {
			u.DialectOnly, u.OracleOnly = &l1res.DialectOnly, &l1res.OracleOnly
		}
		if calls != nil {
			u.Calls = &unmatchedCalls{Oracle: calls.oracle, FathomOnly: pairsOnlyIn(calls.mine, calls.truth), OracleOnly: pairsOnlyIn(calls.truth, calls.mine)}
		}
		if imps != nil && len(imps.misses) > 0 {
			u.Imports = imps.misses
		}
		if fileEdges != nil {
			u.FileEdges = &unmatchedFileEdges{FathomOnly: edgesOnlyIn(fileEdges.mine, fileEdges.truth), OracleOnly: edgesOnlyIn(fileEdges.truth, fileEdges.mine), OracleBlind: fileEdges.blind}
			for _, k := range u.FileEdges.FathomOnly {
				u.FileEdges.Via = append(u.FileEdges.Via, fileEdges.via[k])
				u.FileEdges.By = append(u.FileEdges.By, fileEdges.by[k])
				if fileEdges.kinds != nil {
					u.FileEdges.Kinds = append(u.FileEdges.Kinds, fileEdges.kinds[k])
				}
			}
		}
		b, err := json.MarshalIndent(u, "", " ")
		if err != nil {
			return res, err
		}
		if err := os.WriteFile(opts.unmatched, b, 0o644); err != nil {
			return res, err
		}
	}
	if opts.adjudicate != "" {
		if l1res == nil {
			return res, fmt.Errorf("-adjudicate needs an L1 oracle on this subject")
		}
		b, err := os.ReadFile(opts.adjudicate)
		if err != nil {
			return res, err
		}
		var adj langbench.Adjudication
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&adj); err != nil {
			return res, fmt.Errorf("%s: %w", opts.adjudicate, err)
		}
		before := l1res.Counts[langbench.CntAdjudicated]
		accepted, refused := langbench.Adjudicate(l1res, nodes, adj, func(p string) []byte {
			b, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
			return b
		})
		sub.Counts[langbench.CntAdjudicated] += l1res.Counts[langbench.CntAdjudicated] - before
		sub.Adjudicated = accepted
		if len(refused) > 0 {
			res.notes = append(res.notes, fmt.Sprintf("%s: %d adjudication claims refused by the deterministic check (e.g. %s)", name, len(refused), refused[0]))
		}
	}
	res.subject = sub
	return res, nil
}

// toolOracleFile is a tool-native oracle's output, converted by the worker
// (design §13.4 step 19) into this neutral shape.
type toolOracleFile struct {
	Tool    string `json:"tool"`
	Version string `json:"version"`
	Tier    string `json:"tier"`
	// Source, Revision and Licence are a third-party query's provenance
	// (ADR-048's D5 extension, LB-F52), copied into the reading's oracle run.
	Source      string `json:"source,omitempty"`
	Revision    string `json:"revision,omitempty"`
	Licence     string `json:"licence,omitempty"`
	Definitions []struct {
		Path string `json:"path"`
		Name string `json:"name"`
		Line int    `json:"line"`
		End  int    `json:"end"`
		Kind string `json:"kind"`
	} `json:"definitions"`
	References []oraclePair `json:"references"`
	// ActiveLines (LB-F80) are the source lines the build's own preprocessor
	// keeps in each file, as [first, last] ranges: scope for L1 against a
	// SCIP index of one build configuration (scip-clang's).
	ActiveLines map[string][][2]int `json:"active_lines,omitempty"`
	// Calls are a language's call pairs (§1.4: enclosing definition's
	// name, callee's name), scored as call_recall/call_precision at the
	// file's tier — e.g. tier C from the grammar's upstream tags.scm run
	// by an independent runtime (owner decision D5).
	Calls []oraclePair `json:"calls"`
	// CallSites (LB-F116) are the build's own compiler's call sites
	// (calibration/oracles/clang-calls), read beside a scip-clang index
	// (clangSitesOf).
	CallSites []clangCallSite `json:"call_sites,omitempty"`
	// Scope (LB-Q-hlsl, H5-R4) is the measured files the oracle reads, when
	// it reads only part of them (a compiler front end that refuses some
	// units): its definitions, calls and references are graded on those
	// files alone, so a file outside is neither a recall nor a precision
	// miss, while the run (tenancy, file edges) still covers every file.
	Scope []string `json:"scope,omitempty"`
	// QueryClauses (LB-F287) count the LINQ query clauses the C# compiler binds
	// (calibration/oracles/roslyn calls mode), disclosed beside its call sites.
	QueryClauses int `json:"query_clauses,omitempty"`
	// CSharpFileEdges and CSharpMultiDeclared (LB-F287) are the C# compiler's
	// cross-file edges and its count of names bound to a symbol two files
	// declare (calibration/oracles/roslyn calls mode), read by
	// csharpCompilerOf.
	CSharpFileEdges     [][2]string `json:"file_edges,omitempty"`
	CSharpMultiDeclared int         `json:"file_edges_multi_declared,omitempty"`
	// CSharpCallShapes and CSharpCallsInLambda (LB-F287) count the C#
	// compiler's call sites by their written shape and those a lambda
	// encloses, disclosed by csharpSiteNotes.
	CSharpCallShapes    map[string]int `json:"call_shapes,omitempty"`
	CSharpCallsInLambda int            `json:"calls_in_lambda,omitempty"`
	// CallExclusions (wave lf5, W1) take a callee out of a file's L2 pairs
	// on both sides, counted (applyCallExclusions, langoracle_html.go): an id
	// two elements of an HTML document carry names no one element.
	CallExclusions []callExclusion `json:"call_exclusions,omitempty"`
	// Disclose (wave lf5) is the oracle's own counts a reading names in its
	// notes (toolOracleDisclosures): parse5's unmatched references, lychee's
	// links that name no file.
	Disclose []string `json:"disclose,omitempty"`
	// CrossReferences are LB-F30's cross-file references (xrefOracle).
	CrossReferences []xrefOracleEntry `json:"cross_references,omitempty"`
	// Imports are a language's import statements resolved to files of the
	// subject (LB-F40, scoreImportsOracle): a language-native import dump.
	Imports []importOracleEntry `json:"imports,omitempty"`
}

// oraclePair is one (from, to) pair in a file.
type oraclePair struct {
	Path string `json:"path"`
	From string `json:"from"`
	To   string `json:"to"`
}

// sub is a pointer: on success (len(defs) > 0), this is the subject's ONLY
// source of KindIntersection/FamilyL1 when no ctags oracle ran (the common
// case for a tool format), so the caller's copy must observe the write —
// a by-value sub previously dropped the KindIntersection slice-reassignment
// silently, refusing every tool-oracle-only reading at Validate (§1.4)
// with "required kind ... outside the kind intersection" even though the
// oracle and manifest agreed (LB-Q-package-json).
//
// inScope is the file list fathom read (§1.4 Scope): an oracle entry on
// any other file is dropped and counted, never scored as a miss.
//
// The *callDiff, non-nil when the file carries calls, is the L2 comparison
// -unmatched writes (LB-F64).
func scoreToolOracle(path string, m langbench.Manifest, nodes []core.SymbolNode, sub *langbench.Subject, inScope map[string]bool) (langbench.OracleRun, int, *langbench.L1Result, *callDiff, error) {
	fail := func(err error) (langbench.OracleRun, int, *langbench.L1Result, *callDiff, error) {
		return langbench.OracleRun{}, 0, nil, nil, err
	}
	var l1res *langbench.L1Result
	var calls *callDiff
	b, err := os.ReadFile(path)
	if err != nil {
		return fail(err)
	}
	var tf toolOracleFile
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&tf); err != nil {
		return fail(fmt.Errorf("%s: %w", path, err))
	}
	tier := orDefault(tf.Tier, "A")
	if tf.Tool == "" || !slices.Contains([]string{"A", "B", "C", "D"}, tier) {
		return fail(fmt.Errorf("%s: tool and a tier A-D are required", path))
	}
	if len(tf.Scope) > 0 {
		inScope, nodes = toolOracleScope(tf.Scope, inScope, nodes)
	}
	dropped := 0
	var defs []langbench.OracleDef
	for _, d := range tf.Definitions {
		if !inScope[d.Path] {
			dropped++
			continue
		}
		defs = append(defs, langbench.OracleDef{Path: d.Path, Name: d.Name, Line: d.Line, End: d.End, Kind: d.Kind})
	}
	if len(defs) > 0 {
		if _, done := sub.Families[langbench.FamilyL1]; done {
			return fail(fmt.Errorf("%s: definitions given alongside ctags; measure one L1 oracle per subject", path))
		}
		l1 := langbench.ScoreL1(m, "tool", nodes, defs)
		l1res = &l1
		addCounts(sub.Counts, l1.Counts)
		addCounts(sub.ExcludedByKind, l1.Excluded)
		sub.KindIntersection = l1.Intersection
		sub.Families[langbench.FamilyL1] = tier
	}
	if len(tf.Calls) > 0 {
		if _, done := sub.Families[langbench.FamilyCalls]; done {
			return fail(fmt.Errorf("%s: calls given alongside another calls oracle (a SCIP index or an earlier file); measure one per subject", path))
		}
		truth := map[langbench.CallPair]bool{}
		files := map[string]bool{}
		for _, c := range tf.Calls {
			if !inScope[c.Path] {
				dropped++
				continue
			}
			truth[langbench.CallPair{Path: c.Path, Caller: m.MatchName(c.From), Callee: m.MatchName(c.To)}] = true
			files[c.Path] = true
		}
		found := m.CallPairs(ownCallers(m, nodes, files, sub.Counts), files) // wave lf5: HTML's pairs are its own kinds' (langoracle_html.go); lf7: Astro's less its styles' (langoracle_astro.go)
		applyCallExclusions(m, tf.CallExclusions, inScope, truth, found, sub.Counts)
		if m.LangTag == "css" {
			truth, found = cssNameScope(m, &tf, truth, found, sub.Counts) // design 1.8 W2 (LB-F460, langoracle_css.go)
		}
		langbench.ScorePairs(truth, found, sub.Counts, langbench.CntCallTruth, langbench.CntCallTruthFound, langbench.CntCallFathom, langbench.CntCallFathomInTrue)
		sub.Families[langbench.FamilyCalls] = tier
		calls = &callDiff{oracle: tf.Tool, truth: truth, mine: found}
	}
	if len(tf.References) > 0 {
		truth := map[langbench.CallPair]bool{}
		files := map[string]bool{}
		for _, r := range tf.References {
			if !inScope[r.Path] {
				dropped++
				continue
			}
			truth[langbench.CallPair{Path: r.Path, Caller: m.MatchName(r.From), Callee: m.MatchName(r.To)}] = true
			files[r.Path] = true
		}
		found := m.CallPairs(nodes, files)
		langbench.ScorePairs(truth, found, sub.Counts, langbench.CntRefTruth, langbench.CntRefTruthFound, langbench.CntRefFathom, langbench.CntRefFathomInTrue)
		sub.Families[langbench.FamilyRefs] = tier
	}
	return langbench.OracleRun{Tier: tier, Tool: tf.Tool, Version: tf.Version, Source: tf.Source, Revision: tf.Revision, Licence: tf.Licence}, dropped, l1res, calls, nil
}

// toolOracleScope narrows the files a tool oracle grades to its own scope:
// the measured files it names, and fathom's symbols on them.
func toolOracleScope(scope []string, inScope map[string]bool, nodes []core.SymbolNode) (map[string]bool, []core.SymbolNode) {
	in := make(map[string]bool, len(scope))
	for _, f := range scope {
		if inScope[f] {
			in[f] = true
		}
	}
	kept := make([]core.SymbolNode, 0, len(nodes))
	for _, n := range nodes {
		if in[n.FilePath] {
			kept = append(kept, n)
		}
	}
	return in, kept
}

// noteToolOracleScope discloses a tool oracle's scope (H5-R4).
const noteToolOracleScope = "%s: %s grades definitions and calls on its scope, %d of %d measured files (the files it reads); a file outside is neither a recall nor a precision miss, while tenancy and file edges cover every measured file"

var universalCtagsRE = regexp.MustCompile(`Universal Ctags ([0-9][^,( ]*)`)

// findCtags resolves a universal-ctags binary and its version; why is
// non-empty when none is usable (BSD ctags, absent).
func findCtags(ctx context.Context, flagPath string) (bin, version, why string) {
	candidates := []string{flagPath}
	if flagPath == "" {
		candidates = []string{filepath.Join("workspace", "tools", "ctags")}
		if p, err := exec.LookPath("ctags"); err == nil {
			candidates = append(candidates, p)
		}
	}
	why = "no universal-ctags found (workspace/tools/ctags or PATH)"
	for _, c := range candidates {
		if c == "" {
			continue
		}
		outb, err := exec.CommandContext(ctx, c, "--version").Output()
		if err != nil {
			why = fmt.Sprintf("%s: %v", c, err)
			continue
		}
		if m := universalCtagsRE.FindStringSubmatch(string(outb)); m != nil {
			return c, m[1], ""
		}
		why = c + " is not universal-ctags"
	}
	return "", "", why
}

func runCtags(ctx context.Context, bin, root string, files []string, cs langbench.CtagsSpec) ([]langbench.OracleDef, error) {
	list, err := os.CreateTemp("", "langbench-files-*.txt")
	if err != nil {
		return nil, err
	}
	defer os.Remove(list.Name())
	if _, err := list.WriteString(strings.Join(files, "\n") + "\n"); err != nil {
		return nil, err
	}
	if err := list.Close(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, oracleTimeout)
	defer cancel()
	args := append([]string(nil), langbench.CtagsArgs...)
	args = append(args, "--languages="+strings.Join(cs.Languages, ","))
	args = append(args, cs.Args...)
	args = append(args, "-L", list.Name())
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	outb, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ctags: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return langbench.ParseCtagsJSON(bytes.NewReader(outb))
}

func loadOrNewReading(path string, m langbench.Manifest, target string) (langbench.Reading, error) {
	r, err := loadReading(path)
	if errors.Is(err, os.ErrNotExist) {
		return langbench.NewReading(m, target), nil
	}
	if err != nil {
		return langbench.Reading{}, err
	}
	if r.Dialect != m.ID {
		return langbench.Reading{}, fmt.Errorf("%s holds dialect %q, not %q", path, r.Dialect, m.ID)
	}
	r.RequiredKinds = m.RequiredCanonical()
	return r, nil
}

// loadReading decodes a committed reading strictly.
func loadReading(path string) (langbench.Reading, error) {
	var r langbench.Reading
	b, err := os.ReadFile(path)
	if err != nil {
		return r, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return r, fmt.Errorf("%s: %w", path, err)
	}
	return r, nil
}

func mergeOracles(have, add []langbench.OracleRun) []langbench.OracleRun {
	byTool := map[string]langbench.OracleRun{}
	for _, o := range append(append([]langbench.OracleRun(nil), have...), add...) {
		if cur, ok := byTool[o.Tool]; ok && cur.Status == "" && o.Status != "" {
			continue // a subject that ran the oracle outranks one that could not
		}
		byTool[o.Tool] = o
	}
	out := make([]langbench.OracleRun, 0, len(byTool))
	for _, k := range sortedKeys(byTool) {
		out = append(out, byTool[k])
	}
	return out
}

func mergeNotes(have, add []string) []string {
	out := append([]string(nil), have...)
	for _, n := range add {
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

func addCounts(dst, src map[string]int) {
	for k, v := range src {
		dst[k] += v
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// subjectSHA is the commit a subject is measured at (round 3 A1, F1): sha
// when the caller names one (the driver passes the clone's, since its scratch
// copy has no .git), else the subject's own HEAD, and none when root is not
// its repository's top level (a copy inside the fathom checkout would
// otherwise report fathom's own commit).
func subjectSHA(ctx context.Context, root, sha string) (string, error) {
	if sha != "" {
		if !fullSHA.MatchString(sha) {
			return "", fmt.Errorf("-subject-sha %q: want a full 40-hex commit", sha)
		}
		return sha, nil
	}
	top := gitOut(ctx, root, "rev-parse", "--show-toplevel")
	if top == "" || !samePath(top, root) {
		return "", nil
	}
	return gitOut(ctx, root, "rev-parse", "HEAD"), nil
}

var fullSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)

// allowUnpinnedSubject lets lang-oracle measure a subject with no commit of
// its own; only the package's tests set it (their subjects are temporary
// directories).
var allowUnpinnedSubject = false

// samePath reports whether two paths name one directory once symlinks are
// resolved.
func samePath(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return false
	}
	ra, _ = filepath.Abs(ra)
	rb, _ = filepath.Abs(rb)
	return ra == rb
}

// unwrittenWhy names, per dialect, what an index's callable reference with no
// call written at its range is (scip_calls_unwritten): rust-analyzer's
// operators are rust's, never another dialect's (round 3 A4, F4).
func unwrittenWhy(dialect string) string {
	switch dialect {
	case "dart":
		return "a tear-off such as `onError: c.addError`, an operator, the super(...) of an initializer"
	case "python":
		return "a function or method referenced without a call: passed as a value, as in Depends(get_db), or a decorator applied by name"
	case "cuda":
		return "a function passed as a value (a callback argument, a kernel passed to a launch or attribute API), or a call a macro's expansion makes (check inside checkCudaErrors), which scip-clang references at the macro's own name"
	case "c", "cpp":
		return "a function passed as a value (a callback argument), or a call a macro's expansion makes (abort and fprintf inside an ASSERT), which scip-clang references at the macro's own name"
	case "typescript", "javascript":
		return "a function or method passed as a value (a callback argument), or an accessor read, which scip-typescript references as `<get>x`"
	}
	return "an operator rust-analyzer resolves to its trait method: ?, ==, !x, *x, a[i], a + b"
}

func scipClangNoImportsNote(fileEdgesTier string) string {
	return strings.Replace(noteScipClangNoImports, "cross-file references (tier A)", "cross-file references (tier "+fileEdgesTier+")", 1)
}

func gitOut(ctx context.Context, dir string, args ...string) string {
	outb, err := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(outb))
}

// toolsCarryDefinitions reports whether any neutral oracle file carries
// definitions (an unreadable file is left to scoreToolOracle to report).
func toolsCarryDefinitions(paths []string) bool {
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var probe struct {
			Definitions []json.RawMessage `json:"definitions"`
		}
		if json.Unmarshal(b, &probe) == nil && len(probe.Definitions) > 0 {
			return true
		}
	}
	return false
}

// activeLinesOf reads the `active_lines` of every tool oracle that has them
// (LB-F80): per file, the line ranges its translation units' preprocessor keeps.
func activeLinesOf(paths []string) (map[string][][2]int, error) {
	out := map[string][][2]int{}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var probe struct {
			ActiveLines map[string][][2]int `json:"active_lines"`
		}
		if err := json.Unmarshal(b, &probe); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		for f, r := range probe.ActiveLines {
			out[f] = r
		}
	}
	return out, nil
}

// lineActive reports whether line lies in one of the sorted [first, last] ranges.
func lineActive(ranges [][2]int, line int) bool {
	i := sort.Search(len(ranges), func(i int) bool { return ranges[i][1] >= line })
	return i < len(ranges) && ranges[i][0] <= line
}

// scopedFileEdgeTruth keeps the index's cross-file edges between measured files.
// scopedFileEdgeTruthTo is scopedFileEdgeTruth whose targets may also be the
// reached headers (round 3 F5): a pair from a measured file to a measured file
// or a reached one.
func scopedFileEdgeTruthTo(truth map[[2]string]bool, inScope, reached map[string]bool) map[[2]string]bool {
	out := make(map[[2]string]bool, len(truth))
	for k := range truth {
		if inScope[k[0]] && (inScope[k[1]] || reached[k[1]]) {
			out[k] = true
		}
	}
	return out
}

func scopedFileEdgeTruth(truth map[[2]string]bool, inScope map[string]bool) map[[2]string]bool {
	out := make(map[[2]string]bool, len(truth))
	for k := range truth {
		if inScope[k[0]] && inScope[k[1]] {
			out[k] = true
		}
	}
	return out
}
