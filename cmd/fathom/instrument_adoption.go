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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// defaultCallLogPath is the fallback when neither --call-log nor
// [daemon].call_log in config.toml name one — the same literal serve-http's
// own --call-log flag default uses (servehttp.go), so a repo that never
// customized either place gets the log the daemon is actually writing.

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
	// ResultCount / MatchedVia mirror internal/mcp.callLogEntry's IQ-1
	// outcome fields; nil/"" on rows logged before they existed.
	ResultCount *int   `json:"result_count,omitempty"`
	MatchedVia  string `json:"matched_via,omitempty"`
	// VerdictOutcome (T5-7, docs/plans/trust-and-tokenomics.md) mirrors
	// internal/mcp.callLogEntry's own field of the same JSON name: a
	// gate-shaped result summary a row may carry — verify_plan's
	// {claimed,verified,failing} (via the MCP dispatch path) or review's
	// per-checked-dimension finding counts (via appendReviewCallLogEntry,
	// review.go's CLI/hook forced-channel write). Nil for every row logged
	// by a binary built before this field existed — buildAdoptionReport
	// must count those separately as "uninstrumented" (a binary predating the
	// field, OR a gate call with zero claimed tasks — indistinguishable), never as a
	// fabricated zero catch.
	VerdictOutcome map[string]int `json:"verdict_outcome,omitempty"`
	// ResponseBytes / ElidedCount (BA-6/BA-1, docs/plans/ambient-fathom.md)
	// mirror internal/mcp.callLogEntry's own fields of the same JSON names:
	// what this call's response cost the agent's context, and whether (and
	// how much) it disclosed eliding. 0 / nil on rows logged before BA-6
	// existed — buildCostPanel (BA-8) must treat those as "not accounted",
	// never as a genuine zero-byte or zero-elision call.
	ResponseBytes int     `json:"response_bytes,omitempty"`
	ElidedCount   *int    `json:"elided_count,omitempty"`
	LatencyMS     float64 `json:"latency_ms"`
	Error         string  `json:"error,omitempty"`
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
	// CallLogReadError (IT-1 classification, observation O17) is set only
	// when the read failed for a reason OTHER than the log simply not
	// existing yet (ENOENT stays the honest, silent "no log yet" case —
	// CallLogRead:false alone already says so). A non-empty value here is a
	// real defect — commonly EPERM on a wedged macOS TCC verdict, the
	// external-volume playbook CLAUDE.md names — carries the errno and the
	// tccWedgeHint remedy already appended, and means ZeroCallTools/
	// RegisteredNotOnAgent/ZeroCallAgentTools below were deliberately left
	// uncomputed rather than fabricated from an empty read that never
	// actually happened (never "zero-call registered tools (88 of 88)").
	CallLogReadError string `json:"call_log_read_error,omitempty"`
	TotalCalls       int    `json:"total_calls"`
	// SkippedMalformedLines counts call-log lines that failed to decode
	// (torn writes, editing debris) and were skipped line-wise (H2) — when
	// nonzero, the counts above are a floor over a partially readable log,
	// and both output forms say so.
	SkippedMalformedLines int `json:"skipped_malformed_lines"`

	SurfaceTraffic []SurfaceTrafficRow `json:"surface_traffic"`
	TopTools       []ToolCallRow       `json:"top_tools"`

	RegisteredToolCount int      `json:"registered_tool_count"`
	ZeroCallTools       []string `json:"zero_call_tools"`

	// IO-10 (O6): the surface-scoped partition — the agent surface's own
	// zero-call list (actionable: "use these") vs the count of registered
	// tools the surface never exposed (actionable: "expose or prune").
	ZeroCallAgentTools   []string `json:"zero_call_agent_tools"`
	RegisteredNotOnAgent int      `json:"registered_not_on_agent"`
	// Service scoping (IO-10): non-empty when --service filtered the log;
	// rows without a service field are excluded AND counted, never silent.
	ServiceFilter        string `json:"service_filter,omitempty"`
	UnattributedExcluded int    `json:"unattributed_excluded,omitempty"`
	ToolRegistryNote     string `json:"tool_registry_note,omitempty"`

	BriefCalls           int      `json:"brief_calls"`
	BriefCallsWithClaims int      `json:"brief_calls_with_claims"`
	ClaimsAttachHitRate  *float64 `json:"claims_attach_hit_rate"` // nil: no brief calls in the window — undefined, not a fake 0

	// VerifyGateCatchCount (T5-7, docs/plans/trust-and-tokenomics.md;
	// re-sourced 2026-08-21, IT-11) derives from the verify ledger
	// (~/.fathom/verify-ledger.jsonl, verifyledger.go — FATHOM_VERIFY_LEDGER
	// injectable) rather than the call log: the call-log-derived figure
	// double-counted (the 215/4940-family; O6/DQ-5) — an MCP verify_plan/
	// review TOOL call is a different, overlapping population from the
	// `fathom verify` CLI/pre-push runs the ledger records, and the two were
	// never reconciled before being folded into one number. The ledger
	// records exactly one row per (plan, run) — a refusal is one ledger row
	// whose Failing count is > 0 — an EVENT, never a sum of standing
	// failures across rows (FR-2, docs/plans/exploration-ledger.md — the
	// 264,487-catches artifact this discipline already guarded against, now
	// carried forward onto the ledger source). Nil only when the ledger has
	// zero rows — still literally not instrumented, never a fabricated
	// zero.
	VerifyGateRefusals *int `json:"verify_gate_refusals"`
	// VerifyGateInstrumentedCalls is the verify-ledger row count — the
	// denominator VerifyGateNote names in its own sentence (IT-11).
	VerifyGateInstrumentedCalls int `json:"verify_gate_instrumented_calls"`
	// VerifyGatePredatesInstrumentation is always 0 now that the
	// verify-gate figure is ledger-sourced (IT-11): every verify-ledger row
	// has always carried Total/Claimed/Verified/Failing (schema v1,
	// verifyledger.go) — there is no pre-instrumentation era to split out,
	// unlike the call-log verdict_outcome field this replaced. Kept, rather
	// than removed, so the JSON contract stays stable for existing readers.
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
	callLogFlag := fs.String("call-log", "", "MCP tool-call log (JSONL); default: [daemon].call_log in config.toml, else "+defaultCallLogPath())
	storePath := fs.String("store", "", "optional store path — builds the registered-tool list the same way serve-http constructs its server; omit to use a backend-independent registry (tool registration never depends on the backend, see registeredToolNames)")
	root := fs.String("root", ".", "project root for .fathom/observations.md")
	serviceFlag := fs.String("service", "", "scope the report to one service's call-log rows (unattributed rows are excluded and counted) — answers \"is fathom helping in repo X\" from anywhere (IO-10)")
	url := fs.String("url", defaultFathomURL(), "the daemon whose record names the call log")
	jsonOut := fs.Bool("json", false, "machine-readable JSON output")
	if err := fs.Parse(args); err != nil {
		return 0
	}
	report, resolved := instrumentAdoption(adoptionQuery{CallLog: *callLogFlag, Store: *storePath, Root: *root, Service: *serviceFlag, BaseURL: *url}, stderr)

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(withCallLog(report, resolved))
	} else {
		renderAdoptionReport(stdout, report)
		fmt.Fprintln(stdout, callLogLine(resolved))
	}
	return 0
}

