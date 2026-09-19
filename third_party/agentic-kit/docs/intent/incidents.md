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
