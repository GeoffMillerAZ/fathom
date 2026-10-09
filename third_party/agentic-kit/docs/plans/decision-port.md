# decision — typed fast-decision port

**Mission** — a typed fast-decision port: callers ask typed `Question`s
(`KindChoice`/`KindScore`/`KindPredicate`) about a blob of state and get
probabilities back, never text. The kinds began as a mirror of TypeSafe AI's
System One primitives; DP-12 made them generic, and each adapter maps them
onto its own wire. This plan lands the port with three adapters: `None` in
`foundations/decision`, `LLM` in `foundations/decision/llm` and `TypeSafe`
in `foundations/decision/typesafe` (DP-14 moved both out of the core; DP-10
promoted `TypeSafe` from parked when fathom, a real workload, pulled for it
on owner direction 2026-09-22; see "Design constraints" below). The exported
API (`Kind`, `Option`, `Question` (including `Levels`), `Request`, `Answer`,
`Usage`, `Response`, `Decider`, `ErrNoProvider`, `Validate`, `Confidence`,
`Band`/`BandOf`, `Record`, `Response.Records` in `decision`;
`NewLLM`/`LLMOption`/`WithMaxTokens`/`WithNow` in `llm`;
`NewTypeSafe`/`TypeSafeConfig`/`TypeSafeError`/`DefaultTypeSafeBaseURL`/
`DefaultTypeSafeModel` in `typesafe`) is a contract external consumers are
coding against concurrently with this change — the names are fixed, not a
first draft. The OpenAI adapter (`NewOpenAI`/`OpenAIConfig`/`OpenAIError`
in `openai`) joined on 2026-10-09 (DP-25..DP-30 below).

## Tasks

- [x] `Kind`/`Option`/`Question`/`Request` types + `Validate` (IDs
  non-empty/unique, known kinds, choice 2..255 unique-named options, score
  Max>Min, non-empty State) {id: DP-1, symbol: Validate, package: foundations/decision}
