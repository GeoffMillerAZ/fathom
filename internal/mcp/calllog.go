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
	TS        string  `json:"ts"`
	Tool      string  `json:"tool"`
	Service   string  `json:"service,omitempty"`
	Group     string  `json:"group,omitempty"`
	Surface   string  `json:"surface,omitempty"`
	Via       string  `json:"via,omitempty"` // "batch" for batch sub-calls
	LatencyMS float64 `json:"latency_ms"`
	Error     string  `json:"error,omitempty"`
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
func (l *CallLogger) log(tool, service, group, surface, via string, latency time.Duration, callErr error) {
	if l == nil {
		return
	}
	entry := callLogEntry{
		TS:        time.Now().UTC().Format(time.RFC3339Nano),
		Tool:      tool,
		Service:   service,
		Group:     group,
		Surface:   surface,
		Via:       via,
		LatencyMS: float64(latency.Milliseconds()),
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
