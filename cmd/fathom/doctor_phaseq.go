package main

// doctor_phaseq.go — phase-Q doctor checks (observability surfaces,
// docs/design/observability-surfaces.md §8 item 3). House honesty
// discipline (doctor.go's own doc comment): an uncheckable state is
// `skipped` with a reason, never a guessed pass; a real, observable
// condition worth attention but not a wiring failure is `warn`, never
// `fail`.

import (
	"fmt"

	"github.com/geoffmilleraz/fathom/internal/metrics"
)

// checkMetricsTier reports the active FATHOM_METRICS tier — always
// doctorPass (an operator's deliberate choice, mirrors checkChannelsEnv's
// exact discipline: this is a diagnostic, not a wiring-broken verdict).
// An invalid env value still passes but names the fallback explicitly, so
// a typo'd FATHOM_METRICS is visible rather than silently defaulting.
func checkMetricsTier() doctorCheck {
	const name = "metrics_tier"
	tier, note := metrics.ResolveMetricsTier()
	reason := fmt.Sprintf("FATHOM_METRICS tier: %s (no tier spends LLM tokens)", tier)
	if note != "" {
		reason += "; " + note
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
}
