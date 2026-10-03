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
// its own reader's import specs and the rego-package strategy (LB-F131).
var fileResolveLang = map[string]string{"typescript": "javascript", "javascript": "javascript", "python": "python", "c": "c", "cpp": "cpp", "php": "php", "rust": "rust", "ruby": "ruby", "dart": "dart", "csharp": "csharp", "java": "java", "rego": "rego"}

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
	scipDefs := fs.Bool("scip-defs", false, "score L1 against the -scip index's definitions (tier A) instead of universal-ctags (owner decision D2: a stronger oracle where ctags is deficient)")
	var toolOracles multiFlag
	fs.Var(&toolOracles, "tool-oracle", "oracle JSON in the neutral shape (definitions, calls, references; tier A unless it says otherwise; repeatable)")
	callsFrom := fs.String("calls-from", "scip", "which oracle grades L2 beside -scip: scip (the index's call sites, tier A) or tool (the -tool-oracle files' calls at their own tier, the index still grading definitions, typed edges and imports; LB-F204, O264)")
	adjudicate := fs.String("adjudicate", "", "oracle-deficiency disclosure JSON (owner decision D3): dialect-only definitions the L1 oracle is verified to miss, each admitted only by a deterministic file:line, name and module-scope check, listed in the reading and left out of sym_precision's denominator")
	unmatched := fs.String("unmatched", "", "write this subject's L1 misses both ways (dialect_only, oracle_only) and its L2 call pairs both ways (calls.fathom_only, calls.oracle_only) as JSON — the worker's diagnosis and the source of D3 adjudication candidates; never part of the reading")
	only := fs.String("only", "", "comma-separated globs (subject-relative, * within a segment, ** across): measure only these files — e.g. the SQL families a grammar supports (D2); the reading names the restriction")
	out := fs.String("out", "", "reading to write; an existing reading of the same dialect is merged (required)")
	queuePath := fs.String("queue", "docs/plans/language-breadth.queue.json", "the queue (for the dialect's target level)")
	date := fs.String("date", time.Now().UTC().Format("2006-01-02"), "measured_at")
	commit := fs.String("commit", "", "fathom commit measured (default: git rev-parse --short HEAD)")
	tq := tagsQueryFlags(fs) // a third-party definitions query as a tier-C L1 oracle (LB-F46, langoracle_tagsquery.go)
	if err := fs.Parse(args); err != nil {
		return 2
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
	var with extractor.Extractor
	if source == "dialect" {
		switch spec.Manifest.Engine {
		case "tags+doc":
			// One manifest, two engines (LB-Q-helm): documents go to the doc
			// engine, every other file the manifest owns to the tags engine.
			td, tderr := extractor.NewTagsDocExtractor(*dialect)
			if tderr != nil {
				fmt.Fprintln(stderr, "lang-oracle:", tderr)
				return 1
			}
			defer td.Close()
			with = td
		case "tags":
			ts, tserr := extractor.NewTSExtractor(*dialect)
			if tserr != nil {
				fmt.Fprintln(stderr, "lang-oracle:", tserr)
				return 1
			}
			defer ts.Close()
			with = ts
		case "doc":
			doc, docerr := extractor.NewDocDialectExtractor(*dialect)
			if docerr != nil {
				fmt.Fprintln(stderr, "lang-oracle:", docerr)
				return 1
			}
			with = doc
		case "native":
			// A native dialect with a stand-alone extractor (go-mod, LB-Q-go-mod;
			// terraform's HCLExtractor, LB-F84) is measured through it, so its
			// DocResult reaches the cross-file scoring; any other native dialect
			// is measured through fusion's dispatch, as before.
			if nat, ok := extractor.NativeDialectExtractor(*dialect); ok {
				with = nat
			}
		}
	}

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
	m, err := measureSubject(ctx, spec, *subject, name, oracleOpts{ctags: *ctagsPath, scip: *scipPath, scipDefs: *scipDefs, callsFromTool: *callsFrom == "tool", tools: tools, only: onlyGlobs, adjudicate: *adjudicate, unmatched: *unmatched}, with)
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
		spec := langbench.BenchSpec{Manifest: m}
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
	ctags    string // universal-ctags binary ("" = workspace/tools/ctags, then PATH)
	scip     string // SCIP index (tier A)
	scipDefs bool   // the SCIP index's definitions are the L1 oracle (D2)
	// callsFromTool: the -tool-oracle files' calls grade L2 beside -scip
	// (LB-F204, O264); the index's call truth, its call counts and its call
	// notes are not part of the reading.
	callsFromTool bool
	tools         []string // neutral oracle JSON files (definitions, calls, references)
	only          []string // globs restricting the subject's files (D2, SQL)
	adjudicate    string   // oracle-deficiency disclosure file (D3)
	unmatched     string   // where to write the L1 and call-pair misses both ways (diagnosis)
}

