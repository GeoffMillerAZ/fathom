// Package event defines the canonical telemetry event shape for agentic-kit.
// Every agentic tool call, LLM invocation, and infrastructure operation MUST
// be represented as an Event before being forwarded to a Sink. The CUE
// counterpart in schema.cue is authoritative for the shape of data; Go is
// authoritative for behavior. Drift between the two is a bug.
//
// Relationship to foundations/contracts: telemetry/event.Event is the
// canonical RUNTIME telemetry shape — the typed, in-process value that live
// components emit into foundations/telemetry/sink Sinks (typed Kind enum,
// time.Time timestamp, string-only attributes). contracts.EventShape is the
// canonical CONTRACT/routing shape — the validated, serialization-oriented
// envelope (string Kind, RFC 3339 string timestamp, open Payload) used when
// events cross package or process boundaries under a routing contract.
// Emit Event at runtime; exchange EventShape where a contract governs the
// hop between systems.
//
// Concurrency: Event values must not be shared across goroutines without external synchronisation; TraceContext is an immutable value type and safe for concurrent use.
package event

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Kind enumerates the valid telemetry event categories.
// CUE mirror: #Kind in schema.cue.
type Kind string

const (
	// KindToolCall is emitted when an MCP tool is invoked.
	KindToolCall Kind = "tool_call"

	// KindLLMCall is emitted on every LLM completion or streaming request.
	KindLLMCall Kind = "llm_call"

	// KindError is emitted when a recoverable or unrecoverable error occurs.
	KindError Kind = "error"

	// KindInfo is emitted for general informational checkpoints.
	KindInfo Kind = "info"
)

// Event is the atomic telemetry unit emitted by every agentic component.
// All fields except Attributes are required (non-empty/non-zero) after
// Normalize has been called. Normalize + Validate are the canonical entry
// point before handing an Event to a Sink.
//
// CUE mirror: #Event in schema.cue.
type Event struct {
	// ID uniquely identifies this event. Normalize fills a default when empty.
	ID string

	// Kind classifies the event. Must be one of the KindXxx constants.
	Kind Kind

	// Source identifies the emitting component (e.g. "llmclient/ladder",
	// "orchestration/workflow-engine"). Normalize trims and lower-cases the value.
	Source string

	// Timestamp is when the event occurred. Normalize fills time.Now().UTC()
	// when the zero value is given.
	Timestamp time.Time

	// Attributes carries arbitrary key–value metadata. Keys are canonicalised
	// to lower_snake_case by Normalize; nil and empty maps are both valid.
	Attributes map[string]string
}

// validKinds is the set of accepted Kind values, kept in sync with the CUE
// #Kind enum in schema.cue.
var validKinds = map[Kind]struct{}{
	KindToolCall: {},
	KindLLMCall:  {},
	KindError:    {},
	KindInfo:     {},
}

// Normalize canonicalises e in-place and returns it for chaining.
//
// Rules applied:
//   - ID: if empty, set to "<source>/<timestamp-unix-nano>" to give a stable
//     default without an external ID generator.
//   - Kind: trimmed of whitespace and lower-cased.
//   - Source: trimmed of whitespace and lower-cased.
//   - Timestamp: if zero, set to time.Now().UTC().
//   - Attributes keys: trimmed, lower-cased, spaces replaced with underscores.
//     Duplicate keys (after normalisation) last-writer-wins in iteration order;
//     the caller should ensure uniqueness before passing.
func (e *Event) Normalize() *Event {
	// Source — trim and lower-case before using in ID default.
	e.Source = strings.ToLower(strings.TrimSpace(e.Source))

	// Timestamp default.
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}

	// Kind — trim and lower-case.
	e.Kind = Kind(strings.ToLower(strings.TrimSpace(string(e.Kind))))

	// ID default: stable composite without external deps.
	if strings.TrimSpace(e.ID) == "" {
		e.ID = fmt.Sprintf("%s/%d", e.Source, e.Timestamp.UnixNano())
	}
	e.ID = strings.TrimSpace(e.ID)

	// Attributes — canonicalise keys; rebuild map in stable key order.
	if len(e.Attributes) > 0 {
		keys := make([]string, 0, len(e.Attributes))
		for k := range e.Attributes {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		normalised := make(map[string]string, len(e.Attributes))
		for _, k := range keys {
			nk := canonicalKey(k)
			normalised[nk] = e.Attributes[k]
		}
		e.Attributes = normalised
	}

	return e
}

// Validate reports an error if the event is structurally invalid after
// Normalize has been applied. It does NOT call Normalize itself — callers are
// expected to call Normalize first, then Validate.
//
// Conditions checked:
//   - ID must be non-empty.
//   - Kind must be one of the KindXxx values.
//   - Source must be non-empty.
//   - Timestamp must be non-zero.
func (e *Event) Validate() error {
	var errs []string
	if strings.TrimSpace(e.ID) == "" {
		errs = append(errs, "ID must not be empty")
	}
	if _, ok := validKinds[e.Kind]; !ok {
		errs = append(errs, fmt.Sprintf("Kind %q is not a valid telemetry kind", e.Kind))
	}
	if strings.TrimSpace(e.Source) == "" {
		errs = append(errs, "Source must not be empty")
	}
	if e.Timestamp.IsZero() {
		errs = append(errs, "Timestamp must not be zero")
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.New("event validation failed: " + strings.Join(errs, "; "))
}

// canonicalKey normalises an attribute key: trim whitespace, lower-case,
// replace spaces with underscores.
func canonicalKey(k string) string {
	k = strings.TrimSpace(k)
	k = strings.ToLower(k)
	k = strings.ReplaceAll(k, " ", "_")
	return k
}