// adoptionQuery parameterizes instrumentAdoption: the invocation's
// --call-log, --store, --root and --service, and the daemon whose record
// names the call log (--url).
type adoptionQuery struct {
	CallLog, Store, Root, Service, BaseURL string
	// preread, when set, is a read of the resolved log the caller already
	// made (the console's Operate page reads it once for every section).
	preread *adoptionPreread
}

// adoptionPreread is one read of the call log: its usage rows' fold, the
// malformed lines skipped, and the read error. A query naming a service
// reads the log itself (the fold is not per service).
type adoptionPreread struct {
	fold    *callLogFold
	skipped int
	err     error
}

// instrumentAdoption is `fathom instrument --adoption`'s whole computation;
// the console's Operate page calls it for its section (P10).
func instrumentAdoption(q adoptionQuery, stderr io.Writer) (AdoptionReport, console.ResolvedCallLog) {
	// OC-2 (console_calllog.go): the one call-log resolution — the
	// daemon's record first (the log the daemon actually writes), then J5's
	// named-service identity (its recorded root, that root's config, its
	// default), then config and the legacy default with doctor's
	// daemon-workspace probes rooted at --root (IO-10's cwd-independence).
	resolved := resolveConsoleCallLogFor(callLogQuery{Flag: q.CallLog, Service: q.Service, ProbeRoot: q.Root, BaseURL: q.BaseURL})
	callLogPath := resolved.Path

	// IT-1 read-error classification (observation O17, filed 2026-08-23):
	// readCallLog's own bool collapses "file absent" and "file present but
	// unreadable" into the same false signal — a wedged macOS TCC verdict
	// (stat succeeds, read fails; CLAUDE.md's own external-volume playbook)
	// rendered exactly like an honest missing log, and fed straight into
	// the zero-call math below as if the log had been read clean:
	// "zero-call registered tools (88 of 88 registered)" from a read that
	// never actually happened. classifyCallLogRead (flywheel.go's own
	// classification, mirrored here per SP-B3) tells "no log yet" (ENOENT)
	// apart from a real defect.
	var (
		fold             *callLogFold
		skippedMalformed int
		callLogErr       error
	)
	unattributedExcluded := 0
	if q.preread != nil && q.Service == "" {
		fold, skippedMalformed, callLogErr = q.preread.fold, q.preread.skipped, q.preread.err
		if fold == nil {
			fold = newCallLogFold()
		}
	} else {
		var entries []adoptionCallEntry
		entries, skippedMalformed, callLogErr = classifyCallLogRead(callLogPath)
		if q.Service != "" {
			entries, unattributedExcluded = filterEntriesByService(entries, q.Service)
		}
		fold = foldCallLog(entries)
	}
	read := callLogErr == nil
	realReadErr := callLogErr != nil && !os.IsNotExist(callLogErr)
	callLogReadError := ""
	if realReadErr {
		callLogReadError = callLogErr.Error()
	}

	toolNames, toolErr := registeredToolNames(q.Store)
	toolNote := ""
	if toolErr != nil {
		toolNote = fmt.Sprintf("could not resolve the registered-tool list: %v", toolErr)
		fmt.Fprintf(stderr, "warn: instrument --adoption: %s\n", toolNote)
	}

	var openObs *int
	if content, existed, obsErr := readObservations(q.Root); obsErr == nil && existed {
		n := countOpenObservations(content)
		openObs = &n
	} else if obsErr != nil {
		fmt.Fprintf(stderr, "warn: instrument --adoption: reading %s: %v\n", observationsPath(q.Root), obsErr)
	}

	// J5: one identical headline across all three instruments (J5(d)) —
	// "unknown", never a bare 0, whether the log is simply absent or
	// genuinely unreadable.
	if !read {
		fmt.Fprintln(stderr, "warn: instrument --adoption: "+callLogIdentityLine(callLogPath, callLogErr))
	}

	// IT-11: the verify-gate figure sources from the verify ledger, never
	// the call log — readVerifyLedger() already honors FATHOM_VERIFY_LEDGER
	// (verifyledger.go), the "path injectable" this row asks for.
	report := buildAdoptionReportFold(callLogPath, read, callLogReadError, fold, skippedMalformed, toolNames, toolNote, openObs, observationsPath(q.Root), readVerifyLedger())
	report.ServiceFilter = q.Service
	report.UnattributedExcluded = unattributedExcluded
	// O17: same fabrication risk as buildAdoptionReport's zeroCall — the
	// agent-surface partition is equally derived from adoptionToolCounts
	// (empty on a real read error), so it stays uncomputed (nil/0) rather
	// than claiming every agent-surface tool is zero-call too.
	if !realReadErr {
		report.ZeroCallAgentTools, report.RegisteredNotOnAgent = partitionZeroCalls(toolNames, mcp.AgentSurfaceTools(), fold.toolCallCounts())
	}

	return report, resolved
}

