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
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/internal/cost"
)

// callLogSchemaVersion identifies this record type's field semantics.
// v1 = the unstamped legacy shape (rows without the field); v2 adds the
// version stamps (vision 14 §3 / plan IO-11); v3 (RT-64, design §13.3)
// makes outcome a field of every row (ok | partial | refused | abandoned |
// abandoned-by-governor | error, was set only for abandoned-by-governor)
// and adds class, queued_ms, bytes_read and caller. Bump on any semantic
// change to an existing field — additive fields alone do not require a
// bump.
const callLogSchemaVersion = 3

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
	// way callLogFold (cmd/fathom) treats it as "not accounted", never as a
	// genuine zero-byte call.
	ResponseBytes int `json:"response_bytes,omitempty"`
	// ResponseTokens (PE-20, docs/design/progress-and-economics.md §5) is
	// receiptTokens' estimate for this same response, carried beside
	// ResponseBytes: internal/cost.Estimate over the exact marshaled bytes
	// ResponseBytes counted, named as an estimate (four characters per
	// token), never presented as a measured count. 0 on a row logged before
	// this field existed — indistinguishable from a genuine zero-byte
	// response, exactly how ResponseBytes' own zero already reads.
	ResponseTokens int `json:"response_tokens,omitempty"`
	// ElidedCount (BA-1/BA-8) mirrors the response's own "elided.count" when
	// present — nil for a call whose result carries no elided block at all
	// (either a non-truncating tool, or a truncating one that returned
	// everything), letting elision rate be computed as a fraction of calls
	// that COULD disclose, never confused with calls that had nothing to.
	ElidedCount *int `json:"elided_count,omitempty"`

	LatencyMS float64 `json:"latency_ms"`
	Error     string  `json:"error,omitempty"`
	// Outcome is how the call ended (RT-64, v3): ok, partial (a producer
	// stopped at the per-call ceiling), refused (admission), abandoned
	// (its deadline passed or its client left), abandoned-by-governor (the
	// refuse rung cancelled it, RT-172) or error. Omitted only for a call
	// rejected before admission (unknown tool, bad params).
	Outcome string `json:"outcome,omitempty"`
	// Class, QueuedMS and BytesRead are the call's admission (RT-64,
	// design §13.3): its cost class, how long it queued for a slot and the
	// bytes its producers charged to the per-call meter.
	Class     string `json:"class,omitempty"`
	QueuedMS  int64  `json:"queued_ms"`
	BytesRead int64  `json:"bytes_read"`
	// Caller is the X-Fathom-Caller identity (§10.4), when sent.
	Caller *callLogCaller `json:"caller,omitempty"`

	// Pointers (RC-25, docs/vision/28-receipts.md §4) are this call's top
	// discovery pointers — symbol ids ("pkgDir.Name") and repo-relative
	// file paths, in the same untyped grammar cmd/fathom's `ctx:` plan
	// annotation and `fathom plans context` already resolve — extracted
	// from the tool's STRUCTURED result by resultPointers (discoveries_
	// tool.go), never rendered text. At most maxResultPointers (8); nil
	// when the tool's result carries nothing pointer-shaped.
	Pointers []string `json:"pointers,omitempty"`
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

	// path is the log's own file path, when w is an *os.File — read back
	// by registerDiscoveries (RC-27) to answer the "discoveries" tool from
	// the same rows this logger just wrote. "" for any other writer (a
	// test's bytes.Buffer, or a caller that never opened a real file):
	// the discoveries tool then degrades to "no call log configured"
	// rather than guessing a path (law 3).
	path string
}

// NewCallLogger creates a CallLogger that writes to w.
// Pass nil to create a no-op logger.
func NewCallLogger(w io.Writer) *CallLogger {
	if w == nil {
		return nil
	}
	l := &CallLogger{
		w:             w,
		surfaceCalls:  make(map[string]int64),
		surfaceLastTS: make(map[string]time.Time),
	}
	// *os.File and serve-http's rotating writer both name their file.
	if f, ok := w.(interface{ Name() string }); ok {
		l.path = f.Name()
	}
	return l
}

// Path returns the log's own file path (RC-27), or "" when this logger was
// not built over a real file (nil receiver included).
func (l *CallLogger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
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
	// ResponseTokens (PE-20) is receiptTokens' estimate for the same bytes
	// ResponseBytes counted — set by the same caller, from the same []byte,
	// so the two numbers can never disagree about which response they cost.
	ResponseTokens int
	ElidedCount    *int
	// Pointers (RC-25) is the pre-extracted discovery-pointer list for
	// this call's result — computed by dispatch.go's pointer-extraction
	// hook (resultPointers) before the call is logged.
	Pointers []string
	// Outcome is the call-log row's outcome name (RT-64), "" for none.
	Outcome string
	// Class, QueuedMS, BytesRead and Caller are the admission columns
	// (RT-64).
	Class     string
	QueuedMS  int64
	BytesRead int64
	Caller    *callLogCaller
}

// callLogCaller is a row's caller identity (X-Fathom-Caller).
type callLogCaller struct {
	PID     int    `json:"pid,omitempty"`
	PPID    int    `json:"ppid,omitempty"`
	Session string `json:"session,omitempty"`
	Test    bool   `json:"test"`
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
		ResponseTokens: outcome.ResponseTokens,
		ElidedCount:    outcome.ElidedCount,
		LatencyMS:      float64(latency.Milliseconds()),
		Pointers:       outcome.Pointers,
		Outcome:        outcome.Outcome,
		Class:          outcome.Class,
		QueuedMS:       outcome.QueuedMS,
		BytesRead:      outcome.BytesRead,
		Caller:         outcome.Caller,
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
	// The console reading itself is logged (auditable) but is not traffic:
	// server_stats' surface_traffic answers "is anything flowing?", and a
	// page render must never make an idle surface look used.
	if !IsConsoleRead(origin) {
		l.surfaceCalls[surface]++
		l.surfaceLastTS[surface] = now
	}
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

// receiptTokens estimates one receipt's token cost from the exact bytes its
// ResponseBytes count came from (PE-20, docs/design/progress-and-economics.md
// §5): internal/cost.Estimate's four-characters-per-token heuristic over the
// marshaled response, named as an estimate beside the measured byte count —
// never presented as a measured token count.
func receiptTokens(response []byte) int {
	return cost.Estimate(string(response))
}
