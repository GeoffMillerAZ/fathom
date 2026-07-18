package llminstrument_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	costtracker "github.com/geoffmilleraz/agentic-kit/foundations/cost/cost-tracker"
	budgetguard "github.com/geoffmilleraz/agentic-kit/foundations/governance/budget-guard"
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	llminstrument "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/instrument"
)

// ---- helpers ----------------------------------------------------------------

// newBudgetGuard creates a Guard with ample req limits and the given USD cap.
func newBudgetGuard(maxUSDPerDay float64) *budgetguard.Guard {
	cfg := budgetguard.Config{
		MaxReqPerMin: 1000,
		MaxReqPerDay: 10000,
		MaxUSDPerDay: maxUSDPerDay,
	}
	ts := testBase
	return budgetguard.New(cfg, func() time.Time { return ts })
}

// tightBudgetGuard creates a Guard with req/min=1 so a single Charge fills it.
func tightBudgetGuard() *budgetguard.Guard {
	cfg := budgetguard.Config{
		MaxReqPerMin: 1,
		MaxReqPerDay: 10000,
		MaxUSDPerDay: 100.0,
	}
	ts := testBase
	return budgetguard.New(cfg, func() time.Time { return ts })
}

// budgetTable has one model with known prices for deterministic cost assertions.
var budgetTable = costtracker.PriceTable{
	"test-model": {InputPerMTok: 1.0, OutputPerMTok: 2.0},
}

// buildBudgetClient constructs a WithBudget-wrapped MockClient for testing.
func buildBudgetClient(mock *llmclient.MockClient, guard *budgetguard.Guard) llmclient.LLMClient {
	return llminstrument.WithBudget(mock, guard, budgetTable, nil, "test-model")
}

// ---- Tests ------------------------------------------------------------------

// TestWithBudget_ExceededPreFlight_NoCallMade verifies that a pre-flight
// budget denial returns *BudgetExceededError and does not call the inner client.
func TestWithBudget_ExceededPreFlight_NoCallMade(t *testing.T) {
	guard := tightBudgetGuard()
	mock := llmclient.NewMockClient(
		llmclient.MockEntryOK("first"),
		llmclient.MockEntryOK("second"),
	)
	client := buildBudgetClient(mock, guard)

	// First call fills the req/min=1 quota.
	_, err := client.Complete(context.Background(), llmclient.Request{Model: "test-model", Prompt: "x"})
	if err != nil {
		t.Fatalf("first call: unexpected error: %v", err)
	}

	// Second call should be blocked by the guard.
	_, err = client.Complete(context.Background(), llmclient.Request{Model: "test-model", Prompt: "x"})
	if err == nil {
		t.Fatal("second call: expected *BudgetExceededError, got nil")
	}
	var be *llminstrument.BudgetExceededError
	if !errors.As(err, &be) {
		t.Fatalf("second call: expected *BudgetExceededError, got %T: %v", err, err)
	}

	// The inner client must have been called exactly once (first call only).
	if got := mock.CallCount(); got != 1 {
		t.Errorf("MockClient.CallCount = %d, want 1 (second call must not reach inner)", got)
	}
}

// TestWithBudget_SuccessSettlesActual verifies that on a successful call the
// guard's USDThisDay reflects the actual cost (not the estimate).
func TestWithBudget_SuccessSettlesActual(t *testing.T) {
	guard := newBudgetGuard(10.0)
	// Script: 100 input, 50 output tokens → actual cost =
	//   100/1e6 * 1.0 + 50/1e6 * 2.0 = 0.0001 + 0.0001 = 0.0002
	actual := llmclient.Usage{InputTokens: 100, OutputTokens: 50}
	mock := llmclient.NewMockClient(usageEntry("ok", actual))
	client := buildBudgetClient(mock, guard)

	_, err := client.Complete(context.Background(), llmclient.Request{Model: "test-model", Prompt: "hello"})
	if err != nil {
		t.Fatalf("Complete: unexpected error: %v", err)
	}

	snap := guard.Snapshot()
	wantUSD := float64(100)*1.0/1e6 + float64(50)*2.0/1e6
	if snap.USDThisDay < wantUSD-1e-9 || snap.USDThisDay > wantUSD+1e-9 {
		t.Errorf("guard.USDThisDay = %.9f, want %.9f (actual cost)", snap.USDThisDay, wantUSD)
	}
}

