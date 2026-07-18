package llminstrument_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	llminstrument "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/instrument"
	"github.com/geoffmilleraz/agentic-kit/foundations/telemetry/event"
	"github.com/geoffmilleraz/agentic-kit/foundations/telemetry/sink"
)

// ---- Complete: golden event ------------------------------------------------------

func TestWithTelemetry_CompleteSuccess_GoldenEvent(t *testing.T) {
	mock := llmclient.NewMockClient(usageEntry("hello", llmclient.Usage{
		InputTokens:              120,
		OutputTokens:             45,
		CacheReadInputTokens:     64,
		CacheCreationInputTokens: 32,
	}))
	mem := &sink.MemorySink{}
	clk := newStepClock(250 * time.Millisecond)
	client := llminstrument.WithTelemetry(mock, mem, "instrument-test/golden", clk.Now)

	trace := event.TraceContext{RunID: "run-0001", SpanID: "span-0001", ParentSpanID: "span-0000"}
	ctx := event.WithTrace(context.Background(), trace)

	resp, err := client.Complete(ctx, llmclient.Request{Model: "example-model", Prompt: "hi"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "hello" {
		t.Fatalf("response passed through changed: %q", resp.Text)
	}

	events := mem.Events()
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	e := events[0]
	if e.Kind != event.KindLLMCall {
		t.Errorf("Kind = %q, want %q", e.Kind, event.KindLLMCall)
	}
	if e.Source != "instrument-test/golden" {
		t.Errorf("Source = %q", e.Source)
	}
	// start = base, end = base+250ms; Timestamp is the end instant.
	if want := testBase.Add(250 * time.Millisecond); !e.Timestamp.Equal(want) {
		t.Errorf("Timestamp = %v, want %v", e.Timestamp, want)
	}

	wantAttrs := map[string]string{
		llminstrument.AttrModel:               "example-model",
		llminstrument.AttrDurationMS:          "250",
		llminstrument.AttrInputTokens:         "120",
		llminstrument.AttrOutputTokens:        "45",
		llminstrument.AttrCacheReadTokens:     "64",
		llminstrument.AttrCacheCreationTokens: "32",
		llminstrument.AttrStopReason:          "end_turn",
		llminstrument.AttrAttempts:            "0",
		llminstrument.AttrError:               "",
		event.AttrRunID:                       "run-0001",
		event.AttrSpanID:                      "span-0001",
		event.AttrParentSpanID:                "span-0000",
	}
	if !reflect.DeepEqual(e.Attributes, wantAttrs) {
		t.Errorf("attributes mismatch:\n got: %#v\nwant: %#v", e.Attributes, wantAttrs)
	}
	if client.DroppedEvents() != 0 {
		t.Errorf("DroppedEvents = %d, want 0", client.DroppedEvents())
	}
}

func TestWithTelemetry_NoTraceInContext_NoTraceAttrs(t *testing.T) {
	mock := llmclient.NewMockClient(llmclient.MockEntryOK("ok"))
	mem := &sink.MemorySink{}
	client := llminstrument.WithTelemetry(mock, mem, "instrument-test/notrace", newStepClock(time.Millisecond).Now)

	if _, err := client.Complete(context.Background(), llmclient.Request{Prompt: "x"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	attrs := mem.Events()[0].Attributes
	for _, k := range []string{event.AttrRunID, event.AttrSpanID, event.AttrParentSpanID} {
		if _, ok := attrs[k]; ok {
			t.Errorf("attribute %q must be absent without a trace in ctx", k)
		}
	}
}

// ---- Complete: error classes ------------------------------------------------------

func TestWithTelemetry_CompleteError_Classes(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantClass string
	}{
		{name: "rate limit", err: &llmclient.RateLimitError{Message: "slow"}, wantClass: "rate_limit"},
		{name: "server", err: &llmclient.ServerError{StatusCode: 503, Message: "boom"}, wantClass: "server"},
		{name: "invalid request", err: &llmclient.InvalidRequestError{Message: "bad"}, wantClass: "invalid_request"},
		{name: "auth", err: &llmclient.AuthError{Message: "denied"}, wantClass: "auth"},
		{name: "wrapped server", err: fmt.Errorf("attempt failed: %w", &llmclient.ServerError{StatusCode: 500}), wantClass: "server"},
		{name: "other", err: errors.New("mystery"), wantClass: "other"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := llmclient.NewMockClient(llmclient.MockEntryErr(tt.err))
			mem := &sink.MemorySink{}
			client := llminstrument.WithTelemetry(mock, mem, "instrument-test/errors", newStepClock(time.Millisecond).Now)

			_, err := client.Complete(context.Background(), llmclient.Request{Model: "example-model", Prompt: "x"})
			if !errors.Is(err, tt.err) {
				t.Fatalf("error not passed through: %v", err)
			}
			events := mem.Events()
			if len(events) != 1 {
				t.Fatalf("got %d events, want 1", len(events))
			}
			attrs := events[0].Attributes
			if got := attrs[llminstrument.AttrError]; got != tt.wantClass {
				t.Errorf("error attr = %q, want %q", got, tt.wantClass)
			}
			if got := attrs[llminstrument.AttrStopReason]; got != "" {
				t.Errorf("stop_reason attr = %q, want empty on failure", got)
			}
			if got := attrs[llminstrument.AttrInputTokens]; got != "0" {
				t.Errorf("input_tokens attr = %q, want %q", got, "0")
			}
		})
	}
}

func TestWithTelemetry_CompleteCanceledContext(t *testing.T) {
	mock := llmclient.NewMockClient(llmclient.MockEntryOK("never"))
	mem := &sink.MemorySink{}
	client := llminstrument.WithTelemetry(mock, mem, "instrument-test/cancel", newStepClock(time.Millisecond).Now)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Complete(ctx, llmclient.Request{Prompt: "x"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if got := mem.Events()[0].Attributes[llminstrument.AttrError]; got != "canceled" {
		t.Errorf("error attr = %q, want %q", got, "canceled")
	}
}

// ---- Stream ----------------------------------------------------------------------

func TestWithTelemetry_StreamSuccess(t *testing.T) {
	scripted := llmclient.Usage{InputTokens: 7, OutputTokens: 11, CacheReadInputTokens: 13, CacheCreationInputTokens: 17}
	mock := llmclient.NewMockClient(usageEntry("streamed text", scripted))
	mem := &sink.MemorySink{}
	clk := newStepClock(250 * time.Millisecond)
	client := llminstrument.WithTelemetry(mock, mem, "instrument-test/stream", clk.Now)

	trace := event.TraceContext{RunID: "run-0002", SpanID: "span-0002"}
	ctx := event.WithTrace(context.Background(), trace)

	ch, err := client.Stream(ctx, llmclient.Request{Model: "example-model", Prompt: "hi"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	text, terminal := drain(t, ch)

	// Chunks forwarded unchanged.
	if text != "streamed text" {
		t.Errorf("forwarded text = %q, want %q", text, "streamed text")
	}
	if terminal.StopReason != llmclient.StopEndTurn || terminal.Usage == nil || *terminal.Usage != scripted {
		t.Errorf("terminal chunk altered: %+v", terminal)
	}

	// The wrapped channel closes only after the event is emitted.
	events := mem.Events()
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	attrs := events[0].Attributes
	want := map[string]string{
		llminstrument.AttrModel:               "example-model",
		llminstrument.AttrDurationMS:          "250", // Stream() at base, terminal at base+250ms
		llminstrument.AttrInputTokens:         "7",
		llminstrument.AttrOutputTokens:        "11",
		llminstrument.AttrCacheReadTokens:     "13",
		llminstrument.AttrCacheCreationTokens: "17",
		llminstrument.AttrStopReason:          "end_turn",
		llminstrument.AttrAttempts:            "0",
		llminstrument.AttrError:               "",
		event.AttrRunID:                       "run-0002",
		event.AttrSpanID:                      "span-0002",
	}
	if !reflect.DeepEqual(attrs, want) {
		t.Errorf("attributes mismatch:\n got: %#v\nwant: %#v", attrs, want)
	}
}

func TestWithTelemetry_StreamMidError(t *testing.T) {
	mock := llmclient.NewMockClient(llmclient.MockEntryStreamErr("partial", &llmclient.ServerError{StatusCode: 500, Message: "boom"}))
	mem := &sink.MemorySink{}
	client := llminstrument.WithTelemetry(mock, mem, "instrument-test/streamerr", newStepClock(time.Millisecond).Now)

	ch, err := client.Stream(context.Background(), llmclient.Request{Model: "example-model", Prompt: "x"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	text, terminal := drain(t, ch)
	if text != "partial" {
		t.Errorf("partial text = %q, want %q", text, "partial")
	}
	if terminal.Err == nil {
		t.Fatal("terminal error chunk not forwarded")
	}

	attrs := mem.Events()[0].Attributes
	if got := attrs[llminstrument.AttrError]; got != "server" {
		t.Errorf("error attr = %q, want %q", got, "server")
	}
	if got := attrs[llminstrument.AttrStopReason]; got != "" {
		t.Errorf("stop_reason attr = %q, want empty", got)
	}
}

func TestWithTelemetry_StreamPreStreamError(t *testing.T) {
	mock := llmclient.NewMockClient(llmclient.MockEntryErr(&llmclient.AuthError{Message: "denied"}))
	mem := &sink.MemorySink{}
	client := llminstrument.WithTelemetry(mock, mem, "instrument-test/prestream", newStepClock(time.Millisecond).Now)

	ch, err := client.Stream(context.Background(), llmclient.Request{Model: "example-model", Prompt: "x"})
	if err == nil || ch != nil {
		t.Fatalf("Stream = (%v, %v), want pre-stream failure", ch, err)
	}
	events := mem.Events()
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if got := events[0].Attributes[llminstrument.AttrError]; got != "auth" {
		t.Errorf("error attr = %q, want %q", got, "auth")
	}
}

// TestWithTelemetry_AbandonedStream guards the abandonment contract: the
// caller reads one chunk, cancels ctx, and never reads again. The wrapper
// goroutine must still drain the inner channel, emit exactly one event, and
// close the wrapped channel.
func TestWithTelemetry_AbandonedStream(t *testing.T) {
	mock := llmclient.NewMockClient(usageEntry("abcdefgh", llmclient.Usage{InputTokens: 3, OutputTokens: 4}))
	sig := newSignalSink()
	client := llminstrument.WithTelemetry(mock, sig, "instrument-test/abandon", newStepClock(time.Millisecond).Now)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := client.Stream(ctx, llmclient.Request{Model: "example-model", Prompt: "x"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	// Read a single chunk, then abandon by cancelling.
	select {
	case <-ch:
	case <-time.After(guardTimeout):
		t.Fatal("no first chunk")
	}
	cancel()

	// The event must still be emitted once the inner channel closes —
	// proving the wrapper goroutine ran to completion.
	select {
	case <-sig.ch:
	case <-time.After(guardTimeout):
		t.Fatal("no event after abandoning the stream: wrapper goroutine stuck")
	}
	if got := sig.mem.Len(); got != 1 {
		t.Errorf("events = %d, want 1", got)
	}

	// And the wrapped channel must close (drain whatever was in flight).
	guard := time.After(guardTimeout)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-guard:
			t.Fatal("wrapped channel did not close after abandonment")
		}
	}
}

// ---- Sink failure isolation --------------------------------------------------------

func TestWithTelemetry_SinkFailuresSwallowedAndCounted(t *testing.T) {
	mock := llmclient.NewMockClient(
		usageEntry("one", llmclient.Usage{InputTokens: 1}),
		usageEntry("two", llmclient.Usage{InputTokens: 2}),
	)
	client := llminstrument.WithTelemetry(mock, failSink{err: errors.New("sink down")}, "instrument-test/sinkfail", newStepClock(time.Millisecond).Now)

	resp, err := client.Complete(context.Background(), llmclient.Request{Prompt: "x"})
	if err != nil || resp.Text != "one" {
		t.Fatalf("sink failure leaked into the call path: (%q, %v)", resp.Text, err)
	}
	if got := client.DroppedEvents(); got != 1 {
		t.Fatalf("DroppedEvents = %d, want 1", got)
	}

	ch, err := client.Stream(context.Background(), llmclient.Request{Prompt: "x"})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if text, _ := drain(t, ch); text != "two" {
		t.Errorf("stream text = %q, want %q", text, "two")
	}
	if got := client.DroppedEvents(); got != 2 {
		t.Errorf("DroppedEvents = %d, want 2", got)
	}
}

func TestWithTelemetry_InvalidEventDroppedAndCounted(t *testing.T) {
	mock := llmclient.NewMockClient(llmclient.MockEntryOK("ok"))
	mem := &sink.MemorySink{}
	// Empty source fails event validation; the event must be dropped and
	// counted, never emitted, and never break the call.
	client := llminstrument.WithTelemetry(mock, mem, "", newStepClock(time.Millisecond).Now)

	if _, err := client.Complete(context.Background(), llmclient.Request{Prompt: "x"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if mem.Len() != 0 {
		t.Errorf("invalid event was emitted; got %d events", mem.Len())
	}
	if got := client.DroppedEvents(); got != 1 {
		t.Errorf("DroppedEvents = %d, want 1", got)
	}
}
