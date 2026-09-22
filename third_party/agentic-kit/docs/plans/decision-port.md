# decision — typed fast-decision port

**Mission** — a typed fast-decision port: callers ask typed `Question`s
(`KindChoice`/`KindScore`/`KindNoul`) about a blob of state and get
probabilities back, never text. The question kinds deliberately mirror
TypeSafe AI's System One primitives (Choice/Score/Noul) so a future
`typesafe` adapter maps onto `foundations/decision`'s types 1:1 instead of
needing a shape-translation layer. This plan lands the package with two
adapters, `None` and `LLM`; a `typesafe` adapter calling TypeSafe's System
One API directly is deliberately parked (D8: no named second provider
consumer yet) — see "Parked" below. The exported API (`Kind`, `Option`,
`Question`, `Request`, `Answer`, `Usage`, `Response`, `Decider`,
`ErrNoProvider`, `Validate`, `Confidence`, `Band`/`BandOf`, `Record`,
`Response.Records`, `NewLLM`/`LLMOption`/`WithMaxTokens`/`WithNow`) is a
contract external consumers are coding against concurrently with this
change — the names are fixed, not a first draft.

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

## Parked

- [-] **`typesafe` adapter** — a `Decider` calling TypeSafe AI's System One
  API directly (the vendor this package's Kind vocabulary and Confidence
  formula are deliberately modeled on), so a caller can switch from `LLM`
  (a generic `llmclient` completion asked to emulate the same contract) to
  the vendor's own decision endpoint by construction swap alone. Not built
  in this pass: no TypeSafe API key is configured anywhere in the fleet, and
  no consumer has measured whether the vendor's native endpoint beats the
  `LLM` adapter's generic-completion approximation for cost, latency, or
  accuracy. Building it now would be exactly the "demos well, no real
  workload pulls for it" hazard `docs/roadmap.md` warns against. {id: DP-10,
  disposition: deferred, reason: "no TypeSafe API key configured; no
  consumer has run the A/B this adapter would need to justify itself over
  the LLM adapter", trigger: "Promote when: an API key exists and a
  consumer's A/B shows a gain over the LLM adapter"}

## Design constraints

- **D8 holds for DP-10**: the row above gets no typing, no package
  skeleton, and no stub file in this pass — filing it is the deliverable,
  exactly as `docs/plans/fathom-uplift-asks.md`'s FA-11/FA-12 rows and this
  repo's own `docs/plans/claudecli-ping.md` KP-3 row establish for a parked
  ask with a named trigger.
- If DP-10's trigger fires, the `typesafe` adapter must produce the exact
  same `Response`/`Answer`/`Confidence` shape as `LLM` — a consumer
  switching adapters must see no type-level difference, only a possible
  change in the numbers themselves.

## Acceptance = proof

1. `task validate` green (fmt-check, vet, cue-vet, `constitution-lint`,
   `go test -race ./...`).
2. `foundations/decision`'s own suite: 34 test funcs, `-race` clean, zero
   network (the `LLM` adapter is exercised entirely through
   `llmclient.MockClient`).
3. `constitution-lint` clean on this package specifically: schema-presence,
   tracking-coverage, compat-coverage, catalog-fresh, readme-count,
   doc-coverage (every exported identifier documented), concurrency-contract
   (doc.go states the contract), org-agnostic.
4. `cue vet ./...` and `cue vet ./testdata/` clean from
   `foundations/decision/`.
