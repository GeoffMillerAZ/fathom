package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/hookcache"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/testevidence"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// parseModeFlag converts a user-supplied mode string to a client.Mode constant.
func parseModeFlag(s string) (client.Mode, bool) {
	switch s {
	case "local":
		return client.ModeLocal, true
	case "faux":
		return client.ModeFaux, true
	case "github":
		return client.ModeGitHub, true
	}
	return 0, false
}

// newIngestClient builds the client cmdIngest ingests through. A package
// variable so a test can route the ingest through a connector that simulates
// per-file read failures against a real store (ingest_honesty_test.go,
// IT-14 / IT-15) — never by touching a real TCC grant.
var newIngestClient = func(m client.Mode, storePath string) (*client.Client, error) {
	return client.NewConstantClient(m, storePath)
}

// docElidedShown caps how many budget-elided doc units the summary names
// inline (the full list is on the Report and, counted, in --json).
const docElidedShown = 5

// printDocIndex is the ingest summary's doc-unit block (KR-8/KR-9,
// internal/fusion's docindexpass.go): what the pass did, what the byte budget
// cut — named, never a silent trim — what could not be indexed, and why the
// pass did not run (I38).
func printDocIndex(stdout, stderr io.Writer, r client.Report) {
	if r.DocIndexSkipped != "" {
		fmt.Fprintf(stdout, "docs: skipped — %s\n", r.DocIndexSkipped)
	}
	d := r.DocIndex
	if d.FilesScanned > 0 || d.FilesDeleted > 0 {
		fmt.Fprintf(stdout, "docs: %d file(s) scanned, %d re-indexed (%d unit(s) written), %d unchanged, %d removed\n",
			d.FilesScanned, d.FilesChanged, d.UnitsWritten, d.FilesUnchanged, d.FilesDeleted)
		if d.LinksWritten > 0 {
			// KR-6: the doc-to-code links the re-indexed units hold.
			fmt.Fprintf(stdout, "docs: %d doc-to-code link(s) held by the re-indexed units\n", d.LinksWritten)
		}
	}
	if d.UnitsElided > 0 {
		names := make([]string, 0, docElidedShown)
		for i, e := range d.Elided {
			if i == docElidedShown {
				break
			}
			names = append(names, e.Path+" > "+e.HeadingPath)
		}
		more := ""
		if d.UnitsElided > len(names) {
			more = fmt.Sprintf("; ... and %d more", d.UnitsElided-len(names))
		}
		fmt.Fprintf(stdout, "docs: %d unit(s) (%d bytes) elided — past the per-service index byte budget, so not searchable: %s%s\n",
			d.UnitsElided, d.ElidedBytes, strings.Join(names, "; "), more)
	}
	for _, f := range d.Failures {
		fmt.Fprintf(stderr, "warning: docs: %s not indexed: %s\n", f.Path, f.Err)
	}
}

// ingestPassClock times `fathom ingest`'s passes (EW-48,
// internal/fusion/passtimes.go); a test sets a fake, or fusion.PassClock{}
// for none.
var ingestPassClock = fusion.SystemPassClock()

// printPassTimes prints each pass's wall/CPU seconds on one line, a
// sub-pass in parentheses after the pass it is part of (EW-48); nothing
// when the ingest was not timed.
func printPassTimes(w io.Writer, passes []fusion.PassTime) {
	if len(passes) == 0 {
		return
	}
	secs := func(ns int64) string { return fmt.Sprintf("%.3fs", float64(ns)/1e9) }
	var b strings.Builder
	b.WriteString("pass-times (wall/cpu):")
	open := false
	for i, p := range passes {
		if p.Parent != "" {
			if !open {
				b.WriteString(" (")
				open = true
			} else {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s %s/%s", p.Name, secs(p.WallNS), secs(p.CPUNS))
			continue
		}
		if open {
			b.WriteString(")")
			open = false
		}
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, " %s %s/%s", p.Name, secs(p.WallNS), secs(p.CPUNS))
	}
	if open {
		b.WriteString(")")
	}
	fmt.Fprintln(w, b.String())
}

