// Package mcp — call logger.
//
// CallLogger appends JSONL entries to an io.Writer whenever a tools/call
// completes. Each entry records: ts (RFC3339Nano), tool, service (if
// parseable), group, surface, via ("batch" for batch sub-calls), latency_ms,
// error (if any).
//
// NewCallLogger accepts any io.Writer (tests inject a bytes.Buffer); pass nil
// for a no-op logger. The default log path is the serve-http --call-log flag.
package mcp

import (
	"bufio"
	"encoding/json"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
)

// callLogSchemaVersion identifies this record type's field semantics.
// v1 = the unstamped legacy shape (rows without the field); v2 adds the
// version stamps (vision 14 §3 / plan IO-11). Bump on any semantic change
// to an existing field — additive fields alone do not require a bump.
const callLogSchemaVersion = 2

// callLogEntry is the JSONL record written per tools/call invocation.
type callLogEntry struct {
	SchemaVersion int    `json:"schema_version"`
	FathomVersion string `json:"fathom_version"`

	TS         string `json:"ts"`
	Tool       string `json:"tool"`
	Service    string `json:"service,omitempty"`
	Group      string `json:"group,omitempty"`
	Surface    string `json:"surface,omitempty"`
	Via        string `json:"via,omitempty"`         // "batch" for batch sub-calls
	Origin     string `json:"origin,omitempty"`      // caller-declared channel (X-Fathom-Origin); "" = agent choice
	OffSurface bool   `json:"off_surface,omitempty"` // executed despite not being on the request surface (ADR-014 amendment)
	// ClaimsAttached (D1-2, docs/plans/expert-depth.md): the brief tool's
	// count of governed semantic claims it attached to context, when >0 —
	// makes claims-block hit rate visible per call log row. Generic (any
	// handler's result can opt in via claimsAttachedFrom), not brief-specific
	// plumbing.
	ClaimsAttached int `json:"claims_attached,omitempty"`
	// VerdictOutcome (T5-7, docs/plans/trust-and-tokenomics.md) is an
	// optional gate-shaped result summary a tool handler's result may set —
	// generalizes ClaimsAttached's opt-in pattern from a single scalar to
	// named counts. verify_plan sets {claimed, verified, failing} (counts
	// over claimed-and-not-skipped tasks, mirroring internal/verify.Gate's
	// own semantics). Omitted (nil) for every non-gate-shaped call, and for
	// a gate-shaped call with nothing to report (e.g. zero claimed tasks) —
	// that omission is indistinguishable from "logged by a binary built
	// before this field existed," which is fine: neither case has a real
	// count to contribute to an aggregate.
	VerdictOutcome map[string]int `json:"verdict_outcome,omitempty"`
	// ResultCount / MatchedVia (IQ-1, docs/plans/impact-questions.md): the
	// call's OUTCOME — how many things the tool actually returned, and how
	// the match landed ("hit", the result's own matched_via, "did_you_mean"
	// for a consolation, "miss" for a recognized-but-empty list). nil/""
	// when the result shape carries no recognizable list — which is also
	// what rows logged by older binaries look like, and both honestly mean
	// "outcome not recorded", never zero.
	ResultCount *int   `json:"result_count,omitempty"`
	MatchedVia  string `json:"matched_via,omitempty"`

	// ResponseBytes (BA-6, docs/plans/ambient-fathom.md): the size of the
	// wrapped MCP content envelope this call sent the agent — what fathom
	// itself cost the agent's context, the denominator docs/design/
	// budgeted-answers.md §1 says the program has never measured. A real
	// envelope is never empty JSON, so <=0 is unambiguous: a row logged by a
	// binary built before this field existed (the key absent entirely) reads
	// back as the Go zero value, exactly like an explicit 0 would — either
	// way recordResponseCost treats it as "not accounted", never as a
	// genuine zero-byte call.
	ResponseBytes int `json:"response_bytes,omitempty"`
	// ElidedCount (BA-1/BA-8) mirrors the response's own "elided.count" when
	// present — nil for a call whose result carries no elided block at all
	// (either a non-truncating tool, or a truncating one that returned
	// everything), letting elision rate be computed as a fraction of calls
	// that COULD disclose, never confused with calls that had nothing to.
	ElidedCount *int `json:"elided_count,omitempty"`

	LatencyMS float64 `json:"latency_ms"`
	Error     string  `json:"error,omitempty"`
}