// resolveCallLogPath mirrors runServeHTTP's own --call-log default
// resolution (servehttp.go): config.toml's [daemon].call_log, falling back
// to the literal default the daemon itself defaults to when unconfigured —
// so a repo that never customized either place gets the log the daemon is
// actually writing, not a guess.
func resolveCallLogPath() string {
	if p, ok := configuredCallLogPath(); ok {
		return p
	}
	return defaultCallLogPath()
}

// configuredCallLogPath returns the EXPLICITLY configured call-log path —
// FATHOM_CALL_LOG, then [daemon].call_log — and false when neither names
// one. Doctor needs the distinction (O23): an unconfigured log lives with
// the daemon's working directory, not with whatever repo is being doctored,
// so the default must never be resolved from doctor's own cwd.
func configuredCallLogPath() (string, bool) {
	// An explicit override, so a test can exercise the no-call-log path
	// without chdir-ing the whole process — which broke every later test
	// that resolved a relative path (2026-08-12).
	if p := os.Getenv("FATHOM_CALL_LOG"); p != "" {
		return p, true
	}
	loaded, err := config.Load(config.DefaultUserPath(), config.DefaultProjectPath())
	if err != nil || loaded.Config.Daemon.CallLog == "" {
		return "", false
	}
	return loaded.Config.Daemon.CallLog, true
}

