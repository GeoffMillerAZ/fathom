// Package convergenceoracle decides whether an investigation has converged
// based on the number of findings collected, the level of agreement between
// them, and the number of unresolved contradictions.
//
// Decision rules (applied in order):
//
//  1. If findings < minFindings → NeedsMoreSources (not enough data yet).
//  2. If contradictions > maxContradictions → Contradicted (material
//     contradictions are unresolved; the investigation cannot be declared
//     converged until they are addressed or accepted within tolerance).
//  3. If agreement < minAgreement → Exploring (data present but no consensus).
//  4. Otherwise → Converged.
//
// # Agreement and evidence-grade bounding (Invariant 4)
//
// The oracle accepts agreement as an opaque proportion in [0, 1]. It does NOT
// enforce evidence-grade bounding internally: this is a caller contract.
// Callers MUST compute agreement only from findings whose evidence grade meets
// or exceeds the minimum required grade for the investigation. Passing an
// agreement value derived from low-grade (e.g. AI-inferred) findings without
// filtering violates Throughline Invariant 4 — a claim's confidence grade must
// not exceed the weakest tier of its necessary evidence.
//
// CUE contracts live in schema.cue; this file is the Go counterpart and must
// stay in sync with that schema.
//
// Concurrency contract: all exported functions are pure (no shared mutable
// state) and safe to call concurrently from multiple goroutines.
package convergenceoracle

import (
	"errors"
	"fmt"
)

// ErrInvalidInput is returned by AssessWithThresholdsE when any input
// parameter is outside its valid range. Callers may use errors.Is to detect
// this sentinel.
var ErrInvalidInput = errors.New("convergenceoracle: invalid input")

// Verdict is the outcome of a convergence assessment.
type Verdict string

const (
	// Converged means the investigation has reached a stable conclusion.
	// Sufficient findings exist, agreement is high enough, and contradictions
	// are within the accepted tolerance.
	Converged Verdict = "converged"

	// Exploring means findings exist but they have not yet reached the
	// required agreement threshold. The investigation should continue.
	Exploring Verdict = "exploring"

	// NeedsMoreSources means the investigation has too few findings to draw
	// any conclusion. More evidence sources must be consulted.
	NeedsMoreSources Verdict = "needs_sources"

	// Contradicted means the investigation has material unresolved
	// contradictions that exceed the accepted tolerance. The investigation
	// cannot be declared converged until contradictions are resolved or
	// explicitly accepted. Converging on a contradiction is not convergence.
	Contradicted Verdict = "contradicted"
)

// DefaultMinFindings is the minimum number of findings required before the
// oracle will consider convergence. Exported so callers can reference the
// package default without magic numbers.
const DefaultMinFindings = 3

// DefaultMinAgreement is the minimum agreement ratio (in [0, 1]) required for
// the oracle to declare convergence.
const DefaultMinAgreement = 0.75

// DefaultMaxContradictions is the maximum number of unresolved contradictions
// permitted before the Contradicted verdict is returned. The default of 0
// means any contradiction blocks convergence — the strictest legal standard.
const DefaultMaxContradictions = 0

// defaultMinFindings and defaultMinAgreement are unexported aliases that keep
// the existing Assess/AssessWithThresholds call sites compiling unchanged.
const (
	defaultMinFindings  = DefaultMinFindings
	defaultMinAgreement = DefaultMinAgreement
)

// Assess returns the convergence Verdict for an investigation using the
// package defaults. It assumes zero contradictions (strictest standard: any
// contradiction blocks convergence).
//
//   - findings: total number of independent findings collected so far.
//   - agreement: proportion of findings agreeing with the leading hypothesis,
//     in the closed interval [0, 1].
//
// Identical inputs always produce the same Verdict. For contradiction-aware
// assessment use AssessWithContradictions or AssessWithThresholdsE.
func Assess(findings int, agreement float64) Verdict {
	return AssessWithThresholds(findings, agreement, defaultMinFindings, defaultMinAgreement)
}

