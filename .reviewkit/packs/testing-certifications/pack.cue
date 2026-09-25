package reviewkit

// Optional: the judge's half of vision 33 / ADR-0105 (test-kit declares
// what a suite certifies; fathom joins the declaration against its own
// index and computes whether it is live; this pack decides whether live
// is enough). fathom asserts a fact — a target has, or has not, a live
// certificate at a given strength — and never an "adequate" verdict; that
// verdict is this pack's requirement and this org's own severity.
pack: #Pack & {
	title: "Test certifications"
	requirements: {
		"testing.certified-targets": {
			why:     "A deny-severity invariant with no live certificate is a claim the organization requires evidence for and has none of — fathom lists the gap, this requirement is what makes the gap block."
			remedy:  "declare `certifies` in tests/<component>.cue or a `// certifies:` marker above the test; ingest the attested run"
			subject: "artifact"
			check: {
				kind:    "evidence"
				adapter: "fathom-certify"
				binding: {kind: "invariant", min_strength: "asserts"}
				min_tier: "verified"
			}
			enforcement: {severity: "off"}
		}
	}
	findings: {
		"testing.certificate-gap": {
			class:          "test-gap"
			why:            "A target this organization requires certified evidence for has no live certificate at or above its bound strength — fathom computed the fact, this pack is the one judging it insufficient."
			remedy:         "declare `certifies` in tests/<component>.cue or a `// certifies:` marker above the test; ingest the attested run"
			requiresExpiry: true
			fixLane:        "police"
			enforcement: {severity: "off"}
		}
	}
}