// readCallLog parses path as call-log JSONL, line-wise. read is false when
// the file doesn't exist, can't be opened or cannot be read completely.
// Callers must distinguish unavailable input from measured zero. A JSON
// decode failure on an individual line is skipped and counted, never a
// reason to drop the rest of the file: the earlier
// json.Decoder-over-the-stream version BROKE at the first bad token, so a
// single torn mid-file line (a daemon killed mid-write before O_APPEND
// atomicity applied, hand-editing debris) silently discarded every entry
// after it (critic H2). skipped is surfaced in the report so a truncated
// measurement is visible, not laundered.
func readCallLog(path string) (entries []adoptionCallEntry, skippedMalformed int, read bool) {
	entries, skippedMalformed, err := classifyCallLogRead(path)
	return entries, skippedMalformed, err == nil
}

// classifyCallLogRead owns parsing and the IT-1 read-error classification
// (observation O17, filed 2026-08-23) — mirrors
// flywheelCallLog's own classification (flywheel.go) so both consumers of
// the call log tell "no log yet" apart from "the log is right there and
// unreadable" the same way. err is nil on a clean read; os.IsNotExist(err)
// is the honest ENOENT case. Other errors preserve their cause and path.
// Open errors retain the existing tccWedgeHint behavior; scanner failures
// are reported without inferring a permissions cause. An incomplete scan
// discards prefix rows, never presenting partial data as a complete read.
//
// Rows the console stamped with its own origin (mcp.OriginConsole: the
// Observatory reading the registry to render a page) are dropped here, so
// no usage reader — tool tables, call totals, fleet, impact, flywheel,
// doctor, research — counts the console reading itself as traffic;
// callLogReadCounting reports how many.
func classifyCallLogRead(path string) (entries []adoptionCallEntry, skippedMalformed int, err error) {
	entries, skippedMalformed, _, err = callLogReadCounting(path)
	return entries, skippedMalformed, err
}