// readFailuresShown caps how many unreadable files the summary names inline;
// the count is always exact and the first entry always carries its errno.
const readFailuresShown = 5

func cmdIngest(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("ingest", stderr)
	service := fs.String("service", "", "logical service ID (required)")
	path := fs.String("path", ".", "path to service source root (local mode)")
	storePath := fs.String("store", defaultStorePath(), "store path (.sqlite/.db)")
	mode := fs.String("mode", "local", "connector mode: local or faux")
	jsonOut := fs.Bool("json", false, "emit the run as JSON (counts, read_failures with errno, outcome ok|degraded|refused)")
	dryRun := fs.Bool("dry-run", false, "discover and stat only: report files, bytes, oversize files and pruned directories without reading a file or opening the store")
	rehash := fs.Bool("rehash", false, "read and hash every file, ignoring the stat gate")
	statsFlag := fs.Bool("stats", false, "with --dry-run: print the discovery statistics (the default output of --dry-run)")
	respectGitignore := fs.Bool("respect-gitignore", false, "with --dry-run: discover a git work tree through 'git ls-files' as [connector] respect_gitignore would — run with and without to measure the delta (RT-M11)")
	var scipPaths stringList
	fs.Var(&scipPaths, "scip", "SCIP index file to import as typed edges (repeatable). Unset: auto-detects index.scip at the repo root and .fathom/scip/*.scip")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *service == "" {
		fmt.Fprintln(stderr, "error: --service is required")
		fs.Usage()
		return 1
	}

	m, ok := parseModeFlag(*mode)
	if !ok {
		fmt.Fprintf(stderr, "error: unknown --mode %q (want local or faux)\n", *mode)
		return 1
	}

	if *respectGitignore && !*dryRun {
		fmt.Fprintln(stderr, "error: --respect-gitignore needs --dry-run (an ingest reads [connector] respect_gitignore from the corpus)")
		return 1
	}
	if *statsFlag && !*dryRun {
		fmt.Fprintln(stderr, "error: --stats needs --dry-run")
		return 1
	}
	if *dryRun {
		if m != client.ModeLocal {
			fmt.Fprintln(stderr, "error: --dry-run needs --mode local")
			return 1
		}
		abs, aerr := filepath.Abs(*path)
		if aerr != nil {
			fmt.Fprintf(stderr, "error: resolve --path: %v\n", aerr)
			return 1
		}
		lc := connector.NewLocalConnector()
		lc.RespectGitignore = *respectGitignore
		return runIngestDryRunStats(context.Background(), lc, client.ServiceRef{ID: *service, Path: abs}, stdout, *jsonOut)
	}

	c, err := newIngestClient(m, *storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: opening client: %v\n", err)
		return 1
	}
	defer c.Close()

	// For ModeFaux the connector resolves svc.Path as the fauxtown subdirectory
	// name, so we pass the service ID.  For ModeLocal we use --path.
	svcPath := *path
	if m == client.ModeFaux {
		svcPath = *service
	}

	c.SetRehash(*rehash)
	// RT-180: a one-shot ingest reads only the files whose stat moved since
	// the last ingest this store committed.
	c.WithStoreStats()
	// RT-106: the typed pass's toolchain children run capped and niced in
	// a CLI ingest too, not only inside serve-http.
	if undo, terr := fusion.InstallProcessToolchain(""); terr != nil {
		fmt.Fprintf(stderr, "warn: toolchain children run at normal priority: %v\n", terr)
	} else {
		defer undo()
	}
	report, err := c.Ingest(fusion.WithPassClock(context.Background(), ingestPassClock), client.ServiceRef{
		ID:   *service,
		Path: svcPath,
	})
	// Ingest honesty (docs/plans/instrument-truth.md IT-14 / IT-15, O16): a
	// *client.ReadFailuresError arrives beside a COMPLETE report — every
	// readable file was processed — so the summary prints in full with its
	// failed column, then the failure is named on stderr and the exit is
	// non-zero, as a fetch error always was. A Refused one wrote nothing at
	// all (the store already held evidence and this run could read nothing).
	// Any other error is the abort it always was.
	var readErr *client.ReadFailuresError
	if err != nil && !errors.As(err, &readErr) {
		fmt.Fprintf(stderr, "error: ingest failed: %v\n", err)
		return 1
	}
	// Critic finding F2 (BLOCKING, phase-P fix round): bust the
	// service-identity cache (identcache.go) on every ingest that wrote
	// — a stale cached identity must never outlive the ingest that
	// changed the answer, regardless of TTL (the O15 class). A refused
	// ingest changed nothing.
	if readErr == nil || !readErr.WroteNothing() {
		hookcache.BustIdent()
	}

	// SCIP import (ADR-041 decision 5, SU-9): only after a syntax ingest
	// actually wrote something — a refused ingest (IT-15) leaves the store
	// untouched, and importing typed edges for a service the syntax pass
	// just refused to index would fabricate coverage the symbol table
	// doesn't have. Best-effort per file: one bad SCIP index never aborts
	// the ingest that already succeeded.
	var scipResults []scipImportResult
	if readErr == nil || !readErr.WroteNothing() {
		for _, p := range resolveSCIPPaths(svcPath, scipPaths) {
			imp, ierr := c.ImportSCIP(context.Background(), *service, p)
			if ierr != nil {
				fmt.Fprintf(stderr, "warning: scip import %s: %v\n", p, ierr)
				continue
			}
			scipResults = append(scipResults, scipImportResult{Path: p, Import: imp})
		}
	}

	// [tests] results_dir (vision 33 / ADR-043 stage 0, TC-2): a plain
	// `fathom ingest` also picks up attested #TestResult envelopes when
	// configured, via internal/testevidence.IngestRuns ONLY — never
	// test_suites, so a suite set a separate `fathom tests ingest
	// --manifest` call declared survives an ordinary ingest untouched
	// (IngestRuns' own doc comment). Fail-open like every other
	// config/optional read at a CLI entry point (I14): a problem here
	// warns and never turns an otherwise-successful ingest into a
	// failure.
	newRuns := false
	if readErr == nil || !readErr.WroteNothing() {
		runsBefore, snapErr := lastRunDigests(context.Background(), *storePath, *service)
		if n, cerr := ingestConfiguredTestResults(context.Background(), *storePath, *service, svcPath); cerr != nil {
			fmt.Fprintf(stderr, "warning: [tests] results_dir envelope pickup: %v\n", cerr)
		} else {
			if n > 0 {
				runsAfter, aerr := lastRunDigests(context.Background(), *storePath, *service)
				newRuns = snapErr != nil || aerr != nil || runsBefore != runsAfter
			}
			if n > 0 && !*jsonOut {
				fmt.Fprintf(stdout, "tests: %d run(s) picked up from [tests] results_dir\n", n)
			}
		}
	}

	// `// certifies:` marker scan + certifications compile (vision 33 /
	// ADR-043 stage 1, TC-6's own wiring instruction): a plain `fathom
	// ingest` for a Go service also scans for declared certifications and
	// recompiles the certifications layer from them, so `fathom certify
	// gaps` works even with no attested-run envelope at all (declarations
	// only → every certificate reads unproven, never silently absent).
	// Hash-gated (runCertifyCompile) so a repeat ingest against an
	// unchanged declaration/watermark set is a cheap no-op.
	if readErr == nil || !readErr.WroteNothing() {
		// The certificate sweep re-derives state from files and runs: it
		// runs only when this ingest read a file, deleted one, or picked
		// up a run (O1270).
		sweep := report.Extracted+report.Skipped-report.StatSkipped > 0 || report.Deleted > 0 || newRuns
		ingestConfiguredCertifications(context.Background(), *storePath, *service, svcPath, stdout, stderr, *jsonOut, sweep)
	}

	if *jsonOut {
		return renderIngestJSON(stdout, *service, svcPath, report, readErr, scipResults) // RP-14: ingest_json.go
	}

	fmt.Fprintf(stdout, "service:    %s\n", *service)
	fmt.Fprintf(stdout, "discovered: %d\n", report.Discovered)
	fmt.Fprintf(stdout, "extracted:  %d\n", report.Extracted)
	fmt.Fprintf(stdout, "skipped:    %d\n", report.Skipped)
	if report.StatSkipped > 0 {
		fmt.Fprintf(stdout, "stat-skipped: %d (not read: size, mtime and inode unchanged)\n", report.StatSkipped)
	}
	if report.PeakInflightBytes > 0 {
		fmt.Fprintf(stdout, "peak-inflight: %d bytes of file content held at once\n", report.PeakInflightBytes)
	}
	fmt.Fprintf(stdout, "symbols:    %d\n", report.Symbols)
	// The failed column (IT-14): every file the connector could not read,
	// with its errno text — a wedged reader must never print a clean
	// zero-symbol summary. Non-zero-only like the counters below.
	if report.ReadFailed > 0 {
		fmt.Fprintf(stdout, "read-failed: %d file(s) — stored evidence for them left untouched\n", report.ReadFailed)
		for i, f := range report.ReadFailures {
			if i == readFailuresShown {
				fmt.Fprintf(stdout, "  ... and %d more\n", report.ReadFailed-readFailuresShown)
				break
			}
			fmt.Fprintf(stdout, "  %s: %s\n", f.Path, f.Err)
		}
	}
	// Unclassified/oversize/classified-zero (ADR-021 §2 / TI-6, F5): print
	// only when non-zero so the common case stays as quiet as it always was
	// — a clean chart/repo classifies everything and these counters are 0.
	if report.Unclassified > 0 {
		fmt.Fprintf(stdout, "unclassified: %d\n", report.Unclassified)
	}
	if report.Oversize > 0 {
		fmt.Fprintf(stdout, "oversize:   %d\n", report.Oversize)
	}
	if report.ClassifiedZero > 0 {
		fmt.Fprintf(stdout, "classified-but-zero-symbols: %d\n", report.ClassifiedZero)
	}
	// LB-F9/LB-F10 (language-breadth §5): the dialect gate's counts, only
	// when non-zero.
	if report.SkippedGenerated > 0 {
		fmt.Fprintf(stdout, "skipped_generated: %d\n", report.SkippedGenerated)
	}
	if report.ClassifiedBySniff > 0 {
		fmt.Fprintf(stdout, "classified_by_sniff: %d\n", report.ClassifiedBySniff)
	}
	if report.AmbiguousUnclassified > 0 {
		fmt.Fprintf(stdout, "ambiguous_unclassified: %d\n", report.AmbiguousUnclassified)
	}
	// Unsupported languages (O69): a known programming language with no
	// registered extractor — named and counted rather than left
	// indistinguishable from the generic unclassified/no-extractor path.
	// Sorted for deterministic output.
	if len(report.UnsupportedLanguages) > 0 {
		langs := make([]string, 0, len(report.UnsupportedLanguages))
		for lang := range report.UnsupportedLanguages {
			langs = append(langs, lang)
		}
		sort.Strings(langs)
		fmt.Fprintf(stdout, "unsupported languages (no extractor):\n")
		for _, lang := range langs {
			fmt.Fprintf(stdout, "  %s: %d file(s)\n", lang, report.UnsupportedLanguages[lang])
		}
	}
	// Typed-edge counters (ADR-021 §3) and the ingest honesty disclosure
	// (wave P3 / O82, .fathom/observations.md): printed whenever a typed
	// pass was actually ATTEMPTED this ingest — PackagesSeen>0, or a
	// disclosed failure with nothing yet seen (e.g. a whole-load error) —
	// never the pre-O82 "non-zero packages/failures only" condition, which
	// is exactly what let a Go tree with ZERO typed edges (packages seen,
	// none processed) print nothing at all.
	if report.TypedPackagesSeen > 0 || report.TypedPackages > 0 || report.TypedPackagesFailed > 0 || report.TypedFailed {
		edges := report.TypedEdgesDirect + report.TypedEdgesIfaceDispatch + report.TypedEdgesUnresolved
		fmt.Fprintf(stdout, "typed-edges: %d/%d pkg(s) processed (%d failed), %d edge(s) (%d direct / %d iface-dispatch / %d unresolved)\n",
			report.TypedPackages, report.TypedPackagesSeen, report.TypedPackagesFailed,
			edges, report.TypedEdgesDirect, report.TypedEdgesIfaceDispatch, report.TypedEdgesUnresolved)
		if report.TypedFallback != "" {
			fmt.Fprintf(stdout, "typed-edges: an ambient Go workspace was overridden (%s)\n", report.TypedFallback)
		}
		if report.TypedFailed {
			fmt.Fprintf(stderr, "warning: typed-edge pass failed: %s — test_impact/verify test_reaches read a syntax-only graph for this service until it is fixed\n", report.TypedFailReason)
		}
	}
	// RT-104/RT-105: a gated-off pass is named, never silent.
	if report.TypedSkipped != "" {
		fmt.Fprintf(stdout, "typed-edges: skipped — %s (stored typed edges kept)\n", report.TypedSkipped)
	}
	if report.CUESkipped != "" {
		fmt.Fprintf(stdout, "cue-typed-edges: skipped — %s (stored CUE edges kept)\n", report.CUESkipped)
	}
	// SCIP-imported typed edges (ADR-041 decision 5, SU-9): one line per
	// index, naming the producer so a reader can tell a native Go pass
	// (the block above) from an imported one at a glance.
	for _, r := range scipResults {
		fmt.Fprintf(stdout, "typed_edges: %d edge(s) (%d direct / %d unresolved) via %s [%s]\n",
			r.Import.Report.Edges, r.Import.Report.Direct, r.Import.Report.Unresolved, r.Import.Report.Producer.String(), r.Path)
		// O115 disclosure: a definition this import could not join to the
		// service's own symbol table contributes no edge at all — never
		// silent (a real producer never sets DisplayName, see
		// .fathom/observations.md O115).
		if u := r.Import.Report.Unmapped; u > 0 {
			fmt.Fprintf(stderr, "warning: scip import %s: %d definition(s) could not be mapped to a fathom symbol and contributed no edge (O115)\n", r.Path, u)
		}
	}
	// CUE typed-edge counters (ADR-041 decision 5 / SU-10): printed only
	// when the CUE pass actually ran (packages processed or failed) — a
	// non-CUE service stays as quiet as it always was.
	if report.CUEPackages > 0 || report.CUEPackagesFailed > 0 {
		cueEdges := report.CUEEdgesDirect + report.CUEEdgesUnresolved
		fmt.Fprintf(stdout, "cue: %d pkg(s) processed (%d failed), %d edge(s) (%d direct / %d unresolved)\n",
			report.CUEPackages, report.CUEPackagesFailed, cueEdges, report.CUEEdgesDirect, report.CUEEdgesUnresolved)
	}
	// Per-file resolver counters (ADR-041 decision 5 / SU-11): printed only
	// when it produced at least one edge — always heuristic tier, never
	// presented as typed (core.EdgeConfidenceOf).
	if report.FileResolvedEdges > 0 {
		fmt.Fprintf(stdout, "file_resolved: %d (%d file(s) reused their recorded imports)\n", report.FileResolvedEdges, report.FileResolveReused)
	}
	// RT-108: import-parse faults are named, never a silent skip; a skipped
	// pass says why.
	if report.FileResolveFaults > 0 {
		fmt.Fprintf(stderr, "warning: file_resolved: %d file(s) whose imports could not be parsed (%s) — their calls resolve no import edges until their bytes change\n",
			report.FileResolveFaults, strings.Join(report.FileResolveFaultFiles, ", "))
	}
	if report.FileResolveSkipped != "" {
		fmt.Fprintf(stdout, "file_resolved: skipped — %s (stored edges kept)\n", report.FileResolveSkipped)
	}
	printDocIndex(stdout, stderr, report)
	// Scoped config (docs/design/scoped-config.md §4 disclosure): report the
	// project/user pattern-count split that governed this ingest. Ingest has
	// already succeeded above (Discover already ran LoadIgnoreScoped as its
	// own enforcement point — a violating user-scope file would have failed
	// the ingest itself, never reached this line), so a second, cheap read
	// here is purely for display and cannot itself fail loudly.
	if _, counts, ierr := connector.LoadIgnoreScopedFor(svcPath, *service); ierr == nil && (counts.Project > 0 || counts.User > 0 || counts.Service > 0) {
		fmt.Fprintf(stdout, "excluded: %d pattern(s) (%d project, %d user-scope ~/.fathom/ignore, %d service-scoped ignore.d/%s)\n",
			counts.Project+counts.User+counts.Service, counts.Project, counts.User, counts.Service, *service)
	}
	printDefaultIgnoredDirs(stdout, report.DefaultIgnoredDirs)
	printPassTimes(stdout, report.PassTimes)
	if readErr != nil {
		outcome := "degraded"
		switch {
		case readErr.Refused:
			outcome = "refused"
		case readErr.Incomplete:
			outcome = "incomplete"
		}
		fmt.Fprintf(stderr, "error: ingest %s: %v\n", outcome, readErr)
		if h := readErr.Hint(); h != "" {
			fmt.Fprintf(stderr, "hint: %s\n", h)
		}
		return 1
	}
	// ok:false-grade disclosure (wave P3 / O82): the syntax index is
	// untouched (kept, never rolled back), but a Go tree whose typed pass
	// produced nothing to trust must never exit 0 as though nothing were
	// wrong — the exact silent gap that let a push be refused downstream
	// with no earlier signal in the ingest that ran it.
	if report.TypedFailed {
		fmt.Fprintf(stderr, "error: ingest degraded: typed-edge pass failed: %s\n", report.TypedFailReason)
		return 1
	}
	return 0
}

