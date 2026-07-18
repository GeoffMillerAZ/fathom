package llminstrument

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/agentic-kit/foundations/telemetry/event"
	"github.com/geoffmilleraz/agentic-kit/foundations/telemetry/sink"
)

// Attribute keys stamped on every llm_call event emitted by WithTelemetry.
// Trace correlation keys (event.AttrRunID etc.) are added on top when the
// context carries a TraceContext.
const (
	// AttrModel is Request.Model — "" when the client's configured default
	// model applies (this middleware cannot see provider defaults).
	AttrModel = "model"

	// AttrDurationMS is the wall-clock duration of the call in integer
	// milliseconds, measured with the injected clock. For streams it spans
	// from the Stream call to the close of the inner channel.
	AttrDurationMS = "duration_ms"

	// AttrInputTokens is Usage.InputTokens (regular input; excludes cache
	// reads and writes per Anthropic semantics). "0" when not reported.
	AttrInputTokens = "input_tokens"

	// AttrOutputTokens is Usage.OutputTokens.
	AttrOutputTokens = "output_tokens"

	// AttrCacheReadTokens is Usage.CacheReadInputTokens.
	AttrCacheReadTokens = "cache_read_tokens"

	// AttrCacheCreationTokens is Usage.CacheCreationInputTokens.
	AttrCacheCreationTokens = "cache_creation_tokens"

	// AttrStopReason is the response's StopReason; "" on failure.
	AttrStopReason = "stop_reason"

	// AttrAttempts is Response.Attempts as stamped by llmclient.WithRetry —
	// "0" when no retry wrapper is present (streams report "0": WithRetry
	// does not stamp attempt counts on streams).
	AttrAttempts = "attempts"

	// AttrError is "" on success, otherwise the error class: one of
	// rate_limit, server, invalid_request, auth, canceled, deadline_exceeded,
	// or other.
	AttrError = "error"
)

// TelemetryClient wraps an inner LLMClient and emits one event.KindLLMCall
// telemetry event per Complete call and per Stream call (including
// pre-stream failures). It implements llmclient.LLMClient. Construct it with
// WithTelemetry; see the package doc for the recommended composition order
// and the failure-isolation contract.
//
// TelemetryClient is safe for concurrent use when the inner client and the
// sink are (both of which their contracts require).
type TelemetryClient struct {
	inner   llmclient.LLMClient
	sink    sink.Sink
	source  string
	now     func() time.Time
	dropped atomic.Int64
}

var _ llmclient.LLMClient = (*TelemetryClient)(nil)

// WithTelemetry wraps inner so that every Complete and Stream call emits one
// llm_call event to s, attributed to source (the event.Event.Source value,
// e.g. "kits/investigation"). now is the injected clock used for timestamps
// and duration_ms; nil falls back to time.Now — always inject a deterministic
// clock in tests.
//
// Trace correlation: when ctx carries an event.TraceContext (via
// event.WithTrace), its run/span IDs are stamped onto the event with
// event.Apply.
//
// Telemetry never breaks the call path: emit failures and invalid events are
// swallowed and counted (DroppedEvents); the inner client's responses and
// errors pass through unchanged.
func WithTelemetry(inner llmclient.LLMClient, s sink.Sink, source string, now func() time.Time) *TelemetryClient {
	if now == nil {
		now = time.Now
	}
	return &TelemetryClient{inner: inner, sink: s, source: source, now: now}
}

// DroppedEvents reports how many events were dropped because the sink's Emit
// returned an error or the event failed validation. It exists so tests (and
// health checks) can detect silent telemetry loss.
func (c *TelemetryClient) DroppedEvents() int64 {
	return c.dropped.Load()
}

