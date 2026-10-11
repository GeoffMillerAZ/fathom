# Future directions

Things that might happen. NOT requirements; nothing here authorizes building anything.
An entry exists so today's design does not wall off a plausible tomorrow when keeping the
door open is free, and so agents stop re-proposing what has been considered.

Confidence: **likely** (a commitment or sponsor statement exists) · **plausible** (fits
the vision, no commitment) · **speculative** (must not influence design; listed only to
stop re-proposal). Horizon: **next** · **later** · **someday**. Only likely and plausible
futures may justify a free option today. The repo's own sequence is
`docs/design/product-vision.md` §4 (H1 → H2 → H3) and `docs/vision/08-roadmap.md`.

## Index

Every future, in the order it was recorded, and the member under `futures/` that holds it byte-intact.
`fathom intent audit` (U15) names an id a member defines that this table does not list, one it lists that
no member defines, and one defined twice.

| Id | Future | Member |
|---|---|---|
| F2 | Public release (H2): public repos, tagged binaries, brew tap, MCP registry, plugin bundle | `futures/f01-f10.md` |
| F3 | Federation over portable stores (H3) | `futures/f01-f10.md` |
| F4 | Layer 2: the failure-mode playbook (incident precedent) | `futures/f01-f10.md` |
| F5 | Provider pack type (TX-3): promoted 2026-10-09 | `futures/f01-f10.md` |
| F6 | Concern-pack library growth (TX-4): review packs, compliance packs | `futures/f01-f10.md` |
| F7 | The `ai-derived` evidence tier | `futures/f01-f10.md` |
| F8 | Hosted read-only expert host (law 9) | `futures/f01-f10.md` |
| F9 | Machine-evaluated plan triggers | `futures/f01-f10.md` |
| F10 | CI fails closed (`--strict` with its own store) | `futures/f01-f10.md` |
| F11 | Adopt the kit's `mcp-server` under `internal/mcp` | `futures/f11-f22.md` |
| F12 | An LLM-judged evaluation lane on `qa-harness` | `futures/f11-f22.md` |
| F13 | Vectors as an additive in-store column | `futures/f11-f22.md` |
| F14 | CI/forge integration as its own repo | `futures/f11-f22.md` |
| F15 | A second user or second machine | `futures/f11-f22.md` |
| F17 | A saved-query discovery router | `futures/f11-f22.md` |
| F18 | Cross-repo intent federation (one corpus over many repos) | `futures/f11-f22.md` |
| F19 | Anchored-memory export into harness memory formats | `futures/f11-f22.md` |
| F21 | A `typesafe` adapter for the decision port (TypeSafe AI's Jev) | `futures/f11-f22.md` |
| F20 | Spec-scaffold interop: scaffold task checklists as plan rows and a converge report | `futures/f11-f22.md` |
| F22 | A cross-language port ledger | `futures/f11-f22.md` |
| F23 | Decision-port roles in refactoring: frontier ordering and same-concept questions | `futures/f23-f32.md` |
| F24 | Architectural decisions compiled into refutable dependency rules | `futures/f23-f32.md` |
| F25 | Data, schema and infrastructure refactors | `futures/f23-f32.md` |
| F26 | Security, performance and concurrency conservation | `futures/f23-f32.md` |
| F27 | A shared index cache keyed by commit (reframed 2026-10-09, K5) | `futures/f23-f32.md` |
| F28 | Answer events for background agents | `futures/f23-f32.md` |
| F29 | A Rust-compiled wasm grammar or query module under wazero | `futures/f23-f32.md` |
| F30 | An onboarding path for a newcomer | `futures/f23-f32.md` |
| F31 | Capability reach (network, filesystem, exec) and capability drift on upgrade | `futures/f23-f32.md` |
| F32 | Review Kit merges into fathom | `futures/f23-f32.md` |
| F33 | Review Kit moves into its own repository beside fathom | `futures/f33-f44.md` |
| F34 | `fathom ask`: judged predicates over spans the graph selects | `futures/f33-f44.md` |
| F35 | Tier 2 of the test-run hook: a failure-kind judgment | `futures/f33-f44.md` |
| F36 | Question-directed traversal (`localize kind: question`) | `futures/f33-f44.md` |
| F37 | Files at scale: judged semantic columns (level 9) | `futures/f33-f44.md` |
| F38 | When-to-compact advice | `futures/f33-f44.md` |
| F39 | Fitted recalibration and site auto-disable | `futures/f33-f44.md` |
| F40 | The judged-telemetry sampler, queue, background judge, judgments table and durable memo | `futures/f33-f44.md` |
| F41 | Guard proposals from rule outcomes | `futures/f33-f44.md` |
| F42 | MCP exposure of test-run or ask output | `futures/f33-f44.md` |
| F43 | A `--plan-only` export for other harnesses | `futures/f33-f44.md` |
| F44 | Set algebra over exploration maps | `futures/f33-f44.md` |
| F45 | ADR-to-rule pair judgments | `futures/f45-f53.md` |
| F46 | Doc-drift triage | `futures/f45-f53.md` |
| F47 | An injection screen for served third-party text | `futures/f45-f53.md` |
| F48 | Skill suggestion at the prompt | `futures/f45-f53.md` |
| F49 | Friction episode classification and product feedback | `futures/f45-f53.md` |
| F50 | Durable live maps: saved queries in the layers' vocabulary | `futures/f45-f53.md` |
| F51 | Model proposals for feature-map verdicts | `futures/f45-f53.md` |
| F52 | Review Kit concern packs declare their scope as live-map queries | `futures/f45-f53.md` |
| F53 | `propose_check`: a typed choice over the closed check kinds for a prose rule | `futures/f45-f53.md` |
| F54 | Orchestrator constraints on a sub-agent | `futures/f54-f61.md` |
| F55 | Fathom as the code engine under an agent-memory tool | `futures/f54-f61.md` |
| F56 | Incident maps: pre-shaped incident declarations and the suspect list | `futures/f54-f61.md` |
| F57 | Incident probes and MCP next queries: command templates rendered, never run | `futures/f54-f61.md` |
| F58 | Upstream drift of vendored and pinned dependencies, restricted to what the product reaches | `futures/f54-f61.md` |
| F59 | Promises as code: SLOs, alerts and dashboards linked to the code that keeps them | `futures/f54-f61.md` |
| F60 | A per-commit "already applied" landed test for `plans lines` | `futures/f54-f61.md` |
| F61 | Runtime topology graph: typed cross-artifact edges from proxies, IaC, policy and containers | `futures/f54-f61.md` |
| F62 | Tech pack verbs: commands a kit adds and what they prove | `futures/f62-f67.md` |
| F63 | Framework edges as data in a tech pack | `futures/f62-f67.md` |
| F64 | Pack-carried maps | `futures/f62-f67.md` |
| F65 | A generated repository map readable without fathom | `futures/f62-f67.md` |
| F66 | Telemetry name joins: metric and span names to their emitters (ADR-062 Stage B) | `futures/f62-f67.md` |
| F67 | A log-template index: a log line to the call that wrote it (TL-4) | `futures/f62-f67.md` |
| F68 | Reference maps: researched implementations linked to this project's specifications | `futures/f68-f73.md` |
| F69 | Judged sites offered by default at onboarding, or on whenever a provider is configured | `futures/f68-f73.md` |
| F70 | Ref overlays: a branch or pull request read as an overlay over its merge-base index snapshot | `futures/f68-f73.md` |
| F71 | Resilience facts in code: client construction with its literal options and library version | `futures/f68-f73.md` |
| F72 | Versioned cross-repository concern joins across index snapshots | `futures/f68-f73.md` |
| F73 | Per-extractor epoch salts | `futures/f68-f73.md` |
| F74 | Environment daemon: a read-only fathom serving one lineup | `futures/f74-f79.md` |