// unmatchedFile is -unmatched's JSON: the subject's L1 misses both ways when
// an L1 oracle ran, and its L2 call pairs both ways when a calls oracle ran
// (LB-F64). Diagnosis only; never part of the reading.
type unmatchedFile struct {
	DialectOnly *[]langbench.OracleDef `json:"dialect_only,omitempty"`
	OracleOnly  *[]langbench.OracleDef `json:"oracle_only,omitempty"`
	Calls       *unmatchedCalls        `json:"calls,omitempty"`
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
// says so) is measured on exactly the files ingest sends it.
func ownedSourceRefs(m langbench.Manifest, root string, refs []core.SourceRef, only []string, contentDialect func(rel string, content []byte) string) []core.SourceRef {
	byContent := contentDialect != nil && m.Engine == "doc" && len(m.Match.Extensions)+len(m.Match.Basenames)+len(m.Match.Globs) == 0 && m.Match.Sniff == nil
	var out []core.SourceRef
	for _, ref := range refs {
		if !onlyMatch(only, ref.Path) {
			continue
		}
		if m.Owns(ref.Path) {
			out = append(out, ref)
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
		if (m.ID == "cpp" || m.ID == "c") && strings.EqualFold(filepath.Ext(ref.Path), ".h") {
			// The router sends a .h to C++ on a C++-only signal and to C
			// otherwise (LB-F80: a C include's target is a header, so without
			// them C's import_resolution and file edges have no denominator).
			if b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ref.Path))); err == nil && extractor.LooksLikeCppHeader(b) == (m.ID == "cpp") {
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
// (Pipeline.CloudPolicyDialect).
func contentDialectOf(pipe *fusion.Pipeline, refs []core.SourceRef) func(rel string, content []byte) string {
	return func(rel string, content []byte) string {
		if d := pipe.ContentDialect(rel, content, refs); d != "" {
			return d
		}
		if d := pipe.CloudPolicyDialect(rel, content, refs); d != "" {
			return d
		}
		return fusion.AppFrameworkContentDialect(rel, content) // an ARM template (LB-F146)
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

	sub := langbench.Subject{Name: name, SHA: gitOut(ctx, root, "rev-parse", "HEAD"), Counts: map[string]int{},
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
		res.notes = append(res.notes, fmt.Sprintf("%s: restricted to %s (-only; the files outside are a disclosed gap, not measured)", name, strings.Join(opts.only, ", ")))
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
	if len(files) == 0 {
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
		n, unparsed, err := filesWithError(ctx, spec.Manifest.ID, root, files)
		switch {
		case err != nil:
			res.notes = append(res.notes, fmt.Sprintf("%s: files_with_error not measured: %v", name, err))
		default:
			sub.Tenancy.FilesWithError = &n
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

	var l1res *langbench.L1Result // the subject's one L1 scoring, for D3
	var calls *callDiff           // the subject's one L2 scoring, for -unmatched
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
		imps = &subjectImports{ctx: ctx, dialect: spec.Manifest.ID, root: root, name: name, files: srcFiles, nodes: nodes, inScope: inScope}
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
		var active map[string][][2]int // the build's kept lines (LB-F80), when an oracle gives them
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
			sub.Families[langbench.FamilyL1] = "A"
		}
		scipOpts := langbench.SCIPOptionsFor(spec.Manifest.ID, func(p string) []byte {
			b, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
			return b
		})
		if clangIndex {
			// LB-F116: the build's own compiler's call sites, when an oracle
			// file carries them, settle what the index cannot.
			if scipOpts.ClangSites, err = clangSitesOf(opts.tools); err != nil {
				return res, err
			}
			if len(active) > 0 {
				scipOpts.ActiveLines = active
			}
		}
		sr, err := langbench.ScoreSCIPWith(raw, service, scoped, scipOpts)
		if err != nil {
			return res, err
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
		res.notes = append(res.notes, clangSiteNotes(name, sr.Counts)...)
		if n := sr.Counts[langbench.CntScipCallsUnwritten]; n > 0 {
			why := "an operator rust-analyzer resolves to its trait method: ?, ==, !x, *x, a[i], a + b"
			switch spec.Manifest.ID {
			case "dart":
				why = "a tear-off such as `onError: c.addError`, an operator, the super(...) of an initializer"
			case "python":
				why = "a function or method referenced without a call: passed as a value, as in Depends(get_db), or a decorator applied by name"
			case "c", "cpp":
				why = "a function passed as a value (a callback argument), or a call a macro's expansion makes (abort and fprintf inside an ASSERT), which scip-clang references at the macro's own name"
			case "typescript", "javascript":
				why = "a function or method passed as a value (a callback argument), or an accessor read, which scip-typescript references as `<get>x`"
			}
			res.notes = append(res.notes, fmt.Sprintf("%s: %d callable references have no call written at their range (%s) and are left out of the call truth (L2 is call sites)", name, n, why))
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
		if sr.EnclosingFallback > 0 {
			res.notes = append(res.notes, fmt.Sprintf("%s: %d SCIP definitions carried no enclosing_range; each took its mapped fathom definition's line span so call truth stays measurable (D5)", name, sr.EnclosingFallback))
		}
		addCounts(sub.Counts, sr.Counts)
		if !opts.callsFromTool {
			calls = &callDiff{oracle: orDefault(sr.Tool, "scip"), truth: sr.CallTruth, mine: sr.CallFound}
			sub.Families[langbench.FamilyCalls] = "A"
		}
		sub.Families[langbench.FamilyDefMap] = "A"
		sub.Families[langbench.FamilyTyped] = "A"
		if lang, ok := fileResolveLang[spec.Manifest.ID]; ok {
			edges, rep := fusion.ResolveFileEdges(ctx, lang, root, srcFiles, nodes)
			if rep.Skipped {
				res.notes = append(res.notes, name+": file-resolved edges not measured: "+rep.SkipReason)
			} else {
				edges = edgesFromIndexed(edges, sr.Docs, name, &res.notes)
				edges = scopeFileEdgesNote(edges, nodes, active, name, &res.notes)
				truth := scopedFileEdgeTruth(sr.FileEdges, inScope)
				if n := len(sr.FileEdges) - len(truth); n > 0 {
					res.notes = append(res.notes, fmt.Sprintf("%s: %d index %s", name, n, noteFileEdgeTruthScoped))
				}
				langbench.ScoreFileEdges(truth, edges, nodes, sub.Counts)
				sub.Families[langbench.FamilyFileEdges] = "A"
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
				langbench.ScoreFileEdges(sr.FileEdges, edges, nodes, sub.Counts)
				sub.Families[langbench.FamilyFileEdges] = "A"
				res.notes = append(res.notes, noteGoTypedFileEdges)
				if rep.PackagesFailed > 0 {
					res.notes = append(res.notes, fmt.Sprintf("%s: %d of %d packages failed the typed pass and give no edge", name, rep.PackagesFailed, rep.PackagesTotal))
				}
			}
		}
		res.oracles = append(res.oracles, langbench.OracleRun{Tier: "A", Tool: orDefault(sr.Tool, "scip"), Version: sr.Version})
		if imps != nil {
			if sr.Tool == scipClangTool {
				// LB-F80: scip-clang records no reference on an `#include` line
				// and gives no occurrence the Import role, so the references it
				// does carry coincide with an include's line only by accident:
				// import_resolution is a dump's (clang-includes), never the index's.
				res.notes = append(res.notes, noteScipClangNoImports)
			} else if err := imps.score(sr.ImportRefs, "A", orDefault(sr.Tool, "scip"), &sub); err != nil {
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
		if imps != nil {
			if err := scoreImportsOracle(toolOracle, imps, &sub); err != nil {
				return res, err
			}
		}
		// LB-F57: no SCIP index, so a dump's imports are the file-edge truth.
		if lang, ok := fileResolveLang[spec.Manifest.ID]; ok && opts.scip == "" && spec.Manifest.Kind != "tooling" {
			notes, err := scoreImportFileEdges(ctx, toolOracle, lang, root, name, srcFiles, nodes, &sub, inScope)
			if err != nil {
				return res, err
			}
			res.notes = append(res.notes, notes...)
		}
		// ── LB-F30: cross-file references (langoracle_xref.go) ──
		if doc, ok := with.(extractor.DocResulter); ok {
			n, err := scoreXrefOracle(toolOracle, doc, service, srcFiles, &sub, inScope)
			if err != nil {
				return res, err
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
	if fx != nil {
		res.notes = append(res.notes, fx.notes...)
	}
	if imps != nil {
		res.notes = append(res.notes, imps.notes...)
	}
	if opts.unmatched != "" && (l1res != nil || calls != nil) {
		var u unmatchedFile
		if l1res != nil {
			u.DialectOnly, u.OracleOnly = &l1res.DialectOnly, &l1res.OracleOnly
		}
		if calls != nil {
			u.Calls = &unmatchedCalls{Oracle: calls.oracle, FathomOnly: pairsOnlyIn(calls.mine, calls.truth), OracleOnly: pairsOnlyIn(calls.truth, calls.mine)}
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
		found := m.CallPairs(nodes, files)
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
func scopedFileEdgeTruth(truth map[[2]string]bool, inScope map[string]bool) map[[2]string]bool {
	out := make(map[[2]string]bool, len(truth))
	for k := range truth {
		if inScope[k[0]] && inScope[k[1]] {
			out[k] = true
		}
	}
	return out
}