// AssessWithContradictions is the same as Assess but also accepts the count of
// unresolved contradictions. The package-default MaxContradictions (0) applies:
// any contradiction returns Contradicted.
//
//   - findings: total findings collected.
//   - agreement: proportion agreeing with the leading hypothesis, in [0, 1].
//   - contradictions: number of unresolved contradictions detected.
func AssessWithContradictions(findings int, agreement float64, contradictions int) Verdict {
	v, _ := AssessWithThresholdsE(
		findings, agreement, contradictions,
		DefaultMinFindings, DefaultMinAgreement, DefaultMaxContradictions,
	)
	return v
}

// AssessWithThresholds is the same as Assess but allows callers to override
// the convergence thresholds. Useful for tests and templates that declare
// custom convergence criteria. Assumes zero contradictions.
//
//   - minFindings: minimum number of findings required.
//   - minAgreement: minimum agreement ratio required.
//
// This signature is preserved unchanged for backward compatibility.
func AssessWithThresholds(findings int, agreement float64, minFindings int, minAgreement float64) Verdict {
	v, _ := AssessWithThresholdsE(findings, agreement, 0, minFindings, minAgreement, DefaultMaxContradictions)
	return v
}

// AssessWithThresholdsE is the fully-parameterized, error-returning variant.
// It validates all inputs and returns a wrapped ErrInvalidInput on bad data.
//
// Parameters:
//   - findings: total findings collected. Must be >= 0.
//   - agreement: proportion agreeing with the leading hypothesis. Must be in [0, 1].
//   - contradictions: unresolved contradictions count. Must be >= 0.
//   - minFindings: minimum findings for any convergence verdict. Must be >= 1.
//   - minAgreement: minimum agreement ratio required. Must be in [0, 1].
//   - maxContradictions: maximum tolerated contradictions. Must be >= 0.
//
// Decision order:
//  1. NeedsMoreSources if findings < minFindings.
//  2. Contradicted if contradictions > maxContradictions.
//  3. Exploring if agreement < minAgreement.
//  4. Converged.
func AssessWithThresholdsE(
	findings int,
	agreement float64,
	contradictions int,
	minFindings int,
	minAgreement float64,
	maxContradictions int,
) (Verdict, error) {
	if err := validateInputs(findings, agreement, contradictions, minFindings, minAgreement, maxContradictions); err != nil {
		return "", err
	}
	return assess(findings, agreement, contradictions, minFindings, minAgreement, maxContradictions), nil
}

// validateInputs returns a wrapped ErrInvalidInput if any parameter is outside
// its valid range.
func validateInputs(
	findings int,
	agreement float64,
	contradictions int,
	minFindings int,
	minAgreement float64,
	maxContradictions int,
) error {
	if findings < 0 {
		return fmt.Errorf("%w: findings must be >= 0, got %d", ErrInvalidInput, findings)
	}
	if agreement < 0 || agreement > 1 {
		return fmt.Errorf("%w: agreement must be in [0, 1], got %g", ErrInvalidInput, agreement)
	}
	if contradictions < 0 {
		return fmt.Errorf("%w: contradictions must be >= 0, got %d", ErrInvalidInput, contradictions)
	}
	if minFindings < 1 {
		return fmt.Errorf("%w: minFindings must be >= 1, got %d", ErrInvalidInput, minFindings)
	}
	if minAgreement < 0 || minAgreement > 1 {
		return fmt.Errorf("%w: minAgreement must be in [0, 1], got %g", ErrInvalidInput, minAgreement)
	}
	if maxContradictions < 0 {
		return fmt.Errorf("%w: maxContradictions must be >= 0, got %d", ErrInvalidInput, maxContradictions)
	}
	return nil
}

// assess is the pure, post-validation decision function. All inputs must
// already be in range; no validation is performed here.
func assess(
	findings int,
	agreement float64,
	contradictions int,
	minFindings int,
	minAgreement float64,
	maxContradictions int,
) Verdict {
	if findings < minFindings {
		return NeedsMoreSources
	}
	if contradictions > maxContradictions {
		return Contradicted
	}
	if agreement < minAgreement {
		return Exploring
	}
	return Converged
}
