// Package typesafe is the TypeSafe adapter for the foundations/decision
// port: a decision.Decider that calls TypeSafe AI's System One API directly
// (POST {BaseURL}/v1/systemone) instead of asking a generic completion to
// emulate it. A caller switches between it and the llm adapter by
// construction alone: both produce the same Response, Answer and Confidence
// shapes.
//
// # Wire
//
// Decide sends every Question in a Request as ONE request body keyed by
// Question.ID; the reply is keyed the same way. The kinds map onto the
// vendor's primitives (choice, score and "noul" for decision.KindPredicate),
// and the vendor's names live in this package only. The request is pinned
// byte for byte by testdata/typesafe_request.golden. The wire's own
// "confidence" field is never read: Confidence is always decision.Confidence's
// formula. TypeSafe's doc carries the kind mapping and the score-level
// synthesis.
//
// # Limits and refusal
//
// TypeSafe declares its Capabilities (every kind, up to 255 options, 2 to 10
// score levels, one POST per Request, sub-second latency, low cost, native
// probabilities that are not documented as calibrated) and refuses a
// question outside them through decision.OutsideDeclaredLimits: the question
// lands in Response.Invalid by ID and never reaches the wire.
//
// # Credentials
//
// NewTypeSafe returns an error when TypeSafeConfig.APIKey is empty, so a
// TypeSafe without a key cannot be constructed; a caller with no key builds
// another Decider (llm.LLM, or decision.None) on purpose, and TypeSafe never
// falls back on its own. A non-2xx reply is a *TypeSafeError, whose
// Retryable is true for 429 and 529; the key is never assembled into it.
//
// Composition: imports only the standard library (net/http for the wire) and
// the parent foundations/decision package. No SDK and no vendor client. Its
// tests use httptest only; nothing here calls a live endpoint. The core port
// imports none of this package, so a consumer that never constructs a
// TypeSafe links no HTTP transport through the port.
//
// Concurrency: a TypeSafe holds no mutable state after construction (API
// key, base URL, model, HTTP client and clock are fixed at NewTypeSafe time),
// so it is safe for concurrent use.
//
// Plan row DP-14 moved this adapter here from foundations/decision; its
// exported names (NewTypeSafe, TypeSafe, TypeSafeConfig, TypeSafeError,
// DefaultTypeSafeBaseURL, DefaultTypeSafeModel) and its behavior are
// unchanged.
package typesafe
