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
)

// callLogEntry is the JSONL record written per tools/call invocation.
type callLogEntry struct {
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
	LatencyMS      float64        `json:"latency_ms"`
	Error          string         `json:"error,omitempty"`
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
func (l *CallLogger) log(tool, service, group, surface, via, origin string, offSurface bool, claimsAttached int, verdictOutcome map[string]int, latency time.Duration, callErr error) {
	if l == nil {
		return
	}
	now := time.Now().UTC()
	entry := callLogEntry{
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
