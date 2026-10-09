// Package openai is the OpenAI Decisions API adapter for the
// foundations/decision port: a decision.Decider that sends a Request to
// POST {BaseURL}/decisions (BaseURL defaults to https://api.openai.com/v1)
// and maps the typed answers back onto decision.Answer. It sits beside
// foundations/decision/typesafe and foundations/decision/llm: a caller
// switches between them by construction alone, each produces the same
// Response, Answer and Confidence shapes, and none is ever the default (a
// consumer with no provider configured builds decision.None).
//
// Facts below are from OpenAI's Decisions guide
// (developers.openai.com/api/docs/guides/decisions), read 2026-10-09: a
// public beta; gpt-6-luna is the only supported model; US$0.10 per million
// input tokens and no output charge. The guide makes no calibration claim
// (it says to set thresholds from your own labelled examples) and states no
// model pinning, seed, rate limit or question, choice or level limit, so
// this adapter declares none. No live call has been made from the kit:
// tests use httptest only.
//
// # Wire
//
// Decide sends every Question in a Request as ONE request, in request
// order; each question's name is its Question.ID, which the API echoes on
// its answer. The kinds map one to one: decision.KindPredicate is
// "predicate", KindChoice is "choice" (each Option as {value: Name,
// description: Criteria}) and KindScore is "score" (each level as {label},
// lowest first: Question.Levels verbatim, or the integers of an integral
// [Min, Max] range). The predicate's TrueCriteria and FalseCriteria and a
// score's Rubric have no field of their own on the wire, so they are
// appended to the instructions as "True when:", "False when:" and
// "Rubric:" lines. Input is State as a string, or, when the Request
// carries Images, one user message holding State as input_text and each
// image as an input_image base64 data URL (the only image form the API
// accepts). The request is pinned byte for byte by testdata/*.golden.
//
// # Answers
//
// A predicate's probability is Answer.Noul; a choice's value and its
// per-option probabilities are Answer.Choice and Answer.Probabilities; a
// score (the API's probability-weighted average of 0-based level indices,
// which may fall between levels) is Answer.Score, shifted by Min for a
// synthesized range, with its level distribution in Answer.Probabilities
// keyed by level position. Confidence is always derived from those
// probabilities by the same rule every adapter uses (decision.Confidence
// for a choice and a score, max(p, 1-p) for a predicate); the API's own
// "confidence" field is never read, because it is not comparable across
// providers. An answer of type "refusal" lands in Response.Declined by
// question ID: it is never read as false, zero or an option. A malformed
// answer, a value outside the options or levels, or a distribution that
// does not sum to within 0.01 of 1 lands in Response.Invalid; Decide never
// fails the batch for one bad answer.
//
// # Credentials
//
// The key comes from one source, tried in order: OpenAIConfig.APIKey (set
// in code), APIKeyFile (a path, "~" expanded, read at the first Decide,
// whitespace-trimmed, refused with the fix "chmod 600 <path>" when group or
// other users can access it) or APIKeyEnv (a variable name). A file wins
// over a variable, because a daemon run by a service manager does not
// inherit the login shell's environment; the key never appears in
// configuration. NewOpenAI errors when no source is set; a missing, empty
// or too-open key is a Decide error naming the source, never the key. A
// non-2xx reply is an *OpenAIError whose Message is the reply body with the
// key and any echoed Authorization header, bearer token or "sk-" key shape
// redacted.
//
// Composition: imports only the standard library (net/http for the wire),
// the parent foundations/decision package and its internal key helper. No
// SDK and no new module requirement. The core port imports none of this
// package.
//
// Concurrency: an OpenAI is safe for concurrent use: its base URL, model,
// HTTP client and clock are fixed at NewOpenAI time, and the key read at
// first use is guarded by a sync.Mutex.
package openai
