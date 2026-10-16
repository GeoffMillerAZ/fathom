package llminstrument_test

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"

	costtracker "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/cost/cost-tracker"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	llminstrument "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient/instrument"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/event"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/sink"
)

// guardTimeout bounds every blocking wait so a regression fails the test
// instead of hanging it. It is a guard, not a synchronization sleep.
const guardTimeout = 5 * time.Second

var testBase = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

// stepClock is a deterministic injected clock: every Now call returns the
// current instant and advances it by step. Safe for concurrent use.
type stepClock struct {
	mu   sync.Mutex
	t    time.Time
	step time.Duration
}

func newStepClock(step time.Duration) *stepClock {
	return &stepClock{t: testBase, step: step}
}

func (c *stepClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.t
	c.t = c.t.Add(c.step)
	return now
}

// failSink always rejects emits, for the swallow-and-count tests.
type failSink struct{ err error }

func (f failSink) Emit(event.Event) error { return f.err }

// signalSink records events like sink.MemorySink and additionally signals
// each emit on a channel, so tests can await an emission from a background
// goroutine without polling or sleeping.
type signalSink struct {
	mem *sink.MemorySink
	ch  chan event.Event
}

func newSignalSink() *signalSink {
	return &signalSink{mem: &sink.MemorySink{}, ch: make(chan event.Event, 16)}
}

func (s *signalSink) Emit(e event.Event) error {
	_ = s.mem.Emit(e)
	s.ch <- e
	return nil
}

// usageEntry scripts a successful mock response with explicit usage.
func usageEntry(text string, u llmclient.Usage) llmclient.MockEntry {
	return llmclient.MockEntryWithResponse(llmclient.Response{Text: text, Usage: u})
}

// drain consumes the stream until it closes, returning the concatenated text
// and the terminal chunk. It fails the test if the channel does not close
// within the guard timeout.
func drain(t *testing.T, ch <-chan llmclient.StreamChunk) (text string, terminal llmclient.StreamChunk) {
	t.Helper()
	guard := time.After(guardTimeout)
	for {
		select {
		case chunk, ok := <-ch:
			if !ok {
				return text, terminal
			}
			text += chunk.TextDelta
			if chunk.StopReason != "" || chunk.Err != nil {
				terminal = chunk
			}
		case <-guard:
			t.Fatal("stream did not close within the guard timeout")
		}
	}
}

func floatEq(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// ---- Documented composition order ---------------------------------------------

// TestCompositionOrder_OneEventPerLogicalCall pins the behavior documented in
// the package doc: with WithCost(WithTelemetry(WithRetry(client))), a call
// that needs a retry produces exactly ONE telemetry event (attempts=2) and
// exactly ONE cost record carrying the final usage.
func TestCompositionOrder_OneEventPerLogicalCall(t *testing.T) {
	mock := llmclient.NewMockClient(
		llmclient.MockEntryRateLimit("slow down"),
		usageEntry("ok", llmclient.Usage{InputTokens: 10, OutputTokens: 5}),
	)
	mem := &sink.MemorySink{}
	clk := newStepClock(time.Millisecond)
	tracker := costtracker.New()
	// EXAMPLE prices for the test only.
	table := costtracker.PriceTable{
		"example-model": {InputPerMTok: 1.0, OutputPerMTok: 2.0},
	}

	client := llminstrument.WithCost(
		llminstrument.WithTelemetry(
			llmclient.WithRetry(mock, llmclient.RetryConfig{MaxAttempts: 3}),
			mem, "instrument-test/compose", clk.Now,
		),
		tracker, table, "compose-label",
	)

	resp, err := client.Complete(context.Background(), llmclient.Request{
		Model:  "example-model",
		Prompt: "hello",
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Attempts != 2 {
		t.Fatalf("Attempts = %d, want 2 (one rate limit, one success)", resp.Attempts)
	}

	// Exactly one event per LOGICAL call, final outcome, attempts preserved.
	events := mem.Events()
	if len(events) != 1 {
		t.Fatalf("got %d events, want exactly 1 per logical call", len(events))
	}
	attrs := events[0].Attributes
	if got := attrs[llminstrument.AttrAttempts]; got != "2" {
		t.Errorf("attempts attr = %q, want %q", got, "2")
	}
	if got := attrs[llminstrument.AttrError]; got != "" {
		t.Errorf("error attr = %q, want \"\" (retried into success IS success)", got)
	}
	if got := attrs[llminstrument.AttrInputTokens]; got != "10" {
		t.Errorf("input_tokens attr = %q, want %q", got, "10")
	}

	// Exactly one cost record carrying the final usage.
	totals := tracker.Totals()
	if totals.Calls != 1 {
		t.Fatalf("tracker Calls = %d, want exactly 1 per logical call", totals.Calls)
	}
	wantUSD := 10*1.0/1e6 + 5*2.0/1e6
	if totals.InputTokens != 10 || totals.OutputTokens != 5 || !floatEq(totals.USD, wantUSD) {
		t.Errorf("tracker Totals = %+v, want input=10 output=5 usd=%v", totals, wantUSD)
	}
}

// TestComposedStack_ConcurrentCalls exercises the full recommended stack from
// many goroutines under -race.
func TestComposedStack_ConcurrentCalls(t *testing.T) {
	const (
		goroutines = 8
		perG       = 10
	)
	var entries []llmclient.MockEntry
	for i := 0; i < goroutines*perG; i++ {
		entries = append(entries, usageEntry("ok", llmclient.Usage{InputTokens: 1, OutputTokens: 1}))
	}
	mock := llmclient.NewMockClient(entries...)
	mem := &sink.MemorySink{}
	clk := newStepClock(time.Millisecond)
	tracker := costtracker.New()
	table := costtracker.PriceTable{"example-model": {InputPerMTok: 1.0, OutputPerMTok: 1.0}}

	client := llminstrument.WithCost(
		llminstrument.WithTelemetry(
			llmclient.WithRetry(mock, llmclient.RetryConfig{MaxAttempts: 2}),
			mem, "instrument-test/concurrent", clk.Now,
		),
		tracker, table, "concurrent",
	)

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perG; j++ {
				if _, err := client.Complete(context.Background(), llmclient.Request{Model: "example-model", Prompt: "x"}); err != nil {
					t.Errorf("Complete: %v", err)
				}
			}
		}()
	}
	wg.Wait()

	if got, want := mem.Len(), goroutines*perG; got != want {
		t.Errorf("events = %d, want %d", got, want)
	}
	if got, want := tracker.Totals().Calls, int64(goroutines*perG); got != want {
		t.Errorf("tracker Calls = %d, want %d", got, want)
	}
}
