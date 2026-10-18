# 0018: One decision port; provider adapters are hexagonal, optional and replaceable, not interchangeable

**Status:** accepted
**Type:** architecture
**Date:** 2026-10-09

## Context

The owner directed on 2026-10-09 that the kit support OpenAI's Decisions API hexagonally
alongside Jev, with decision models staying optional: provider none is the default and a
consumer's unconfigured output stays byte-identical (the reference consumer's invariant
I42). OpenAI's API (public beta, read 2026-10-09) asks the same three question shapes the
port already has (predicate, choice, score), takes images, can answer a question with a
refusal, reports its own `confidence` number and makes no calibration claim (it says to set
thresholds from your own labelled examples). Jev is now also reachable through OpenRouter's
Decisions router (POST `https://openrouter.ai/api/alpha/decisions`, the System One wire,
verified against OpenRouter's API reference the same day). Remote adapters had one key
source, an inline string, and a daemon run by a service manager does not inherit the login
shell's environment. Decision [0017](0017-fast-judgment-is-optional-and-calibrated.md) set
the rules this one builds on: judgment is an optional, calibrated tier, and calibration is
per site, question version, provider and resolved model.

## Decision

1. **One port.** `foundations/decision` stays the only judgment contract. A provider is an
   adapter in its own package (`llm`, `typesafe`, `openai`) that imports the core, never the
   reverse, and none is ever the default.
2. **Replaceable at the contract, not interchangeable.** Swapping adapters needs no call-site
   change, but the same question gets different numbers from different providers and routes,
   so a provider (and its `Name`, which names the route: `typesafe`, `typesafe-openrouter`)
   gets its own calibration reading before an answer changes behavior. No adapter claims
   calibration it does not document; OpenAI's and Jev's probabilities are declared
   native-probability, not calibrated-probability.
3. **Refusals are first-class.** A refused question goes to `Response.Declined`, never to
   `Answers` (never read as false, zero or an option) and never to `Invalid`. Cascade
   escalates it; Fallback records it as a disclosed miss (`ErrDeclined`) and tries the next
   Decider; Cached never stores it.
4. **Confidence is derived uniformly.** Every adapter computes `Answer.Confidence` from the
   probabilities by the core's rule: `Confidence` over a choice's options, over a score's
   level distribution when the wire carries one, and max(p, 1-p) for a predicate. A
   provider's own confidence number is never passed through.
5. **Keys come from a file reference or an environment variable name, never inline in
   configuration.** A key file is read at first use, trimmed, and refused when group or
   other users can access it, with the fix (`chmod 600 <path>`) in the error; a file wins
   over a variable because a service-managed daemon does not inherit the shell's
   environment. A value set in code stays possible for a caller with its own secret
   store. Errors name the source, never the key, and an echoed key or Authorization header
   is redacted from provider error text.
6. **Images are a declared capability.** A Request may carry images; a Decider that does not
   declare `Capabilities.Images` answers none of its questions rather than answer without
   the image.

7. **Hexagonal, with named adapters.** Core code speaks only the port's kinds (predicate,
   choice, score); no provider type, field or error crosses the port. Providers are
   adapters: TypeSafe Jev, OpenAI's Decisions API (public beta, `POST /v1/decisions`, kinds
   predicate, choice, score; read 2026-10-16 at
   https://developers.openai.com/api/docs/guides/decisions), an LLM answering JSON, a
   local model, or none. An adapter passes conformance (golden wire fixtures, the three
   kinds, refusal and confidence handling) before any consumer uses it. Names map across
   vendors: a TypeSafe `noul` is the port's predicate.
8. **Consent and kill.** A repository-scoped configuration never enables a provider or
   spends an ambient key: enabling needs user-scope consent. A user-scope kill switch
   beats every other layer. When off (unconfigured, unconsented or killed) the code does
   no environment lookup, no key read, no exec and no dial.

## Alternatives rejected

- **A second port for image or refusal-capable providers.** Splits every decorator and
  every consumer. Wins if: a provider's question shapes stop fitting the three kinds.
- **Passing the provider's `confidence` through, or keeping it beside ours.** Not comparable
  across providers, so a threshold tuned on one would silently move on another. Wins if: a
  provider documents its number as calibrated on data a consumer can re-read.
- **Reading a refusal as an Invalid answer.** Conflates "the reply was malformed" with "the
  provider would not answer", which a consumer handles differently. Wins if: never.
- **Keys inline in configuration, or environment variables only.** Puts secrets in files
  that are copied, logged and committed, or depends on an environment the daemon does not
  have. Wins if: never for configuration; code may still pass a resolved value.
- **An SDK per provider.** Breaks stdlib-only (I1) for a single POST. Wins if: a provider's
  wire needs streaming or signing stdlib cannot reasonably do.

## Consequences

`Response.Declined`, `Request.Images`, `Image`, `Capabilities.Images`,
`Capabilities.ModelPinning`, `ErrDeclined` and `UnsupportedImages` are additive; existing
Deciders, Responses and transcripts serialize as before (the new fields are omitted when
empty, and a text-only `RequestKey` is pinned unchanged). A typesafe score answer now
carries its level distribution in `Answer.Probabilities`. No live call to OpenAI or
OpenRouter has been made from the kit: both wires are pinned by golden request fixtures
written from the providers' published references, and whether OpenRouter accepts the bare
slug `jev-1.13` is unverified, so the default is the documented `typesafe/jev-1.13`.

## Revisit when

- A live reading on a consumer's data shows two providers' numbers agree closely enough
  that a shared threshold is safe for a site (then say so for that site, never globally).
- OpenAI documents calibration, model pinning or limits, or the Decisions API leaves beta
  with a changed wire.
- OpenRouter's Decisions router leaves alpha or changes its path or model slugs.
- A provider's refusal carries a reason worth surfacing beyond "refused".
- A consumer needs images through a decorator that cannot forward them (Cached keys by
  State alone and so never caches an image Request).
