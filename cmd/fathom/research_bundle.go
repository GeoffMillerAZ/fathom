package main

// research_bundle.go — `fathom research bundle`: the flywheel's opt-in
// publish step (docs/design/research-flywheel.md's Privacy section). It
// aggregates the exact same local evidence `fathom research report` reads
// down into internal/research/bundle.Input (typed, already-reduced scalar
// fields — see that type's own doc comment for why this is the sanitization
// boundary) and calls bundle.Build, which is the only place a Bundle value
// is ever constructed. NO network I/O anywhere in this file — fathom never
// phones home; --dry-run prints the exact bundle and exits without writing
// anything.
import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"
	"github.com/geoffmilleraz/fathom/internal/contextbench"
	"github.com/geoffmilleraz/fathom/internal/metrics"
	"github.com/geoffmilleraz/fathom/internal/research/bundle"
)

// defaultResearchBundleOutDir is where `fathom research bundle` writes its
// two shareable files (critic Finding 2, 2026-07-28): a dedicated
// subdirectory of defaultResearchOutDir containing ONLY bundle.json and
// bundle.md — never report.md (deliberately unsanitized: plaintext service
// names, the hooks log path, the username in it) and never the salt
// (relocated entirely outside this tree, hash.go). A user who shares "the
// bundle folder" in isolation shares exactly the two sanitized files and
// nothing else; the warning printed after every write still tells them not
// to share the PARENT workspace/artifacts/research/ directory, which does
// hold report.md.
const defaultResearchBundleOutDir = defaultResearchOutDir + "/bundle"

// labelPattern is fathom's own tool/surface/bench-arm label shape:
// lowercase ASCII letters, digits, underscore, 1-64 chars — the shape every
// fathom-defined tool/command/bench-arm name actually has. A call-log Tool
// or Surface string is CLIENT-CONTROLLED, not fathom-controlled, despite
// being typed as a plain Go string: internal/mcp/dispatch.go logs the raw
// REQUESTED tool name to the call log even when it fails to resolve to a
// real tool, and the daemon that logs it is always-on and unauthenticated
// by design (bare /rpc serves the full registry to any local process) — so
// any local process, or a prompt-injected agent, can put arbitrary text
// (a path, a key-shaped string, Markdown-table-breaking characters) into
// either field. Critic Finding 1 (2026-07-28) proved this end-to-end: a
// fake path in Tool reached a written, schema-passing bundle before this
// fix. sanitizeLabel is what keeps that trust from resting on a doc
// comment; schema/research-bundle/schema.cue's #Label is the same
// constraint enforced a second, independent way (a recipient's own
// validation catches it even if this function had a bug).
var labelPattern = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)

// unrecognizedLabel is the sentinel bucket a non-conforming tool/surface/
// bench-arm label collapses into. It deliberately matches labelPattern
// itself, so the CUE schema needs no alternation to allow it.
const unrecognizedLabel = "unrecognized"

// sanitizeLabel returns s unchanged if it matches labelPattern, else
// unrecognizedLabel. allowEmpty additionally accepts "" — the surface
// field's own legitimate bare/unscoped value (mcp.CallLogger.SurfaceTraffic's
// doc comment); Tool and bench Arm never allow empty.
func sanitizeLabel(s string, allowEmpty bool) string {
	if allowEmpty && s == "" {
		return s
	}
	if labelPattern.MatchString(s) {
		return s
	}
	return unrecognizedLabel
}