// Complete delegates to the inner client and emits one llm_call event with
// the call's model, duration, token usage, stop reason, attempts, and error
// class.
func (c *TelemetryClient) Complete(ctx context.Context, req llmclient.Request) (llmclient.Response, error) {
	start := c.now()
	resp, err := c.inner.Complete(ctx, req)
	end := c.now()
	c.emit(ctx, end, llmAttrs(req.Model, end.Sub(start), resp.Usage, resp.StopReason, resp.Attempts, err))
	return resp, err
}

// Stream delegates to the inner client and wraps the returned channel:
// chunks are forwarded unchanged, and one llm_call event is emitted after the
// inner stream ends — with the terminal chunk's usage and stop reason on
// success, or the error class on a terminal Err chunk. A pre-stream failure
// (non-nil error return) also emits one event. When the inner stream ends
// with no terminal chunk at all and ctx was cancelled, the event carries the
// context's error class. See the package doc, "Streaming", for abandonment
// semantics.
func (c *TelemetryClient) Stream(ctx context.Context, req llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	start := c.now()
	inner, err := c.inner.Stream(ctx, req)
	if err != nil {
		end := c.now()
		c.emit(ctx, end, llmAttrs(req.Model, end.Sub(start), llmclient.Usage{}, "", 0, err))
		return nil, err
	}
	return watchStream(ctx, inner, func(stop llmclient.StopReason, usage *llmclient.Usage, streamErr error) {
		var u llmclient.Usage
		if usage != nil {
			u = *usage
		}
		if streamErr == nil && stop == "" && ctx.Err() != nil {
			// Inner stream closed without a terminal chunk after
			// cancellation (terminal delivery is best-effort on cancel).
			streamErr = ctx.Err()
		}
		end := c.now()
		c.emit(ctx, end, llmAttrs(req.Model, end.Sub(start), u, stop, 0, streamErr))
	}), nil
}

// emit builds, normalizes, validates, and emits one llm_call event. Failures
// are swallowed and counted — telemetry never breaks the call path.
func (c *TelemetryClient) emit(ctx context.Context, ts time.Time, attrs map[string]string) {
	e := event.Event{
		Kind:       event.KindLLMCall,
		Source:     c.source,
		Timestamp:  ts.UTC(),
		Attributes: attrs,
	}
	if tc, ok := event.TraceFrom(ctx); ok {
		event.Apply(tc, &e)
	}
	e.Normalize()
	if e.Validate() != nil {
		c.dropped.Add(1)
		return
	}
	if c.sink.Emit(e) != nil {
		c.dropped.Add(1)
	}
}

// llmAttrs builds the attribute map for one llm_call event.
func llmAttrs(model string, d time.Duration, u llmclient.Usage, stop llmclient.StopReason, attempts int, err error) map[string]string {
	return map[string]string{
		AttrModel:               model,
		AttrDurationMS:          strconv.FormatInt(d.Milliseconds(), 10),
		AttrInputTokens:         strconv.Itoa(u.InputTokens),
		AttrOutputTokens:        strconv.Itoa(u.OutputTokens),
		AttrCacheReadTokens:     strconv.Itoa(u.CacheReadInputTokens),
		AttrCacheCreationTokens: strconv.Itoa(u.CacheCreationInputTokens),
		AttrStopReason:          string(stop),
		AttrAttempts:            strconv.Itoa(attempts),
		AttrError:               errClass(err),
	}
}

// errClass maps an error to the stable class names used in the error
// attribute. Wrapped errors are unwrapped via errors.As/Is. The class set is
// deliberately coarse — telemetry wants stable cardinality, not messages.
func errClass(err error) string {
	if err == nil {
		return ""
	}
	var (
		rateLimit *llmclient.RateLimitError
		server    *llmclient.ServerError
		invalid   *llmclient.InvalidRequestError
		auth      *llmclient.AuthError
	)
	switch {
	case errors.As(err, &rateLimit):
		return "rate_limit"
	case errors.As(err, &server):
		return "server"
	case errors.As(err, &invalid):
		return "invalid_request"
	case errors.As(err, &auth):
		return "auth"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline_exceeded"
	default:
		return "other"
	}
}
