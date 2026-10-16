package llmclient_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
)

func TestSwappable_UnconfiguredFailsTyped(t *testing.T) {
	s := llmclient.NewSwappable(nil)

	if s.Available() {
		t.Error("Available() = true for a nil inner, want false")
	}

	t.Run("Complete", func(t *testing.T) {
		_, err := s.Complete(context.Background(), llmclient.Request{Prompt: "p"})
		var ue *llmclient.UnavailableError
		if !errors.As(err, &ue) {
			t.Fatalf("err = %T %v, want *llmclient.UnavailableError", err, err)
		}
	})
	t.Run("StreamPreStream", func(t *testing.T) {
		ch, err := s.Stream(context.Background(), llmclient.Request{Prompt: "p"})
		if ch != nil {
			t.Error("unconfigured Stream must fail pre-stream: no channel")
		}
		var ue *llmclient.UnavailableError
		if !errors.As(err, &ue) {
			t.Fatalf("err = %T %v, want *llmclient.UnavailableError", err, err)
		}
	})
}

func TestSwappable_NewWithInnerIsAvailable(t *testing.T) {
	s := llmclient.NewSwappable(llmclient.NewMockClient(llmclient.MockEntryOK("hi")))
	if !s.Available() {
		t.Fatal("Available() = false with a configured inner, want true")
	}
	resp, err := s.Complete(context.Background(), llmclient.Request{Prompt: "p"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "hi" {
		t.Errorf("Text = %q, want %q", resp.Text, "hi")
	}
}

// TestSwappable_SwapHotSwapsAndUnconfigures walks the full Settings-page
// lifecycle: unconfigured → credential entered (Swap real client) →
// credential cleared (Swap nil) → credential re-entered.
func TestSwappable_SwapHotSwapsAndUnconfigures(t *testing.T) {
	s := llmclient.NewSwappable(nil)
	ctx := context.Background()

	// Credentials arrive: swap in a client.
	s.Swap(llmclient.NewMockClient(llmclient.MockEntryOK("first")))
	if !s.Available() {
		t.Fatal("Available() = false after Swap(client), want true")
	}
	if resp, err := s.Complete(ctx, llmclient.Request{Prompt: "p"}); err != nil || resp.Text != "first" {
		t.Fatalf("Complete after swap = (%q, %v), want (first, nil)", resp.Text, err)
	}

	// Credentials cleared: back to unconfigured.
	s.Swap(nil)
	if s.Available() {
		t.Fatal("Available() = true after Swap(nil), want false")
	}
	_, err := s.Complete(ctx, llmclient.Request{Prompt: "p"})
	var ue *llmclient.UnavailableError
	if !errors.As(err, &ue) {
		t.Fatalf("err after Swap(nil) = %T %v, want *llmclient.UnavailableError", err, err)
	}

	// Re-entered: a fresh client takes over.
	s.Swap(llmclient.NewMockClient(llmclient.MockEntryOK("second")))
	if resp, err := s.Complete(ctx, llmclient.Request{Prompt: "p"}); err != nil || resp.Text != "second" {
		t.Fatalf("Complete after re-swap = (%q, %v), want (second, nil)", resp.Text, err)
	}
}

// TestSwappable_UnavailableNotRetried pins the retry classification:
// UnavailableError must fail fast under WithRetry (Attempts == 1) — callers
// surface "configure your key", they never retry their way to a credential.
func TestSwappable_UnavailableNotRetried(t *testing.T) {
	c := llmclient.WithRetry(llmclient.NewSwappable(nil), llmclient.RetryConfig{MaxAttempts: 3})

	resp, err := c.Complete(context.Background(), llmclient.Request{Prompt: "p"})
	var ue *llmclient.UnavailableError
	if !errors.As(err, &ue) {
		t.Fatalf("err = %T %v, want *llmclient.UnavailableError", err, err)
	}
	if resp.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1 (UnavailableError must fail fast, not retry)", resp.Attempts)
	}

	ch, err := c.Stream(context.Background(), llmclient.Request{Prompt: "p"})
	if ch != nil {
		t.Error("Stream must fail pre-stream: no channel")
	}
	if !errors.As(err, &ue) {
		t.Fatalf("Stream err = %T %v, want *llmclient.UnavailableError", err, err)
	}
}

// TestSwappable_ConcurrentSwapAndCalls hammers Swap (alternating client/nil)
// concurrently with Complete and Stream callers under -race. Every call must
// land on exactly one of the two valid outcomes — success or a typed
// UnavailableError — and every returned stream must close.
func TestSwappable_ConcurrentSwapAndCalls(t *testing.T) {
	const (
		swappers   = 4
		callers    = 8
		iterations = 50
	)
	s := llmclient.NewSwappable(llmclient.NewMockClient()) // exhausted mock: serves fallback responses forever
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < swappers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for n := 0; n < iterations; n++ {
				if (id+n)%2 == 0 {
					s.Swap(llmclient.NewMockClient())
				} else {
					s.Swap(nil)
				}
				s.Available() // racing reads must be safe too
			}
		}(i)
	}

	check := func(err error) {
		if err == nil {
			return
		}
		var ue *llmclient.UnavailableError
		if !errors.As(err, &ue) {
			t.Errorf("err = %T %v, want nil or *llmclient.UnavailableError", err, err)
		}
	}
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < iterations; n++ {
				_, err := s.Complete(ctx, llmclient.Request{Prompt: "p"})
				check(err)

				ch, err := s.Stream(ctx, llmclient.Request{Prompt: "p"})
				check(err)
				if err == nil {
					for range ch { // drain until close per the contract
					}
				}
			}
		}()
	}
	wg.Wait()
}
