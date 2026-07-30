package main

// research_report.go — `fathom research report`: the flywheel's collect
// step. One deterministic-given-inputs report over local evidence already
// produced by fathom's own instrumentation (docs/design/research-flywheel.md):
// the MCP call log (internal/mcp/calllog.go's JSONL wire format, read the
// same way instrument_adoption.go does — same package, no import needed),
// the hooks beat log (beats.go's heartbeat JSONL, per-user), .fathom/
// observations.md (observations.go), and, when present, a context-bench
// results artifact (internal/contextbench).
//
// Every metric that cannot be computed from what is actually on disk prints
// an explicit "not measurable: <reason>" note rather than a fabricated
// zero (docs/vision/11-dial-to-eleven.md law 3, "never silent"). This
// command never queries the live daemon or opens a store: every input is a
// local file, so there is nothing to "degrade" when the daemon is down —
// the report is unaffected by daemon reachability by construction.

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"
	"github.com/geoffmilleraz/fathom/internal/contextbench"
)

// defaultResearchOutDir is where `fathom research report`/`bundle` write
// their artifacts (repo-scoped, under the workspace/ convention).
const defaultResearchOutDir = "workspace/artifacts/research"

// defaultBenchDir is where scripts/context-bench.sh writes its results
// artifacts (B1-2, docs/plans/falsifiability.md).
const defaultBenchDir = "workspace/artifacts/context-bench"

// ResearchReport is the flywheel's collect-step output — markdown and JSON
// renderings of the exact same value (renderResearchReportMarkdown /
// json.Marshal). Every optional/honestly-uncertain metric carries an
// explicit note string alongside its value; a present value with an empty
// note is a real measurement, never a guess.
type ResearchReport struct {
	GeneratedAt string `json:"generated_at"`
	Since       string `json:"since"`
	Until       string `json:"until"`
	Service     string `json:"service,omitempty"`

	CallLogPath           string `json:"call_log_path"`
	CallLogRead           bool   `json:"call_log_read"`
	TotalCallsInWindow    int    `json:"total_calls_in_window"`
	SkippedMalformedLines int    `json:"skipped_malformed_lines"`

	SurfaceTraffic []SurfaceTrafficRow `json:"surface_traffic"`
	TopTools       []ToolCallRow       `json:"top_tools"`

	LatencyP50MS *float64 `json:"latency_p50_ms"`
	LatencyP95MS *float64 `json:"latency_p95_ms"`
	LatencyNote  string   `json:"latency_note,omitempty"`

	HooksLogPath          string   `json:"hooks_log_path"`
	HooksLogRead          bool     `json:"hooks_log_read"`
	InjectionSeen         int      `json:"injection_seen"`
	InjectionAnswered     int      `json:"injection_answered"`
	InjectionCoverageRate *float64 `json:"injection_coverage_rate"`
	InjectionScopeNote    string   `json:"injection_scope_note"`

	ObligationsServed      int    `json:"obligations_served"`
	ObligationsEvictedNote string `json:"obligations_evicted_note"`

	// Guarded-edits coverage (docs/design/guarded-edits.md §2.4 / phase-O
	// instrumentation): pre|post split of the SAME edit-hook seen/answered
	// beats above. GuardPreSeen/GuardPreServed are 0 whenever the --pre
	// hook has never fired in this window (not installed, or `fathom
	// init` predates guarded edits) — GuardCoverageNote names that
	// explicitly rather than presenting a silent zero as "no guards fire."
	// GuardDedupSuppressed counts guardstamp.go's post-hook collapses
	// (design §2.3's "no double-billing").
	GuardPreSeen         int    `json:"guard_pre_seen"`
	GuardPreServed       int    `json:"guard_pre_served"`
	GuardPostSeen        int    `json:"guard_post_seen"`
	GuardPostServed      int    `json:"guard_post_served"`
	GuardDedupSuppressed int    `json:"guard_dedup_suppressed"`
	GuardCoverageNote    string `json:"guard_coverage_note"`

	// Module-tagged serving (docs/design/platform-modules.md §3.5): rules
	// served per bound module, from the edit-hook's "module-served" beats
	// (both pre and post phases combined — a bound module either serves or
	// it doesn't, independent of which hook moment carried it). Empty map
	// is the pre-phase-O default (no bindings configured anywhere this
	// window observed) and ModuleServingNote names that explicitly.
	ModuleRulesServed map[string]int `json:"module_rules_served,omitempty"`
	ModuleServingNote string         `json:"module_serving_note"`

	VerifyClaimed                 int    `json:"verify_claimed"`
	VerifyVerified                int    `json:"verify_verified"`
	VerifyFailing                 int    `json:"verify_failing"`
	VerifyInstrumentedCalls       int    `json:"verify_instrumented_calls"`
	VerifyPredatesInstrumentation int    `json:"verify_predates_instrumentation"`
	ReviewActiveFindings          int    `json:"review_active_findings"`
	VerifyNote                    string `json:"verify_note"`

	StalenessReportCalls int    `json:"staleness_report_calls"`
	StalenessNote        string `json:"staleness_note,omitempty"`
	ReAnchorNote         string `json:"re_anchor_note"`

	AgentSurfaceCalls  int    `json:"agent_surface_calls"`
	DiscoveryShareNote string `json:"discovery_share_note"`

	ObservationsPath       string         `json:"observations_path"`
	ObservationsRead       bool           `json:"observations_read"`
	TotalObservations      *int           `json:"total_observations"`
	OpenObservations       *int           `json:"open_observations"`
	ObservationsByTool     map[string]int `json:"observations_by_tool,omitempty"`
	ObservationsWindowNote string         `json:"observations_window_note"`

	BenchResultsPath string            `json:"bench_results_path,omitempty"`
	BenchGeneratedAt string            `json:"bench_generated_at,omitempty"`
	BenchArms        []BenchArmSummary `json:"bench_arms,omitempty"`
	BenchNote        string            `json:"bench_note"`
}