// ingestConfiguredTestResults implements the `[tests] results_dir` half of
// vision 33 / ADR-043 stage 0 (TC-2): when a project or user
// .fathom/config.toml names a results_dir, a PLAIN `fathom ingest` also
// loads and joins its attested #TestResult envelopes — never test_suites
// (see internal/testevidence.IngestRuns' own doc comment; only `fathom
// tests ingest --manifest` declares suites). resultsDir resolves relative
// to svcPath (the service root this ingest just walked) and must stay
// inside it (config.RepoPath, SEC-2). Returns the number of runs ingested;
// (0, nil) when results_dir is unset or the directory does not exist —
// most services have no test evidence at all, and that is not a failure.
func ingestConfiguredTestResults(ctx context.Context, storePath, service, svcPath string) (int, error) {
	loaded, err := config.Load(config.DefaultUserPath(), config.DefaultProjectPath())
	if err != nil {
		return 0, fmt.Errorf("load config: %w", err)
	}
	resultsDir := loaded.Config.Tests.ResultsDir
	if resultsDir == "" {
		return 0, nil
	}
	// SEC-2: results_dir is a repo-path field — inside the service root,
	// never an absolute path, a .. escape or a symlink out (named error).
	resultsPath, perr := config.RepoPath(svcPath, resultsDir)
	if perr != nil {
		return 0, fmt.Errorf("[tests] results_dir %w", perr)
	}
	if _, statErr := os.Stat(resultsPath); statErr != nil {
		if os.IsNotExist(statErr) {
			return 0, nil
		}
		return 0, fmt.Errorf("stat %s: %w", resultsPath, statErr)
	}

	runs, err := testevidence.LoadEnvelopes(resultsPath)
	if err != nil {
		return 0, fmt.Errorf("load envelopes from %s: %w", resultsPath, err)
	}
	if len(runs) == 0 {
		return 0, nil
	}

	st, err := store.Open(storePath)
	if err != nil {
		return 0, fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	watermark := testsIngestWatermark(ctx, st, service)
	report, err := testevidence.IngestRuns(ctx, st, service, watermark, runs)
	if err != nil {
		return 0, fmt.Errorf("ingest runs: %w", err)
	}
	return report.RunsWritten, nil
}

// lastRunDigests is a stable fingerprint of each joined test's newest
// attested run: it differs before and after a pickup exactly when the
// pickup added a run that became some test's newest, the only new run the
// certificate sweep can read differently.
func lastRunDigests(ctx context.Context, storePath, service string) (string, error) {
	st, err := store.Open(storePath)
	if err != nil {
		return "", err
	}
	defer st.Close()
	runs, err := st.NewestTestCaseRuns(ctx, service)
	if err != nil {
		return "", err
	}
	parts := make([]string, 0, len(runs))
	for _, r := range runs {
		parts = append(parts, r.TestSymbolKey+"="+r.RunDigest)
	}
	sort.Strings(parts)
	return strings.Join(parts, "\n"), nil
}

// ingestConfiguredCertifications implements the marker-scan half of
// vision 33 / ADR-043 stage 1's plain-ingest wiring (TC-6): scan svcPath
// for `// certifies:` markers (the same internal/testevidence.
// IngestCertifyMarkers `fathom tests ingest` uses) and, when anything is
// declared, hash-gate and run the certifications compile
// (runCertifyCompile, certify_compile.go). Fail-open like every other
// best-effort ingest enrichment at this CLI entry point: a problem here
// warns and never turns an otherwise-successful ingest into a failure.
func ingestConfiguredCertifications(ctx context.Context, storePath, service, svcPath string, stdout, stderr io.Writer, jsonOut, sweep bool) {
	st, err := store.Open(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "warning: certify: open store: %v\n", err)
		return
	}
	defer st.Close()

	watermark := testsIngestWatermark(ctx, st, service)
	markerReport, err := testevidence.IngestCertifyMarkers(ctx, st, service, watermark, svcPath)
	if err != nil {
		fmt.Fprintf(stderr, "warning: certify: scan certifies markers: %v\n", err)
		return
	}
	for _, w := range markerReport.Warnings {
		fmt.Fprintf(stderr, "warning: certify: malformed certifies marker: %s\n", w)
	}
	if markerReport.Declared == 0 {
		return
	}
	if !jsonOut {
		fmt.Fprintf(stdout, "certify: %d declared certification(s) found in %d file(s)\n", markerReport.Declared, markerReport.FilesScanned)
	}
	runCertifyCompileGated(ctx, st, service, watermark, svcPath, stdout, stderr, jsonOut, sweep)
}

