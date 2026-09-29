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

// oracleTimeout bounds one external oracle run (design §13.4 step 19).
const oracleTimeout = 20 * time.Minute

// fileResolveLang names the dialects today's file resolver serves
// (fusion.ResolveFileEdges), so file-edge metrics are measured for them.
var fileResolveLang = map[string]string{"typescript": "javascript", "javascript": "javascript", "python": "python"}

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
	adjudicate := fs.String("adjudicate", "", "oracle-deficiency disclosure JSON (owner decision D3): dialect-only definitions the L1 oracle is verified to miss, each admitted only by a deterministic file:line, name and module-scope check, listed in the reading and left out of sym_precision's denominator")
	unmatched := fs.String("unmatched", "", "write this subject's L1 misses both ways (dialect_only, oracle_only) as JSON — the worker's diagnosis and the source of D3 adjudication candidates; never part of the reading")
	only := fs.String("only", "", "comma-separated globs (subject-relative, * within a segment, ** across): measure only these files — e.g. the SQL families a grammar supports (D2); the reading names the restriction")
	out := fs.String("out", "", "reading to write; an existing reading of the same dialect is merged (required)")
	queuePath := fs.String("queue", "docs/plans/language-breadth.queue.json", "the queue (for the dialect's target level)")
	date := fs.String("date", time.Now().UTC().Format("2006-01-02"), "measured_at")
	commit := fs.String("commit", "", "fathom commit measured (default: git rev-parse --short HEAD)")
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
	// pipeline.go's extension map (§11 step 2's merge gate not yet passed)
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
		case "tags", "tags+doc":
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
		}
	}

	if *scipDefs && *scipPath == "" {
		fmt.Fprintln(stderr, "lang-oracle: -scip-defs needs -scip")
		return 2
	}
	var onlyGlobs []string
	for _, g := range strings.Split(*only, ",") {
		if g = strings.TrimSpace(g); g != "" {
			onlyGlobs = append(onlyGlobs, g)
		}
	}
	m, err := measureSubject(ctx, spec, *subject, name, oracleOpts{ctags: *ctagsPath, scip: *scipPath, scipDefs: *scipDefs, tools: toolOracles, only: onlyGlobs, adjudicate: *adjudicate, unmatched: *unmatched}, with)
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
	ctags      string   // universal-ctags binary ("" = workspace/tools/ctags, then PATH)
	scip       string   // SCIP index (tier A)
	scipDefs   bool     // the SCIP index's definitions are the L1 oracle (D2)
	tools      []string // neutral oracle JSON files (definitions, calls, references)
	only       []string // globs restricting the subject's files (D2, SQL)
	adjudicate string   // oracle-deficiency disclosure file (D3)
	unmatched  string   // where to write the L1 misses both ways (diagnosis)
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
	owned := func(p string) bool { return spec.Manifest.Owns(p) && onlyMatch(opts.only, p) }
	if len(opts.only) > 0 {
		res.notes = append(res.notes, fmt.Sprintf("%s: restricted to %s (-only; the files outside are a disclosed gap, not measured)", name, strings.Join(opts.only, ", ")))
	}
	for _, ref := range refs {
		if !owned(ref.Path) {
			continue
		}
		if content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ref.Path))); err == nil {
			if ex, _ := extract(pipe, service, refs, ref.Path, content); ex.Skipped == "" {
				res.notes = append(res.notes, "per-file times exclude the one-time grammar compile (an untimed warm-up extraction precedes timing)")
				break
			}
		}
	}
	for _, ref := range refs {
		if !owned(ref.Path) {
			continue
		}
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
	if retimed > 0 {
		res.notes = append(res.notes, "a file whose extraction instantiated a grammar module after the warm-up (an injected language) is timed on a second, warm extraction")
	}
	for _, k := range sortedKeys(skipped) {
		res.notes = append(res.notes, fmt.Sprintf("%s: %d files skipped by ingest's gates (%s)", name, skipped[k], k))
	}

	var l1res *langbench.L1Result // the subject's one L1 scoring, for D3
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

	if opts.scip != "" {
		raw, err := os.ReadFile(opts.scip)
		if err != nil {
			return res, err
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
			l1 := langbench.ScoreL1(spec.Manifest, "scip", nodes, defs)
			l1res = &l1
			addCounts(sub.Counts, l1.Counts)
			addCounts(sub.ExcludedByKind, l1.Excluded)
			sub.KindIntersection = l1.Intersection
			sub.Families[langbench.FamilyL1] = "A"
		}
		sr, err := langbench.ScoreSCIP(raw, service, nodes)
		if err != nil {
			return res, err
		}
		if sr.EnclosingFallback > 0 {
			res.notes = append(res.notes, fmt.Sprintf("%s: %d SCIP definitions carried no enclosing_range; each took its mapped fathom definition's line span so call truth stays measurable (D5)", name, sr.EnclosingFallback))
		}
		addCounts(sub.Counts, sr.Counts)
		sub.Families[langbench.FamilyCalls] = "A"
		sub.Families[langbench.FamilyDefMap] = "A"
		sub.Families[langbench.FamilyTyped] = "A"
		if lang, ok := fileResolveLang[spec.Manifest.ID]; ok {
			edges, rep := fusion.ResolveFileEdges(ctx, lang, root, srcFiles, nodes)
			if rep.Skipped {
				res.notes = append(res.notes, name+": file-resolved edges not measured: "+rep.SkipReason)
			} else {
				langbench.ScoreFileEdges(sr.FileEdges, edges, nodes, sub.Counts)
				sub.Families[langbench.FamilyFileEdges] = "A"
			}
		}
		res.oracles = append(res.oracles, langbench.OracleRun{Tier: "A", Tool: orDefault(sr.Tool, "scip"), Version: sr.Version})
		res.notes = append(res.notes, "import_resolution not measured: in-repo spec counts arrive with the L3 strategies (LB-F11)")
	}

	for _, toolOracle := range opts.tools {
		run, dropped, l1, err := scoreToolOracle(toolOracle, spec.Manifest, nodes, &sub, inScope)
		if err != nil {
			return res, err
		}
		if l1 != nil {
			l1res = l1
		}
		if dropped > 0 {
			res.notes = append(res.notes, fmt.Sprintf("%s: %d tool-oracle entries on files fathom does not read (ingest's gates, discovery's exclusions) were left out of scope (§1.4)", name, dropped))
		}
		res.oracles = append(res.oracles, run)
		// ── LB-F30: cross-file references (langoracle_xref.go) ──
		if doc, ok := with.(*extractor.DocExtractor); ok {
			n, err := scoreXrefOracle(toolOracle, doc, service, srcFiles, &sub, inScope)
			if err != nil {
				return res, err
			}
			if n > 0 {
				res.notes = append(res.notes, fmt.Sprintf("%s: %d oracle cross-references on files fathom does not read were left out of scope (§1.4)", name, n))
			}
		}
		// ── end LB-F30 ──
	}
	if opts.unmatched != "" && l1res != nil {
		b, err := json.MarshalIndent(map[string][]langbench.OracleDef{"dialect_only": l1res.DialectOnly, "oracle_only": l1res.OracleOnly}, "", " ")
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
	Tool        string `json:"tool"`
	Version     string `json:"version"`
	Tier        string `json:"tier"`
	Definitions []struct {
		Path string `json:"path"`
		Name string `json:"name"`
		Line int    `json:"line"`
		End  int    `json:"end"`
		Kind string `json:"kind"`
	} `json:"definitions"`
	References []oraclePair `json:"references"`
	// Calls are a language's call pairs (§1.4: enclosing definition's
	// name, callee's name), scored as call_recall/call_precision at the
	// file's tier — e.g. tier C from the grammar's upstream tags.scm run
	// by an independent runtime (owner decision D5).
	Calls []oraclePair `json:"calls"`
	// CrossReferences are LB-F30's cross-file references (xrefOracle).
	CrossReferences []xrefOracleEntry `json:"cross_references,omitempty"`
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
func scoreToolOracle(path string, m langbench.Manifest, nodes []core.SymbolNode, sub *langbench.Subject, inScope map[string]bool) (langbench.OracleRun, int, *langbench.L1Result, error) {
	fail := func(err error) (langbench.OracleRun, int, *langbench.L1Result, error) {
		return langbench.OracleRun{}, 0, nil, err
	}
	var l1res *langbench.L1Result
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
	return langbench.OracleRun{Tier: tier, Tool: tf.Tool, Version: tf.Version}, dropped, l1res, nil
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