// SurfaceStat is one surface's process-lifetime traffic snapshot (A1-1,
// docs/plans/activation.md): how many tools/call requests it has served
// since this CallLogger was created, and when the most recent one landed.
type SurfaceStat struct {
	Calls  int64  `json:"calls"`
	LastTS string `json:"last_ts"` // RFC3339Nano UTC
}

// CallLogger serialises callLogEntry records to an io.Writer as JSONL.
// It is safe for concurrent use.
//
// It also accumulates process-lifetime per-surface call counts and last-call
// timestamps in memory (A1-1): server_stats' surface_traffic block reads
// these via SurfaceTraffic so "is anything flowing on /rpc/agent?" is
// answerable from the daemon itself, without tailing the JSONL file.
type CallLogger struct {
	mu sync.Mutex
	w  io.Writer

	surfaceCalls  map[string]int64
	surfaceLastTS map[string]time.Time
}

// NewCallLogger creates a CallLogger that writes to w.
// Pass nil to create a no-op logger.
func NewCallLogger(w io.Writer) *CallLogger {
	if w == nil {
		return nil
	}
	return &CallLogger{
		w:             w,
		surfaceCalls:  make(map[string]int64),
		surfaceLastTS: make(map[string]time.Time),
	}
}

// log writes one JSONL entry and bumps the in-memory per-surface counters.
// It ignores marshal / write errors to avoid disrupting tool call results.
// callOutcome carries IQ-1's per-call outcome extraction to the logger.
type callOutcome struct {
	ResultCount *int
	MatchedVia  string
	// ResponseBytes / ElidedCount (BA-6/BA-1) travel through the same
	// per-call outcome bundle as ResultCount/MatchedVia so log's signature
	// does not grow another positional parameter.
	ResponseBytes int
	ElidedCount   *int
}

func (l *CallLogger) log(tool, service, group, surface, via, origin string, offSurface bool, claimsAttached int, verdictOutcome map[string]int, outcome callOutcome, latency time.Duration, callErr error) {
	if l == nil {
		return
	}
	now := time.Now().UTC()
	entry := callLogEntry{
		SchemaVersion:  callLogSchemaVersion,
		FathomVersion:  buildinfo.Version(),
		TS:             now.Format(time.RFC3339Nano),
		Tool:           tool,
		Service:        service,
		Group:          group,
		Surface:        surface,
		Via:            via,
		Origin:         origin,
		OffSurface:     offSurface,
		ClaimsAttached: claimsAttached,
		VerdictOutcome: verdictOutcome,
		ResultCount:    outcome.ResultCount,
		MatchedVia:     outcome.MatchedVia,
		ResponseBytes:  outcome.ResponseBytes,
		ElidedCount:    outcome.ElidedCount,
		LatencyMS:      float64(latency.Milliseconds()),
	}
	if callErr != nil {
		entry.Error = callErr.Error()
	}
	b, err := json.Marshal(entry)
	if err != nil {
		return
	}
	// One record = one Write: the file is opened O_APPEND, so a single write
	// keeps each JSONL line atomic even across a daemon kill/restart.
	b = append(b, '\n')
	l.mu.Lock()
	defer l.mu.Unlock()
	l.surfaceCalls[surface]++
	l.surfaceLastTS[surface] = now
	_, _ = l.w.Write(b)
}

