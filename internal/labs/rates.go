// Package labs provides deterministic, file-only feature-lab readings.
package labs

import (
	"math"

	"github.com/geoffmilleraz/fathom/internal/ask"
)

// ConfusionTable holds nonnegative counts against binary ground truth.
type ConfusionTable struct{ TP, FP, TN, FN int }

// Rate is an estimate and its two-sided 95% Wilson score interval.
type Rate struct{ Value, Lo, Hi float64 }

// Rates separates recall, specificity, precision and negative predictive value.
type Rates struct{ TPR, TNR, PPV, NPV Rate }

// BinaryRates reads a confusion table. An empty margin is unmeasured:
// both the estimate and interval are NaN, rather than a measured zero.
func BinaryRates(table ConfusionTable) Rates {
	rate := func(k, other int) Rate {
		n := k + other
		if n == 0 {
			return Rate{math.NaN(), math.NaN(), math.NaN()}
		}
		lo, hi := ask.Wilson(k, n)
		return Rate{float64(k) / float64(n), lo, hi}
	}
	return Rates{
		TPR: rate(table.TP, table.FN),
		TNR: rate(table.TN, table.FP),
		PPV: rate(table.TP, table.FP),
		NPV: rate(table.TN, table.FN),
	}
}