// BenchArmSummary is one context-bench arm's aggregate outcome, carried
// straight from internal/contextbench.Aggregate.
type BenchArmSummary struct {
	Arm          string  `json:"arm"`
	TaskCount    int     `json:"task_count"`
	SuccessCount int     `json:"success_count"`
	SuccessRate  float64 `json:"success_rate"`
	TotalTokens  int64   `json:"total_tokens"`
	MeanTokens   float64 `json:"mean_tokens"`
	MeanWallMS   float64 `json:"mean_wall_ms"`
}

// cmdResearchReport implements `fathom research report`.
func cmdResearchReport(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("research report", stderr)
	service := fs.String("service", "", "optional service ID to scope the report to (matches the call log's service field)")
	since := fs.Duration("since", 7*24*time.Hour, "report window, measured back from now")
	root := fs.String("root", ".", "project root — .fathom/observations.md and --out resolve under here")
	callLogFlag := fs.String("call-log", "", "MCP tool-call log (JSONL); default: [daemon].call_log in config.toml, else "+defaultCallLogPath)
	hooksLogFlag := fs.String("hooks-log", "", "hooks beat log (JSONL); default: "+"$FATHOM_HOOKS_LOG or ~/.fathom/hooks.jsonl")
	benchDirFlag := fs.String("bench-dir", "", "context-bench results directory; default: --root/"+defaultBenchDir)
	out := fs.String("out", "", "output directory for report.md/report.json; default: --root/"+defaultResearchOutDir)
	jsonOut := fs.Bool("json", false, "print JSON to stdout instead of the human report (files are always written in both formats)")
	noWrite := fs.Bool("no-write", false, "print only; skip writing report.md/report.json")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	now := time.Now
	sinceT := now().UTC().Add(-*since)
	untilT := now().UTC()

	callLogPath := *callLogFlag
	if callLogPath == "" {
		callLogPath = resolveCallLogPath()
	}
	entries, skippedMalformed, callLogRead := readCallLog(callLogPath)

	hooksLogPath := *hooksLogFlag
	if hooksLogPath == "" {
		hooksLogPath = beatLogPath()
	}
	beats, hooksLogRead := readBeatLog(hooksLogPath)

	obsContent, obsRead, obsErr := readObservations(*root)
	if obsErr != nil {
		fmt.Fprintf(stderr, "warn: research report: reading %s: %v\n", observationsPath(*root), obsErr)
	}

	benchDir := *benchDirFlag
	if benchDir == "" {
		benchDir = filepath.Join(*root, filepath.FromSlash(defaultBenchDir))
	}
	benchPath, benchResults, benchErr := readLatestBenchResults(benchDir)
	if benchErr != nil {
		fmt.Fprintf(stderr, "warn: research report: reading bench results under %s: %v\n", benchDir, benchErr)
	}

	report := buildResearchReport(researchReportInputs{
		service: strings.ToLower(*service),
		since:   sinceT,
		until:   untilT,
		now:     untilT,

		callLogPath:      callLogPath,
		callLogRead:      callLogRead,
		entries:          entries,
		skippedMalformed: skippedMalformed,

		hooksLogPath: hooksLogPath,
		hooksLogRead: hooksLogRead,
		beats:        beats,

		observationsPath: observationsPath(*root),
		obsRead:          obsRead,
		obsContent:       obsContent,

		benchPath:    benchPath,
		benchResults: benchResults,
	})

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
	} else {
		renderResearchReportText(stdout, report)
	}

	if !*noWrite {
		outDir := *out
		if outDir == "" {
			outDir = filepath.Join(*root, filepath.FromSlash(defaultResearchOutDir))
		}
		if err := writeResearchReport(outDir, report); err != nil {
			fmt.Fprintf(stderr, "error: writing report to %s: %v\n", outDir, err)
			return 1
		}
		fmt.Fprintf(stderr, "wrote %s\n", filepath.Join(outDir, "report.md"))
		fmt.Fprintf(stderr, "wrote %s\n", filepath.Join(outDir, "report.json"))
	}
	return 0
}