// claimsAttachedFrom extracts an optional "claims_attached" integer
// telemetry field a tool handler's result may set (D1-2, docs/plans/
// expert-depth.md — currently only the brief tool, D1-1). Handles both
// result shapes callers pass it: the raw handler result (batch sub-calls,
// extra_tools.go registerBatch — a plain map straight from Go code, so the
// value is a genuine int) and the wrapped MCP content envelope
// ({"content":[{"type":"text","text":"<json>"}]}, dispatch.go's
// loggedToolsCall — the value arrives JSON-decoded). Generic by design: any
// future handler can opt in by adding the same field to its result map,
// without dispatch-layer changes.
func claimsAttachedFrom(result any) int {
	m, ok := result.(map[string]any)
	if !ok {
		return 0
	}
	if content, ok := m["content"].([]any); ok {
		if len(content) == 0 {
			return 0
		}
		first, ok := content[0].(map[string]any)
		if !ok {
			return 0
		}
		text, ok := first["text"].(string)
		if !ok {
			return 0
		}
		var probe struct {
			ClaimsAttached int `json:"claims_attached"`
		}
		if err := json.Unmarshal([]byte(text), &probe); err != nil {
			return 0
		}
		return probe.ClaimsAttached
	}
	switch v := m["claims_attached"].(type) {
	case int:
		return v
	case float64:
		return int(v)
	}
	return 0
}

// verdictOutcomeFrom extracts an optional "verdict_outcome" object a
// gate-shaped tool handler's result may set (T5-7, docs/plans/
// trust-and-tokenomics.md — currently only verify_plan). Mirrors
// claimsAttachedFrom's two-result-shape handling exactly (raw handler
// result for batch sub-calls, wrapped MCP content envelope for the
// top-level dispatch) generalized from a scalar to a map. Returns nil when
// absent, so a call with nothing gate-shaped to report logs no
// verdict_outcome field at all (omitempty).
func verdictOutcomeFrom(result any) map[string]int {
	m, ok := result.(map[string]any)
	if !ok {
		return nil
	}
	if content, ok := m["content"].([]any); ok {
		if len(content) == 0 {
			return nil
		}
		first, ok := content[0].(map[string]any)
		if !ok {
			return nil
		}
		text, ok := first["text"].(string)
		if !ok {
			return nil
		}
		var probe struct {
			VerdictOutcome map[string]int `json:"verdict_outcome"`
		}
		if err := json.Unmarshal([]byte(text), &probe); err != nil {
			return nil
		}
		return probe.VerdictOutcome
	}
	switch v := m["verdict_outcome"].(type) {
	case map[string]int:
		return v
	case map[string]any:
		out := make(map[string]int, len(v))
		for k, val := range v {
			switch n := val.(type) {
			case int:
				out[k] = n
			case float64:
				out[k] = int(n)
			}
		}
		return out
	}
	return nil
}

// elidedCountFrom extracts an optional "elided.count" integer a truncating
// tool's result may carry (BA-1's elisionFor). Mirrors claimsAttachedFrom's
// two-result-shape handling (raw handler result for batch sub-calls, wrapped
// MCP content envelope for the top-level dispatch). Returns nil when the
// result carries no "elided" object at all — a non-truncating tool, or a
// truncating one whose answer was complete — never a fabricated 0.
func elidedCountFrom(result any) *int {
	m, ok := result.(map[string]any)
	if !ok {
		return nil
	}
	extract := func(payload map[string]any) *int {
		el, ok := payload["elided"].(map[string]any)
		if !ok {
			return nil
		}
		switch v := el["count"].(type) {
		case int:
			return &v
		case float64:
			n := int(v)
			return &n
		}
		return nil
	}
	if content, ok := m["content"].([]any); ok {
		if len(content) == 0 {
			return nil
		}
		first, ok := content[0].(map[string]any)
		if !ok {
			return nil
		}
		text, ok := first["text"].(string)
		if !ok {
			return nil
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(text), &payload); err != nil {
			return nil
		}
		return extract(payload)
	}
	return extract(m)
}