// callLogReadCounting is classifyCallLogRead that also counts the console's
// own reads it left out, for a figure that names them (console.CallTotal).
func callLogReadCounting(path string) (entries []adoptionCallEntry, skippedMalformed, consoleReads int, err error) {
	f, openErr := os.Open(path)
	if openErr != nil {
		if os.IsNotExist(openErr) {
			return nil, 0, 0, openErr // honest "no log yet"
		}
		return nil, 0, 0, fmt.Errorf("%w%s", openErr, tccWedgeHint(path))
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	// Call-log lines carry error strings and tool payload metadata; give
	// the scanner room well past bufio's 64KB default before a long line
	// can make the read incomplete.
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		switch e, kind := decodeCallLogLine(sc.Bytes()); kind {
		case callLogLineMalformed:
			skippedMalformed++
		case callLogLineConsole:
			consoleReads++
		case callLogLineEntry:
			entries = append(entries, e)
		}
	}
	if scanErr := sc.Err(); scanErr != nil {
		// Partial counts are not a complete observation. Discard even valid
		// prefix rows so callers cannot report a truncated file as measured.
		return nil, 0, 0, fmt.Errorf("read call log %s: %w", path, scanErr)
	}
	return entries, skippedMalformed, consoleReads, nil
}

// callLogLineKind is what one call-log line is to a usage reader.
type callLogLineKind int

const (
	callLogLineBlank     callLogLineKind = iota // a separator: not an entry, not malformed
	callLogLineMalformed                        // not a JSON entry
	callLogLineConsole                          // the console reading itself (mcp.OriginConsole)
	callLogLineEntry                            // a call a usage reader counts
)

