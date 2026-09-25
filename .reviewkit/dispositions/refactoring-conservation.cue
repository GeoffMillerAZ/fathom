// Dispositions for the refactoring-conservation pack and the
// testing-certifications requirement (vision 34 / ADR-044 decision 7;
// ADR-043's 2026-09-24 amendment). Tailoring only ever raises severity —
// the floor law (ADR-0096 D4) — and each tailor's own kill clause (below)
// is satisfied by firing at warn, the level it is armed at, never deny.
package reviewkit

dispositions: #Dispositions & {
	"tailor-refactoring-no-undeclared-loss-severity": #Tailor & {
		ref:       "refactoring.no-undeclared-loss"
		rationale: "ADR-044 decision 7: the refactoring-conservation pack is armed at warn on this repository from day one — an undeclared loss on any conservation plane should be visible at every sounding, not merely available on request."
		criteria: severity: "warn"
		author: "claude-session-cv3-arming"
		date:   "2026-09-25"
	}
	"tailor-refactoring-proof-integrity-severity": #Tailor & {
		ref:       "refactoring.proof-integrity"
		rationale: "ADR-044 decision 3: proof tampering (a certifying test deleted, weakened, skipped, build-tagged out, or its golden data or certifies marker moved) is the field's own 'Blindness' failure mode and should warn on every sounding, independent of the no-undeclared-loss rule."
		criteria: severity: "warn"
		author: "claude-session-cv3-arming"
		date:   "2026-09-25"
	}
	"tailor-testing-certified-targets-severity": #Tailor & {
		ref:       "testing.certified-targets"
		rationale: "ADR-043's 2026-09-24 amendment: the testing-certifications requirement is armed at warn on this repository, with the kill rule corrected to count firing at warn (an off requirement can never fire at all)."
		criteria: severity: "warn"
		author: "claude-session-cv3-arming"
		date:   "2026-09-25"
	}
}