// cmdResearchBundle implements `fathom research bundle`.
func cmdResearchBundle(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("research bundle", stderr)
	service := fs.String("service", "", "optional service ID to scope the underlying report to")
	since := fs.Duration("since", 7*24*time.Hour, "report window, measured back from now")
	root := fs.String("root", ".", "project root — .fathom/observations.md, the salt file, and --out resolve under here")
	callLogFlag := fs.String("call-log", "", "MCP tool-call log (JSONL); default: [daemon].call_log in config.toml, else "+defaultCallLogPath)
	hooksLogFlag := fs.String("hooks-log", "", "hooks beat log (JSONL); default: $FATHOM_HOOKS_LOG or ~/.fathom/hooks.jsonl")
	benchDirFlag := fs.String("bench-dir", "", "context-bench results directory; default: --root/"+defaultBenchDir)
	out := fs.String("out", "", "output directory for bundle.json/bundle.md; default: --root/"+defaultResearchBundleOutDir)
	dryRun := fs.Bool("dry-run", false, "print the exact bundle to stdout; write nothing")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	now := time.Now().UTC()
	sinceT := now.Add(-*since)

	callLogPath := *callLogFlag
	if callLogPath == "" {
		callLogPath = resolveCallLogPath()
	}
	entries, _, callLogRead := readCallLog(callLogPath)

	hooksLogPath := *hooksLogFlag
	if hooksLogPath == "" {
		hooksLogPath = beatLogPath()
	}
	beats, hooksLogRead := readBeatLog(hooksLogPath)

	obsContent, obsRead, obsErr := readObservations(*root)
	if obsErr != nil {
		fmt.Fprintf(stderr, "warn: research bundle: reading %s: %v\n", observationsPath(*root), obsErr)
	}

	benchDir := *benchDirFlag
	if benchDir == "" {
		benchDir = filepath.Join(*root, filepath.FromSlash(defaultBenchDir))
	}
	_, benchResults, benchErr := readLatestBenchResults(benchDir)
	if benchErr != nil {
		fmt.Fprintf(stderr, "warn: research bundle: reading bench results under %s: %v\n", benchDir, benchErr)
	}

	// LoadOrCreateSalt resolves (and, on first run in this repo, migrates or
	// generates) the salt at workspace/state/research.salt — deliberately
	// outside workspace/artifacts/research/ entirely (critic Finding 2).
	salt, err := bundle.LoadOrCreateSalt(*root)
	if err != nil {
		fmt.Fprintf(stderr, "error: research bundle: %v\n", err)
		return 1
	}

	in := aggregateBundleInput(strings.ToLower(*service), sinceT, now, entries, callLogRead, beats, hooksLogRead, obsContent, obsRead, benchResults, salt)
	b := bundle.Build(in)

	if err := bundle.Validate(b); err != nil {
		// Never write or print an invalid bundle: an allowlist violation is
		// a bug in aggregateBundleInput/Build, not a sharable artifact.
		fmt.Fprintf(stderr, "error: research bundle: built bundle failed schema validation (this is a bug, please file it): %v\n", err)
		return 1
	}

	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "error: marshal bundle: %v\n", err)
		return 1
	}
	md := bundle.RenderMarkdown(b)

	if *dryRun {
		fmt.Fprintln(stdout, string(data))
		fmt.Fprintln(stderr, "\n--dry-run: nothing written, nothing transmitted. This is a preview of the exact bundle "+
			"`fathom research bundle` would write as a local file. Sharing it anywhere is entirely your own manual choice.")
		return 0
	}

	outDir := *out
	if outDir == "" {
		outDir = filepath.Join(*root, filepath.FromSlash(defaultResearchBundleOutDir))
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "error: mkdir %s: %v\n", outDir, err)
		return 1
	}
	jsonPath := filepath.Join(outDir, "bundle.json")
	mdPath := filepath.Join(outDir, "bundle.md")
	if err := os.WriteFile(jsonPath, data, 0o644); err != nil {
		fmt.Fprintf(stderr, "error: writing %s: %v\n", jsonPath, err)
		return 1
	}
	if err := os.WriteFile(mdPath, []byte(md), 0o644); err != nil {
		fmt.Fprintf(stderr, "error: writing %s: %v\n", mdPath, err)
		return 1
	}

	fmt.Fprintf(stdout, "wrote %s\n", jsonPath)
	fmt.Fprintf(stdout, "wrote %s\n", mdPath)
	fmt.Fprintln(stdout, "\nThese are plain local files. Nothing was transmitted anywhere — fathom never phones home. "+
		"Review them yourself before sharing; sharing is entirely your own manual choice (email, paste, upload — whatever you pick).")
	fmt.Fprintf(stdout, "\nShare only these two files (or this bundle/ subdirectory in isolation) — never the parent "+
		"%s directory, which also contains report.md (that file is NOT sanitized: it carries plaintext service names "+
		"and the local hooks-log path).\n", filepath.Join(*root, filepath.FromSlash(defaultResearchOutDir)))
	return 0
}

