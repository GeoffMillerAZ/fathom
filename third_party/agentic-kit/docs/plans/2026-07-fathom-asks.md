# Fathom asks — kit seams the fleet's instrument needs (2026-07-16)

**Mission** — Fathom (the fleet's code-intelligence/evidence engine, this
repo's first downstream) filed a wishlist grounded in observed failures.
Per D8 discipline, only items with a **named, arriving consumer** are built;
the rest park with subscribe-when triggers. Done means: the three W1 seams
shipped test-first with the kit gate green, TRACKING/CHANGELOG in the same
change, and fathom able to adopt each within its next program slice.
This plan is gated by `fathom verify` at pre-push — claims carry anchors.

**Write scope** — this repo only. The kit never imports fathom; every seam
below is a primitive fathom composes downstream.

## W1 — build now (named consumer: fathom, next slice)

- [x] `Template.TextHash()` — lowercase hex sha256 of `Template.Text`: content-addressed template identity for consumers' digest gates. Fathom's PromptVersion digest-gate bug (a prompt bump that silently kept 27/32 stale sections) is the motivating failure; H4 hand-rolls this pin today. Additive; `Rendered` unchanged. {id: FA-1, symbol: TextHash, package: foundations/prompt/prompt-template}
- [x] `foundations/llmclient/structured` — `Complete(ctx, c, req, schema, out)`: sets `Request.OutputSchema`, calls the client, and makes the two silent failure modes TYPED: `ErrTruncated` when `StopReason == StopMaxTokens` (fathom's first fleet compile aborted mid-JSON on exactly this), `ErrMalformed` when the response does not strictly decode into `out`. Callers keep their own skip/abort policy — the helper only classifies. Behavioral-only package (no schema.cue per package-shape rule). {id: FA-2, symbol: Complete, package: foundations/llmclient/structured}
- [x] Typed errors distinguish truncation from malformed output and are `errors.Is`-matchable. {id: FA-3, symbol: TestComplete_TruncationIsTyped, package: foundations/llmclient/structured, skip_test: true, skip: "the test IS the anchor (ErrTruncated is a var, not call-reachable); nothing reaches a test"}
- [x] `foundations/telemetry/heartbeat` — byte-stable JSONL beats (`Beat` records (Channel/Kind/At/Meta), injected clock, `Append`/`Load` accept-partial) plus two pure projections: `Rollup` (per-channel/kind counts by day) and `CheckLiveness` (paired expectations: "if channel A beat in the window, channel B must have too" → `[]Alarm`). Motivating failure: fathom's flagship injection channel was dark for four days and nothing noticed — the log recorded effects, not activity, so coverage had no denominator. Second subscriber (subscribe-when, not built here): this repo's own proof-gate liveness (audit headline #1 was a silently-RED gate). {id: FA-4, symbol: CheckLiveness, package: foundations/telemetry/heartbeat}
- [x] Heartbeat records round-trip byte-stably and projections are deterministic (fixed clock, sorted output). {id: FA-5, symbol: Rollup, package: foundations/telemetry/heartbeat}
- [x] Tag `v0.7.0` — version identity for downstream go.mod pinning; unblocks fathom dropping go.work coupling (`fathom init` on cold clones); v0.1.0 was burned pre-uplift, versions ran to v0.6.0. NOT the D4 public/1.0 decision — that stays owner-ratified. {id: FA-6, symbol: TextHash, skip_test: true, skip: "ops action (git tag); anchored to a W1 symbol shipping in the tagged commit"}

## Parked — subscribe-when triggers (D8: no consumer, no code)

- [-] `verdict-journal` (generalize eval-journal's byte-stable signed-verdict discipline) {id: FA-7, disposition: deferred, reason: "consumer is fathom's claim_events wiring (findings-as-claims slice); not landed yet", trigger: "a consumer needs claim-event interchange outside fathom's store (store-format spec v0 export) or a second repo hand-rolls byte-stable signed-verdict JSONL"}

  *(Trigger revised 2026-07-16, vision-fit audit — the prior trigger, "fathom claim refute|corroborate lands and needs an event format," was rewritten because its first clause has not fired (no `cmdClaim` in fathom's `cmd/fathom`; findings-tracer FT-3 is still `[ ]` as of 2026-07-16 — the slice is committed on an unmerged branch, which satisfies the old trigger's first clause textually but not in substance) and its second clause was never satisfiable as written: fathom has carried its own event format since ADR-016 — `core.ClaimEvent{ClaimID, Verdict, Author, Note, At}`, persisted via `Store.PutClaims` — and fathom's vision/design for FT-3/FT-4 wires exactly that store-native struct, asking for no kit JSONL journal. Re-verify this row when FT-3/FT-4 land.)*
- [-] `tier1contract` conformance-test package for `evidenceladder.Tier1` adapters {id: FA-8, disposition: deferred, reason: "consumer is H4's IndexTier1; H4's trigger (sextant branch merge) has not fired", trigger: "H4 Stage 5 begins"}
- [-] eval verdict reader lib (stable #EvalRecord read API for external gates) {id: FA-9, disposition: deferred, reason: "fathom's eval oracle class is itself deferred behind a first behavioral claim", trigger: "first gated plan carrying a behavioral: claim"}
- [-] shared tier/verdict vocabulary as data {id: FA-10, disposition: deferred, reason: "don't pre-unify; duplication must demonstrably hurt first", trigger: "store-format spec v0 lands and both repos duplicate the vocabulary"}

## Design constraints

- **Additive only; kit imports fathom never.** `Rendered`/`aiprovenance` field sets unchanged (FA-1 is a method, not a schema change).
- **structured**: `OutputSchema` already exists on `Request` (Wave 1) — the package composes it, never re-implements provider mapping. Strict decode = `json.Decoder` with `DisallowUnknownFields`. A `StopToolUse` response is out of scope (text-JSON contract only). Respect the open `StopReason` set — never switch exhaustively.
- **heartbeat**: injected `now func() time.Time` everywhere; no wall-clock in projections (windows are computed against a caller-supplied reference time); JSONL append-only, accept-partial on load (mirror extraction-gap-log). Beats are SIGNALS — `CheckLiveness` alarms are advisory data, never process exits.
- Package shape per foundations-overview: heartbeat ships `schema.cue` + valid/invalid testdata; structured is behavioral-only (skip cue); TRACKING.md rows + CHANGELOG in the same change-set (constitution-lint coverage check).

## Acceptance = proof

1. `task validate` green (incl. constitution-lint, catalog regen if it flags).
2. This plan verifies via the pre-push gate with every W1 box `[x]` → anchors resolving (the gate is non-vacuous as of fathom b7f6476).
3. FA-2/FA-3: table-driven tests cover truncation (mock returns StopMaxTokens), malformed (garbage text), unknown-field strictness, and the happy path — no network, `NewMockClient` only.
4. FA-4/FA-5: a fixture where channel A beats and channel B is silent alarms; both-beat and neither-beat do not; byte-stable round-trip asserted.
5. v0.7.0 tag exists and points at the W1 commit.
