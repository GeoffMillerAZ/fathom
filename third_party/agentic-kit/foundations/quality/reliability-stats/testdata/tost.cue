package reliabilitystats

import "github.com/geoffmilleraz/agentic-kit/foundations/quality/reliability-stats:reliabilitystats"

validTOSTEquivalent: reliabilitystats.#TOSTResult & {verdict: "equivalent", equivalent: true, unexpected_effect: false, low: -0.03, high: 0.03, upper_95: 0.03}
validTOSTHarm: reliabilitystats.#TOSTResult & {verdict: "harm", equivalent: false, unexpected_effect: false, low: -2.2, high: -1.8, upper_95: -1.8}
validTOSTUnexpectedEffect: reliabilitystats.#TOSTResult & {verdict: "inconclusive", equivalent: false, unexpected_effect: true, low: 1.8, high: 2.2, upper_95: 2.2}
validTOSTInconclusive: reliabilitystats.#TOSTResult & {verdict: "inconclusive", equivalent: false, unexpected_effect: false, low: -3, high: 3, upper_95: 3}
validTOSTZeroUpperBoundary: reliabilitystats.#TOSTResult & {verdict: "inconclusive", equivalent: false, unexpected_effect: false, low: -2, high: 0, upper_95: 0}
validTOSTMarginBoundary: reliabilitystats.#TOSTResult & {verdict: "inconclusive", equivalent: false, unexpected_effect: false, low: 0.5, high: 0.5, upper_95: 0.5}

// Invalid counterparts, uncomment individually to see cue vet reject them:
// invalidTOSTInverted: reliabilitystats.#TOSTResult & {verdict: "inconclusive", equivalent: false, unexpected_effect: false, low: 1, high: -1, upper_95: -1}
// invalidTOSTVerdict: reliabilitystats.#TOSTResult & {verdict: "not equivalent", equivalent: false, unexpected_effect: false, low: 1, high: 2, upper_95: 2}
// invalidTOSTHarmBoundary: reliabilitystats.#TOSTResult & {verdict: "harm", equivalent: false, unexpected_effect: false, low: -1, high: 0, upper_95: 0}
// invalidTOSTBool: reliabilitystats.#TOSTResult & {verdict: "equivalent", equivalent: false, unexpected_effect: false, low: -0.03, high: 0.03, upper_95: 0.03}
// invalidTOSTUnexpectedHarm: reliabilitystats.#TOSTResult & {verdict: "harm", equivalent: false, unexpected_effect: true, low: -2, high: -1, upper_95: -1}
