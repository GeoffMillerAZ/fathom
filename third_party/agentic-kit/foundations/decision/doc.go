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
// # Adapters
//
// Three Deciders ship: None (the zero-configuration stub, always
// ErrNoProvider), LLM (one llmclient.LLMClient completion per Decide call,
// asked to emulate TypeSafe's Choice/Score/Noul contract via a rendered
// prompt and JSON schema), and TypeSafe (calling TypeSafe AI's System One
// API — POST {BaseURL}/v1/systemone — directly, native rather than
// emulated). A caller switches from LLM to TypeSafe by construction swap
// alone: both produce the identical Response/Answer/Confidence shape.
//
// A TypeSafe with no API key configured cannot be constructed at all —
// NewTypeSafe returns an error when TypeSafeConfig.APIKey is empty. A
// caller with no key configured should construct LLM or None instead;
// TypeSafe never silently falls back on your behalf.
//
// # TypeSafe AI mapping
//
// The three Kind values mirror TypeSafe AI's System One primitives on
// purpose, so the typesafe adapter maps onto this package's types 1:1
// instead of needing a shape translation layer:
//
//	decision.Kind    TypeSafe System One primitive   Shape
//	KindChoice       Choice                          N named options -> a
//	                                                  probability distribution
//	KindScore        Score                           an ordered set of 2..10
//	                                                  Levels (explicit, or
//	                                                  synthesized from an
//	                                                  integral [Min, Max])
//	KindNoul         Noul                             a probability that a
//	                                                  stated criterion is true
//
// TypeSafe's wire request batches every Question in a Decide call's
// Request into ONE POST body keyed by Question.ID (a Go map — deterministic
// on the wire because encoding/json always emits map[string]T keys sorted);
// the reply is the same shape keyed by Question.ID back. A KindScore
// question with neither Levels set nor an integral 2..10-level [Min, Max]
// range cannot be sent to TypeSafe's "score" primitive at all and is
// recorded in Response.Invalid instead, never reaching the wire.
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
// This holds for every adapter, TypeSafe included: TypeSafe's own wire
// response carries a "confidence" field (and, for score, a "probabilities"
// map keyed by level index), but this package NEVER trusts it — Confidence
// is always recomputed from the wire's probabilities/probability-map by
// this package's own formula. Jev's documented confidence statistic is the
// same formula, so the numbers agree; the invariant is about where the
// number comes from, not a disagreement in the math.
//
// BandOf classifies a Confidence value into a Band a caller can switch on
// without re-deriving thresholds: below UnsureBelow is BandUnsure, at or
// above ProceedAtOrAbove is BandProceed, otherwise BandConfirm.
//
// # Batching
//
// Every adapter answers a whole Request.Questions batch in ONE round trip:
// LLM as one llmclient completion, TypeSafe as one POST to
// /v1/systemone carrying every question keyed by ID. Batch your questions
// per state instead of calling Decide once per question.
//
// # Concurrency
//
// None is stateless and safe for concurrent use. LLM holds no mutable state
// after construction (its clock and token-budget options are fixed at
// NewLLM time) and is safe for concurrent use precisely when the wrapped
// llmclient.LLMClient is — the same contract every llmclient adapter
// documents for itself. TypeSafe likewise holds no mutable state after
// construction (API key, base URL, model, HTTP client and clock are fixed
// at NewTypeSafe time) and is safe for concurrent use.
package decision
