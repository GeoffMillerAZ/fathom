package llminstrument_test

import (
	"errors"
	"testing"

	llminstrument "github.com/geoffmilleraz/agentic-kit/foundations/llmclient/instrument"
)

// TestErrBudgetExceeded_Sentinel verifies the three required compatibility
// properties of the ErrBudgetExceeded sentinel:
//
//  1. errors.Is(err, ErrBudgetExceeded) is true for a *BudgetExceededError.
//  2. errors.As(err, &be) still works so callers can read Reason.
//  3. errors.Is does NOT match an unrelated error.
func TestErrBudgetExceeded_Sentinel(t *testing.T) {
	concrete := &llminstrument.BudgetExceededError{Reason: "req/min exceeded"}

	t.Run("errors.Is matches sentinel", func(t *testing.T) {
		if !errors.Is(concrete, llminstrument.ErrBudgetExceeded) {
			t.Errorf("errors.Is(%v, ErrBudgetExceeded) = false, want true", concrete)
		}
	})

	t.Run("errors.As still extracts Reason", func(t *testing.T) {
		var be *llminstrument.BudgetExceededError
		if !errors.As(concrete, &be) {
			t.Fatal("errors.As(*BudgetExceededError) = false, want true")
		}
		if be.Reason != "req/min exceeded" {
			t.Errorf("Reason = %q, want %q", be.Reason, "req/min exceeded")
		}
	})

	t.Run("errors.Is does not match unrelated error", func(t *testing.T) {
		other := errors.New("something else")
		if errors.Is(other, llminstrument.ErrBudgetExceeded) {
			t.Error("errors.Is(unrelated, ErrBudgetExceeded) = true, want false")
		}
	})

	t.Run("errors.Is works through wrapping", func(t *testing.T) {
		// Consumers may wrap the error further; sentinel must still match.
		wrapped := &wrappedErr{inner: concrete}
		if !errors.Is(wrapped, llminstrument.ErrBudgetExceeded) {
			t.Error("errors.Is through one extra wrap = false, want true")
		}
	})

	t.Run("sentinel alias interop", func(t *testing.T) {
		// Verify a caller can alias the sentinel and still match.
		alias := llminstrument.ErrBudgetExceeded
		if !errors.Is(concrete, alias) {
			t.Error("errors.Is with aliased sentinel = false, want true")
		}
	})
}

// wrappedErr is a minimal error that wraps another for the through-wrap test.
type wrappedErr struct{ inner error }

func (w *wrappedErr) Error() string { return "wrapped: " + w.inner.Error() }
func (w *wrappedErr) Unwrap() error { return w.inner }
