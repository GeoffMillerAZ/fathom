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
- **constitution** — the kit's hard rules as running code: the 11 checks in
  `cmd/constitution-lint`. **guard** — a pre-edit rule in `fathom-pack/guard-pack.cue`,
  scoped by file or command glob, injected before an edit lands (5 kit-self, 4 consumer).
- **junk gate** — the definition of done for any addition: a contract, deterministic
  `-race`-clean tests pinning the central claim, docs stating non-guarantees, a named
  consumer, the full gate green.
- **consumer** — a downstream repo importing kit code. Named: fathom (on disk as
  `../fathom` or `../sextant`), throughline, growing-gardens-bi. **subscriber** — the
  consumer that pulls a roadmap item into existence; every item has a subscribe-when.
- **classification** — `stable` / `experimental` / `internal-ish` per package in
  `docs/compat.md`; **promotion** is a documented, one-way event.
- **cassette** — a JSONL-recorded LLM exchange replayed in tests; no key, no network.
- **evidence span** — the atomic evidence unit: `Grade` (epistemic tier), `Author`
  (human or AI with reproduction key), `Locator` (click-traceable source). No provenance,
  not storable.
- **governed** — work that runs under the kit's budgets, guardrails, escalation ladder,
  and provenance. **HITL** — human-in-the-loop, a growing-gardens-bi requirement.
- **catalog** — either the generated package listing (`docs/catalog.md`, `task
  docs-index`) or `foundations/knowledge/catalog`, the kit for catalogs that cannot lie.
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

## Assumptions

- Go 1.26+, CUE 0.16+, go-task, lefthook on the machine. If false: the gate does not
  run and nothing lands.
- Consumers are sibling checkouts; `downstream-check` skips a missing sibling. If
  false: I21 is silently vacuous; a CI checkout of the consumers is the fix.
- Tests never need a key or the network. If false: the ladder in `practices.md` §6 is
  broken and cassettes are the fix, not a skip.
- One owner (Geoff) ratifies promise-changing decisions; agents write almost all code.
  If false: `autonomy.md` and P10 need a second signature path.
- fathom's index resolves at `../fathom` or `../sextant`. If false: fathom tools fail
  closed; not documented as fail-open (contrast `downstream-check`).
- Anthropic is the first battery, not the architecture. If false for a consumer: the
  `LLMClient` contract already has openai, bedrock, claudecli, codexcli adapters.