// decodeCallLogLine is the one reading of a call-log line every usage
// reader shares (callLogReadCounting, the console's incremental tail), so
// a whole-file read and an appended-bytes read can never count differently.
func decodeCallLogLine(raw []byte) (adoptionCallEntry, callLogLineKind) {
	line := bytes.TrimSpace(raw)
	if len(line) == 0 {
		return adoptionCallEntry{}, callLogLineBlank
	}
	var e adoptionCallEntry
	if err := json.Unmarshal(line, &e); err != nil {
		return adoptionCallEntry{}, callLogLineMalformed
	}
	if mcp.IsConsoleRead(e.Origin) {
		return e, callLogLineConsole
	}
	return e, callLogLineEntry
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
func buildAdoptionReport(callLogPath string, read bool, callLogReadError string, entries []adoptionCallEntry, skippedMalformed int, registeredTools []string, toolNote string, openObs *int, obsPath string, ledgerRows []verifyLedgerRow) AdoptionReport {
	return buildAdoptionReportFold(callLogPath, read, callLogReadError, foldCallLog(entries), skippedMalformed, registeredTools, toolNote, openObs, obsPath, ledgerRows)
}

// buildAdoptionReportFold is buildAdoptionReport over the log's fold (the
// console's tail keeps the fold, never the rows — RT-112).
func buildAdoptionReportFold(callLogPath string, read bool, callLogReadError string, fold *callLogFold, skippedMalformed int, registeredTools []string, toolNote string, openObs *int, obsPath string, ledgerRows []verifyLedgerRow) AdoptionReport {
	surfaceCalls := map[string]int{}
	surfaceLastTS := map[string]string{}
	for s, sf := range fold.surfaces {
		// entries are file-order == write-order (append-only JSONL), so
		// the fold's last write per surface is the one that wins
		surfaceCalls[s], surfaceLastTS[s] = sf.calls, sf.lastTS
	}
	toolCalls := fold.toolCallCounts()
	briefCalls, briefWithClaims := fold.briefCalls, fold.briefClaims

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

	// O17: a real (non-ENOENT) call-log read error leaves entries/seenCalled
	// empty for a reason that has nothing to do with actual tool usage —
	// computing zeroCall from that would fabricate "every registered tool
	// is zero-call" out of a read that never happened. Only the ENOENT "no
	// log yet" case and a genuinely clean read compute this list; a real
	// read error leaves it nil, and callLogReadError (set on the report
	// below) is what the renderer checks to say so honestly instead.
	var zeroCall []string
	if callLogReadError == "" {
		for _, t := range registeredTools {
			if !seenCalled[t] {
				zeroCall = append(zeroCall, t)
			}
		}
		sort.Strings(zeroCall)
	}

	var hitRate *float64
	if briefCalls > 0 {
		r := float64(briefWithClaims) / float64(briefCalls)
		hitRate = &r
	}

	gateCatchCount, instrumented, verifyNote := gateCatchStatsFromLedger(ledgerRows)
	predates := 0

	return AdoptionReport{
		CallLogPath:                       callLogPath,
		CallLogRead:                       read,
		CallLogReadError:                  callLogReadError,
		TotalCalls:                        fold.rows,
		SkippedMalformedLines:             skippedMalformed,
		SurfaceTraffic:                    surfaceTraffic,
		TopTools:                          topTools,
		RegisteredToolCount:               len(registeredTools),
		ZeroCallTools:                     zeroCall,
		ToolRegistryNote:                  toolNote,
		BriefCalls:                        briefCalls,
		BriefCallsWithClaims:              briefWithClaims,
		ClaimsAttachHitRate:               hitRate,
		VerifyGateRefusals:                gateCatchCount,
		VerifyGateInstrumentedCalls:       instrumented,
		VerifyGatePredatesInstrumentation: predates,
		VerifyGateNote:                    verifyNote,
		OpenObservationCount:              openObs,
		ObservationsPath:                  obsPath,
	}
}

// gateCatchStatsFromLedger derives the verify-gate figure from the verify
// ledger (~/.fathom/verify-ledger.jsonl, FATHOM_VERIFY_LEDGER-injectable —
// verifyLedgerPath in verifyledger.go), replacing the call-log-derived
// computation that double-counted (IT-11, the 215/4940-family; O6/DQ-5): an
// MCP verify_plan/review TOOL call (the call log) and a `fathom verify`
// CLI/pre-push run (the ledger) are two overlapping, unreconciled
// populations of the same underlying fact. The ledger records exactly one
// row per (plan, run) — a "gate refusal" is one ledger row whose Failing
// count is > 0, an EVENT, never a sum of standing failures across rows
// (FR-2, docs/plans/exploration-ledger.md — the same 264,487-catches
// discipline, carried forward onto the new source). catchCount is nil only
// when the ledger has zero rows — still literally not instrumented, never a
// fabricated zero; instrumented (the row count) is the denominator the
// returned note names explicitly.
func gateCatchStatsFromLedger(rows []verifyLedgerRow) (catchCount *int, instrumented int, note string) {
	// One computation for every surface (PC-13): verifyFigure owns the
	// refusal-event and run counts; this adapter keeps the gauge's own
	// pointer-nil-means-uninstrumented contract.
	fig := verifyFigure(rows)
	catches := fig.Refusals
	instrumented = fig.Runs

	if instrumented == 0 {
		return nil, 0, "not instrumented — the verify ledger (~/.fathom/verify-ledger.jsonl) has no recorded `fathom verify` run yet (a placeholder, not a measured zero)"
	}

	n := catches
	note = fmt.Sprintf("%d gate refusal(s) across %d verify-ledger run(s) (~/.fathom/verify-ledger.jsonl) — one run's verdict carried at least one failing claimed row", catches, instrumented)
	return &n, instrumented, note
}

// renderAdoptionReport writes the human-readable table form of report.
func renderAdoptionReport(w io.Writer, r AdoptionReport) {
	fmt.Fprintln(w, "fathom instrument --adoption")
	fmt.Fprintln(w)
	switch {
	case r.CallLogReadError != "":
		// O17: a real read error (never ENOENT — that renders below as the
		// honest "not found" case) — the errno and tccWedgeHint's remedy
		// already ride in the message. Every figure derived from the call
		// log (zero-call tools chief among them) is uncomputed, not
		// fabricated, and says so explicitly further down.
		fmt.Fprintf(w, "call log: %s — READ ERROR: %s\n", r.CallLogPath, r.CallLogReadError)
	case !r.CallLogRead:
		fmt.Fprintf(w, "call log: %s (not found — 0 calls observed, not measured)\n", r.CallLogPath)
	default:
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

	if r.ServiceFilter != "" {
		fmt.Fprintf(w, "\nscoped to service: %s (%d call(s) after filtering; %d unattributed row(s) excluded)\n", r.ServiceFilter, r.TotalCalls, r.UnattributedExcluded)
	}
	if len(r.ZeroCallAgentTools) > 0 || r.RegisteredNotOnAgent > 0 {
		fmt.Fprintf(w, "\nzero-call AGENT-surface tools (%d of %d on the surface — the actionable list):\n  %s\n", len(r.ZeroCallAgentTools), len(r.ZeroCallTools)+0, strings.Join(r.ZeroCallAgentTools, ", "))
		fmt.Fprintf(w, "registered but not exposed on the agent surface: %d (expose or prune — a different remedy than \"use more tools\")\n", r.RegisteredNotOnAgent)
	}
	// O17: r.CallLogReadError set means ZeroCallTools was deliberately left
	// uncomputed (buildAdoptionReport) — say so honestly instead of
	// printing "zero-call registered tools (88 of 88 registered)" for a
	// read that never happened.
	if r.CallLogReadError != "" {
		fmt.Fprintln(w, "\nzero-call registered tools: NOT COMPUTED — the call log failed to read (see the READ ERROR above); this is not a measurement of usage")
	} else {
		fmt.Fprintf(w, "\nzero-call registered tools (%d of %d registered):\n", len(r.ZeroCallTools), r.RegisteredToolCount)
		if r.ToolRegistryNote != "" {
			fmt.Fprintf(w, "  (%s)\n", r.ToolRegistryNote)
		} else if len(r.ZeroCallTools) == 0 {
			fmt.Fprintln(w, "  (none — every registered tool has been called at least once)")
		} else {
			fmt.Fprintf(w, "  %s\n", strings.Join(r.ZeroCallTools, ", "))
		}
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

// filterEntriesByService keeps rows whose service matches (case-insensitive)
// and counts rows carrying NO service field — excluded from a scoped report
// but never silently: the count renders beside the filter (IO-10, law 3).
func filterEntriesByService(entries []adoptionCallEntry, service string) ([]adoptionCallEntry, int) {
	var kept []adoptionCallEntry
	unattributed := 0
	for _, e := range entries {
		switch {
		case e.Service == "":
			unattributed++
		case strings.EqualFold(e.Service, service):
			kept = append(kept, e)
		}
	}
	return kept, unattributed
}

// partitionZeroCalls splits zero-call evidence by the agent surface (O6):
// registry tools the surface exposes but nothing called (the actionable
// "use these" list) vs registry tools the surface never exposed at all
// (an exposure/pruning question, not an adoption one).
func partitionZeroCalls(registered, agentTools []string, counts map[string]int) ([]string, int) {
	agent := map[string]bool{}
	for _, name := range agentTools {
		agent[name] = true
	}
	var zeroAgent []string
	notExposed := 0
	for _, name := range registered {
		if !agent[name] {
			notExposed++
			continue
		}
		if counts[name] == 0 {
			zeroAgent = append(zeroAgent, name)
		}
	}
	sort.Strings(zeroAgent)
	return zeroAgent, notExposed
}

// adoptionToolCounts folds entries into per-tool call counts.
func adoptionToolCounts(entries []adoptionCallEntry) map[string]int {
	return foldCallLog(entries).toolCallCounts()
}