// SurfaceTraffic returns a snapshot of process-lifetime per-surface call
// counts and last-call timestamps (A1-1). The empty string "" is the
// bare/unscoped surface — a stdio session, or an HTTP request to bare
// /rpc — which is real traffic and reported like any named surface.
//
// Nil-vs-empty is load-bearing (bounce finding 3 / K3's existence check):
// a nil receiver (no logger wired — the instrument doesn't exist) returns
// nil, and server_stats omits the block; a wired logger that has seen ZERO
// calls returns a non-nil empty map, and server_stats emits an empty
// block — a freshly restarted daemon truthfully reads as "instrument live,
// zero traffic", never as "instrument missing".
func (l *CallLogger) SurfaceTraffic() map[string]SurfaceStat {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[string]SurfaceStat, len(l.surfaceCalls))
	for surface, calls := range l.surfaceCalls {
		out[surface] = SurfaceStat{
			Calls:  calls,
			LastTS: l.surfaceLastTS[surface].Format(time.RFC3339Nano),
		}
	}
	return out
}

// outcomeListKeys are the result-payload list keys outcomeFrom recognizes,
// in priority order (first present wins). Deliberately short: a key here
// means "this list IS the answer" — adding one is a semantic decision.
var outcomeListKeys = []string{"symbols", "nodes", "callers", "references", "results", "chunks", "entrypoints", "services"}

// outcomeFrom extracts IQ-1's call outcome from a tools/call result: the
// answer-list length and how the match landed. (nil, "") when no
// recognizable list is present — "outcome not recorded", never zero.
func outcomeFrom(result any) (*int, string) {
	m, ok := result.(map[string]any)
	if !ok {
		return nil, ""
	}
	content, ok := m["content"].([]any)
	if !ok || len(content) == 0 {
		return nil, ""
	}
	first, ok := content[0].(map[string]any)
	if !ok {
		return nil, ""
	}
	text, ok := first["text"].(string)
	if !ok {
		return nil, ""
	}
	var payload map[string]any
	if json.Unmarshal([]byte(text), &payload) != nil {
		return nil, ""
	}
	for _, key := range outcomeListKeys {
		list, ok := payload[key].([]any)
		if !ok {
			continue
		}
		n := len(list)
		via := "hit"
		if n == 0 {
			via = "miss"
			if dym, ok := payload["did_you_mean"].([]any); ok && len(dym) > 0 {
				via = "did_you_mean"
			}
		} else if mv, ok := payload["matched_via"].(string); ok && mv != "" && mv != "none" {
			via = mv
		}
		return &n, via
	}
	return nil, ""
}

// ToolCost is one tool's response-size totals over a window (BA-6): what
// fathom put into agent contexts on that tool's behalf.
type ToolCost struct {
	Tool       string `json:"tool"`
	Calls      int64  `json:"calls"`
	TotalBytes int64  `json:"total_bytes"`
	// ElidedCalls counts calls whose result carried an elided block
	// (elided.count > 0) — the numerator for BA-8's elision rate.
	// TruncatingCalls is the denominator: calls whose result carried an
	// "elided" object AT ALL (present, whether or not it disclosed a cut).
	// A call log row that never mentions elision (an exempt tool, or a
	// binary predating BA-1) counts toward neither.
	ElidedCalls     int64 `json:"elided_calls"`
	TruncatingCalls int64 `json:"truncating_calls"`
}

// recordResponseCost is BA-6's aggregation seam: it reads already-logged
// JSONL call-log rows (the per-call accounting BA-6 adds to callLogEntry
// above) and totals response bytes and elision counts per tool across
// whatever window r's rows span — no new log, just the fold. A malformed or
// blank line is skipped rather than aborting the whole read, matching how
// every other JSONL reader in this program tolerates a partial file.
func recordResponseCost(r io.Reader) []ToolCost {
	totals := map[string]*ToolCost{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e callLogEntry
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		if e.Tool == "" {
			continue
		}
		tc, ok := totals[e.Tool]
		if !ok {
			tc = &ToolCost{Tool: e.Tool}
			totals[e.Tool] = tc
		}
		tc.Calls++
		if e.ResponseBytes > 0 {
			tc.TotalBytes += int64(e.ResponseBytes)
		}
		if e.ElidedCount != nil {
			tc.TruncatingCalls++
			if *e.ElidedCount > 0 {
				tc.ElidedCalls++
			}
		}
	}
	out := make([]ToolCost, 0, len(totals))
	for _, tc := range totals {
		out = append(out, *tc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Tool < out[j].Tool })
	return out
}
