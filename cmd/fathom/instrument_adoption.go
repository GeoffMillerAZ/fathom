package main

// instrument_adoption.go — FW-1 (docs/plans/pack-taxonomy-adoption.md): the
// flywheel's gauge. `fathom instrument --adoption` is one deterministic-
// given-inputs report over the MCP tool-call log
// (internal/mcp/calllog.go's JSONL wire format) plus the served binary's
// own tool registry and a project's .fathom/observations.md (FW-2,
// observations.go) — a CLI report, not a dashboard; nothing hosted, no
// network calls (risk register R1).

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// defaultCallLogPath is the fallback when neither --call-log nor
// [daemon].call_log in config.toml name one — the same literal serve-http's
// own --call-log flag default uses (servehttp.go), so a repo that never
// customized either place gets the log the daemon is actually writing.
const defaultCallLogPath = "workspace/logs/mcp-calls.jsonl"

// maxTopTools caps the human-table rendering of "top tools by calls" (the
// JSON report always carries the full ranked list — the cap is a display
// convenience, not a data loss).
const maxTopTools = 10

// adoptionCallEntry mirrors internal/mcp.callLogEntry's JSON shape
// (internal/mcp/calllog.go). Kept as fathom's own decoupled struct rather
// than importing the unexported type: the JSONL wire format is the
// contract between the writer and any reader, not the Go type — a reader
// in a different package has no business depending on mcp's unexported
// struct even if it could.
type adoptionCallEntry struct {
	TS             string `json:"ts"`
	Tool           string `json:"tool"`
	Service        string `json:"service,omitempty"`
	Group          string `json:"group,omitempty"`
	Surface        string `json:"surface,omitempty"`
	Via            string `json:"via,omitempty"`
	Origin         string `json:"origin,omitempty"`
	OffSurface     bool   `json:"off_surface,omitempty"`
	ClaimsAttached int    `json:"claims_attached,omitempty"`
	// VerdictOutcome (T5-7, docs/plans/trust-and-tokenomics.md) mirrors
	// internal/mcp.callLogEntry's own field of the same JSON name: a
	// gate-shaped result summary a row may carry — verify_plan's
	// {claimed,verified,failing} (via the MCP dispatch path) or review's
	// per-checked-dimension finding counts (via appendReviewCallLogEntry,
	// review.go's CLI/hook forced-channel write). Nil for every row logged
	// by a binary built before this field existed — buildAdoptionReport
	// must count those separately as "predates instrumentation", never as a
	// fabricated zero catch.
	VerdictOutcome map[string]int `json:"verdict_outcome,omitempty"`
	LatencyMS      float64        `json:"latency_ms"`
	Error          string         `json:"error,omitempty"`
}

// SurfaceTrafficRow is one surface's traffic in the adoption report — the
// empty Surface string is the bare/unscoped surface (a stdio session or a
// bare POST /rpc request), real traffic like any named surface, exactly as
// mcp.CallLogger.SurfaceTraffic documents it.
type SurfaceTrafficRow struct {
	Surface string `json:"surface"`
	Calls   int    `json:"calls"`
	LastTS  string `json:"last_ts"`
}

// ToolCallRow is one tool's call count, ranked.
type ToolCallRow struct {
	Tool  string `json:"tool"`
	Calls int    `json:"calls"`
}