// readBeatLog reads the hooks beat log at path. read is false when the file
// doesn't exist or can't be opened — reported honestly, mirroring
// readCallLog's own absent-vs-empty discipline.
func readBeatLog(path string) (beats []heartbeat.Beat, read bool) {
	if path == "" {
		return nil, false
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	beats, err = heartbeat.Load(f, true) // acceptPartial: a torn tail write must not blank the whole history
	if err != nil {
		return nil, false
	}
	return beats, true
}

// readLatestBenchResults finds the most recently-named results-*.json under
// dir (context-bench's own filename convention, scripts/context-bench.sh)
// and parses it as internal/contextbench.RunResults. A missing directory or
// zero matching files is reported as (path="", nil, nil) — not an error;
// the caller turns that into an honest "not measurable" note.
func readLatestBenchResults(dir string) (path string, results *contextbench.RunResults, err error) {
	entries, rerr := os.ReadDir(dir)
	if rerr != nil {
		return "", nil, nil // absent dir: honest "no bench has run" state, not an error
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), "results-") && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return "", nil, nil
	}
	sort.Strings(names) // results-<RFC3339-ish-timestamp>.json sorts chronologically as strings
	latest := names[len(names)-1]
	full := filepath.Join(dir, latest)
	data, rerr := os.ReadFile(full)
	if rerr != nil {
		return "", nil, fmt.Errorf("read %s: %w", full, rerr)
	}
	var rr contextbench.RunResults
	if uerr := json.Unmarshal(data, &rr); uerr != nil {
		return "", nil, fmt.Errorf("parse %s: %w", full, uerr)
	}
	return full, &rr, nil
}

// researchReportInputs bundles buildResearchReport's parameters — pure over
// its inputs, exactly like buildAdoptionReport, so it is unit-testable
// without any I/O.
type researchReportInputs struct {
	service      string
	since, until time.Time
	now          time.Time

	callLogPath      string
	callLogRead      bool
	entries          []adoptionCallEntry
	skippedMalformed int

	hooksLogPath string
	hooksLogRead bool
	beats        []heartbeat.Beat

	observationsPath string
	obsRead          bool
	obsContent       string

	benchPath    string
	benchResults *contextbench.RunResults
}

