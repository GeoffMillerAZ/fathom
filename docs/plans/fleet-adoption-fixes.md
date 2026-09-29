# Fleet adoption fixes — three real bugs found by dogfooding the fleet (2026-07-30)

Owner-facing: the owner is wiring fathom into every repo he touches right
now, so these three bugs bite every repo, not just this one.

## Bug 1 (HIGH) — `fathom init` duplicated user-scope hooks

Reproduced live: the owner's `~/.claude/settings.json` wires (globally)
`PostToolUse Edit|Write|MultiEdit → fathom edit-hook`, `SessionStart →
fathom session-hook`, `Stop → fathom stop-hook`. Running `fathom init` in a
repo ALSO wrote those same three into the repo's `.claude/settings.json`
(project scope). Claude Code applies both scopes, so the post-hook fired
twice per edit (4,596 logged calls in the reproduction). This repo's own
`.claude/settings.json` was hand-fixed in commit `e2bb43d` before this plan
started, but every OTHER repo the owner inits would still hit it — the bug
lived in `cmd/fathom/init.go`, not in any one repo's settings file.

## Bug 2 (MEDIUM) — `hooks_installed` conflated hook variants, wrong remedy

Live output when the two phase-O `PreToolUse` guard groups were genuinely
missing: doctor reported them as `edit-hook --pre, edit-hook --pre`
(duplicated, not naming the two matcher groups), inferred "wired somewhere
I can't see" from call-log traffic that actually belonged to the DIFFERENT
`PostToolUse` edit-hook (both variants share one CLI and one call-log
`origin`, so a post-channel call can never be evidence for a `--pre`
variant specifically), and told the owner not to run `fathom init` — when
running it was exactly the fix.

## Bug 3 (MEDIUM) — three verb-JSON gaps the phase-K studio hit

`fathom-studio` (law 10: it may only consume verb JSON, never reach into
fathom's internals) correctly refused to work around three gaps instead of
reaching past them: (a) no manifest summary in any verb JSON — a consumer
could not show a module's description/license/tier_profile/provenance; (b)
`module doctor`'s structured facts (precision/recall, compiled_by/at,
license, compile lane) existed only as free-text prose inside
`Criterion.Evidence`; (c) no compile-run history verb — only the
just-triggered run was visible.

## Bug 4 (HIGH, 2026-09-29) — a row sharing a matcher group read as wired (O175)

TR-2 put a second fathom row in the PostToolUse(Bash) group beside edit-hook.
Hook presence was keyed by (event, matcher) alone, so edit-hook's entry stood
for the test-run row: doctor's hooks_installed read PASS, reconcile's dry-run
named nothing missing and its apply left the row out with a "not fathom's
template spelling" note, and a piped failing test run (exit 0) reached no
test-run hook in this repository.

## Tasks