// AdoptionReport is FW-1's deterministic-given-inputs report shape.
// Json-tagged for --json; renderAdoptionReport renders the human table
// from the exact same values.
type AdoptionReport struct {
	CallLogPath string `json:"call_log_path"`
	CallLogRead bool   `json:"call_log_read"` // false: log absent/unreadable — every count below is honestly zero, not a guess
	TotalCalls  int    `json:"total_calls"`
	// SkippedMalformedLines counts call-log lines that failed to decode
	// (torn writes, editing debris) and were skipped line-wise (H2) — when
	// nonzero, the counts above are a floor over a partially readable log,
	// and both output forms say so.
	SkippedMalformedLines int `json:"skipped_malformed_lines"`

	SurfaceTraffic []SurfaceTrafficRow `json:"surface_traffic"`
	TopTools       []ToolCallRow       `json:"top_tools"`

	RegisteredToolCount int      `json:"registered_tool_count"`
	ZeroCallTools       []string `json:"zero_call_tools"`
	ToolRegistryNote    string   `json:"tool_registry_note,omitempty"`

	BriefCalls           int      `json:"brief_calls"`
	BriefCallsWithClaims int      `json:"brief_calls_with_claims"`
	ClaimsAttachHitRate  *float64 `json:"claims_attach_hit_rate"` // nil: no brief calls in the window — undefined, not a fake 0

	// VerifyGateCatchCount (T5-7, docs/plans/trust-and-tokenomics.md)
	// replaces the v1 always-nil "not instrumented" placeholder with a real
	// derived count: the sum, across every verify_plan/review call-log row
	// carrying a verdict_outcome, of verify_plan's "failing" count (claimed
	// tasks the gate would fail) plus review's per-checked-dimension active
	// finding counts. Nil only when this window contains ZERO such rows —
	// still literally not instrumented, over this window.
	VerifyGateCatchCount *int `json:"verify_gate_catch_count"`
	// VerifyGateInstrumentedCalls is how many verify_plan/review rows
	// carried a verdict_outcome and so contributed to VerifyGateCatchCount.
	VerifyGateInstrumentedCalls int `json:"verify_gate_instrumented_calls"`
	// VerifyGatePredatesInstrumentation counts verify_plan/review rows
	// logged by a binary built before this field existed — no
	// verdict_outcome, so their pass/fail verdict is unrecoverable from the
	// log. Reported separately and NEVER folded into VerifyGateCatchCount
	// as a fabricated zero (the honest-split requirement).
	VerifyGatePredatesInstrumentation int    `json:"verify_gate_predates_instrumentation"`
	VerifyGateNote                    string `json:"verify_gate_note"`

	// OpenObservationCount is nil when .fathom/observations.md doesn't
	// exist yet (distinct from "0 open observations", a real state a
	// project reaches once every filed observation is resolved).
	OpenObservationCount *int   `json:"open_observation_count"`
	ObservationsPath     string `json:"observations_path"`
}

// cmdInstrumentAdoption implements `fathom instrument --adoption` (FW-1).
// Advisory only, like its parent command: exit 0 always.
func cmdInstrumentAdoption(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("instrument -adoption", stderr)
	_ = fs.Bool("adoption", false, "select this report (already selected)")
	callLogFlag := fs.String("call-log", "", "MCP tool-call log (JSONL); default: [daemon].call_log in config.toml, else "+defaultCallLogPath)
	storePath := fs.String("store", "", "optional store path — builds the registered-tool list the same way serve-http constructs its server; omit to use a backend-independent registry (tool registration never depends on the backend, see registeredToolNames)")
	root := fs.String("root", ".", "project root for .fathom/observations.md")
	jsonOut := fs.Bool("json", false, "machine-readable JSON output")
	if err := fs.Parse(args); err != nil {
		return 0
	}

	callLogPath := *callLogFlag
	if callLogPath == "" {
		callLogPath = resolveCallLogPath()
	}

	entries, skippedMalformed, read := readCallLog(callLogPath) // absent/unreadable log: honestly report zero, never fatal (advisory tool)

	toolNames, toolErr := registeredToolNames(*storePath)
	toolNote := ""
	if toolErr != nil {
		toolNote = fmt.Sprintf("could not resolve the registered-tool list: %v", toolErr)
		fmt.Fprintf(stderr, "warn: instrument --adoption: %s\n", toolNote)
	}

	var openObs *int
	if content, existed, obsErr := readObservations(*root); obsErr == nil && existed {
		n := countOpenObservations(content)
		openObs = &n
	} else if obsErr != nil {
		fmt.Fprintf(stderr, "warn: instrument --adoption: reading %s: %v\n", observationsPath(*root), obsErr)
	}

	report := buildAdoptionReport(callLogPath, read, entries, skippedMalformed, toolNames, toolNote, openObs, observationsPath(*root))

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
	} else {
		renderAdoptionReport(stdout, report)
	}
	return 0
}

