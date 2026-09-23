// Package testevidence loads and ingests test-kit evidence (vision 33 /
// ADR-043 stage 0): declared manifests (tests/*.cue, validated against
// the mirrored schema/testkit schema), attested #TestResult result
// envelopes (plain or DSSE-wrapped), the case-to-symbol join, and the
// additive store tables both land in. fathom never runs a test — it
// ingests test-kit's own record (design doc §2's "fathom asserts a fact,
// never adequacy").
package testevidence

// Suite is one declared suite from a tests/<component>.cue manifest
// (test-kit's #Suite, the fields fathom's stage-0 facts need — TC-2).
type Suite struct {
	Name string
	// Class mirrors #SuiteClass: unit | integration | e2e | conformance |
	// bench | fixture-gate.
	Class string
	// Tier mirrors #Tier: blocking | advisory | quarantined.
	Tier string
	// Port is "" unless Class is "conformance" (the schema requires it
	// there — ADR-0003's "every port has a shared conformance suite").
	Port string
	// Certifies is always empty this round: test-kit's #Suite.certifies
	// field is a stage-1 ADR-0042 amendment (plan row TC-17) that has not
	// landed upstream yet.
	Certifies []CertifyRef
}

// CertifyRef is one #Certifies entry declared on a suite (design doc §1).
// Unused until test-kit lands #Suite.certifies (TC-17); defined now so
// stage 1 needs no shape change to start populating it.
type CertifyRef struct {
	Kind     string
	ID       string
	Strength string
	Tests    []string
}

// Manifest is one loaded and schema-validated tests/<component>.cue file.
type Manifest struct {
	// File is the repo-relative path the manifest was loaded from.
	File      string
	Component string
	// Suites is sorted by Name.
	Suites []Suite
}

// CaseFailure mirrors #CaseFailure.
type CaseFailure struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Where   string `json:"where,omitempty"`
}

// Case mirrors #Case — one test result within an envelope.
type Case struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Status   string       `json:"status"` // pass | fail | skip | flake
	Duration string       `json:"duration"`
	Failure  *CaseFailure `json:"failure,omitempty"`
	Attempts int          `json:"attempts"`
}

// Totals mirrors #Totals.
type Totals struct {
	Run     int `json:"run"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
	Flaked  int `json:"flaked"`
}

// CoverageReport mirrors #CoverageReport.
type CoverageReport struct {
	Statements *float64           `json:"statements,omitempty"`
	Branches   *float64           `json:"branches,omitempty"`
	PerPackage map[string]float64 `json:"perPackage,omitempty"`
}

// ArtifactRef mirrors #ArtifactRef — a CAS reference, never a path.
type ArtifactRef struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

// SuiteRef mirrors #TestResult.suite.
type SuiteRef struct {
	Component string `json:"component"`
	Name      string `json:"name"`
	Class     string `json:"class"`
}

// Run is one ingested #TestResult envelope, plus fathom-side provenance.
type Run struct {
	Suite     SuiteRef        `json:"suite"`
	Key       string          `json:"key"`
	Verdict   string          `json:"verdict"`
	Cases     []Case          `json:"cases"`
	Totals    Totals          `json:"totals"`
	Coverage  *CoverageReport `json:"coverage,omitempty"`
	Artifacts []ArtifactRef   `json:"artifacts,omitempty"`
	StartedAt string          `json:"-"` // from timing.startedAt, when present

	// Digest identifies this run: for a DSSE-wrapped envelope, sha256 of
	// the decoded payload bytes (design doc §2 — "the DSSE envelope's
	// digest is recorded as the run's identity"); for a bare envelope,
	// sha256 of the raw JSON bytes as read. Always "sha256:<hex>".
	Digest string `json:"-"`
	// SignerTier is the DSSE envelope's own tier field ("unsigned" |
	// "local" | "cosign-keyless") when the run arrived wrapped, "" for a
	// bare envelope. fathom records this as a fact and never attempts
	// signature verification itself (design doc §2: "signature
	// verification is test-kit's job").
	SignerTier string `json:"-"`
	// SourceFile is the file this run was read from.
	SourceFile string `json:"-"`
}

// timingView is the subset of #TestResult.timing LoadEnvelopes reads —
// kept private since Run only surfaces StartedAt.
type timingView struct {
	StartedAt string `json:"startedAt"`
}
