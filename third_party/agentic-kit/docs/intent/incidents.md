# Incidents

Every incident yields an invariant, a guard, a lint check, a test, or a decision. Open
records are ≤ 10 lines; closed ones compress to one line. The kit's generic anti-patterns
are the ledger in `docs/practices.md`; only specific, dated failures go here.

Classes: gate-bypassed · wrong-done-claim · intent-violated · platform-fact ·
contract-broken · other. Record with `intent-incident`.

## Open

### INC-7 2026-09-02 an unanchored `.gitignore` pattern hid every file under `cmd/kit-new/`
What: unanchored `kit-new` ignored `cmd/kit-new/`, hiding new sources from git status.
Evidence: `git check-ignore -v cmd/kit-new/package.go`; anchored fix in 86be7e6.
Would have caught it: a tracked-path/ignore-rule collision check; none exists.
Class: platform-fact
Outcome: pattern repaired; tracked files were retained. A generic constitution check
remains proposed, not built: gate edits need the owner's call under autonomy policy.

## Closed

- INC-26 (2026-10-24) Pre-push fixtures assumed canonical temporary paths and one direct Git subprocess inherited caller routing. Controlled reproduction wrote a foreign unmerged test entry into the real index; the index was backed up and only that absent-from-HEAD entry removed, without changing HEAD or working files. Canonical fixture roots, shared Git command isolation, `TestToyTempAlias` and foreign-index conservation inside `TestSnapshotBoundsSymlinkAndUnmergedIndex` now pass red/green race and full normal-root gates. [Retained evidence](../evidence/prose-headless/2026-10-24-integration/README.md). No production control or hook was relaxed. Class: contract-broken.

- INC-25 (2026-10-24) HMT native/store factories and HSE child preparation wrapped nil pointers in nonnil interfaces on errors. The actual invalid-native-pin CLI printed a panic, and `TestHookDefaultFactoriesDoNotReturnTypedNil` failed all three factories. Factories now return explicit nil interfaces on failure; red/green regressions and native negative retest preserve fixed unavailable diagnoses without cleanup calls on nonexistent resources. [HSE review](../evidence/command-tools/2026-10-24-spawn/security-review.md). No authority grant or source disclosure observed. Class: contract-broken.

- INC-24 (2026-10-24) Plausible clean private-runner results with started=false passed version/input completion, counting one completed but zero invoked calls. `TestScannerUnstartedSuccessRejected` reproduced both failures; explicit start witnesses now hold, preserve deadline diagnoses and keep actual counters. Race/full gates, native pinned-scanner trial and strict source proof pass. [OR evidence](../evidence/command-tools/2026-10-24-reuse/README.md). Production Start errors already held; no repository-controlled bypass or secret disclosure demonstrated. Class: contract-broken.

- INC-23 (2026-10-24) Linux CI removed a replacement during scaffold rollback after comparing closed-file metadata. Rooted handles now stay live through cleanup; mode/actual-byte checks preserve detected edits and unknown ownership. Original replacement regression plus new failure cases pass in native Linux run 40093730634 at `c7633a4`, both Mac lanes and full gates. [Evidence](../evidence/command-tools/2026-10-24-rollback/README.md). No atomic namespace or native Windows guarantee. Class: contract-broken.

- INC-22 (2026-10-24) Local CUE 0.16.1 passed schemas that CI 0.17.1 rejected. Numeric byte constraints and explicit control unions preserve floors; concrete ASCII/UTF-8 bounds and strict Go/CUE regressions pass both versions. Native Linux run 40093730634 passed CUE 0.17.1, race and WASI/WASM. [Evidence](../evidence/command-tools/2026-10-24-cue/README.md). Class: contract-broken.

