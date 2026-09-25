package reviewkit

// Review Kit's half of vision 34 / ADR-044 (docs/design/refactor-conservation.md
// §7): fathom computes, per plane, what a change kept and what it lost, and
// whether a declaration excuses a loss — never an adequacy verdict of its own
// (I43/I44). fathom asserts a fact; this pack is what makes an undeclared loss
// or a tampered proof block. "Complete functional-test coverage" is not the
// target: every capability in the touched surface keeps a live, unedited,
// commit-bound certificate, or its loss is declared outside the range.
pack: #Pack & {
	title: "Refactoring conservation"
	requirements: {
		"refactoring.no-undeclared-loss": {
			why:     "A refactor that removes or changes a capability, a certified behaviour, or a governance binding without declaring the loss (surface_change, retires, a disposition, or a prior plan version) is an unreviewed regression fathom already has the facts to name — this requirement is what makes an undeclared loss block."
			remedy:  "declare the loss (`surface_change:`/`retires:` on the plan row, or a disposition) BEFORE the range that drops it, or restore the capability; a declaration added inside the same change range excuses nothing (ADR-044 decision 4)."
			subject: "artifact"
			check: {
				kind:    "evidence"
				adapter: "fathom-conserve"
				binding: {rule: "no-undeclared-loss"}
				min_tier: "verified"
			}
			enforcement: {severity: "off"}
		}
		"refactoring.proof-integrity": {
			why:     "A certifying, enforcing or surface-pinning test that was deleted, weakened, skipped, build-tagged out, or had its golden data or certifies marker changed inside the range is the proof itself being tampered with — the field's own name for this is Blindness, and no declaration should be able to excuse it (ADR-044 decision 3 gates ahead of self-declaration)."
			remedy:  "restore the test's assertions, marker or golden data, or replace it with an equal-or-stronger test that certifies the same target; a rename alone is not a loss, but weakening what a test actually checks is."
			subject: "artifact"
			check: {
				kind:    "evidence"
				adapter: "fathom-conserve"
				binding: {rule: "proof-integrity"}
				min_tier: "verified"
			}
			enforcement: {severity: "off"}
		}
	}
	findings: {
		"refactoring.undeclared-loss": {
			class:          "contract-violation"
			why:            "fathom's conservation report names a plane finding whose `declared` is not `prior` — a capability, certified behaviour or governance binding this organization requires evidence for is gone or changed, and nothing on record excused it before the change range began."
			remedy:         "declare the loss outside the range (surface_change/retires/disposition on a prior plan version) or restore the capability; ingest a fresh conservation report once fixed."
			requiresExpiry: true
			fixLane:        "police"
			enforcement: {severity: "off"}
		}
		"refactoring.proof-tampered": {
			class:          "test-gap"
			why:            "fathom's integrity plane found a guarded test deleted, weakened, skipped, build-tagged out, or with its golden data or certifies marker moved inside the change range — the proof that a capability was kept is itself compromised, independent of whether the capability loss was declared."
			remedy:         "restore the test's original assertions, skip guard, golden data or certifies marker, or replace it with an equal-or-stronger test; a legitimate rename is not itself a finding."
			requiresExpiry: true
			fixLane:        "police"
			enforcement: {severity: "off"}
		}
		"refactoring.uncovered-behaviour": {
			class:          "test-gap"
			why:            "Beyond what a compiler or a certificate can prove, the typed-judgment lane (ADR-0104) asks whether the residue of a refactor — behaviour no test reaches — was preserved; the answer is satisfied, violated or unknown, advisory only, and this class alone never fails a gate (Review Kit's severity ladder is not wired to it)."
			remedy:         "add a test or a certifies declaration that reaches the residual behaviour, or accept the finding with a reason if the residue is genuinely out of scope."
			requiresExpiry: true
			fixLane:        "unrepresentable"
			enforcement: {severity: "off"}
		}
	}
}