Note (house convention, verify's markdown parser is line-based): each task's
description and `{id: …}` annotation live on ONE physical line; the nested
sub-bullet beneath each row names the test that proves it, matching
`docs/plans/expert-suite-o.md`'s own convention.

- [x] `cmd/fathom/init.go`: `mergeHookSettings` reads Claude Code's USER-scope settings (`~/.claude/settings.json`, the scope `checkHooksInstalled` already reads) via `userScopeHookKeys` and SKIPS writing any fathom hook spec already wired there, printing an explicit `skip …` line per skip (`wireHooks`) — never silent; a hook wired under a DIFFERENT command is not counted as wired (reuses `hookMarkerPresent`'s marker check), so fathom's own still installs {id: FA-1, symbol: initWiringSpecFor, package: cmd/fathom}
  - Proven by `TestMergeHookSettings_SkipsUserScopeWiring` (user scope wires the pre-phase-O three → project gets only the two PreToolUse guard groups; user scope empty → project gets everything; user scope wires a different command on the same event/matcher → fathom's own still installs; re-running after a skip stays idempotent) and `TestMergeHookSettings_PreservesExisting`'s existing coverage (unaffected, now HOME-isolated).

- [x] `cmd/fathom/doctor.go`: `hookMarkerPresent` is scoped to a spec's OWN (event, matcher) group, mirroring `ensureHookEntry`'s write-path scoping, so the two `PreToolUse` matcher groups (`Edit|Write|MultiEdit` and `Bash`, sharing one marker) are tracked independently instead of a marker in either group reading as both being wired {id: FA-2, symbol: hookMarkerPresent, package: cmd/fathom}
  - Proven by `TestCheckHooksInstalled`'s `the two PreToolUse matcher groups are evidenced independently` subtest, which seeds beat-log evidence for only ONE of the two groups and asserts the reason names both matcher groups distinctly.

- [x] `cmd/fathom/doctor.go`: `checkHooksInstalled`'s contradiction probe (`hookVariantSeenEvidence`) sources evidence PER MISSING HOOK VARIANT — `editHookBeatCount` reads the hooks beat log's `phase`/`target` meta (the only signal distinguishing edit-hook's PostToolUse invocation from its two PreToolUse matcher groups, which all share one CLI and call-log origin); `originCallCount` covers session-hook/stop-hook, unambiguous in the call log already; a missing hook with no evidence anywhere now carries the `fathom init` remedy (safe post-FA-1), one WITH evidence keeps the do-not-duplicate warning, and a MIXED set reports both correctly {id: FA-3, symbol: checkHooksInstalled, package: cmd/fathom}
  - Proven by `TestCheckHooksInstalled`'s `post-channel call-log traffic is never mistaken for --pre evidence` subtest (the exact live-repro shape: pre-phase-O three wired, both PreToolUse groups genuinely missing, heavy post-channel call-log traffic present — asserts the init remedy, not the duplicate-avoidance warning) and `beat-log pre-phase evidence still blocks the duplicate-installing remedy` (the mirror case, now sourced from the beat log).

- [x] `cmd/fathom/module_doctor.go`: `DoctorReport` gains `Manifest *ManifestSummary` (name, version, description, license, tier_profile, provenance.compiled_by/at, corpus selector count, watermark), populated by `runDoctor` once the manifest loads, nil when it doesn't — `module doctor --json` is the one place this lives (doctor already loads the manifest on every call, so this is a zero-extra-read addition over a separate `module show --json`) {id: FA-4, symbol: runDoctor, package: cmd/fathom}
  - Proven by `TestModuleDoctor_LadderProgression`'s new assertions on `report.Manifest` (name/license/corpus selector count/watermark) at the draft stage.

- [x] `cmd/fathom/module_doctor.go`: `Criterion` gains an optional `Facts map[string]string` field — typed values alongside `Evidence`'s free-text prose, additive only, populated for eval-passed (precision/recall/lane/prompt_version), servable's watermark-fresh criterion (manifest_watermark), and published's license/provenance/compile-lane criteria (license, compiled_by/at, lane, model) {id: FA-5, symbol: runDoctor, package: cmd/fathom}
  - Proven by `TestModuleDoctor_LadderProgression`'s new assertions on `evalCriterion.Facts`, the watermark criterion's `Facts["manifest_watermark"]`, and `laneCriterion.Facts["lane"]`/`["model"]` at the eval-passed and published stages of the same ladder-progression fixture.

- [x] `cmd/fathom/module_runs.go` (new): `fathom module runs --manifest --store [--service] [--limit N] [--json]` — compile-run history for the module's layer (id/started_at/model/lane/watermark/claims_emitted/cost_usd, plus dry_run/refused/budget_stopped when true), newest-first, deterministic, reusing `Store.CompileRuns` exactly as `module_doctor.go`'s `loadStoreState` already does (already newest-first by `ran_at DESC, run_id` — no new store method), wired into `cmdModule`'s dispatch and `printModuleUsage` {id: FA-6, symbol: cmdModuleRuns, package: cmd/fathom}
  - Proven by `TestModuleRuns_ListsCompileHistory` (empty-before-compile, one run after a `--mock` compile with the expected lane/model/watermark/claims, and `--limit 0` returning it unbounded).

- [x] `cmd/fathom/doctor.go`: hook presence is keyed per template row, the (event, matcher) group plus the row's own marker (`hookSpecKeyOf`), so an entry wired in a shared group stands only for its own row: the PostToolUse(Bash) group carries edit-hook (O57) and the test-run hook (TR-2), and edit-hook's entry no longer reads as the test-run row wired — doctor's hooks_installed fails naming it, `fleet reconcile`'s dry-run names it missing and `--apply` adds it to that group, and `fathom init` plans it (O175) {id: FA-7, symbol: hookSpecKeyOf, package: cmd/fathom, reachable_from: TestHookRowSharingAMatcherGroupIsCheckedByItsOwnMarker}
  - Proven by `TestHookRowSharingAMatcherGroupIsCheckedByItsOwnMarker`: a temp HOME and a repo whose settings wire every row but the test-run one (edit-hook alone under PostToolUse(Bash)), at project scope and with edit-hook served from user scope.

## Deferred (none)

Every deliverable in the mission brief above is implemented and proven in
this plan — nothing was cut for scope.

## Docs touched

- `docs/adoption/claude-code.md` §7 — documents `fathom init`'s new
  user-scope skip behavior (a "User-scope dedup" note right after the hook
  stack's intro).
- `docs/use-cases/getting-started.md` and `docs/use-cases/platform-modules.md`
  — checked; neither currently documents the `fathom module` verb family's
  CLI shapes (that family has no living reference doc outside the closed
  `docs/plans/expert-suite-j.md` plan and the high-level
  `docs/vision/13-expert-suite.md` roadmap row) — nothing to update there
  without inventing a doc scope this task didn't ask for. `module doctor
  --json`'s new `manifest`/`facts` fields and `module runs --json`'s shape
  are documented at the source instead: `ManifestSummary`/`Criterion.Facts`
  (`cmd/fathom/module_doctor.go`) and `RunsReport`/`CompileRunSummary`
  (`cmd/fathom/module_runs.go`) each carry a doc comment explaining the
  field and why it exists; `printModuleUsage` (`cmd/fathom/module.go`)
  documents the new `runs` verb's flags.
- `docs/mcp-tools.md` — unaffected: these are CLI verb changes, not MCP
  tool responses (no MCP tool's wire shape changed).

## Verify

    bin/fathom verify --service fathom --plan docs/plans/fleet-adoption-fixes.md --strict