// resolveCallLogPath mirrors runServeHTTP's own --call-log default
// resolution (servehttp.go): config.toml's [daemon].call_log, falling back
// to the literal default the daemon itself defaults to when unconfigured —
// so a repo that never customized either place gets the log the daemon is
// actually writing, not a guess.
func resolveCallLogPath() string {
	loaded, err := config.Load(config.DefaultUserPath(), config.DefaultProjectPath())
	if err != nil {
		return defaultCallLogPath
	}
	return firstNonEmpty(loaded.Config.Daemon.CallLog, defaultCallLogPath)
}

// readCallLog parses path as call-log JSONL, line-wise. read is false when
// the file doesn't exist or can't be opened — reported honestly by the
// caller (zero calls is then a fact about "never observed", not "measured
// zero"). A malformed individual LINE is skipped and counted, never fatal
// and never a reason to drop the rest of the file: the earlier
// json.Decoder-over-the-stream version BROKE at the first bad token, so a
// single torn mid-file line (a daemon killed mid-write before O_APPEND
// atomicity applied, hand-editing debris) silently discarded every entry
// after it (critic H2). skipped is surfaced in the report so a truncated
// measurement is visible, not laundered.
func readCallLog(path string) (entries []adoptionCallEntry, skippedMalformed int, read bool) {
	f, openErr := os.Open(path)
	if openErr != nil {
		return nil, 0, false
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	// Call-log lines carry error strings and tool payload metadata; give
	// the scanner room well past bufio's 64KB default before a long line
	// (which Scan would otherwise abort on) can truncate the report.
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue // blank separators are not entries and not malformed
		}
		var e adoptionCallEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			skippedMalformed++
			continue
		}
		entries = append(entries, e)
	}
	return entries, skippedMalformed, true
}

// registeredToolNames returns the sorted names of every tool the fathom
// binary registers — "the served binary's own tool list" (FW-1's own
// framing). Tool REGISTRATION never depends on the backend: every
// register* function in internal/mcp calls Server.registerTool
// unconditionally (only a tool's runtime call-time behavior branches on a
// nil backend — see internal/mcp/tools.go's own doc comment, "always
// registered (even when backend == nil)"). So the cleanest source is the
// exact tools/list a live daemon would answer, built without depending on
// one: mcp.NewServer(nil) for the backend-independent case, or — when
// storePath is given — the same store-direct construction cmd/fathom/
// layers.go already uses, going through the real client.Client the way
// serve-http's own server does.
func registeredToolNames(storePath string) ([]string, error) {
	var srv *mcp.Server
	if storePath == "" {
		srv = mcp.NewServer(nil)
	} else {
		st, err := store.Open(storePath)
		if err != nil {
			return nil, fmt.Errorf("open store %q: %w", storePath, err)
		}
		defer st.Close()
		c := client.New(nil, st)
		srv = mcp.NewServerWithFleet(c, nil)
	}

	req := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	resp, hasResp := srv.HandleRequest(context.Background(), req)
	if !hasResp {
		return nil, fmt.Errorf("tools/list produced no response")
	}
	var envelope struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(resp, &envelope); err != nil {
		return nil, fmt.Errorf("parse tools/list response: %w", err)
	}
	if envelope.Error != nil {
		return nil, fmt.Errorf("tools/list: %s", envelope.Error.Message)
	}
	names := make([]string, 0, len(envelope.Result.Tools))
	for _, t := range envelope.Result.Tools {
		names = append(names, t.Name)
	}
	sort.Strings(names)
	return names, nil
}

