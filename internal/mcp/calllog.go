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
	ClaimsAttached int     `json:"claims_attached,omitempty"`
	LatencyMS      float64 `json:"latency_ms"`
	Error          string  `json:"error,omitempty"`
}

// CallLogger serialises callLogEntry records to an io.Writer as JSONL.
// It is safe for concurrent use.
type CallLogger struct {
	mu sync.Mutex
	w  io.Writer
}

// NewCallLogger creates a CallLogger that writes to w.
// Pass nil to create a no-op logger.
func NewCallLogger(w io.Writer) *CallLogger {
	if w == nil {
		return nil
	}
	return &CallLogger{w: w}
}

// log writes one JSONL entry.  It ignores marshal / write errors to avoid
// disrupting tool call results.
func (l *CallLogger) log(tool, service, group, surface, via, origin string, offSurface bool, claimsAttached int, latency time.Duration, callErr error) {
	if l == nil {
		return
	}
	entry := callLogEntry{
		TS:             time.Now().UTC().Format(time.RFC3339Nano),
		Tool:           tool,
		Service:        service,
		Group:          group,
		Surface:        surface,
		Via:            via,
		Origin:         origin,
		OffSurface:     offSurface,
		ClaimsAttached: claimsAttached,
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
