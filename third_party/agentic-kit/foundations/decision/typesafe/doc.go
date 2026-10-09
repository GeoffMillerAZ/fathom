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
// lands in Response.Invalid by ID and never reaches the wire. It takes no
// images: a Request carrying Images has every question in Response.Invalid
// (decision.UnsupportedImages) and makes no call. Its wire has no refusal,
// so Response.Declined stays nil. A score answer carries the wire's level
// distribution in Answer.Probabilities, keyed by level position.
//
// # Credentials
//
// The key comes from one source, tried in order: TypeSafeConfig.APIKey (set
// in code), APIKeyFile (a path, "~" expanded, read at the first Decide,
// whitespace-trimmed, refused with the fix "chmod 600 <path>" when group or
// other users can access it) or APIKeyEnv (a variable name). A file wins
// over a variable: a daemon run by a service manager does not inherit the
// login shell's environment, so a file reference is the portable way to
// hand it a key, and the key never appears in configuration. NewTypeSafe
// returns an error when no source is set, so a TypeSafe without a key
// cannot be constructed; a caller with no key builds another Decider
// (llm.LLM, or decision.None) on purpose, and TypeSafe never falls back on
// its own. A missing, empty or too-open key is a Decide error naming the
// source, never the key. A non-2xx reply is a *TypeSafeError, whose
// Retryable is true for 429 and 529; its Message is the reply body with the
// key and any echoed Authorization header or bearer token redacted.
//
// # Transports
//
// The same wire reaches Jev two ways (TypeSafeConfig.Transport):
// TransportTypeSafe, TypeSafe's own POST {BaseURL}/v1/systemone (the
// default), and TransportOpenRouter, OpenRouter's Decisions router at POST
// https://openrouter.ai/api/alpha/decisions with an OpenRouter key and the
// model "typesafe/jev-1.13" by default (OpenRouter's API reference,
// verified 2026-10-09; the router is an alpha endpoint, and no live call has
// been made from the kit). OpenRouter requires instructions on every
// question, so that transport sends an empty string where the default one
// omits the field. BaseURL, Path and Model override either transport's
// defaults. Name is "typesafe" or "typesafe-openrouter", so a Response says
// which route answered.
//
// Composition: imports only the standard library (net/http for the wire) and
// the parent foundations/decision package. No SDK and no vendor client. Its
// tests use httptest only; nothing here calls a live endpoint. The core port
// imports none of this package, so a consumer that never constructs a
// TypeSafe links no HTTP transport through the port.
//
// Concurrency: a TypeSafe is safe for concurrent use: its base URL, path,
// model, HTTP client and clock are fixed at NewTypeSafe time, and the key
// read at first use is guarded by a sync.Mutex.
//
// Plan row DP-14 moved this adapter here from foundations/decision; its
// exported names (NewTypeSafe, TypeSafe, TypeSafeConfig, TypeSafeError,
// DefaultTypeSafeBaseURL, DefaultTypeSafeModel) and its behavior are
// unchanged.
package typesafe