// buildAdoptionReport is pure over its inputs (deterministic given inputs,
// per FW-1's own requirement) — cmdInstrumentAdoption does all the I/O,
// this function does none.
func buildAdoptionReport(callLogPath string, read bool, entries []adoptionCallEntry, skippedMalformed int, registeredTools []string, toolNote string, openObs *int, obsPath string) AdoptionReport {
	surfaceCalls := map[string]int{}
	surfaceLastTS := map[string]string{}
	toolCalls := map[string]int{}
	briefCalls, briefWithClaims := 0, 0

	for _, e := range entries {
		surfaceCalls[e.Surface]++
		surfaceLastTS[e.Surface] = e.TS // entries are file-order == write-order (append-only JSONL), so the last write wins
		toolCalls[e.Tool]++
		if e.Tool == "brief" {
			briefCalls++
			if e.ClaimsAttached > 0 {
				briefWithClaims++
			}
		}
	}

	surfaceNames := make([]string, 0, len(surfaceCalls))
	for s := range surfaceCalls {
		surfaceNames = append(surfaceNames, s)
	}
	sort.Strings(surfaceNames)
	surfaceTraffic := make([]SurfaceTrafficRow, 0, len(surfaceNames))
	for _, s := range surfaceNames {
		surfaceTraffic = append(surfaceTraffic, SurfaceTrafficRow{Surface: s, Calls: surfaceCalls[s], LastTS: surfaceLastTS[s]})
	}

	toolNames := make([]string, 0, len(toolCalls))
	seenCalled := map[string]bool{}
	for t := range toolCalls {
		toolNames = append(toolNames, t)
		seenCalled[t] = true
	}
	sort.Slice(toolNames, func(i, j int) bool {
		ci, cj := toolCalls[toolNames[i]], toolCalls[toolNames[j]]
		if ci != cj {
			return ci > cj
		}
		return toolNames[i] < toolNames[j]
	})
	topTools := make([]ToolCallRow, 0, len(toolNames))
	for _, t := range toolNames {
		topTools = append(topTools, ToolCallRow{Tool: t, Calls: toolCalls[t]})
	}

	var zeroCall []string
	for _, t := range registeredTools {
		if !seenCalled[t] {
			zeroCall = append(zeroCall, t)
		}
	}
	sort.Strings(zeroCall)

	var hitRate *float64
	if briefCalls > 0 {
		r := float64(briefWithClaims) / float64(briefCalls)
		hitRate = &r
	}

	gateCatchCount, instrumented, predates, verifyNote := gateCatchStats(entries)

	return AdoptionReport{
		CallLogPath:                       callLogPath,
		CallLogRead:                       read,
		TotalCalls:                        len(entries),
		SkippedMalformedLines:             skippedMalformed,
		SurfaceTraffic:                    surfaceTraffic,
		TopTools:                          topTools,
		RegisteredToolCount:               len(registeredTools),
		ZeroCallTools:                     zeroCall,
		ToolRegistryNote:                  toolNote,
		BriefCalls:                        briefCalls,
		BriefCallsWithClaims:              briefWithClaims,
		ClaimsAttachHitRate:               hitRate,
		VerifyGateCatchCount:              gateCatchCount,
		VerifyGateInstrumentedCalls:       instrumented,
		VerifyGatePredatesInstrumentation: predates,
		VerifyGateNote:                    verifyNote,
		OpenObservationCount:              openObs,
		ObservationsPath:                  obsPath,
	}
}

// gateCatchStats derives the real "gate catches" accounting (T5-7,
// docs/plans/trust-and-tokenomics.md) from every verify_plan/review row's
// optional verdict_outcome: a "catch" is a claimed-but-failing verify_plan
// task, or an active review finding on a checked dimension — the count of
// times the gate actually caught something. Rows of either tool logged
// before this field existed (VerdictOutcome nil) carry no recoverable
// verdict and are counted SEPARATELY as predates-instrumentation, never
// folded into the catch count as a fabricated zero (the honest-split
// requirement). catchCount is nil only when there are zero instrumented
// rows in this window — still literally not instrumented, over this
// window.
func gateCatchStats(entries []adoptionCallEntry) (catchCount *int, instrumented, predates int, note string) {
	catches := 0
	for _, e := range entries {
		if e.Tool != "verify_plan" && e.Tool != "review" {
			continue
		}
		if e.VerdictOutcome == nil {
			predates++
			continue
		}
		instrumented++
		switch e.Tool {
		case "verify_plan":
			catches += e.VerdictOutcome["failing"]
		case "review":
			for _, n := range e.VerdictOutcome {
				catches += n
			}
		}
	}

	if instrumented == 0 {
		note = "not instrumented — no verify_plan/review call in this window carries a " +
			"verdict_outcome (older call-log rows predate the field; a placeholder, not a measured zero)"
		if predates > 0 {
			note += fmt.Sprintf("; %d row(s) predate instrumentation and are excluded", predates)
		}
		return nil, instrumented, predates, note
	}

	n := catches
	note = fmt.Sprintf("%d catch(es) across %d instrumented call(s) (verify_plan claimed-but-failing tasks + review active findings)", catches, instrumented)
	if predates > 0 {
		note += fmt.Sprintf("; %d row(s) predate instrumentation and are excluded (not counted as zero)", predates)
	}
	return &n, instrumented, predates, note
}