- INC-21 (2026-10-24) Valid file inspection with a one-byte accepted cap emitted a 325-byte JSON or 382-byte text failure at `07e28b0`. Cap reset plus unchecked final emit caused the overrun. Effective cap now survives failures and includes newline; `TestInspectFailureOutputCaps`, `TestInspectFailureExactOutputBoundary` and success/writer regressions plus independent native json/text/compact probes pass. [Evidence](../evidence/command-tools/2026-10-24-compact/README.md). Class: contract-broken.

- INC-20 (2026-10-23) Exploratory command discovery exceeded its metadata envelope; unmeasured discovery and open aggregate vocabulary invalidate bounded-audit claims. [Assessment](../assessments/2026-10-23-command-governance.md) retains the failure. 0035 ships explicit-source intake with no discovery; `TestAuditExactByteCaps`, `TestAuditCoverageBoundsPrivacy` and wrapper/native malformed cases prove byte caps, fixed output and partial coverage. Audit skill forbids private-history discovery. This closes the reference remedy, not a global agent-read boundary. Class: intent-violated.

- INC-19 (2026-10-21) Fathom's rewritten dependency mirror made the sibling go.work check miss the candidate kit, while raw upstream mirror tests exercised unsupported scope. Owner authorized repair: `scripts/fathom-check.sh` clones committed inputs, syncs/verifies the candidate mirror and runs consumer-owned gates. Lifecycle regressions check isolation, ignored-data exclusion, failure propagation and cleanup. Class: contract-broken.

### INC-18 2026-10-21 checked plan failed after its unclaimed version passed
What: CK-5 was checked after an unclaimed-plan pass; strict verification then rejected its new library waiver, but a non-fail-fast shell sequence still committed b069ab5 on the topic branch. Main was not merged.
Evidence: strict output named CK-5 library-only; the command chain continued after exit 1. The row was reopened immediately.
Would have caught it: reverify the checked plan and stop the commit chain on a nonzero exit.
Class: wrong-done-claim
Outcome: router sensitive-surface evidence requirement for claimed plans and fail-fast chains; CLI prepare -identity now wires DecisionCacheKey with TestPrepareCommandBindsCurrentDefinition. No verification waiver remains.

### INC-17 2026-09-21 clean-worktree claim contradicted compiled VCS provenance
What: RAC's study notes called an isolated-worktree executable clean; retained metadata said `vcs.modified=true`.
Evidence: historical binary `177777b5…`; Go 1.26.2 Git trace inspected the outer repo, not the clean nested worktree. Independent checkout binary `344099e7…` records the intended revision and `vcs.modified=false`.
Would have caught it: inspect the actual compiled VCS metadata before making the clean-source claim.
Class: platform-fact
Outcome: platform fact in `domain.md`; corrected historical study and required build-stamp check in the 0014 consumer follow-up protocol. Original bytes/metadata are preserved.

- INC-16 (2026-09-16) README advertised hook bypass despite autonomy's prohibition → README now points to the binding autonomy policy; no gate was bypassed or weakened. Evidence: prior README Develop instructions versus `autonomy.md` Never. Class: contract-broken.

### INC-15 2026-09-16 registered fitness checks were not snapshot copies
What: mutating input/Get/List check slices changed a registry that documented independent copies and safe constructed reads.
Evidence: `TestRegistry_ChecksAreSnapshotCopies` failed before repair: original check name became `caller-mutated`.
Would have caught it: nested-slice mutation and concurrent returned-copy tests, not just top-level slice copying.
Class: contract-broken
Outcome: clone checks on registration/read; snapshot and race regressions now cover all three paths.

### INC-14 2026-09-16 evaluator did not bind the complete supplied output set
What: Run silently ignored extra outputs, overwrote duplicates, and accepted prompt/parameter hashes different from the configured producer.
Evidence: review of `Runner.Run`; `TestRun_InvalidOutputBindingRefusedBeforeSideEffects` reproduces each path.
Would have caught it: exact scenario coverage and complete producer-identity checks before probes, grading or journal writes.
Class: contract-broken
Outcome: exact output-set validation and the named regression table; all invalid cases require zero external calls and zero journal writes.

