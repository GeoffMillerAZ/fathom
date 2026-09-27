# impact-observatory-activation — visions 14/15/16 promoted to code

> Governing sources: docs/vision/14-impact-observatory.md, 15-reach.md,
> 16-skills-pack.md (non-binding visions — THIS plan is their binding
> promotion, row by row) and the filed observations O1–O9 (.fathom/
> observations.md — each a bug spec with a repro). Owner directive
> 2026-08-07: "implement everything until fully implemented."
>
> **Covenant (ADR-0070 class, applies here):** tests assert behavior,
> never a stub's own constants; completion claims exist only as verified
> boxes; a diff weakening a gate file is its own justified commit.
> Discipline: TDD, `task validate` green before every commit, dated
> CHANGELOG entry per landed change.

## A — The correctness floor (the trust under every metric)

- [x] resolveFromServices is deterministic: sorted candidates, richer-on-tie, dedup never collapses the list to one (O1 fixes a–c) {id: IO-1, symbol: resolveFromServices, package: cmd/fathom, reachable_from: TestResolveFromServicesDeterministic}
- [x] Daemon-side ingest hygiene: ValidServiceID enforced in the handler, corpus roots canonicalized via EvalSymlinks, duplicate and nested roots refused naming the claimant (O1 fix d + fleet-audit anomalies 1–3) {id: IO-2, symbol: registerIngest, package: internal/mcp, reachable_from: TestIngestHygiene}
- [x] remove_service and rename_service ship over MCP (asked 2026-06-16; both registry surgeries this quarter were sqlite archaeology) {id: IO-3, symbol: registerRemoveService, package: internal/mcp, reachable_from: TestRemoveRenameService}
- [x] session-hook degraded branch: fetchServices returns an error; enumeration failure renders degraded guidance, never a false not-indexed instruction (O2) {id: IO-4, symbol: fetchServices, package: cmd/fathom, reachable_from: TestSessionHookDegraded}
- [x] doctor stops lying about other repos: binary_fresh compares fathom's own source, hooks_firing reads the daemon's call-log path, lefthook_gates survives variable indirection (O5 + audit defect A) {id: IO-5, symbol: lefthookInvokesPattern, package: cmd/fathom, reachable_from: TestLefthookGates_VariableIndirection}
- [x] LintPlan is loud: quote-aware brace tokenizer, an unparseable annotation block is a named lint error never a silent md-NNN downgrade (O4) {id: IO-6, symbol: LintPlan, package: internal/verify, reachable_from: TestLintPlan_UnparseableAnnotationIsLoud}
- [x] init's own scaffolds load clean: three-state loader (absent = none configured, inert-commented = pass with note, malformed = fail), proven against the actual template bytes init writes (O7) {id: IO-7, symbol: LoadBindings, package: internal/bindspec, reachable_from: TestExplicitEmptyBindings}
- [x] compile --dry-run prints an output-token and cost RANGE plus expected duration, never a confident zero (O8) {id: IO-8, symbol: Compile, package: internal/semantic, reachable_from: TestDryRunEstimateRange}
- [x] the review verb never prompts in a non-TTY: pre-push completes or exits with a named verdict (O9) {id: IO-9, symbol: runReview, package: cmd/fathom, reachable_from: TestReviewNonInteractive}
- [x] the adoption gauge is service-scoped and cwd-independent: --service plus daemon-side call-log resolution; zero-call list scoped to the measured surface (O6 + audit) {id: IO-10, symbol: buildAdoptionReportFold, package: cmd/fathom, reachable_from: TestAdoptionServiceScoped}

## B — Version-stamped telemetry (vision 14 §3)

- [x] call-log entries carry schema_version + fathom_version {id: IO-11, symbol: NewCallLogger, package: internal/mcp, reachable_from: TestCallLogVersionStamp}
- [x] hook beats carry schema_version + fathom_version + hook template_version and a service field (audit: beats are user-global, unattributable) {id: IO-12, symbol: stampBeatMeta, package: cmd/fathom, reachable_from: TestBeatVersionStamp}
- [x] session ledger and daily rollups stamped; unstamped old rows read version unknown, never backfilled {id: IO-13, symbol: BuildDailyRollup, package: internal/metrics, reachable_from: TestRollupVersionStamp}

## C — Arms and attribution (vision 14 §2)

- [x] fathom arm off|on --sessions N stamps ledger rows with the arm; doctor announces an active arm loudly {id: IO-14, symbol: cmdArm, package: cmd/fathom, reachable_from: TestArmStamping}
- [x] ledger attribution fields land: verify_catches, discovery_substitutions, injected_rules_used {id: IO-15, symbol: recordSessionLedgerEntry, package: cmd/fathom, reachable_from: TestLedgerAttribution}

