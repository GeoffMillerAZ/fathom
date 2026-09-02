# Custodian — instructed means true (vision 22) and the doctrine plane (vision 23)

> Gate: `fathom verify --service fathom --plan docs/plans/custodian.md`
> Specs: `docs/vision/22-config-custodian.md` (+ §3b instrument-not-hand),
> `docs/vision/23-doctrine-plane.md`, mechanics in
> `docs/design/config-custodian.md`.

## Phase CC — config custodian v1 (mechanical, index-free, offline-safe)

- [x] CC-0 Vision 22 §3b + vision 23 + design doc landed and linked from the vision README {id: CC-0, oracle: lexical, path: docs/vision/README.md, pattern: "\[22-config-custodian\.md\].*the config custodian"}
- [x] CC-1 Concern model + CUE loader: three-state load, `?:` refusal, unknown-field refusal, unknown check KINDS refused by name with the accepted list {id: CC-1, symbol: LoadFile, package: internal/custodian}
- [x] CC-2 Law-13 scoping: project replaces same-ID user concern wholesale; `off: true` disables an inherited concern and the audit still names it {id: CC-2, symbol: Merge, package: internal/custodian}
- [x] CC-3 Audit engine over agent-config files (git-authority discovery, walk fallback) with per-concern holds/violated/unable and `file:line` evidence {id: CC-3, symbol: Audit, package: internal/custodian}
- [x] CC-4 The four v1 check kinds: forbidden_pattern, command_documented_and_exists, max_lines, pointers_resolve {id: CC-4, symbol: checkConcern, package: internal/custodian}
- [x] CC-5 `fathom config audit` verb — text + `--json`, exit 1 iff violated, zero concerns exits 0 with the absence named {id: CC-5, symbol: cmdConfigAudit, package: cmd/fathom}
- [x] CC-6 Doctor row `concerns` — red only on load error or violation; zero concerns is a passing row naming the absence {id: CC-6, symbol: checkConcerns, package: cmd/fathom}
- [x] CC-7 Session-brief attention line on startup/clear when violations exist, bytes on the payload-cost account {id: CC-7, symbol: concernAttentionLine, package: cmd/fathom}
- [x] CC-8 Flywheel custodian loop: CLOSED = audits running AND violations consumed (the custodian audit beat is the attended record); OPEN = accumulating unread; IDLE = no concerns {id: CC-8, symbol: checkLoopCustodian, package: cmd/fathom}
- [ ] CC-9 `fathom config propose` — the diff-offering verb (session applies; fathom never does) {id: CC-9}
- [x] CC-10 `fathom init` scaffolds a commented concerns.cue; fleet reconcile reports repos with zero concern coverage {id: CC-10, symbol: ExampleTemplate, package: internal/custodian}
- [ ] CC-11 Corpus registry seam: `where:` resolves named corpora (agent-config | lefthook | taskfile | ci) behind the one engine — never a per-tool custodian (vision 22 §7) {id: CC-11}
- [ ] CC-12 Deterministic-validation packs: curated concern sets shipped as data (claude-code, codex, lefthook, taskfile), repo-enabled, entry-overridable (law 13); chain checks for docs → lefthook → Taskfile → scripts {id: CC-12}
- [ ] CC-13 Doctor convergence (O27): bespoke doctor checks re-expressed via the concern engine where a pack fully covers them — doctor renders what the audit knows {id: CC-13}
- [x] CC-14 Stanza-current concern: every tool the CLAUDE.md/AGENTS.md fathom stanza names is served by the running daemon's registry — a stanza pointing at a pruned or undeployed tool is an audit finding (the operator contract's "instructed means true" applied to fathom's own instructions) {id: CC-14, symbol: Audit, package: internal/custodian, reachable_from: TestStanzaCurrentConcern}
- [x] CC-15 Completion-cites-instruments concern: a handoff thread note filed at session end that claims shipped work without a verify/retro citation (commit, plan row, or retro artifact) is an audit finding — the operator contract's rule, made mechanical {id: CC-15, symbol: Audit, package: internal/custodian, reachable_from: TestCompletionCitesInstrumentsConcern}

## Phase DP — the doctrine plane (vision 23; spec-first, no code before its design doc)

- [x] DP-1 Doctrine catalog design doc: one CUE catalog, entries declaring their enforcement surface(s) (obligation | concern | conformance); laws + runbooks as entries; ENGINEERING_PRACTICES.md as render/audited view, never parallel truth {id: DP-1, oracle: lexical, path: docs/design/doctrine-catalog.md, pattern: "enforcement surface"}
- [-] DP-2 ADR assertions: accepted ADRs may carry machine-checkable claims; `fathom adr audit` reports decided-but-violated with evidence; assertion-refusal rate measured (vision 23 §6 falsification) {id: DP-2, disposition: refused, reason: "superseded by EP-9 (evidence-plane.md): fathom intent audit over ADR revisit trailers IS the adr audit (ADR-026); a second verb would be the O4 drift; revive only for an ADR assertion the intent audit's oracle vocabulary cannot express"}
- [ ] DP-3 Vision promotion pipeline, computed: every vision names status/trigger or plan rows; links resolve; staleness dated; surfaced per vision {id: DP-3}
- [ ] DP-4 Goals as pre-registered bets only: instrument + threshold + review date or refused at load; existing bets (arm bench, thread bet) enumerated by the same surface {id: DP-4}

## Phase SD — the SDD surface (vision 23 §4; the UI earns its place)

- [x] SD-1 Verify ledger: every `fathom verify` run appends per-plan rows (at, repo, service, counts, commit) to ~/.fathom/verify-ledger.jsonl — user-central like the session ledger; read side tolerates absence (named, law 3); also the recorded-run signal that closed O30 {id: SD-1, symbol: appendVerifyLedger, package: cmd/fathom}
- [ ] SD-2 Observatory SDD panel, fleet + per-repo: computed at render via the plans-status engine; headline is the claimed-vs-proven gap; app-kit doctrine binding (search corpus first, role utilities, `task ui:css`, screenshot both themes before done) {id: SD-2}
- [ ] SD-3 Trend chart gated on ledger data — `stat.Absent` with reason until the ledger has ≥14 days of rows {id: SD-3}
- [ ] SD-4 Panel traffic counted (surface_traffic precedent) so vision 23 §6's remove-if-unvisited falsification is evaluable {id: SD-4}
