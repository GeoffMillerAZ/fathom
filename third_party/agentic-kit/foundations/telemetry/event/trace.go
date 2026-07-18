// trace.go — minimal run/trace correlation for telemetry events.
//
// A TraceContext ties every Event emitted during one logical run (a workflow
// execution, an investigation, an agent session) together via a shared RunID,
// and orders nested work via SpanID/ParentSpanID. The context is carried on
// context.Context (WithTrace/TraceFrom) and stamped onto events as reserved
// attributes (Apply) — the Event shape itself is unchanged, so schema.cue's
// open #Attributes map is unaffected.
//
// Reserved attribute keys written by Apply:
//
//	trace.run_id          — TraceContext.RunID  (AttrRunID)
//	trace.span_id         — TraceContext.SpanID (AttrSpanID)
//	trace.parent_span_id  — TraceContext.ParentSpanID (AttrParentSpanID)
//
// Emitters must not set these keys directly; they are owned by Apply. The
// keys are already lower_snake_case (dots are preserved by Normalize), so
// stamping before or after Normalize yields the same attributes.
//
// Concurrency: TraceContext is an immutable value type; all functions here
// are pure given their randomness source and safe for concurrent use. The
// default randomness source is crypto/rand; inject a deterministic
// randHex(nBytes) func in tests (it must itself be concurrency-safe if
// shared across goroutines).

package event

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

// Reserved attribute keys stamped by Apply. See the file comment above for
// the full contract.
const (
	// AttrRunID carries TraceContext.RunID on a stamped Event.
	AttrRunID = "trace.run_id"

	// AttrSpanID carries TraceContext.SpanID on a stamped Event.
	AttrSpanID = "trace.span_id"

	// AttrParentSpanID carries TraceContext.ParentSpanID on a stamped Event.
	// Absent on root spans (Apply skips empty fields).
	AttrParentSpanID = "trace.parent_span_id"
)

// ID byte lengths, aligned with the W3C trace-context conventions
// (16-byte trace/run IDs, 8-byte span IDs), hex-encoded to 2× characters.
const (
	runIDBytes  = 16 // 32 hex characters
	spanIDBytes = 8  // 16 hex characters
)

// TraceContext correlates events across one logical run. RunID is shared by
// every event in the run; SpanID identifies the current unit of work; and
// ParentSpanID links to the enclosing span ("" for a root span). The zero
// value means "no trace" — Apply ignores it.
type TraceContext struct {
	// RunID identifies the whole run (32 lowercase hex chars by default).
	RunID string

	// SpanID identifies the current span (16 lowercase hex chars by default).
	SpanID string

	// ParentSpanID is the SpanID of the enclosing span, or "" at the root.
	ParentSpanID string
}

// cryptoRandHex returns nBytes cryptographically random bytes hex-encoded
// (2*nBytes lowercase hex characters). It is the default randomness source.
// crypto/rand.Read never returns an error (Go >= 1.24); it crashes the
// program if the kernel entropy source is unavailable.
func cryptoRandHex(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// NewRunID returns a fresh run identifier: 16 random bytes hex-encoded to 32
// lowercase characters. Pass randHex == nil to use crypto/rand; inject a
// deterministic randHex(nBytes) in tests.
func NewRunID(randHex func(nBytes int) string) string {
	if randHex == nil {
		randHex = cryptoRandHex
	}
	return randHex(runIDBytes)
}

// NewSpanID returns a fresh span identifier: 8 random bytes hex-encoded to
// 16 lowercase characters. Pass randHex == nil to use crypto/rand; inject a
// deterministic randHex(nBytes) in tests.
func NewSpanID(randHex func(nBytes int) string) string {
	if randHex == nil {
		randHex = cryptoRandHex
	}
	return randHex(spanIDBytes)
}

// NewTrace returns a root TraceContext with fresh RunID and SpanID and no
// parent. randHex follows the NewRunID/NewSpanID contract (nil = crypto/rand).
func NewTrace(randHex func(nBytes int) string) TraceContext {
	return TraceContext{
		RunID:  NewRunID(randHex),
		SpanID: NewSpanID(randHex),
	}
}

// Child derives a child TraceContext from tc: same RunID, a fresh SpanID,
// and ParentSpanID set to tc.SpanID. randHex follows the NewSpanID contract
// (nil = crypto/rand). tc is not mutated.
func (tc TraceContext) Child(randHex func(nBytes int) string) TraceContext {
	return TraceContext{
		RunID:        tc.RunID,
		SpanID:       NewSpanID(randHex),
		ParentSpanID: tc.SpanID,
	}
}

// IsZero reports whether tc carries no trace information at all.
func (tc TraceContext) IsZero() bool {
	return tc.RunID == "" && tc.SpanID == "" && tc.ParentSpanID == ""
}

// traceCtxKey is the private context key for TraceContext values.
type traceCtxKey struct{}

// WithTrace returns a copy of ctx carrying tc. Later calls replace any
// previously attached TraceContext.
func WithTrace(ctx context.Context, tc TraceContext) context.Context {
	return context.WithValue(ctx, traceCtxKey{}, tc)
}

// TraceFrom extracts the TraceContext attached to ctx by WithTrace.
// ok is false (and tc is the zero value) when none is attached.
func TraceFrom(ctx context.Context) (tc TraceContext, ok bool) {
	tc, ok = ctx.Value(traceCtxKey{}).(TraceContext)
	return tc, ok
}

// Apply stamps tc onto e's Attributes under the reserved keys AttrRunID,
// AttrSpanID, and AttrParentSpanID. Empty TraceContext fields are skipped,
// so a root span leaves AttrParentSpanID unset. A nil Attributes map is
// allocated on demand; any existing values under the reserved keys are
// overwritten. Apply is a no-op when e is nil or tc is the zero value.
func Apply(tc TraceContext, e *Event) {
	if e == nil || tc.IsZero() {
		return
	}
	if e.Attributes == nil {
		e.Attributes = make(map[string]string, 3)
	}
	if tc.RunID != "" {
		e.Attributes[AttrRunID] = tc.RunID
	}
	if tc.SpanID != "" {
		e.Attributes[AttrSpanID] = tc.SpanID
	}
	if tc.ParentSpanID != "" {
		e.Attributes[AttrParentSpanID] = tc.ParentSpanID
	}
}
