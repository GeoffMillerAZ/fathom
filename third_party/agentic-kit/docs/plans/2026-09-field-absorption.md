# Field absorption 2026-09 — truth pass, read-manifest, reliability statistics, tool effects, protocol clocks, the ledger wave

**Mission** — VISION's practice-watch says the kit absorbs the field's new
failure modes as opinionated primitives plus ledger entries, and downstream
is the oracle. A 14-agent evaluation run from fathom on 2026-09-01 (lanes E3
patterns, E4 bench corpora, I3 kit internals; distilled in fathom's
`docs/vision/25-harness-port.md` and `26-proof-credibility.md`) found the
ledger behind the code both ways — claims the code does not keep, and code
the ledger does not know — and four primitives with two consumers each. This
plan lands them under the junk gate: contract, `-race` tests, non-guarantees
stated, a named consumer, the gate green. Truth precedes new entries. Ledger
entries land only where a package or a named consumer carries them (absorb,
don't annotate); the rest are one-line futures with a Promote-when.

**Write scope** — this repo. Fathom consumes by pin bump in its own tree.

**Gate** — `task validate` and `task downstream-check` green per unit; the
ledgers (`TRACKING.md`, `docs/compat.md`, `docs/catalog.md`, `CHANGELOG.md`)
updated in the same change as the code (`docs/intent/repo.md` §Adding).

Verify (from the fathom checkout): `fathom verify --service agentic-kit --plan docs/plans/2026-09-field-absorption.md`.

## Wave 1 — truth pass (belief 6: overclaiming is a defect)

- [x] `claudecli` runs `llmtest.RunConformance`'s core checks against a fake shim binary (all optional `Capabilities` false, each named) {id: FB-1, symbol: TestClaudeCLIConformance, package: foundations/llmclient/claudecli, skip_test: true, skip: "the test is the anchor"}
- [x] `codexcli` runs the same suite the same way {id: FB-2, symbol: TestCodexCLIConformance, package: foundations/llmclient/codexcli, skip_test: true, skip: "the test is the anchor"}
- [x] `noop` runs the same suite {id: FB-3, symbol: TestNoopConformance, package: foundations/llmclient/noop, skip_test: true, skip: "the test is the anchor"}
- [x] `mcp-client`'s package doc no longer claims `CallTool` verifies the pin per call (INC-5 residue; TRACKING H8-1 stays open); a doc-pin test holds the doc to the method comment {id: FB-4, symbol: TestDocDoesNotClaimPinVerification, package: foundations/agent/mcp-client, skip_test: true, skip: "the test is the anchor"}
- [x] `compat.md`: `claudecli` and `codexcli` carry an accepted consumer-on-experimental line naming fathom's compile lanes (promotion to `stable` is the owner's, `repo.md` §Promote; the trigger has fired and is recorded); the `evidence-compactor` row says what is true {id: FB-5, skip_test: true, skip: "docs; evidence is the compat.md diff"}
- [x] `evidence-compactor` is wired: `agentharness.Config` gains an option (`WithEvidenceCompactor`) that installs it as the conversation `Compactor`; `examples/agent-cli` uses it; practices §1 says what is true {id: FB-6, symbol: WithEvidenceCompactor, package: kits/agent-harness}
- [x] Intent refresh: `roadmap.md` #13/#14/#15 (failover, mcp-client, Streamable HTTP shipped), `futures.md` F9 (shipped) and F2's drop evidence (fathom ADR-028), practices §16's skill path, TRACKING's v0.7.0 deprecation-watch trigger recorded as fired, `handoff-h4-sextant.md` Stage 3 dispositioned (fathom carries its own governed client; row `[-]` with trigger) {id: FB-7, skip_test: true, skip: "docs; evidence is intent-audit's finding count in the Verified: line"}

## Wave 2 — primitives with two consumers (D13)