- INC-13 (2026-09-14) JSON decoding replaced unpaired Unicode surrogates with U+FFFD, letting malformed model fact IDs match real evidence and altering CLI principals/lesson labels → private `jsonwire.ValidUnicode` before all three strict decoders; public-entry-point negatives fail when only this guard is bypassed, while valid pairs/backslashes remain accepted. Class: contract-broken.

- INC-12 (2026-09-13) Strict prose checked text but JSON copying normalized malformed principal/provenance bytes before authorization; red tests reached provider, model and publication callbacks with a replacement identity → pre-copy/hash UTF-8 checks and `TestSourcedPrincipalCannotChangeBeforeAuthorization`, with nested metadata and valid-Unicode regressions. Class: contract-broken.

- INC-6 (2026-08-25, closed 2026-09-13) Unreleased changes crossed a tag and expanded into narrative → retroactive roll (64db8a4), release ritual in `project-intent/changes.md`, and a ≤60-line delta; released history retained. Class: gate-bypassed.

- INC-11 (2026-09-13) Native-host review found symlink/.. normalization bypass, writable guard executables and umask permission drift → raw-parent rejection, executable identity protection, explicit mode restoration and independent `TestHostGuardRejectsRawParentTraversal` / `TestHostGuardProtectsExecutableAndHardlinks` / `TestHostInstallPreservesModeUnderRestrictiveUmask` regressions. Class: contract-broken.

- INC-10 (2026-09-13) Sourced prose's CUE schema rejected valid Go JSON (inactive author pointers, derived zero locator, fully redacted map); uninstantiated definitions had passed `cue vet` → canonical JSON projection plus `TestSourcedMarshaledValuesMatchCUE`, including derived facts and actual redacted exports. Class: contract-broken.

- INC-9 (2026-09-13) Both token vaults authorized caller-supplied class rather than stored identity; key copying also raced shredding. Reproducer failed before repair → `TestVaultStoredIdentity`, stored metadata authorization, authenticated encrypted records, and key-copy-under-lock race tests. Class: contract-broken.

- INC-8 (2026-09-12, closed 2026-09-13) A green gate missed prose clock and upward-import violations → `NewPipelineWithClock`, shared telemetry constants with compatible harness aliases, `TestRegenerateUsesInjectedClock`, and `TestFoundationsImportDownward`; I12/I13. Class: intent-violated.
- INC-1 (pre-2026-07) A hook channel died silently for four days while its metric read
  zero → `heartbeat.CheckExistence`/`ExistenceExpectation`/`ExistenceAlarm`; the
  telemetry guard in `fathom-pack/guard-pack.cue` ("every measurement channel has an
  existence check"); I19. Class: platform-fact.
- INC-2 2026-07-16 `bedrock/awscred` let explicit credentials fall through to ambient
  `~/.aws` state and partial pairs sign as the default profile → typed
  `*PartialCredentialsError`, explicit pairs short-circuit, `credential_process` timeout
  and stderr cap ("hardened same-day after critique"). Class: contract-broken.
- INC-3 2026-07 `docs/first-agent.md` claimed `turns=2`; the CLI produced 3 →
  `TestFirstAgentWalkthrough` golden-pins the doc's mock-mode output. Class:
  wrong-done-claim.
- INC-4 2026-07 `WithCost` emitted zero-cost events silently; `MockClient` returned
  zero usage → conformance suite catches both; mock defaults to 1 token. Class:
  contract-broken.
- INC-5 2026-07 H8 Stage-A security self-review found the docs overclaiming a
  `CallTool` pre-call pin check → docs corrected in the same change; TRACKING row H8-1
  closed by opt-in `CallToolVerified` with drift/refusal tests (GW-05); remote metadata
  and call are still not atomic. Class: wrong-done-claim.