// renderAdoptionReport writes the human-readable table form of report.
func renderAdoptionReport(w io.Writer, r AdoptionReport) {
	fmt.Fprintln(w, "fathom instrument --adoption")
	fmt.Fprintln(w)
	if !r.CallLogRead {
		fmt.Fprintf(w, "call log: %s (not found — 0 calls observed, not measured)\n", r.CallLogPath)
	} else {
		fmt.Fprintf(w, "call log: %s (%d calls)\n", r.CallLogPath, r.TotalCalls)
	}
	if r.SkippedMalformedLines > 0 {
		fmt.Fprintf(w, "  ⚠ skipped %d malformed line(s) — counts are a floor over a partially readable log\n", r.SkippedMalformedLines)
	}

	fmt.Fprintln(w, "\nsurface traffic:")
	if len(r.SurfaceTraffic) == 0 {
		fmt.Fprintln(w, "  (no calls observed)")
	} else {
		for _, s := range r.SurfaceTraffic {
			surface := s.Surface
			if surface == "" {
				surface = "(bare)"
			}
			fmt.Fprintf(w, "  %-12s calls=%-6d last_ts=%s\n", surface, s.Calls, s.LastTS)
		}
	}

	fmt.Fprintln(w, "\ntop tools by calls:")
	if len(r.TopTools) == 0 {
		fmt.Fprintln(w, "  (no calls observed)")
	} else {
		shown := r.TopTools
		truncated := 0
		if len(shown) > maxTopTools {
			truncated = len(shown) - maxTopTools
			shown = shown[:maxTopTools]
		}
		for _, t := range shown {
			fmt.Fprintf(w, "  %-24s %d\n", t.Tool, t.Calls)
		}
		if truncated > 0 {
			fmt.Fprintf(w, "  … and %d more (see --json for the full ranked list)\n", truncated)
		}
	}

	fmt.Fprintf(w, "\nzero-call registered tools (%d of %d registered):\n", len(r.ZeroCallTools), r.RegisteredToolCount)
	if r.ToolRegistryNote != "" {
		fmt.Fprintf(w, "  (%s)\n", r.ToolRegistryNote)
	} else if len(r.ZeroCallTools) == 0 {
		fmt.Fprintln(w, "  (none — every registered tool has been called at least once)")
	} else {
		fmt.Fprintf(w, "  %s\n", strings.Join(r.ZeroCallTools, ", "))
	}

	fmt.Fprintln(w, "\nclaims-attach hit rate (brief calls):")
	if r.ClaimsAttachHitRate == nil {
		fmt.Fprintln(w, "  n/a — no brief calls observed")
	} else {
		fmt.Fprintf(w, "  %d/%d = %.0f%%\n", r.BriefCallsWithClaims, r.BriefCalls, *r.ClaimsAttachHitRate*100)
	}

	fmt.Fprintln(w, "\nverify-gate catch count:")
	fmt.Fprintf(w, "  %s\n", r.VerifyGateNote)

	fmt.Fprintln(w, "\nopen observations:")
	if r.OpenObservationCount == nil {
		fmt.Fprintf(w, "  no %s yet (fathom observe creates it on first use)\n", r.ObservationsPath)
	} else {
		fmt.Fprintf(w, "  %d open (%s)\n", *r.OpenObservationCount, r.ObservationsPath)
	}
}