- [x] `foundations/telemetry/read-manifest` (package `readmanifest`, stdlib only, imports nothing from fathom or fusion): `Outcome` (`read | absent | unreadable(errno) | skipped(category)`), `NewSet`, `Set.Record` first-outcome-wins, `Manifest.Complete`, `LowerBound` rendering `≥ N (k unreadable: …)`; `schema.cue` with fixtures {id: FB-8, symbol: NewSet, package: foundations/telemetry/read-manifest}
- [x] `constitution-lint` routes its continue-on-error walk sites through a manifest (`walkWithManifest`), reports counts as lower bounds when incomplete, and FAILS on an unreadable file or directory; `TestUnreadableTreeNeverPassesConstitution` {id: FB-9, symbol: walkWithManifest, package: cmd/constitution-lint}
- [x] Practices §17 "Instruments that degrade toward healthy" (OWASP ASI id where one applies) with the two consumers named (fathom `internal/readset` → this package; constitution-lint); closes FA-20/FA-21 in `fathom-uplift-asks.md` {id: FB-10, skip_test: true, skip: "ledger entry; evidence is practices.md and the asks plan diff"}
- [x] `foundations/quality/reliability-stats` (package `reliabilitystats`): `PairedBootstrapCI` (seeded, task-clustered, byte-stable), `Wilcoxon` (exact for n ≤ 25, normal approximation above), `A12` (Vargha–Delaney), `PassAtK` (the unbiased estimator) and `PassPowK`; known-answer tests from the published definitions (`PassPowK(0.9, 8) = 0.4305`) {id: FB-11, symbol: PairedBootstrapCI, package: foundations/quality/reliability-stats}
- [x] `PassPowK` and `A12` documented with their non-guarantees (no multiple-comparison correction; the caller pre-registers) {id: FB-12, symbol: PassPowK, package: foundations/quality/reliability-stats}
- [x] `eval-runner`: `Regression` reports `pass^3` per scenario from `reliability-stats` (the kit's own consumer of the package; no judge protocol — F14 stays parked) {id: FB-13, symbol: regressionPassPowK, package: kits/eval-runner}
- [x] `tool-registry`: `Effect{ReadOnlyHint, DestructiveHint, IdempotentHint, OpenWorldHint}` — the MCP annotation vocabulary verbatim — on a tool (`EffectOf`; nil = unknown = destructive under `EffectPolicy` Strict) {id: FB-14, symbol: EffectOf, package: foundations/agent/tool-registry}
- [x] `agent-harness`: under Strict, a destructive or undeclared dispatch is routed through the existing `PreDispatch` seam to a `kits/hitl` approval (`RequireApproval`); denied or absent approval yields `is_error` plus a `gate_denied` event; `Defs()` unchanged; the governed-investigation scenario dispatches one destructive tool through it {id: FB-15, symbol: RequireApproval, package: kits/agent-harness}
- [x] `mcp-server` emits MCP tool annotations from `Effect` (`annotationsFromEffect`) {id: FB-16, symbol: annotationsFromEffect, package: kits/mcp-server}
- [x] `mcp-client` maps annotations in as untrusted unless `TrustedServer` (`effectFromAnnotations`) {id: FB-17, symbol: effectFromAnnotations, package: foundations/agent/mcp-client}
- [x] `ProtocolRevision` and `Deprecations{Feature, DeprecatedIn, RemovableAfter, StillImplemented}` exported by `mcp-server` and `mcp-client` for the 2026-07-28 retirements (12-month floor); `DeprecationsDue(now)` with an injected clock fails the moment a still-implemented feature passes its floor {id: FB-18, symbol: DeprecationsDue, package: kits/mcp-server}
- [x] `coordination/task-tree`: one CUE `#TaskState` (pending / running / input-required / completed / failed / cancelled, with terminal and interrupted classes) and `CanonicalState` mapping the three divergent enums onto it; plan-only, I20-safe {id: FB-19, symbol: CanonicalState, package: foundations/coordination/task-tree}
- [x] `agent-harness`: a typed `Handoff` record (`HandoffFor`) emitted on `StopBudget`, `StopMaxTokens` and `StopMaxTurns` — what was tried, the last tool, the checkpoint ref — the receipt of one run, never memory (F2); `examples/agent-cli` prints it {id: FB-20, symbol: HandoffFor, package: kits/agent-harness}

## Wave 3 — tooling and the ledger wave

- [x] `cmd/kit-new -package <root>/<category>/<name> [--practice N]` scaffolds the six-file package shape plus the exact `TRACKING.md`, `compat.md`, practices and README-count lines; idempotent; golden-tested; constitution-lint green on its output {id: FB-21, symbol: scaffoldPackage, package: cmd/kit-new}
- [x] Practices ledger: every entry carries the OWASP Top 10 for Agentic Applications id (ASI01–ASI10) it maps to, where one applies; new entries only where a package or named consumer in this plan carries them — §17 instruments that degrade toward healthy (FB-8/9), §18 irreversible actions without a structural gate (FB-14..17), §19 protocol-revision drift (FB-18), §20 fail-open gates mistaken for gates (consumer: fathom doctor `gate_posture`), §21 self-derived ground truth, n=1 evidence and precision-less set grading (consumer: fathom bench on FB-11), §22 handoff-less stops (FB-20); the remaining field anti-patterns (ambient credentials in tool results, scope creep on benign tasks, self-graded completion, prompt-only guards, non-idempotent replay, cascading failure, immortal runs, always-loaded context, judge bias, pass@1 theater, trust-config drift, first-match instruction files, parse-failure-means-allow, state handles, per-model compensations, approval theater, firehose tool results) go to `futures.md` as one line each with a Promote-when {id: FB-22, skip_test: true, skip: "ledger prose; every STATUS is grep-verifiable against its named package"}
- [x] Decision records `0002` (tool effects: the MCP annotation vocabulary is the kit type; nil means destructive under Strict; approval composes PreDispatch and hitl; orthogonal to the trifecta re-check decision) and `0003` (MCP revisions: track with a clock, never chase; the 2026-07-28 stateless move is a deferred decision with its own floor) {id: FB-23, skip_test: true, skip: "decision records; evidence is the decisions index"}
- [x] FA-22 filed: the harness manifest contract moves here only when this repo deletes `install-intent.sh` and the `integration.md` harness table onto it {id: FB-24, skip_test: true, skip: "an ask row; evidence is fathom-uplift-asks.md"}
- [x] Ledgers: `TRACKING.md` rows, `compat.md` rows (new packages `experimental`), `task docs-index`, README count, `CHANGELOG.md` `[Unreleased]` lines under the two-sentence budget; `task validate` and `task downstream-check` green {id: FB-25, skip_test: true, skip: "ledger upkeep; evidence is constitution-lint's catalog-fresh/readme-count/compat-coverage/tracking-coverage checks in task validate"} — **2026-09-02**: `task validate` exit 0 (catalog 192 packages, README count matched, `compat.md` counts recomputed from its own rows: 124/30/24/178, the summary having drifted to 25-against-28 BEFORE this wave). `task downstream-check`: fathom green (build + full `go test`), growing-gardens-bi green, **throughline red for a pre-existing reason unrelated to the kit** — `go mod tidy` wants `golang.org/x/sys` 0.46→0.47 and `golang.org/x/sync` 0.21→0.22 in ITS module graph; the kit contributes nothing to that graph (`go.mod` has no `require` block and is byte-identical to the pre-merge base), and no exported kit symbol changed under it.
