package mcpserver

import (
	"encoding/json"
	"time"
)

// callLogEntry is the JSONL record appended per dispatched request when
// Config.CallLog is set. Arguments and Result are populated only when
// Config.LogPayloads opts in — they carry user data (see doc.go, "Call log
// privacy").
type callLogEntry struct {
	// TS is the request start time (injected clock), RFC3339Nano, UTC.
	TS string `json:"ts"`
	// Method is the JSON-RPC method that was dispatched.
	Method string `json:"method"`
	// Tool is the tool name for tools/call requests; empty otherwise.
	Tool string `json:"tool,omitempty"`
	// DurationMS is the dispatch duration measured with the injected clock.
	DurationMS float64 `json:"duration_ms"`
	// IsError is true for protocol-level errors and for tools/call results
	// whose tool_result was an in-band error.
	IsError bool `json:"is_error"`
	// Arguments is the raw tools/call arguments JSON. LogPayloads only —
	// SENSITIVE.
	Arguments json.RawMessage `json:"arguments,omitempty"`
	// Result is the tool's result text (success output or in-band error
	// message). LogPayloads only — SENSITIVE.
	Result string `json:"result,omitempty"`
}

// logCall appends one entry to the call log. Failures are deliberately
// swallowed: the call log never disrupts serving.
func (s *Server) logCall(start time.Time, dur time.Duration, method string, meta callMeta, rpcErr *rpcError) {
	entry := callLogEntry{
		TS:         start.UTC().Format(time.RFC3339Nano),
		Method:     method,
		Tool:       meta.tool,
		DurationMS: float64(dur) / float64(time.Millisecond),
		IsError:    rpcErr != nil || meta.isError,
		Arguments:  meta.args,
		Result:     meta.result,
	}
	_ = s.callLog.Append(entry)
}
