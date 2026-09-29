# decision — typed fast-decision port

**Mission** — a typed fast-decision port: callers ask typed `Question`s
(`KindChoice`/`KindScore`/`KindNoul`) about a blob of state and get
probabilities back, never text. The question kinds deliberately mirror
TypeSafe AI's System One primitives (Choice/Score/Noul) so the `typesafe`
adapter maps onto `foundations/decision`'s types 1:1 instead of needing a
shape-translation layer. This plan lands the package with three adapters,
`None`, `LLM`, and `TypeSafe` (calling TypeSafe's System One API directly —
DP-10, promoted from parked: fathom, a real workload, pulled for it on owner
direction 2026-09-22; see "Design constraints" below for what D8 required
before promotion). The exported API (`Kind`, `Option`, `Question`
(including `Levels`), `Request`, `Answer`, `Usage`, `Response`, `Decider`,
`ErrNoProvider`, `Validate`, `Confidence`, `Band`/`BandOf`, `Record`,
`Response.Records`, `NewLLM`/`LLMOption`/`WithMaxTokens`/`WithNow`,
`NewTypeSafe`/`TypeSafeConfig`/`TypeSafeError`,
`DefaultTypeSafeBaseURL`/`DefaultTypeSafeModel`) is a contract external
consumers are coding against concurrently with this change — the names are
fixed, not a first draft.

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
  {id: DP-5, symbol: NewLLM, package: foundations/decision}
- [x] `Response.Records(site)` — flattens answered (non-Invalid) questions
  to a sorted `[]Record`, Usage/Latency copied onto each
  {id: DP-6, symbol: Records, package: foundations/decision}
- [x] `MockDecider` test double for consumers exercising the `Decider`
  interface without a real provider {id: DP-7, symbol: MockDecider, package: foundations/decision}
- [x] Golden fixtures pinning the rendered LLM prompt and OutputSchema for
  one fixed Request (`testdata/prompt.golden`, `testdata/schema.golden`) so
  prompt/schema drift is visible in review {id: DP-8, symbol: TestLLM_Decide_GoldenPromptAndSchema, package: foundations/decision}
- [x] `schema.cue` (`#Kind`/`#Option`/`#Question`/`#Request`) + valid/invalid
  testdata pairs per the package-shape convention {id: DP-9, package: foundations/decision}
- [ ] **`typesafe` adapter** — a `Decider` calling TypeSafe AI's System One
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
  {id: DP-10, symbol: NewTypeSafe, package: foundations/decision,
  reachable_from: TestTypeSafe_Decide}
- [ ] **`Question.Levels`** — an optional ordered `[]string` of level
  descriptions for `KindScore` (2..10 entries when set; `Validate` requires
  Min/Max then be unset or exactly `0`/`len(Levels)-1`), additive to both
  adapters: `LLM` renders it into the prompt/schema when set and is
  byte-identical to before Levels existed when empty (golden-tested);
  `TypeSafe` sends it verbatim as `criteria`, or synthesizes it from an
  integral `[Min, Max]` range when unset. {id: DP-11, symbol: Validate,
  package: foundations/decision, reachable_from: TestValidate}

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

Owner-accepted 2026-09-29; every row below is open and none is started. Recorded in [decision 0017](../intent/decisions/0017-fast-judgment-is-optional-and-calibrated.md);
guidance in [fast-judgment.md](../fast-judgment.md). The port is mostly hexagonal
(`Decider{Name, Decide}`; adapters `None`, `LLM`, `TypeSafe`, `MockDecider`). These rows close the
remaining vendor lock-in leaks so a better or different fast model plugs in without a redesign; the
reference consumer's ADR-056 and its plan rows JF-13, JF-14 and JF-24 track the same work from its side and
bump their vendored mirror after it lands. Sequenced after the reference consumer's in-flight push lands
(owner's sequencing). DP-12 and DP-14 are breaking (external consumers code against the names listed in the
mission, which this section is the one sanctioned exception to), so they land together with DP-24: `compat.md`,
a migration note and a `### Breaking` line.

