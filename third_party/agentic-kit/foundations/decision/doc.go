// Package decision is a typed fast-decision port: callers ask typed
// questions about a blob of state and get probabilities back, never text.
//
// # Why "fast decision"
//
// A fast-decision call answers a narrow, structured question (a choice
// among named options, a score on a range, a yes/no probability) rather
// than carrying on a conversation. The Decider interface is deliberately
// thin — Name and one Decide call taking a batch of Questions against one
// State — so a caller can swap providers without touching call sites.
//
// # TypeSafe AI mapping
//
// The three Kind values mirror TypeSafe AI's System One primitives on
// purpose, so a future "typesafe" adapter (calling TypeSafe's System One API
// directly, parked — see docs/plans/decision-port.md) maps onto this
// package's types 1:1 instead of needing a shape translation layer:
//
//	decision.Kind    TypeSafe System One primitive   Shape
//	KindChoice       Choice                          N named options -> a
//	                                                  probability distribution
//	KindScore        Score                           a continuous value on
//	                                                  [Min, Max]
//	KindNoul         Noul                             a probability that a
//	                                                  stated criterion is true
//
// This brief ships only the None and LLM adapters. A typesafe adapter is
// deliberately NOT built here — see the parked ask in
// docs/plans/decision-port.md (Promote when: an API key exists and a
// consumer's A/B shows a gain).
//
// # Confidence formula
//
// Confidence implements the vendor-documented formula for turning a
// probability distribution into a single confidence number: with k options
// and the winning (maximum) probability max, confidence is 1 when k<=1
// (a degenerate single-outcome "distribution" is certain by construction),
// otherwise (k*max-1)/(k-1), clamped to [0,1]. For k=3 this is exactly
// TypeSafe's documented three-option formula (3*max-1)/2. A Score answer has
// no discrete alternative set to form a distribution over, so its Confidence
// is computed by calling Confidence with a degenerate (k<=1) input — see
// Answer's doc comment. A Noul answer instead uses the simpler two-outcome
// rule documented on Answer.Confidence, because it is not a probability
// distribution over named alternatives.
//
// BandOf classifies a Confidence value into a Band a caller can switch on
// without re-deriving thresholds: below UnsureBelow is BandUnsure, at or
// above ProceedAtOrAbove is BandProceed, otherwise BandConfirm.
//
// # Concurrency
//
// None is stateless and safe for concurrent use. LLM holds no mutable state
// after construction (its clock and token-budget options are fixed at
// NewLLM time) and is safe for concurrent use precisely when the wrapped
// llmclient.LLMClient is — the same contract every llmclient adapter
// documents for itself.
package decision