// scipImportResult pairs one --scip (or auto-detected) index's path with
// its import outcome — kept as its own type so both the text summary and
// renderIngestJSON (ingest_json.go) read the same value.
type scipImportResult struct {
	Path   string
	Import client.ImportSCIPResult
}

// resolveSCIPPaths returns the SCIP index files this ingest should import
// (ADR-041 decision 5, SU-9): explicit --scip values verbatim when any were
// given, else auto-detection — index.scip at the service root, plus every
// *.scip under .fathom/scip/ — the two locations docs/usage.md documents.
// Auto-detection is silent about finding nothing: most ingests are not of
// a language with a SCIP producer at all.
func resolveSCIPPaths(root string, explicit []string) []string {
	if len(explicit) > 0 {
		return explicit
	}
	var out []string
	if p := filepath.Join(root, "index.scip"); fileExists(p) {
		out = append(out, p)
	}
	if matches, err := filepath.Glob(filepath.Join(root, ".fathom", "scip", "*.scip")); err == nil {
		sort.Strings(matches)
		out = append(out, matches...)
	}
	return out
}

// defaultIgnoredShown caps how many default-ignored directories the ingest
// summary names inline; the count is always exact.
const defaultIgnoredShown = 8

// printDefaultIgnoredDirs names the directories the connector's default
// cache list pruned (RT-145) — counted and named, never a silent drop.
func printDefaultIgnoredDirs(w io.Writer, dirs []string) {
	if len(dirs) == 0 {
		return
	}
	shown := dirs
	more := ""
	if len(shown) > defaultIgnoredShown {
		shown = shown[:defaultIgnoredShown]
		more = fmt.Sprintf(", … and %d more", len(dirs)-defaultIgnoredShown)
	}
	fmt.Fprintf(w, "excluded: %d default-ignored cache dir(s) (%s%s) — re-include one with [connector] include in .fathom/config.toml\n",
		len(dirs), strings.Join(shown, ", "), more)
}

// ingestRunner runs `fathom ingest` (registered below; EW-45).
type ingestRunner struct{}

func (ingestRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdIngest(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "ingest", Group: "index", Order: 10, Run: ingestRunner{},
		Summary: "Ingest a service into the knowledge store"})
}
