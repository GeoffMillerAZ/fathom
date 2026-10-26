package reliabilitystats

import "github.com/geoffmilleraz/agentic-kit/foundations/quality/reliability-stats:reliabilitystats"

validMcNemar: reliabilitystats.#McNemarResult & {p: 0.021484375, b: 1, c: 9}
validMcNemarConcordant: reliabilitystats.#McNemarResult & {p: 1, b: 0, c: 0}

// Invalid counterparts, uncomment individually to see cue vet reject them:
// invalidMcNemarZero: reliabilitystats.#McNemarResult & {p: 0, b: 0, c: 1100}
// invalidMcNemarP: reliabilitystats.#McNemarResult & {p: 1.1, b: 1, c: 9}
// invalidMcNemarB: reliabilitystats.#McNemarResult & {p: 1, b: -1, c: 0}
// invalidMcNemarC: reliabilitystats.#McNemarResult & {p: 1, b: 0, c: -1}
