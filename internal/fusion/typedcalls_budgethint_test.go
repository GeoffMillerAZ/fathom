package fusion

import (
	"fmt"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

func TestOverBudgetHintNamesTheEnvAndAValue(t *testing.T) {
	h := overBudgetHint(fmt.Errorf("wrapped: %w", tenancy.ErrOverBudget))
	if !strings.Contains(h, "FATHOM_TENANCY_BUDGET=2GiB") {
		t.Fatalf("hint %q must name FATHOM_TENANCY_BUDGET and a value", h)
	}
	if overBudgetHint(fmt.Errorf("other")) != "" {
		t.Fatal("hint must be empty for a non-budget error")
	}
}
