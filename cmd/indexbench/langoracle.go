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
	toolOracle := fs.String("tool-oracle", "", "tool-native oracle JSON (tier A unless it says otherwise; optional)")
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

	m, err := measureSubject(ctx, spec, *subject, name, *ctagsPath, *scipPath, *toolOracle, with)
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

type subjectMeasurement struct {
	subject langbench.Subject
	oracles []langbench.OracleRun
	notes   []string
	// kinds counts the SourceKinds extracted from the dialect's files (the
	// parity run's kinds diff, §11 step 3).
	kinds map[string]int
}

// measureSubject measures one subject. with, when non-nil, extracts every
// file the dialect owns instead of fusion's dispatch (lang-parity's dialect
// side, §11); nil measures what ingest would run.
func measureSubject(ctx context.Context, spec langbench.BenchSpec, root, name, ctagsBin, scipPath, toolOracle string, with extractor.Extractor) (subjectMeasurement, error) {
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
	for _, ref := range refs {
		if !spec.Manifest.Owns(ref.Path) {
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
		if !spec.Manifest.Owns(ref.Path) {
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ref.Path)))
		if err != nil {
			return res, err
		}
		inst := tsengine.Instantiations()
		start, cpu0 := time.Now(), processCPU()
		ex, err := extract(pipe, service, refs, ref.Path, content)
		elapsed, cpu := time.Since(start), processCPU()-cpu0
		if tsengine.Instantiations() != inst && ex.Skipped == "" {
			// This file instantiated a grammar module the warm-up did not
			// (an injected language's, e.g. HTML's <script>): time it again,
			// warm, so a one-time compile is never charged to throughput.
			start, cpu0 = time.Now(), processCPU()
			ex, err = extract(pipe, service, refs, ref.Path, content)
			elapsed, cpu = time.Since(start), processCPU()-cpu0
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
		sub.Tenancy.MaxFileMS = max(sub.Tenancy.MaxFileMS, elapsed.Milliseconds())
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

	// Tier B: universal-ctags over exactly the files fathom read.
	ctagsRun := langbench.OracleRun{Tier: "B", Tool: "universal-ctags"}
	switch bin, version, why := findCtags(ctx, ctagsBin); {
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
		addCounts(sub.Counts, l1.Counts)
		addCounts(sub.ExcludedByKind, l1.Excluded)
		sub.KindIntersection = l1.Intersection
		sub.Families[langbench.FamilyL1] = "B"
	}
	res.oracles = append(res.oracles, ctagsRun)

	if scipPath != "" {
		raw, err := os.ReadFile(scipPath)
		if err != nil {
			return res, err
		}
		sr, err := langbench.ScoreSCIP(raw, service, nodes)
		if err != nil {
			return res, err
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

	if toolOracle != "" {
		inScope := map[string]bool{}
		for _, f := range files {
			inScope[f] = true
		}
		run, dropped, err := scoreToolOracle(toolOracle, spec.Manifest, nodes, &sub, inScope)
		if err != nil {
			return res, err
		}
		if dropped > 0 {
			res.notes = append(res.notes, fmt.Sprintf("%s: %d tool-oracle entries on files fathom does not read (ingest's gates, discovery's exclusions) were left out of scope (§1.4)", name, dropped))
		}
		res.oracles = append(res.oracles, run)
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
	References []struct {
		Path string `json:"path"`
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"references"`
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
func scoreToolOracle(path string, m langbench.Manifest, nodes []core.SymbolNode, sub *langbench.Subject, inScope map[string]bool) (langbench.OracleRun, int, error) {
	fail := func(err error) (langbench.OracleRun, int, error) { return langbench.OracleRun{}, 0, err }
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
		addCounts(sub.Counts, l1.Counts)
		addCounts(sub.ExcludedByKind, l1.Excluded)
		sub.KindIntersection = l1.Intersection
		sub.Families[langbench.FamilyL1] = tier
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
	return langbench.OracleRun{Tier: tier, Tool: tf.Tool, Version: tf.Version}, dropped, nil
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