- [ ] **1. Vendor vocabulary.** Rename `KindNoul` to a generic kind (for example `KindPredicate`), keep "noul" as a wire and config alias that decodes to it, and stop documenting a 1:1 TypeSafe mirror in the core's `doc.go` and `types.go` {id: DP-12, symbol: ParseKind, package: foundations/decision, reachable_from: TestParseKindNoulAliasDecodesToPredicate}
- [ ] **2. Vendor limits out of domain validation.** `Validate` stops enforcing Score's 2..10 levels; each adapter declares its limits (DP-15) and the core adapts or refuses cleanly, so an out-of-limit question lands in `Response.Invalid` by question ID without reaching the wire {id: DP-13, symbol: Validate, package: foundations/decision, reachable_from: TestValidateHasNoVendorScoreLevelLimit, needs: "DP-15"}
- [ ] **3. Adapters in subpackages.** `LLM` moves to `decision/llm` and `TypeSafe` to `decision/typesafe`; the core imports no vendor code, proven by an import-graph test {id: DP-14, symbol: NewLLM, package: foundations/decision/llm, reachable_from: TestCoreImportsNoAdapterPackage, needs: "DP-12"}
- [ ] **4. `Capabilities()`.** Each Decider declares kinds, max options and levels, batching, latency and cost class, and confidence semantics (calibrated probability, logprob or self-reported); whether it is a method on `Decider` or an optional interface is settled in the change and recorded in `compat.md` {id: DP-15, symbol: CapabilitiesOf, package: foundations/decision, reachable_from: TestCapabilitiesOfDeclaresKindsLimitsBatchingCostAndConfidence}
- [ ] **5. `decisiontest` conformance kit.** One suite every adapter must pass (Validate handling, per-question Invalid classification, abstain, Confidence computed by this package, no key in an error); `None`, `LLM`, `TypeSafe` and `MockDecider` all run it {id: DP-16, symbol: RunConformance, package: foundations/decision/decisiontest, reachable_from: TestLLMPassesConformance, needs: "DP-15"}
- [ ] **6a. Decorator: cascade.** Cheap first, escalate on abstain or low confidence, as a `Decider` {id: DP-17, symbol: NewCascade, package: foundations/decision, reachable_from: TestCascadeEscalatesOnAbstainOrLowConfidence, needs: "DP-15"}
- [ ] **6b. Decorator: fallback.** An offline or second provider when the first fails or is unavailable, as a `Decider` {id: DP-18, symbol: NewFallback, package: foundations/decision, reachable_from: TestFallbackUsesNextDeciderOnError, needs: "DP-15"}
- [ ] **6c. Decorator: cache.** Keyed by (question template, content hash) and stamped with provider and model version, kept apart from any proof, so a better model re-grades cleanly {id: DP-19, symbol: NewCached, package: foundations/decision, reachable_from: TestCachedKeysByTemplateAndContentHashStampsModel, needs: "DP-15"}
- [ ] **6d. Decorator: record and replay.** Deterministic tests and offline runs from a recorded transcript {id: DP-20, symbol: NewReplay, package: foundations/decision, reachable_from: TestReplayReproducesRecordedResponses, needs: "DP-15"}
- [ ] **6e. Policy in configuration.** The decorators are assembled from configuration data, not call-site code, so a consumer changes cascade or fallback order without a rebuild {id: DP-21, symbol: ComposeFromConfig, package: foundations/decision, reachable_from: TestComposeFromConfigBuildsCascadeFallbackCache, needs: "DP-17,DP-18,DP-19,DP-20"}
- [ ] **7. Calibration mapping in the core.** Raw provider confidence maps to a per-site calibrated band from that site's growing calibration set; `BandOf`'s 0.5 and 0.9 stay documented as uncalibrated starting bands {id: DP-22, symbol: CalibratedBand, package: foundations/decision, reachable_from: TestCalibratedBandFromSiteReading, needs: "DP-23"}
- [ ] **7b. Readings library.** For each (site, template version, provider, resolved model): n, accuracy by confidence band, Wilson bounds, Brier score, AUROC, the label provenance mix (human, frontier, computed) and a recalibration-due flag when the model moves; free compute, no table {id: DP-23, symbol: NewReading, package: foundations/decision, reachable_from: TestReadingReportsWilsonBrierAucAndRecalibrationDue}
- [ ] **Records for the breaking moves.** `compat.md` and a migration note under `docs/upgrades/` name the `KindNoul` rename, the wire alias and the adapter move, and `CHANGELOG.md` carries a `### Breaking` line {id: DP-24, oracle: lexical, path: docs/compat.md, pattern: "KindPredicate", needs: "DP-12,DP-14"}
- [-] **Ensemble decorator.** Several providers vote or disagree-and-abstain {id: DP-D1, disposition: deferred, reason: "the owner said ensemble later; cascade and fallback come first", trigger: "two providers pass DP-16 and a consumer's calibration reading shows cascade or fallback leaves a site under its bar"}

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