// buildResearchReport is the pure, deterministic core (buildAdoptionReport's
// sibling): all I/O happens in cmdResearchReport, this function does none.
func buildResearchReport(in researchReportInputs) ResearchReport {
	r := ResearchReport{
		GeneratedAt: in.now.Format(time.RFC3339),
		Since:       in.since.Format(time.RFC3339),
		Until:       in.until.Format(time.RFC3339),
		Service:     in.service,

		CallLogPath: in.callLogPath,
		CallLogRead: in.callLogRead,

		HooksLogPath: in.hooksLogPath,
		HooksLogRead: in.hooksLogRead,
		InjectionScopeNote: "per-user, all repos this user's hooks fired in — the hooks beat log " +
			"(~/.fathom/hooks.jsonl or $FATHOM_HOOKS_LOG) carries no repo/service dimension today, so this " +
			"number cannot be scoped to --service even when one is given",

		ObligationsEvictedNote: "not measurable: no eviction event is logged anywhere in fathom today " +
			"(the edit-hook beat log records seen/answered/error only)",
		ReAnchorNote: "not measurable: no re-anchor event instrumentation exists in fathom today",
		DiscoveryShareNote: "not measurable: fathom cannot observe an agent's non-fathom tool calls " +
			"(grep, read, …) inside its own transcript locally, so a fathom-vs-grep discovery share has " +
			"no denominator; agent_surface_calls below is the numerator alone, informational only",

		ObservationsPath: in.observationsPath,
		ObservationsRead: in.obsRead,
		ObservationsWindowNote: "not windowed by --since: .fathom/observations.md entries carry no " +
			"timestamp field, so the counts below cover the file's entire history, not just this window",

		BenchNote: "no context-bench results artifact found under the bench directory " +
			"(scripts/context-bench.sh writes results-<ts>.json)",
	}

	// ─── call log: surface traffic, top tools, latency percentiles, staleness/agent-surface counts ───
	var latencies []float64
	surfaceCalls := map[string]int{}
	toolCalls := map[string]int{}
	staleness := 0
	agentSurface := 0
	var verifyClaimed, verifyVerified, verifyFailing, verifyInstrumented, verifyPredates, reviewActive int

	for _, e := range in.entries {
		ts, ok := parseCallLogTS(e.TS)
		if !ok || ts.Before(in.since) || ts.After(in.until) {
			continue
		}
		if in.service != "" && !strings.EqualFold(e.Service, in.service) {
			continue
		}
		r.TotalCallsInWindow++
		surfaceCalls[e.Surface]++
		toolCalls[e.Tool]++
		latencies = append(latencies, e.LatencyMS)
		if e.Surface == "agent" {
			agentSurface++
		}
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
					for _, n := range e.VerdictOutcome {
						reviewActive += n
					}
				}
			}
		}
	}
	r.SkippedMalformedLines = in.skippedMalformed
	r.AgentSurfaceCalls = agentSurface
	r.StalenessReportCalls = staleness
	if !in.callLogRead {
		r.StalenessNote = "not measurable: call log absent/unreadable at " + in.callLogPath
	}

	r.VerifyClaimed, r.VerifyVerified, r.VerifyFailing = verifyClaimed, verifyVerified, verifyFailing
	r.VerifyInstrumentedCalls, r.VerifyPredatesInstrumentation, r.ReviewActiveFindings = verifyInstrumented, verifyPredates, reviewActive
	switch {
	case verifyInstrumented == 0 && verifyPredates == 0:
		r.VerifyNote = "no verify_plan/review calls in this window"
	case verifyInstrumented == 0:
		r.VerifyNote = fmt.Sprintf("not instrumented — %d verify_plan/review call(s) in this window predate the verdict_outcome field and cannot be recovered", verifyPredates)
	default:
		r.VerifyNote = fmt.Sprintf("%d verify_plan/review call(s) instrumented this window", verifyInstrumented)
		if verifyPredates > 0 {
			r.VerifyNote += fmt.Sprintf("; %d additional call(s) predate instrumentation and are excluded (not counted as zero)", verifyPredates)
		}
	}

	surfaceNames := make([]string, 0, len(surfaceCalls))
	for s := range surfaceCalls {
		surfaceNames = append(surfaceNames, s)
	}
	sort.Strings(surfaceNames)
	for _, s := range surfaceNames {
		r.SurfaceTraffic = append(r.SurfaceTraffic, SurfaceTrafficRow{Surface: s, Calls: surfaceCalls[s]})
	}

	toolNames := make([]string, 0, len(toolCalls))
	for t := range toolCalls {
		toolNames = append(toolNames, t)
	}
	sort.Slice(toolNames, func(i, j int) bool {
		if toolCalls[toolNames[i]] != toolCalls[toolNames[j]] {
			return toolCalls[toolNames[i]] > toolCalls[toolNames[j]]
		}
		return toolNames[i] < toolNames[j]
	})
	for _, t := range toolNames {
		r.TopTools = append(r.TopTools, ToolCallRow{Tool: t, Calls: toolCalls[t]})
	}

	if len(latencies) == 0 {
		r.LatencyNote = "not measurable: no calls in window"
	} else {
		sort.Float64s(latencies)
		p50 := percentile(latencies, 0.50)
		p95 := percentile(latencies, 0.95)
		r.LatencyP50MS, r.LatencyP95MS = &p50, &p95
	}

	// ─── hooks beat log: injection coverage + obligations served ───
	// Phase-O addition: every edit-hook beat now carries a "phase" meta
	// key ("pre" or "post" — edithook.go/edithook_pre.go). A beat logged
	// before this feature existed (or any beat this package itself never
	// tags) carries no phase at all: treated as "post" here, since every
	// pre-phase-O edit-hook invocation WAS the post/default hook.
	//
	// Critic finding 3 (phase-O merge, BLOCKING): the legacy InjectionSeen/
	// InjectionAnswered/ObligationsServed counters (and their downstream
	// >=90% gate/comparisons in docs/plans/activation.md) MUST count only
	// the post/default channel — NOT phase:"pre" beats. `fathom init`
	// registers PreToolUse for BOTH Edit|Write|MultiEdit AND Bash, so a
	// repo re-running init would otherwise see edit-hook.seen absorb every
	// Bash invocation too, silently rebasing the historical denominator
	// and making every phase-O-era number incomparable to every
	// pre-phase-O one. GuardPreSeen/GuardPreServed report the pre-channel
	// separately; the legacy fields stay byte-for-byte what they always
	// measured.
	seen, answered, servedEmitted := 0, 0, 0
	preSeen, preServed, postSeen, postServed, dedupSuppressed := 0, 0, 0, 0, 0
	moduleServed := map[string]int{}
	for _, b := range in.beats {
		if b.Channel != "edit-hook" || b.At.Before(in.since) || b.At.After(in.until) {
			continue
		}
		phase := b.Meta["phase"]
		if phase == "" {
			phase = "post"
		}
		switch b.Kind {
		case "seen":
			if phase == "pre" {
				preSeen++
				continue
			}
			seen++
			postSeen++
		case "answered":
			if phase == "pre" {
				if b.Meta["emitted"] == "true" {
					preServed++
				}
				continue
			}
			answered++
			if b.Meta["emitted"] == "true" {
				servedEmitted++
				postServed++
			}
		case "dedup-suppressed":
			// Finding 6 (critic): count actual SUPPRESSED RULES, not
			// invocations — a partial dedup (some rules delta-rendered,
			// some already-served) carries its count in meta too, not
			// only the full-suppression case.
			n := 1
			if cs := b.Meta["count"]; cs != "" {
				if parsed, err := strconv.Atoi(cs); err == nil && parsed > 0 {
					n = parsed
				}
			}
			dedupSuppressed += n
		case "module-served":
			// Finding 6 (critic): this field is documented (and named)
			// as RULES served per module, not invocation counts — sum
			// the per-beat rule count carried in meta rather than
			// incrementing once per (module, call).
			if m := b.Meta["module"]; m != "" {
				n := 1
				if rs := b.Meta["rules"]; rs != "" {
					if parsed, err := strconv.Atoi(rs); err == nil && parsed > 0 {
						n = parsed
					}
				}
				moduleServed[m] += n
			}
		}
	}
	r.InjectionSeen, r.InjectionAnswered, r.ObligationsServed = seen, answered, servedEmitted
	if seen > 0 {
		rate := float64(answered) / float64(seen)
		r.InjectionCoverageRate = &rate
	}

	r.GuardPreSeen, r.GuardPreServed, r.GuardPostSeen, r.GuardPostServed, r.GuardDedupSuppressed = preSeen, preServed, postSeen, postServed, dedupSuppressed
	switch {
	case preSeen == 0:
		r.GuardCoverageNote = "not measurable: no PreToolUse edit-hook (--pre) invocations observed in window — the guard hook is not installed (fathom init predates guarded edits), or no PreToolUse event fired at all"
	case preServed == 0:
		r.GuardCoverageNote = fmt.Sprintf("--pre fired %d time(s) but served 0 — no configured guard (.fathom/guard.cue) matched any edited file/command in window", preSeen)
	default:
		r.GuardCoverageNote = fmt.Sprintf("--pre fired %d time(s), served %d; %d post-hook render(s) suppressed as already-served (dedup)", preSeen, preServed, dedupSuppressed)
	}

	if len(moduleServed) > 0 {
		r.ModuleRulesServed = moduleServed
	} else {
		r.ModuleServingNote = "not measurable: no module-tagged rule was served in window — no .fathom/modules.cue binding is configured anywhere observed, or no bound module is mounted on the daemon"
	}

	// ─── observations.md: totals + by-tool breakdown ───
	if in.obsRead {
		total := nextObservationID(in.obsContent) - 1
		open := countOpenObservations(in.obsContent)
		r.TotalObservations, r.OpenObservations = &total, &open
		r.ObservationsByTool = observationsByTool(in.obsContent)
	}

	// ─── context-bench: per-arm token/turn deltas ───
	if in.benchResults != nil {
		r.BenchResultsPath = in.benchPath
		r.BenchGeneratedAt = in.benchResults.GeneratedAt
		arms := make([]string, 0, len(in.benchResults.Summary))
		for a := range in.benchResults.Summary {
			arms = append(arms, a)
		}
		sort.Strings(arms)
		for _, a := range arms {
			agg := in.benchResults.Summary[a]
			r.BenchArms = append(r.BenchArms, BenchArmSummary{
				Arm: a, TaskCount: agg.TaskCount, SuccessCount: agg.SuccessCount,
				SuccessRate: agg.SuccessRate, TotalTokens: agg.TotalTokens,
				MeanTokens: agg.MeanTokens, MeanWallMS: agg.MeanWallMS,
			})
		}
		r.BenchNote = fmt.Sprintf("most recent bench results artifact (generated %s); this report does not filter by --since — compare BenchGeneratedAt against the window yourself", in.benchResults.GeneratedAt)
	}

	return r
}