// TestWithBudget_InnerErrorSettlesZero verifies that when the inner client
// returns an error, the pre-flight estimate is fully reversed ($0 settle).
func TestWithBudget_InnerErrorSettlesZero(t *testing.T) {
	guard := newBudgetGuard(10.0)
	sentinel := errors.New("inner failure")
	mock := llmclient.NewMockClient(llmclient.MockEntryErr(sentinel))
	client := buildBudgetClient(mock, guard)

	_, err := client.Complete(context.Background(), llmclient.Request{Model: "test-model", Prompt: "hello"})
	if err == nil {
		t.Fatal("expected error from inner, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}

	// After error the guard should have settled to $0 — USDThisDay near zero.
	snap := guard.Snapshot()
	if snap.USDThisDay > 1e-9 {
		t.Errorf("guard.USDThisDay after inner error = %.9f, want ≈0 (estimate reversed)", snap.USDThisDay)
	}
}

// TestWithBudget_Stream_SettleAtTerminal verifies that a successful stream
// settles to actual cost when the terminal chunk arrives.
func TestWithBudget_Stream_SettleAtTerminal(t *testing.T) {
	guard := newBudgetGuard(10.0)
	actual := llmclient.Usage{InputTokens: 200, OutputTokens: 100}
	mock := llmclient.NewMockClient(usageEntry("streaming text", actual))
	client := buildBudgetClient(mock, guard)

	ch, err := client.Stream(context.Background(), llmclient.Request{Model: "test-model", Prompt: "stream me"})
	if err != nil {
		t.Fatalf("Stream: unexpected error: %v", err)
	}
	drain(t, ch)

	snap := guard.Snapshot()
	wantUSD := float64(200)*1.0/1e6 + float64(100)*2.0/1e6
	if snap.USDThisDay < wantUSD-1e-9 || snap.USDThisDay > wantUSD+1e-9 {
		t.Errorf("guard.USDThisDay after stream = %.9f, want %.9f (actual cost)", snap.USDThisDay, wantUSD)
	}
}

// TestWithBudget_Stream_PreStreamError verifies that a pre-stream error
// causes the estimate to be settled to $0.
func TestWithBudget_Stream_PreStreamError(t *testing.T) {
	guard := newBudgetGuard(10.0)
	sentinel := errors.New("pre-stream failure")
	mock := llmclient.NewMockClient(llmclient.MockEntryErr(sentinel))
	client := buildBudgetClient(mock, guard)

	_, err := client.Stream(context.Background(), llmclient.Request{Model: "test-model", Prompt: "x"})
	if err == nil {
		t.Fatal("expected pre-stream error, got nil")
	}

	snap := guard.Snapshot()
	if snap.USDThisDay > 1e-9 {
		t.Errorf("guard.USDThisDay after pre-stream error = %.9f, want ≈0", snap.USDThisDay)
	}
}

// TestWithBudget_Stream_MidStreamError verifies that a mid-stream error
// causes the estimate to be settled to $0.
func TestWithBudget_Stream_MidStreamError(t *testing.T) {
	guard := newBudgetGuard(10.0)
	sentinel := errors.New("mid-stream failure")
	mock := llmclient.NewMockClient(llmclient.MockEntryStreamErr("partial", sentinel))
	client := buildBudgetClient(mock, guard)

	ch, err := client.Stream(context.Background(), llmclient.Request{Model: "test-model", Prompt: "x"})
	if err != nil {
		t.Fatalf("Stream: unexpected pre-stream error: %v", err)
	}
	drain(t, ch) // consume stream to let the settle goroutine run

	snap := guard.Snapshot()
	if snap.USDThisDay > 1e-9 {
		t.Errorf("guard.USDThisDay after mid-stream error = %.9f, want ≈0", snap.USDThisDay)
	}
}

// TestWithBudget_RetryInteraction verifies the correct layering: WithBudget
// wraps WithRetry, so retries are invisible to the guard. The guard charges
// once and settles once, but the mock client is called twice (one rate_limit,
// one success).
func TestWithBudget_RetryInteraction(t *testing.T) {
	guard := newBudgetGuard(10.0)
	actual := llmclient.Usage{InputTokens: 50, OutputTokens: 25}
	mock := llmclient.NewMockClient(
		llmclient.MockEntryRateLimit("slow down"),
		usageEntry("ok", actual),
	)

	// WithBudget OUTSIDE WithRetry — correct composition.
	inner := llmclient.WithRetry(mock, llmclient.RetryConfig{MaxAttempts: 3})
	client := llminstrument.WithBudget(inner, guard, budgetTable, nil, "test-model")

	resp, err := client.Complete(context.Background(), llmclient.Request{Model: "test-model", Prompt: "hello"})
	if err != nil {
		t.Fatalf("Complete: unexpected error: %v", err)
	}
	if resp.Attempts != 2 {
		t.Errorf("Attempts = %d, want 2 (one rate_limit, one success)", resp.Attempts)
	}

	// Mock should have been called twice (two network attempts).
	if got := mock.CallCount(); got != 2 {
		t.Errorf("MockClient.CallCount = %d, want 2 (retry is transparent to budget)", got)
	}

	// Guard should reflect actual cost, not double-billed.
	snap := guard.Snapshot()
	wantUSD := float64(50)*1.0/1e6 + float64(25)*2.0/1e6
	if snap.USDThisDay < wantUSD-1e-9 || snap.USDThisDay > wantUSD+1e-9 {
		t.Errorf("guard.USDThisDay = %.9f, want %.9f (one logical charge)", snap.USDThisDay, wantUSD)
	}
}

// TestWithBudget_ConcurrentBudget exercises WithBudget from many goroutines
// under -race to verify no data races.
func TestWithBudget_ConcurrentBudget(t *testing.T) {
	const goroutines = 10

	guard := newBudgetGuard(1000.0)

	var entries []llmclient.MockEntry
	for i := 0; i < goroutines; i++ {
		entries = append(entries, usageEntry("ok", llmclient.Usage{InputTokens: 10, OutputTokens: 5}))
	}
	mock := llmclient.NewMockClient(entries...)
	client := buildBudgetClient(mock, guard)

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = client.Complete(context.Background(), llmclient.Request{Model: "test-model", Prompt: "concurrent"})
		}()
	}
	wg.Wait()

	// Verify the guard's state is self-consistent (no data race detected by -race).
	snap := guard.Snapshot()
	if snap.USDThisDay < 0 {
		t.Errorf("guard.USDThisDay should be non-negative, got %.9f", snap.USDThisDay)
	}
}