- [x] `Confidence(probabilities)` — the vendor-documented k/max formula
  (k<=1 -> 1; else (k*max-1)/(k-1) clamped to [0,1]; k=3 is exactly
  TypeSafe's `(3*max-1)/2`) {id: DP-2, symbol: Confidence, package: foundations/decision}
- [x] `Band`/`BandOf` — unsure below 0.5, proceed at/above 0.9, confirm
  between {id: DP-3, symbol: BandOf, package: foundations/decision}
- [x] `None` — the zero-configuration `Decider`; always `ErrNoProvider`
  {id: DP-4, symbol: None, package: foundations/decision}
- [x] `LLM`/`NewLLM` — one `llmclient.Request` per `Decide` call: a fixed
  System contract, a deterministic JSON Prompt (State + Questions, order
  preserved, no map iteration leaking into the rendering), an OutputSchema
  built per question Kind; per-answer validation classifies a bad answer
  into `Response.Invalid` by question ID instead of failing the batch;
  probabilities are filled-missing-as-zero and renormalised; scores/nouls
  are clamped; Confidence is always this package's own formula, never taken
  from the model; a reply that is not JSON at all is the one hard error
  {id: DP-5, symbol: NewLLM, package: foundations/decision/llm}
- [x] `Response.Records(site)` — flattens answered (non-Invalid) questions
  to a sorted `[]Record`, Usage/Latency copied onto each
  {id: DP-6, symbol: Records, package: foundations/decision}
- [x] `MockDecider` test double for consumers exercising the `Decider`
  interface without a real provider {id: DP-7, symbol: NewMockDecider, package: foundations/decision}
- [x] Golden fixtures pinning the rendered LLM prompt and OutputSchema for
  one fixed Request (`testdata/prompt.golden`, `testdata/schema.golden`) so
  prompt/schema drift is visible in review {id: DP-8, symbol: LLM.Decide, package: foundations/decision/llm}
- [x] `schema.cue` (`#Kind`/`#Option`/`#Question`/`#Request`) + valid/invalid
  testdata pairs per the package-shape convention {id: DP-9, package: foundations/decision}
- [x] **`typesafe` adapter** — a `Decider` calling TypeSafe AI's System One
  API directly (POST `{BaseURL}/v1/systemone`; the vendor this package's
  Kind vocabulary and Confidence formula are deliberately modeled on), so a
  caller can switch from `LLM` (a generic `llmclient` completion asked to
  emulate the same contract) to the vendor's own decision endpoint by
  construction swap alone: `NewTypeSafe`/`TypeSafeConfig`, `TypeSafe.Decide`
  (one request per batch, per-question Invalid classification exactly like
  `LLM`, Confidence always this package's own formula never the wire's),
  `TypeSafe.Ping` (satisfies `llmclient.Pinger`'s method set without
  importing it), `TypeSafeError` (`Retryable()` true for 429/529; the API
  key never appears in it). `Question.Levels` (additive, DP-11 below) gives
  `KindScore` an ordered level list TypeSafe's "score" primitive needs;
  without it, an integral 2..10-level `[Min, Max]` is synthesized, else the
  question lands in `Response.Invalid` without reaching the wire. D8's hold
  ("no named second provider until a workload pulls for it") is LIFTED
  2026-09-22: fathom, a real workload, pulled for it on owner direction —
  see "Design constraints" below for what stays true of the promoted row.
  {id: DP-10, symbol: NewTypeSafe, package: foundations/decision/typesafe,
  reachable_from: TestTypeSafe_Decide_MixedBatch, library: true, reason: "kit foundation API; its consumer is fathom's decide.Open (fathom 1246f7e1, wave jf), exercised by task downstream-check (fathom)"}
- [x] **`Question.Levels`** — an optional ordered `[]string` of level
  descriptions for `KindScore` (2..10 entries when set; `Validate` requires
  Min/Max then be unset or exactly `0`/`len(Levels)-1`) (DP-13 moved the 2..10
  bound to the adapters' declared limits), additive to both
  adapters: `LLM` renders it into the prompt/schema when set and is
  byte-identical to before Levels existed when empty (golden-tested);
  `TypeSafe` sends it verbatim as `criteria`, or synthesizes it from an
  integral `[Min, Max]` range when unset. {id: DP-11, symbol: Validate,
  package: foundations/decision, reachable_from: decision_test.TestValidate, library: true, reason: "kit foundation API; its consumer is verified by task downstream-check (fathom)"}

## Design constraints

- **D8, DP-10**: hold lifted 2026-09-22: fathom pulled for it on owner
  direction. D8 originally read "no named second provider until a workload
  pulls for it" — fathom (`sextant`/`fathom` repo) is that named second
  provider/consumer, pulling for the native TypeSafe endpoint rather than
  the `LLM` adapter's generic-completion approximation of the same
  contract; a companion `docs/plans/*.md` row in that repo names the
  consuming call site.
- DP-10's promoted adapter produces the exact same `Response`/`Answer`/
  `Confidence` shape as `LLM` — a consumer switching adapters sees no
  type-level difference, only a possible change in the numbers themselves.
- DP-10 and DP-11 land together: DP-10's "score" primitive needs an ordered
  level list, which DP-11 (`Question.Levels`) supplies (explicitly, or
  synthesized from an integral `[Min, Max]`); neither is meaningful without
  the other.

## Hexagonal hardening

Owner-accepted 2026-09-29; each box is its row's state, flipped only when `fathom verify --strict` proves it. Recorded in [decision 0017](../intent/decisions/0017-fast-judgment-is-optional-and-calibrated.md);
guidance in [fast-judgment.md](../fast-judgment.md). The port is mostly hexagonal
(`Decider{Name, Decide}`; adapters `None`, `LLM`, `TypeSafe`, `MockDecider`). These rows close the
remaining vendor lock-in leaks so a better or different fast model plugs in without a redesign; the
reference consumer's ADR-056 and its plan rows JF-13, JF-14 and JF-24 track the same work from its side and
bump their vendored mirror after it lands. Sequenced after the reference consumer's in-flight push lands
(owner's sequencing). DP-12 and DP-14 are breaking (external consumers code against the names listed in the
mission, which this section is the one sanctioned exception to), so they land together with DP-24: `compat.md`,
a migration note and a `### Breaking` line.

- [x] **1. Vendor vocabulary.** Rename `KindNoul` to a generic kind (for example `KindPredicate`), keep "noul" as a wire and config alias that decodes to it, and stop documenting a 1:1 TypeSafe mirror in the core's `doc.go` and `types.go` {id: DP-12, symbol: ParseKind, package: foundations/decision, reachable_from: TestParseKindNoulAliasDecodesToPredicate, library: true, reason: "kit foundation API; its consumer is verified by task downstream-check (fathom)"}
- [x] **2. Vendor limits out of domain validation.** `Validate` stops enforcing Score's 2..10 levels; each adapter declares its limits (DP-15) and the core adapts or refuses cleanly, so an out-of-limit question lands in `Response.Invalid` by question ID without reaching the wire {id: DP-13, symbol: Validate, package: foundations/decision, reachable_from: TestValidateHasNoVendorScoreLevelLimit, needs: "DP-15", library: true, reason: "kit foundation API; its consumer is verified by task downstream-check (fathom)"}
- [x] **3. Adapters in subpackages.** `LLM` moves to `decision/llm` and `TypeSafe` to `decision/typesafe`; the core imports no vendor code, proven by an import-graph test {id: DP-14, symbol: NewLLM, package: foundations/decision/llm, reachable_from: TestCoreImportsNoAdapterPackage, needs: "DP-12", library: true, reason: "kit foundation API; its consumer is fathom's decide.Open (fathom 1246f7e1, wave jf), exercised by task downstream-check (fathom)"}
- [x] **4. `Capabilities()`.** Each Decider declares kinds, max options and levels, batching, latency and cost class, and confidence semantics (calibrated probability, logprob or self-reported); whether it is a method on `Decider` or an optional interface is settled in the change and recorded in `compat.md` {id: DP-15, symbol: CapabilitiesOf, package: foundations/decision, reachable_from: TestCapabilitiesOfDeclaresKindsLimitsBatchingCostAndConfidence, library: true, reason: "kit foundation API for downstream consumers; no consumer reaches it yet (fathom adopts it through the judgment program)"}
- [x] **5. `decisiontest` conformance kit.** One suite every adapter must pass (Validate handling, per-question Invalid classification, abstain, Confidence computed by this package, no key in an error); `None`, `LLM`, `TypeSafe` and `MockDecider` all run it {id: DP-16, symbol: RunConformance, package: foundations/decision/decisiontest, reachable_from: TestLLMPassesConformance, needs: "DP-15", library: true, reason: "kit foundation API for downstream consumers; no consumer reaches it yet (fathom adopts it through the judgment program)"}
- [x] **6a. Decorator: cascade.** Cheap first, escalate on abstain or low confidence, as a `Decider` {id: DP-17, symbol: NewCascade, package: foundations/decision, reachable_from: TestCascadeEscalatesOnAbstainOrLowConfidence, needs: "DP-15", library: true, reason: "kit foundation API for downstream consumers; no consumer reaches it yet (fathom adopts it through the judgment program)"}
- [x] **6b. Decorator: fallback.** An offline or second provider when the first fails or is unavailable, as a `Decider` {id: DP-18, symbol: NewFallback, package: foundations/decision, reachable_from: TestFallbackUsesNextDeciderOnError, needs: "DP-15", library: true, reason: "kit foundation API; its consumer is fathom's decide.Open (fathom 1246f7e1, wave jf), exercised by task downstream-check (fathom)"}
- [x] **6c. Decorator: cache.** Keyed by (question template, content hash) and stamped with provider and model version, kept apart from any proof, so a better model re-grades cleanly {id: DP-19, symbol: NewCached, package: foundations/decision, reachable_from: TestCachedKeysByTemplateAndContentHashStampsModel, needs: "DP-15", library: true, reason: "kit foundation API for downstream consumers; no consumer reaches it yet (fathom adopts it through the judgment program)"}
- [x] **6d. Decorator: record and replay.** Deterministic tests and offline runs from a recorded transcript {id: DP-20, symbol: NewReplay, package: foundations/decision, reachable_from: TestReplayReproducesRecordedResponses, needs: "DP-15", library: true, reason: "kit foundation API for downstream consumers; no consumer reaches it yet (fathom adopts it through the judgment program)"}
- [x] **6e. Policy in configuration.** The decorators are assembled from configuration data, not call-site code, so a consumer changes cascade or fallback order without a rebuild {id: DP-21, symbol: ComposeFromConfig, package: foundations/decision, reachable_from: TestComposeFromConfigBuildsCascadeFallbackCache, needs: "DP-17,DP-18,DP-19,DP-20", library: true, reason: "kit foundation API; its consumer is fathom's decide.Open (fathom 1246f7e1, wave jf), exercised by task downstream-check (fathom)"}
- [x] **7. Calibration mapping in the core.** Raw provider confidence maps to a per-site calibrated band from that site's growing calibration set; `BandOf`'s 0.5 and 0.9 stay documented as uncalibrated starting bands {id: DP-22, symbol: CalibratedBand, package: foundations/decision, reachable_from: TestCalibratedBandFromSiteReading, needs: "DP-23", library: true, reason: "kit foundation API for downstream consumers; no consumer reaches it yet (fathom adopts it through the judgment program)"}
- [x] **7b. Readings library.** For each (site, template version, provider, resolved model): n, accuracy by confidence band, Wilson bounds, Brier score, AUROC, the label provenance mix (human, frontier, computed) and a recalibration-due flag when the model moves; free compute, no table {id: DP-23, symbol: NewReading, package: foundations/decision, reachable_from: TestReadingReportsWilsonBrierAucAndRecalibrationDue, library: true, reason: "kit foundation API; its consumer is fathom's Recorder.Record (fathom 1246f7e1, wave jf), exercised by task downstream-check (fathom)"}
- [x] **Records for the breaking moves.** `compat.md` and a migration note under `docs/upgrades/` name the `KindNoul` rename, the wire alias and the adapter move, and `CHANGELOG.md` carries a `### Breaking` line {id: DP-24, oracle: lexical, path: docs/compat.md, pattern: "foundations/decision/typesafe", needs: "DP-12,DP-14"}
- [-] **Ensemble decorator.** Several providers vote or disagree-and-abstain {id: DP-D1, disposition: deferred, reason: "the owner said ensemble later; cascade and fallback come first", trigger: "two providers pass DP-16 and a consumer's calibration reading shows cascade or fallback leaves a site under its bar"}

## Provider adapters, refusals and key sourcing

Owner direction 2026-10-09: support OpenAI's Decisions API hexagonally alongside Jev, with
decision models staying optional (provider none is the default; a consumer's unconfigured
output stays byte-identical). Recorded in
[decision 0018](../intent/decisions/0018-one-decision-port-optional-provider-adapters.md).
Vendor facts were read 2026-10-09 from OpenAI's Decisions guide and OpenRouter's Decisions
API reference; no live call was made, so both wires are pinned by golden request fixtures
written from those references. DP-25 to DP-29 stay open until `fathom verify` proves them:
every check but `wired` passes (symbol present, tests reach it, the named test reaches it),
and their `library: true` waivers were written in the change that adds the rows, which the
gate does not accept as self-grading. A reviewer's acceptance of the waiver, or a product
entry point reaching the symbol, flips them.

- [ ] **OpenAI Decisions adapter.** `openai.NewOpenAI`/`OpenAIConfig`/`OpenAIError`: one POST
  `{BaseURL}/decisions` per Request (BaseURL default `https://api.openai.com/v1`, model
  `gpt-6-luna`), every question named by its ID; predicate to `Noul`, choice to `Choice` and
  per-option `Probabilities`, score to `Score` (shifted by Min for a synthesized range) and
  its level distribution; Confidence derived by the core's rule, the wire's `confidence`
  never read; a `refusal` answer to `Response.Declined`; images as base64 data URLs;
  Capabilities: all three kinds, images, low cost, native (uncalibrated) probabilities, no
  model pinning, latency unknown; stdlib `net/http`, no SDK; `TestOpenAIPassesConformance`
  with refusals and echoed-key redaction {id: DP-25, symbol: NewOpenAI, package: foundations/decision/openai, reachable_from: TestOpenAIPassesConformance, needs: "DP-27,DP-28,DP-29", library: true, reason: "kit foundation API for downstream consumers; no consumer constructs it yet (fathom's decide provider config adopts it on its side)"}
- [ ] **Jev over OpenRouter.** `TypeSafeConfig.Transport` = `TransportOpenRouter` sends the
  same System One wire to POST `https://openrouter.ai/api/alpha/decisions` (verified against
  OpenRouter's API reference) with an OpenRouter key and model `typesafe/jev-1.13` by default;
  `Path` overrides either transport's path; instructions are always sent on that route (its
  schema requires them); `Name` is `typesafe-openrouter`. The default transport's wire and
  golden are unchanged {id: DP-26, symbol: NewTypeSafe, package: foundations/decision/typesafe, reachable_from: TestTypeSafe_OpenRouterTransport, needs: "DP-27", library: true, reason: "kit foundation API; its consumer is fathom's decide.Open (provider typesafe), exercised by task downstream-check (fathom)"}
- [ ] **Key sourcing, shared by every remote adapter.** `internal/apikey`: a key set in code,
  else `APIKeyFile` (`~` expanded, read at first use, trimmed, refused with `chmod 600 <path>`
  when group or other users can access it), else `APIKeyEnv`; a missing key is an error naming
  the source, never a panic and never the key; `Redact` removes the key, echoed Authorization
  headers, bearer tokens and `sk-` shapes from provider error text (closes kit observation O3's
  echo half: `TestTypeSafePassesConformance` now runs with `RedactsEchoedKey: true`) {id: DP-27, symbol: Resolver.Resolve, package: foundations/decision/internal/apikey, reachable_from: TestOpenAIKeySources, library: true, reason: "internal helper of the kit's remote decision adapters; reached through NewTypeSafe and NewOpenAI"}
- [ ] **Refusals are first-class.** `Response.Declined` (additive, omitted from JSON when
  empty): a refused question is never in `Answers` or `Invalid`; `Fallback` records a Response
  that declines as a disclosed miss wrapping `ErrDeclined` and tries the next Decider, returning
  the first declined Response unchanged only when none answers cleanly; `Cascade` escalates a
  decline and reports a last-stage decline in `Declined`; `Cached` never stores one;
  `decisiontest` checks it (`Harness.Refusals`) plus one-outcome-per-question {id: DP-28, symbol: Fallback.DecideReport, package: foundations/decision, reachable_from: TestFallbackTreatsDeclinedAsDisclosedMiss, library: true, reason: "kit foundation API; its consumer is fathom's decide.Open (fallback policy), exercised by task downstream-check (fathom)"}
- [ ] **Images on the port.** `Request.Images` and `Image` (additive; `Validate` checks media
  type and data), `Capabilities.Images` and `Capabilities.ModelPinning`; `UnsupportedImages`
  lets a text-only adapter (`llm`, `typesafe`) answer none of a Request's questions instead of
  answering without the image; `Cascade` and `Cached` forward images to their inner Deciders,
  `Cached` never caches an image Request, and a text-only `RequestKey` is pinned unchanged {id: DP-29, symbol: UnsupportedImages, package: foundations/decision, reachable_from: TestUnsupportedImages, library: true, reason: "kit foundation API for downstream consumers; no consumer sends images yet"}
- [x] **Records.** `compat.md` rows for `foundations/decision/openai` and
  `foundations/decision/internal/apikey`, a `CHANGELOG.md` Added line, decision 0018 {id: DP-30, oracle: lexical, path: docs/compat.md, pattern: "foundations/decision/openai", needs: "DP-25"}

## Acceptance = proof

1. `task validate` green (fmt-check, vet, cue-vet, `constitution-lint`,
   `go test -race ./...`).
2. `foundations/decision`'s own suite: 35 top-level test funcs (plus
   table-driven subtests), `-race` clean, zero network (every adapter is
   exercised entirely through `llmclient.MockClient` or `httptest`).
3. `constitution-lint` clean on this package specifically: schema-presence,
   tracking-coverage, compat-coverage, catalog-fresh, readme-count,
   doc-coverage (every exported identifier documented), concurrency-contract
   (doc.go states the contract), org-agnostic, secret-scan (no API key
   shape anywhere in source or tests).
4. `cue vet ./...` and `cue vet ./testdata/` clean from
   `foundations/decision/`.
5. `fathom verify --service agentic-kit --plan docs/plans/decision-port.md`
   proves DP-10 and DP-11 before either box flips to `[x]` — see "Done
   means proven" in the repo's own operating doctrine; a claim, not a
   proof, does not earn the checkbox.
