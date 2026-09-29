# Domain

What is true about the kit's problem space that the code cannot tell you. The eight
core beliefs (`VISION.md`) and the anti-pattern ledger (`docs/practices.md`) are the
long form; this is the vocabulary, the truths, and the assumptions a design must respect.

## Vocabulary

- **foundation** — a narrow, stdlib-only package: a durable noun (`foundations/`).
  **kit** — a high-order workflow composed from foundations (`kits/`). **runtime** —
  execution, orchestration, policy, connector substrate. **surface** — a human- or
  developer-facing interface. **toolchain** — canonical CUE schemas, generators,
  materialization. Imports point downward through these tiers.
- **the gate** — `task validate` (gofmt, vet, `cue vet` with fixtures, `go test -race`,
  `constitution-lint`) plus a green `task downstream-check`. "The gate is the contract."
- **constitution** — the kit's hard rules expressed as checks in
  `cmd/constitution-lint`. **guard** — a pre-edit rule in `fathom-pack/guard-pack.cue`,
  scoped by file or command glob. Injection timing and coverage depend on the active
  harness; a served instruction is not evidence that an effect was prevented.
- **junk gate** — the definition of done for any addition: a contract, deterministic
  `-race`-clean tests pinning the central claim, docs stating non-guarantees, a named
  consumer or a bounded incubation (0004), the full gate green.
- **consumer** — a downstream repo importing kit code. Named: fathom (on disk as
  `../fathom` or `../sextant`), throughline, growing-gardens-bi, racli (`../resume_as_code`,
  vendored). **subscriber** — the consumer pulling a capability. **Incubation** — a
  sponsored, review-bounded experiment with executable outcomes; it does not imply adoption.
- **classification** — `stable` / `experimental` / `internal-ish` per package in
  `docs/compat.md`; **promotion** is a documented, one-way event.
- **cassette** — a JSONL-recorded LLM exchange replayed in tests; no key, no network.
- **evidence span** — the atomic evidence unit: `Grade` (epistemic tier), `Author`
  (human or AI with reproduction key), `Locator` (click-traceable source). No provenance,
  not storable.
- **governed** — work with explicit authority, bounds, evidence and verification;
  a claim names the actual profile, wiring and execution surface (`assurance.md`).
  **HITL** — human-in-the-loop; preview, authorization, commit and recovery are distinct.
- **provider adapter** — translates model requests and responses. **Harness adapter** —
  integrates policy and evidence with a coding agent's lifecycle. A CLI shell-out that
  supplies completions does not control the caller's tool execution.
- **claim support** — a specific relationship from an assertion to evidence at a source
  version. Reference integrity, semantic entailment, and source authority are different.
- **lesson** — a scoped, versioned failure/reproducer/remedy with provenance and review;
  importing it neither establishes a fact nor grants permission (0006).
- **catalog** — either the generated package listing (`docs/catalog.md`, `task
  docs-index`) or `foundations/knowledge/catalog`, the kit for catalogs that cannot lie.
- **assessment profile** — a versioned, domain-specific choice of applicable criteria,
  required checks and optional scoring. Definition, observation and finding differ;
  a deterministic projection of caller-declared evidence does not verify that evidence.
- **prompt screen** — fallible policy classification before a protected continuation.
  Refuse and review both hold execution; an allow neither authenticates the caller nor
  authorizes tools. Content policy, deployment eligibility and effect authority differ.
- **D<n>** — a numbered decision from the 2026-07 uplift plan (D1–D10), the catalog
  design (D13), or the H2 pull queue (D35-1..5). See `decisions/README.md`.
- **wave** — a stage of the uplift plan (Waves 0–4, executed v0.2.1 → v0.6.0).
- **obligation** — a governing rule fathom injects on edit; fathom's concept, not the kit's.

## Domain truths

- Uncontracted shapes drift silently; a shape that is data needs a schema. Evidence:
  VISION belief 1; `lint schema-presence` exists because of it.
- Summaries that replace their sources sever the evidence link; compaction may shrink
  what the model sees, never what the system knows. Evidence: `practices.md` §1.
- A channel that never fires and a channel that stopped are indistinguishable to a
  liveness check; only an existence check with a recorded start time separates them.
  Evidence: the heartbeat lesson (INC-1), `heartbeat.CheckExistence`.
- Agent tooling drifts under you: an undocumented config key is honored today and
  silently dropped on the next CLI release. A pinned argv needs a tripwire, not a
  comment. Evidence: `llmclient/codexcli` `Ping`; `llmclient/claudecli` PROBES.md.
- Explicit credentials that fall through to ambient state sign as the wrong identity
  without an error. Evidence: `bedrock/awscred` hardening (INC-2).
- Resource confinement is not isolation; a stdlib-only dependency cannot provide a
  sandbox and must not claim one. Evidence: `threat-model.md` §3.
- Anything written outside the tokenization vault (journals, event stores, ledgers,
  cassettes) has no deletion path. Evidence: `threat-model.md` §Deletion.
- A false "done" is worse than an open row: the gate exists because claims outran
  proof. Evidence: `first-agent.md` golden drift (INC-3); H8-1 (INC-5).
- A valid citation ID does not show that evidence supports a sentence. Legacy prose
  retains its coarse bibliography; `SourcedPipeline` checks exact declared support and
  independent review. Neither reveals a model's hidden causal reasoning.
- A deterministic applier only produces the same preview and result for the same
  state and operation. `hitl.Engine.Apply` leaves state-version checks and authorization
  to callers; the shared applier alone does not prevent stale approvals. The additive
  `hitl.Committer` supplies the bound transaction protocol, not a durable database or
  a generic exactly-once external service.
- A repeated cassette replay is an implementation check, not another independent
  observation of model quality. Consumer outcome evaluations need distinct trials.
- A clean nested worktree need not produce a clean compiler VCS stamp: Go 1.26.2
  inspected RAC's outer repository in the observed build (INC-17). Check compiled
  revision/dirty status as well as the source tree; retain discrepancies and rebuild
  from an independent checkout when needed, never rewrite the historical metadata.
- CLI hook support changes with host and version. Current official Codex documentation
  includes SessionStart and tool hooks; this session's availability must still be
  tested. [Assessment sources](../assessments/2026-09-12-agentic-kit.md#platform-guidance).

## Assumptions

- Go 1.26+, CUE 0.16+, go-task, lefthook on the machine. If false: the gate does not
  run and nothing lands.
- Consumer compatibility can be checked without changing the consumer's tree. Current
  sibling checks skip missing repos; a proposed vendoring check mutates one. If false:
  use isolated checkouts and report every checked revision and skip (I21, G5).
- Tests never need a key or the network. If false: the ladder in `practices.md` §6 is
  broken and cassettes are the fix, not a skip.
- One owner (Geoff) ratifies promise-changing decisions; agents write almost all code.
  If false: `autonomy.md` and P10 need a second signature path.
- Fathom's service path comes from `list_services`, not an assumed sibling name.
  If index evidence is unavailable, report it as unknown and use bounded direct
  inspection; `verify` operational exit 2 is not proof of completion.
- Anthropic is the first battery, not the architecture. If false for a consumer: the
  `LLMClient` contract already has openai, bedrock, claudecli, codexcli adapters.