## D — The fleet fold (vision 14 §4)

- [x] fathom fleet report folds per-service report.json + user-central beats/ledger into one local artifact with per-service wiring/freshness/coverage/spend rows {id: IO-16, symbol: cmdFleetReport, package: cmd/fathom, reachable_from: TestFleetReport}

## E — Reconcile and self-heal (vision 14 §7)

- [x] fleet reconcile dry-run: per-repo, per-scope diff of installed hooks vs the current template, stale-binary paths, stanza staleness, third-party wiring lint {id: IO-17, symbol: analyzeWiring, package: cmd/fathom, reachable_from: TestReconcileDiff}
- [x] reconcile --apply applies the safe subset in place (shim repoint, additive matchers, dedupe) at exactly one scope, never both {id: IO-18, symbol: cmdFleetReconcile, package: cmd/fathom, reachable_from: TestReconcileApply}
- [x] the session-hook tripwire: wiring self-check in the session line names the exact remedy on skew {id: IO-19, symbol: buildSessionContext, package: cmd/fathom, reachable_from: TestSessionTripwire}
- [x] init installs the PATH shim (~/.local/bin/fathom → deployed libexec binary) and doctor checks resolvability from a NON-interactive shell {id: IO-20, symbol: installShim, package: cmd/fathom, reachable_from: TestInstallShim}

## F — Narrate v0 (vision 14 §6; story rows only, no LLM weekly yet)

- [x] narrate on --for D|--sessions N with stamped auto-expiry; off is default; doctor shows the active dial {id: IO-21, symbol: cmdNarrate, package: cmd/fathom, reachable_from: TestNarrateExpiry}
- [x] story rows: per-intervention JSONL (rule served + following tool calls; verify catch + claimed diff; discovery substitution) written only while active, version-stamped, never bundled {id: IO-22, symbol: recordStoryRow, package: cmd/fathom, reachable_from: TestStoryRows}

## G — The skills pack (vision 16)

- [x] fathom-rewire and fathom-activate join the embedded pack in both renderings, bodies sourced from docs/adoption/rewire-and-activate-prompts.md {id: IO-23, symbol: installSkills, package: cmd/fathom, reachable_from: TestSkillsAgentsRendering}
- [x] --skills=agents renders the pack spec-conformant to the shared .agents/skills path (Codex + Gemini discovery) {id: IO-24, symbol: resolveSkillsMode, package: cmd/fathom, reachable_from: TestResolveSkillsModeAgents}
- [x] fathom plugin render generates the Claude Code plugin dir (skills + hooks.json + .mcp.json) and marketplace.json from the embedded sources; never hand-maintained {id: IO-25, symbol: cmdPluginRender, package: cmd/fathom, reachable_from: TestPluginRender}
- [x] kit packs verified end-to-end: fathom layers install run against app-kit and agentic-kit fathom-pack dirs, guards demonstrated firing {id: IO-26, oracle: lexical, path: CHANGELOG.md, pattern: "IO-26 integration proof run live"}

## H — Reach (deferred with triggers, vision 15)

- [-] C++ language pack with the Conan dependency adapter, behind a calibration-corpus gate {id: IO-27, disposition: deferred, reason: "trigger FIRED 2026-08-08 by owner directive - superseded by docs/plans/cpp-toolchain.md (CT-1..CT-13), which implements the C++ tier with a CMake/clang/Ninja redirect; Conan stays deferred there as CT-13", trigger: "fired - see cpp-toolchain.md"}
- [-] the observatory web app (rung 2) {id: IO-28, disposition: deferred, reason: "trigger FIRED 2026-08-08 - superseded by docs/plans/observatory.md (OB-1..12) and impact-questions.md (IQ rows) which built the web surfaces once IO-16/IO-21 had data", trigger: "fired - see observatory.md"}
- [-] SCIP emission spike {id: IO-29, disposition: deferred, reason: "evaluate-not-commit per vision 15", trigger: "one free week after IO-25"}

## Exit — proofs, not demos

- [x] the adoption question answers from a consumer repo: instrument --adoption --service run from app-kit reports that repo honestly {id: IO-30, oracle: lexical, path: CHANGELOG.md, pattern: "IO-30: .*instrument --adoption --service"}
- [x] doctor produces zero known-false FAILs on a consumer repo {id: IO-31, oracle: lexical, path: CHANGELOG.md, pattern: "IO-31: doctor on app-kit ran 24 checks"}
- [x] the first counterfactual pilot is pre-registered: criteria + arms filed before the first OFF session {id: IO-32, oracle: lexical, path: CHANGELOG.md, pattern: "IO-32: the counterfactual pilot was pre-registered before any OFF session", skip_test: true, skip: "pre-registration artifact under workspace/artifacts/research with its generation timestamp; published range whatever it says"}