// percentile returns the nearest-rank percentile p (0..1) of a SORTED,
// non-empty slice: rank = ceil(p*n), 1-indexed into sorted, clamped to
// [0, n-1]. Critic Finding 5 (2026-07-28): the prior floor-index formula
// (int(p*float64(n-1))) is NOT nearest-rank despite the doc comment above
// claiming it was — for n=2 it returns sorted[int(p)], which for p95 gives
// int(0.95)=0, the MINIMUM, not the maximum; a flattering bias baked into a
// credibility artifact. This formula matches the doc comment for every n,
// including small n (verified: percentile({10,20}, 0.95) == 20).
func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	idx := int(math.Ceil(p*float64(n))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return sorted[idx]
}

// parseCallLogTS parses a call-log entry's RFC3339Nano timestamp, falling
// back to plain RFC3339 for older/hand-written fixtures.
func parseCallLogTS(ts string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// observationsToolLine matches an entry's own "- **Tool:** <value>" line
// (observe.go's formatObservationEntry).
var observationsToolLine = regexp.MustCompile(`(?m)^\s*-\s\*\*Tool:\*\*\s*(.+)$`)

// observationsByTool counts .fathom/observations.md entries by their
// "- **Tool:**" field — a coarse "category" proxy since entries carry no
// explicit category field. Entries whose tool line is "TODO" (never
// filled in) are counted under "TODO".
func observationsByTool(content string) map[string]int {
	sections := observationEntryHeader.Split(content, -1)
	out := map[string]int{}
	for _, sec := range sections {
		m := observationsToolLine.FindStringSubmatch(sec)
		if m == nil {
			continue
		}
		tool := strings.TrimSpace(m[1])
		if tool == "" {
			continue
		}
		out[tool]++
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
