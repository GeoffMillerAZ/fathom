// Package mcp — call logger.
//
// CallLogger appends JSONL entries to an io.Writer whenever a tools/call
// completes. Each entry records: ts (RFC3339Nano), tool, service (if
// parseable), group, latency_ms, error (if any).
//
// The zero-writer sentinel is DefaultCallLogPath; NewCallLogger accepts any
// io.Writer so tests can inject a bytes.Buffer.
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
func (l *CallLogger) log(tool, service, group string, latency time.Duration, callErr error) {
	if l == nil {
		return
	}
	entry := callLogEntry{
		TS:        time.Now().UTC().Format(time.RFC3339Nano),
		Tool:      tool,
		Service:   service,
		Group:     group,
		LatencyMS: float64(latency.Milliseconds()),
	}
	if callErr != nil {
		entry.Error = callErr.Error()
	}
	b, err := json.Marshal(entry)
	if err != nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.w.Write(b)
	_, _ = l.w.Write([]byte("\n"))
}