// aggregateBundleInput reduces raw local evidence down to
// internal/research/bundle.Input's typed, already-aggregated shape. This is
// the ONE function in the whole feature that is allowed to look at raw call
// log rows and observation text — and it only ever extracts counts/rates
// from them, never a free-text field, into Input's scalar fields. Every
// Tool/Surface/Arm value is passed through sanitizeLabel before it reaches
// Input (critic Finding 1) — Input's own doc comment states this as the
// caller's responsibility, and this is the caller. Once this function
// returns, bundle.Build never sees anything but numbers, label-shaped
// strings, and the salted hashes it computes itself.
func aggregateBundleInput(
	service string,
	since, until time.Time,
	entries []adoptionCallEntry,
	callLogRead bool,
	beats []heartbeat.Beat,
	hooksLogRead bool,
	obsContent string,
	obsRead bool,
	benchResults *contextbench.RunResults,
	salt string,
) bundle.Input {
	surfaceCalls := map[string]int{}
	toolCalls := map[string]int{}
	serviceCalls := map[string]int{}
	var latencies []float64
	total := 0
	var verifyClaimed, verifyVerified, verifyFailing, verifyInstrumented, verifyPredates int
	var reviewSnapshots []metrics.ReviewCallSnapshot
	staleness := 0

	for _, e := range entries {
		ts, ok := parseCallLogTS(e.TS)
		if !ok || ts.Before(since) || ts.After(until) {
			continue
		}
		if service != "" && !strings.EqualFold(e.Service, service) {
			continue
		}
		total++
		surfaceCalls[sanitizeLabel(e.Surface, true)]++
		toolCalls[sanitizeLabel(e.Tool, false)]++
		latencies = append(latencies, e.LatencyMS)
		if e.Service != "" {
			serviceCalls[strings.ToLower(e.Service)]++
		}
		// Matched against the RAW e.Tool (not the sanitized bucket) is
		// intentional and safe: these are exact-literal comparisons against
		// fathom's own known tool names, so a hostile string simply never
		// matches and falls through harmlessly — it still lands in
		// toolCalls[unrecognizedLabel] above.
		if e.Tool == "staleness_report" {
			staleness++
		}
		if e.Tool == "verify_plan" || e.Tool == "review" {
			if e.VerdictOutcome == nil {
				verifyPredates++
			} else {
				verifyInstrumented++
				switch e.Tool {
				case "verify_plan":
					verifyClaimed += e.VerdictOutcome["claimed"]
					verifyVerified += e.VerdictOutcome["verified"]
					verifyFailing += e.VerdictOutcome["failing"]
				case "review":
					// O20's fix — see metrics.LatestReviewActiveFindings's
					// doc comment: a "review" call's verdict_outcome is a
					// snapshot, never an event to sum across the window.
					reviewSnapshots = append(reviewSnapshots, metrics.ReviewCallSnapshot{At: ts, Outcome: e.VerdictOutcome})
				}
			}
		}
	}
	reviewActive, _, _ := metrics.LatestReviewActiveFindings(reviewSnapshots)

	in := bundle.Input{
		WindowSince:   since.Format(time.RFC3339),
		WindowUntil:   until.Format(time.RFC3339),
		GeneratedAt:   until.Format(time.RFC3339),
		FathomVersion: Version,
		CommitSHA:     shortRevision(),
		CommitDirty:   revisionDirty(),

		CallLogRead:  callLogRead,
		HooksLogRead: hooksLogRead,

		TotalCalls: total,

		VerifyClaimed:                 verifyClaimed,
		VerifyVerified:                verifyVerified,
		VerifyFailing:                 verifyFailing,
		VerifyInstrumentedCalls:       verifyInstrumented,
		VerifyPredatesInstrumentation: verifyPredates,
		ReviewActiveFindings:          reviewActive,

		StalenessReportCalls: staleness,

		ServicePlaintextCalls: serviceCalls,
		Salt:                  salt,
	}

	for s, n := range surfaceCalls {
		in.SurfaceTraffic = append(in.SurfaceTraffic, bundle.SurfaceCount{Surface: s, Calls: n})
	}
	for t, n := range toolCalls {
		in.ToolCalls = append(in.ToolCalls, bundle.ToolCount{Tool: t, Calls: n})
	}
	sort.Slice(in.SurfaceTraffic, func(i, j int) bool { return in.SurfaceTraffic[i].Surface < in.SurfaceTraffic[j].Surface })
	sort.Slice(in.ToolCalls, func(i, j int) bool { return in.ToolCalls[i].Tool < in.ToolCalls[j].Tool })

	if len(latencies) > 0 {
		sort.Float64s(latencies)
		p50 := percentile(latencies, 0.50)
		p95 := percentile(latencies, 0.95)
		in.LatencyP50MS, in.LatencyP95MS = &p50, &p95
	}

	seen, answered, servedEmitted := 0, 0, 0
	for _, beat := range beats {
		if beat.Channel != "edit-hook" || beat.At.Before(since) || beat.At.After(until) {
			continue
		}
		switch beat.Kind {
		case "seen":
			seen++
		case "answered":
			answered++
			if beat.Meta["emitted"] == "true" {
				servedEmitted++
			}
		}
	}
	in.InjectionSeen, in.InjectionAnswered, in.ObligationsServed = seen, answered, servedEmitted
	if seen > 0 {
		rate := float64(answered) / float64(seen)
		in.InjectionCoverageRate = &rate
	}

	if obsRead {
		t := nextObservationID(obsContent) - 1
		o := countOpenObservations(obsContent)
		in.TotalObservations, in.OpenObservations = &t, &o
	}

	if benchResults != nil {
		arms := make([]string, 0, len(benchResults.Summary))
		for a := range benchResults.Summary {
			arms = append(arms, a)
		}
		sort.Strings(arms)
		for _, a := range arms {
			agg := benchResults.Summary[a]
			in.BenchArms = append(in.BenchArms, bundle.BenchArmCount{
				Arm: sanitizeLabel(a, false), TaskCount: agg.TaskCount, SuccessCount: agg.SuccessCount,
				SuccessRate: agg.SuccessRate, MeanTokens: agg.MeanTokens, MeanWallMS: agg.MeanWallMS,
			})
		}
	}

	return in
}
