package reliabilitystats

import "github.com/geoffmilleraz/agentic-kit/foundations/quality/reliability-stats:reliabilitystats"

validHolmInput: reliabilitystats.#PValues & [0.01, 0.04, 0.03, 0.005]
validHolmAdjusted: reliabilitystats.#PValues & [0.03, 0.06, 0.06, 0.02]
validHolmEmpty: reliabilitystats.#PValues & []

// Invalid counterparts, uncomment individually to see cue vet reject them:
// invalidHolmNegative: reliabilitystats.#PValues & [-0.01]
// invalidHolmAboveOne: reliabilitystats.#PValues & [1.1]
