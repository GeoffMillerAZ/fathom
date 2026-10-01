// Package llm is the LLM adapter for the foundations/decision port: a
// decision.Decider backed by one llmclient.LLMClient completion per Decide
// call, asked to answer the port's typed contract through a rendered prompt
// and a JSON schema. Any model an llmclient adapter reaches can answer; the
// port's Response, Answer and Confidence shapes are the same as every other
// adapter's.
//
// # Request shape
//
// Decide renders every Question in a Request into ONE llmclient.Request: a
// fixed System contract, a deterministic JSON Prompt carrying State and the
// Questions in their given order (Question.Levels rendered when set), and an
// OutputSchema built per question kind. The contract keeps the predicate
// kind's original wire label "noul", so the prompt and the schema are pinned
// byte for byte by testdata/prompt.golden and testdata/schema.golden: a
// change to either is a prompt change that needs a fresh calibration
// reading.
//
// # Answers
//
// Per question the reply is validated and classified: a valid answer lands
// in Response.Answers; a choice outside the Options, a missing answer or a
// malformed per-question shape lands in Response.Invalid by question ID, and
// the rest of the batch is still answered. Probabilities are filled missing
// as zero and renormalised; a score is clamped to its range (its Levels
// positions when Levels is set) and a predicate to [0, 1]. Confidence is
// always decision.Confidence's formula, never a number the model wrote. A
// reply that is not JSON at all is the one hard error.
//
// LLM declares its Capabilities: every kind, up to 255 options, no
// score-level limit of its own, one completion per Request, and
// self-reported probabilities that need their own calibration; its latency
// and cost are unknown because the wrapped client may be any model.
//
// Composition: imports the standard library, the parent foundations/decision
// package and foundations/llmclient. No SDK. The core port imports none of
// this package, so a consumer that never constructs an LLM links no
// completion client.
//
// Concurrency: an LLM holds no mutable state after construction (client,
// model, MaxTokens and the clock are fixed at NewLLM time), so it is safe for
// concurrent use exactly when the wrapped llmclient.LLMClient is, the same
// contract every llmclient adapter documents for itself.
//
// Plan row DP-14 moved this adapter here from foundations/decision; its
// exported names (NewLLM, LLM, LLMOption, WithMaxTokens, WithNow,
// DefaultMaxTokens) and its behavior are unchanged.
package llm
