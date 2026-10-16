package llmclient_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

// fixedClock returns a clock that always returns t.
func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

// advanceClock returns a pointer to a time that can be mutated, plus a clock
// function that reads it — lets tests simulate time advancing.
func advanceClock(initial time.Time) (*time.Time, func() time.Time) {
	tp := &initial
	return tp, func() time.Time { return *tp }
}

func TestFailover_PrimaryOK(t *testing.T) {
	primary := llmclient.NewMockClient(llmclient.MockEntryOK("primary response"))
	secondary := llmclient.NewMockClient(llmclient.MockEntryOK("secondary response"))

	f := llmclient.NewFailover(
		[]llmclient.LLMClient{primary, secondary},
		llmclient.WithCooldown(5*time.Minute, fixedClock(time.Now())),
	)

	resp, err := f.Complete(context.Background(), llmclient.Request{Prompt: "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "primary response" {
		t.Errorf("expected primary response, got %q", resp.Text)
	}
	if secondary.CallCount() != 0 {
		t.Errorf("secondary must not be called when primary OK; got %d calls", secondary.CallCount())
	}

	d := f.(interface {
		LastDecision() llmclient.FailoverDecision
	}).LastDecision()
	if d.LaneIndex != 0 || d.FallbackActive {
		t.Errorf("unexpected decision: %+v", d)
	}
}

func TestFailover_AuthFail_Secondary(t *testing.T) {
	primary := llmclient.NewMockClient(llmclient.MockEntryErr(&llmclient.AuthError{Message: "bad token"}))
	secondary := llmclient.NewMockClient(llmclient.MockEntryOK("fallback ok"))

	now := time.Now()
	f := llmclient.NewFailover(
		[]llmclient.LLMClient{primary, secondary},
		llmclient.WithCooldown(5*time.Minute, fixedClock(now)),
	)

	resp, err := f.Complete(context.Background(), llmclient.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "fallback ok" {
		t.Errorf("expected fallback ok, got %q", resp.Text)
	}

	d := f.(interface {
		LastDecision() llmclient.FailoverDecision
	}).LastDecision()
	if d.LaneIndex != 1 {
		t.Errorf("expected lane 1, got %d", d.LaneIndex)
	}
	if !d.FallbackActive {
		t.Error("expected FallbackActive=true")
	}
	if d.Reason != llmclient.FailoverReasonAuthError {
		t.Errorf("expected FailoverReasonAuthError, got %q", d.Reason)
	}
}

func TestFailover_RateLimit_Secondary(t *testing.T) {
	primary := llmclient.NewMockClient(llmclient.MockEntryErr(&llmclient.RateLimitError{Message: "slow down"}))
	secondary := llmclient.NewMockClient(llmclient.MockEntryOK("fallback"))

	f := llmclient.NewFailover(
		[]llmclient.LLMClient{primary, secondary},
		llmclient.WithCooldown(5*time.Minute, fixedClock(time.Now())),
	)

	resp, err := f.Complete(context.Background(), llmclient.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "fallback" {
		t.Errorf("expected fallback, got %q", resp.Text)
	}

	d := f.(interface {
		LastDecision() llmclient.FailoverDecision
	}).LastDecision()
	if d.Reason != llmclient.FailoverReasonRateLimit {
		t.Errorf("expected FailoverReasonRateLimit, got %q", d.Reason)
	}
}

func TestFailover_UnavailablePrimary_Secondary(t *testing.T) {
	primary := llmclient.NewMockClient(llmclient.MockEntryErr(&llmclient.UnavailableError{Message: "no client"}))
	secondary := llmclient.NewMockClient(llmclient.MockEntryOK("secondary ok"))

	f := llmclient.NewFailover(
		[]llmclient.LLMClient{primary, secondary},
		llmclient.WithCooldown(5*time.Minute, fixedClock(time.Now())),
	)

	resp, err := f.Complete(context.Background(), llmclient.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "secondary ok" {
		t.Errorf("got %q", resp.Text)
	}

	d := f.(interface {
		LastDecision() llmclient.FailoverDecision
	}).LastDecision()
	if d.Reason != llmclient.FailoverReasonUnavailable {
		t.Errorf("expected FailoverReasonUnavailable, got %q", d.Reason)
	}
}

func TestFailover_InvalidRequestPropagates(t *testing.T) {
	primary := llmclient.NewMockClient(llmclient.MockEntryErr(&llmclient.InvalidRequestError{Message: "bad param"}))
	secondary := llmclient.NewMockClient(llmclient.MockEntryOK("should not reach"))

	f := llmclient.NewFailover(
		[]llmclient.LLMClient{primary, secondary},
		llmclient.WithCooldown(5*time.Minute, fixedClock(time.Now())),
	)

	_, err := f.Complete(context.Background(), llmclient.Request{})
	var ire *llmclient.InvalidRequestError
	if !errors.As(err, &ire) {
		t.Fatalf("expected InvalidRequestError propagated, got %T: %v", err, err)
	}
	if secondary.CallCount() != 0 {
		t.Errorf("secondary must not be called for ineligible error; got %d calls", secondary.CallCount())
	}
}

func TestFailover_StickyWindow_Honored(t *testing.T) {
	tp, clk := advanceClock(time.Unix(0, 0))
	window := 5 * time.Minute

	primary := llmclient.NewMockClient(
		llmclient.MockEntryErr(&llmclient.AuthError{Message: "auth fail"}),
		llmclient.MockEntryOK("primary recovered"),
	)
	secondary := llmclient.NewMockClient(
		llmclient.MockEntryOK("secondary ok"),
		llmclient.MockEntryOK("secondary ok 2"),
	)

	f := llmclient.NewFailover(
		[]llmclient.LLMClient{primary, secondary},
		llmclient.WithCooldown(window, clk),
	)

	// First call: primary fails, secondary serves.
	resp, err := f.Complete(context.Background(), llmclient.Request{})
	if err != nil || resp.Text != "secondary ok" {
		t.Fatalf("first call: got err=%v resp=%q", err, resp.Text)
	}

	// Second call within cooldown window — primary should be skipped.
	*tp = tp.Add(1 * time.Minute) // still inside 5m window
	resp, err = f.Complete(context.Background(), llmclient.Request{})
	if err != nil || resp.Text != "secondary ok 2" {
		t.Fatalf("second call (cooldown active): got err=%v resp=%q", err, resp.Text)
	}
	if primary.CallCount() != 1 {
		t.Errorf("primary should be called only once (cooldown); got %d", primary.CallCount())
	}
}

func TestFailover_StickyWindow_Expiry(t *testing.T) {
	tp, clk := advanceClock(time.Unix(0, 0))
	window := 5 * time.Minute

	primary := llmclient.NewMockClient(
		llmclient.MockEntryErr(&llmclient.AuthError{Message: "auth fail"}),
		llmclient.MockEntryOK("primary recovered"),
	)
	secondary := llmclient.NewMockClient(
		llmclient.MockEntryOK("secondary ok"),
	)

	f := llmclient.NewFailover(
		[]llmclient.LLMClient{primary, secondary},
		llmclient.WithCooldown(window, clk),
	)

	// First call: primary fails → secondary serves.
	f.Complete(context.Background(), llmclient.Request{}) //nolint

	// Advance past cooldown window.
	*tp = tp.Add(6 * time.Minute)

	// Third call after expiry: primary should be tried again.
	resp, err := f.Complete(context.Background(), llmclient.Request{})
	if err != nil || resp.Text != "primary recovered" {
		t.Fatalf("post-expiry: expected primary recovered, got err=%v resp=%q", err, resp.Text)
	}
	if primary.CallCount() != 2 {
		t.Errorf("expected primary called twice (fail + recover), got %d", primary.CallCount())
	}
}

func TestFailover_ZeroDeltaStream_Failover(t *testing.T) {
	// primary stream returns pre-stream RateLimitError
	primary := llmclient.NewMockClient(llmclient.MockEntryErr(&llmclient.RateLimitError{Message: "rate"}))
	secondary := llmclient.NewMockClient(llmclient.MockEntryOK("stream content"))

	f := llmclient.NewFailover(
		[]llmclient.LLMClient{primary, secondary},
		llmclient.WithCooldown(5*time.Minute, fixedClock(time.Now())),
	)

	ch, err := f.Stream(context.Background(), llmclient.Request{})
	if err != nil {
		t.Fatalf("unexpected pre-stream error: %v", err)
	}
	var got string
	for c := range ch {
		got += c.TextDelta
	}
	if got != "stream content" {
		t.Errorf("stream failover: expected %q, got %q", "stream content", got)
	}
}

func TestFailover_MidStreamError_NofailOver(t *testing.T) {
	// primary stream emits partial text then mid-stream error — must NOT failover
	boom := errors.New("mid-stream boom")
	primary := llmclient.NewMockClient(llmclient.MockEntryStreamErr("partial", boom))
	secondary := llmclient.NewMockClient(llmclient.MockEntryOK("secondary should not serve"))

	f := llmclient.NewFailover(
		[]llmclient.LLMClient{primary, secondary},
		llmclient.WithCooldown(5*time.Minute, fixedClock(time.Now())),
	)

	ch, err := f.Stream(context.Background(), llmclient.Request{})
	if err != nil {
		t.Fatalf("unexpected pre-stream error: %v", err)
	}
	chunks := drainStream(t, ch)
	last := chunks[len(chunks)-1]
	if !errors.Is(last.Err, boom) {
		t.Errorf("expected mid-stream boom propagated, got %v", last.Err)
	}
	if secondary.CallCount() != 0 {
		t.Errorf("secondary must not be called after mid-stream error; got %d calls", secondary.CallCount())
	}
}

func TestFailover_DecisionAccessor_BeforeAnyRequest(t *testing.T) {
	primary := llmclient.NewMockClient()
	secondary := llmclient.NewMockClient()

	f := llmclient.NewFailover(
		[]llmclient.LLMClient{primary, secondary},
	)

	d := f.(interface {
		LastDecision() llmclient.FailoverDecision
	}).LastDecision()
	if d.LaneIndex != 0 || d.FallbackActive || d.Reason != llmclient.FailoverReasonNone {
		t.Errorf("before any request, expected zero decision, got %+v", d)
	}
}

func TestFailover_RequiresAtLeastTwoLanes(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for fewer than 2 lanes")
		}
	}()
	llmclient.NewFailover([]llmclient.LLMClient{llmclient.NewMockClient()})
}
