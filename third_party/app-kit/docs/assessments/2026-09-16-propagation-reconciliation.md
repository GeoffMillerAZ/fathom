# G4 propagation reconciliation

Status: incomplete. The navigation improvement has already reached both selected
consumers; a new runtime upgrade is not needed merely to demonstrate a second app.
RAC's artifact drift is now reconciled. Fathom's authorized fixture repair and two-artifact
pin reconciliation now pass the scoped checks below. The original six-item G4 checklist
passes the retained-evidence audit; the owner-admitted theme-guide/core/JIT extensions
remain incomplete. G4 stays open for those requirements, not another navigation upgrade.

## Proven portions and limits

- [Navigation delivery](2026-09-14-navigation-delivery.md) binds kit `63b3a4f`, RAC's
  2,094-check browser result and Fathom's1,207-check result, with their exact
  renderer/CSS delivery and separate source-snapshot builds. Both consumers' current
  nav/shell bytes still match that repair. These are scoped working-tree integrations,
  not clean committed consumer releases or permanent component promotion.
- RAC's `docs/app-kit.md` now contains `RAC-NAV-01`: originating task, failure,
  shared ownership, returned disposition, delivery revision, both consumer receipts,
  maintainer/revisit action and explicit release limits. Initially only this record was added;
  the existing paired agent blocks already route to that contribution section.
- The [fresh host trial](2026-09-13-agent-host-routing.md) replays offline against its
  retained fixture. Both hosts read the same contract/router and selected sources;
  missing-artifact/incompatible-profile controls remain recorded. This does not invoke
  current models or prove current real-consumer host registration.
- The [corrected candidate](2026-09-16-current-candidate-rehearsal.md#corrected-index-exact-candidate)
  retains two matching cold no-sibling consumer builds and migration guidance. Its
  exact receipt still matches; later documentation is not part of that frozen tree.

## Initial check: two failures retained

The frozen reconciliation at `/Volumes/m2ext/appkit-g4-reconciliation.CBp2by` checks
selected artifacts, paired routing, historical receipt identities and the offline host
replay. It passed100/102 assertions and failed these manifest pins at that snapshot:

| Consumer / artifact | Manifest expects SHA-256 | Current SHA-256 |
|---|---|---|
| RAC `vendor/modules.txt` | `865beba044910c1735bc852ab06c6ef446462d1578f97d1fc0047e8f18568e5a` | `68d1ea0788016d762368c9589598c620b3d6e3ca6498fa237927aa6b59c3b359` |
| Fathom `scripts/ui-css.sh` | `7f8e4cfd1ef675b36d135c88fac45c6de50fb56d5c4458fc6b057fbc6b7888fa` | `162d8c36f138b55ef736d57ae3ad46ff384c3115a71e30293175321e4744bdbd` |

`result.json` SHA-256:
`05681521592197ec55d90753970767221c4f178ecb5c47c960263fbae2e670d7`.
The exact checker, replay script, selected inputs, before/after RAC contract and logs
are retained. Neither manifest was changed in that initial check. Matching historical receipt hashes are
identity checks, not new browser/build runs or authentication of historical observations.

Fathom's exact prior script exists in
`workspace/tmp/appkit-display-adoption.1Tmo5v/executed-source/scripts__ui-css.sh`.
The delta sources and invokes `scripts/lib/build-cache.sh`; CSS selection/staging and
publication code are unchanged. At that snapshot the new helper was not in the selected artifact list,
so updating only the script hash would omit an execution dependency. The paused work
must be preserved; the later review below does not change its implementation.

## RAC exact-baseline reconciliation

The exact manifest-pinned baseline was recovered at
`/Volumes/m2ext/rac-settings-sections.pi53Hs/candidate/modules.txt`, with the expected
`865beba0…` hash above. Its diff to current changes only agentic-kit's revision
`8b252304978b` → `6e3e8d71bd2f` and adds14 package paths. Committed consumer change
`e2c7e83330fef16a738a114d5498278f2caea95e` records that dependency advance; the app-kit
block is unchanged. A diff against current Git HEAD alone would have misattributed it.

The retained `/Volumes/m2ext/appkit-rac-shell-state.sX5bLw` run already built the exact
current `go.mod` and `vendor/modules.txt`: the source config, copied bytes and288-pass
browser receipt agree. This is linkage to the earlier scoped run, not new execution or
certification of agentic-kit. Only RAC's authored `vendor/modules.txt` artifact hash
and its delivery note changed now; no dependency, renderer, stylesheet or agent block.

Private review root: `/Volumes/m2ext/appkit-adoption-reconcile.HNknpb`.
The exact retained ADR-0065 checker (`004b35b7…249aaa`) reported RAC exit0; Fathom
still exited1 solely for `scripts/ui-css.sh` at that snapshot. In copied RAC fixtures, restoring the old
artifact and hiding the selected file each produce exit1 with the exact artifact path;
byte restoration returns exit0. All other selected requirements stay unchanged.
These checks bind the retained checker's selected metadata, not a newly built checker,
runtime host registration, effective module graph or release. A separate source review
finds both consumers' selected records unchanged in the current catalog; unrelated
analytics-preview and content-record changes do not alter those selections.

| Receipt under review root | SHA-256 |
|---|---|
| `rac-provenance/result.json` (31 identity/delta checks) | `7ad7dbd9cf0b41c9bc8f21bc04510dc8fb702fdbc3c7be6902ef5f1b92d2e0d6` |
| `adoption-before/result.json` (both real checks exit1 as expected) | `366281bf333d152f6043ab675c5a71d0a66ee75f8ab845b840930ddf880b2a31` |
| `adoption-after/result.json` (RAC pass/refusal/restoration; Fathom still fails) | `00e4c1c657c983acd7af181dc06c6fc188ea3b2a5c38206c1568599f1e207ed3` |

## Fathom cache dependency and CSS fixture

Fathom ADR-037/I39 governs the added helper: opt-in absolute, existing writable root,
derived cache locations, native environment preservation when unset, and advisory
Go-build-cache pruning. It does not promise a hard disk quota or validation of arbitrary
operator-controlled path aliases. Early static concerns about root aliases/lock
contention were not observed destructive behavior and are not treated as policy failures.

Frozen source plus fake Go/du and owned temporary roots pass48 scoped checks, including
the existing helper suite, all five preserved cache/temp variables, exact cleanup argv
and environment, unchanged synthetic sentinels, disabled cleanup, preheld-lock behavior,
and missing/relative/invalid configuration refusal. Invalid limits currently leave empty
derived directories; no side-effect-free refusal is claimed. Fake cleanup verifies
invocation, not real Go deletion, concurrency safety or arbitrary-path containment.

The original CSS regression fixture copied `ui-css.sh` but omitted its newly sourced helper.
A frozen original run exits1 with `scripts/lib/build-cache.sh: No such file or directory`,
before the first positive check. A private candidate adds only creation of `scripts/lib`
and copying the unchanged helper; all nine original CSS checks then pass using an offline
synthetic Go module and private caches. That private-candidate run left active Fathom source and manifest untouched.

| Receipt under review root | SHA-256 |
|---|---|
| `cache-review/result.json` (48 scoped checks) | `741a2a893ad5fcc91fbd5707b6904012c5718ecddf6dab115a2d253750015805` |
| `css-red-retry/result.json` (original fixture fails) | `d9f67a7dabd881cf1753c8619b5c963ec9840711d25db7667d8dd575de484f42` |
| `css-candidate/result.json` (private9/9 pass) | `adf9a12bd22be1ed7e1d39097f5c75e81b3f9cc950f41da1e04ba5f4edbb229a` |

Runners, exact copied inputs, environments, tool identities and outputs remain retained.
The first CSS runner's tool-path resolution failed before invoking a test; its unchanged
runner and `css-red/setup-failure.json` remain separate from the executed retry.

## Authorized active repair

The owner authorized the fixture, selected artifact pins and handoff only. The active
fixture now creates `scripts/lib` and copies the unchanged helper, matching the reviewed
private candidate. Its nine existing assertions are unchanged. The manifest updates
only `scripts/ui-css.sh` to `162d8c36…4bdbd` and adds `scripts/lib/build-cache.sh` at
`03c03345…19342`; the selected capability, module, mirror and other artifacts stay intact.

Retained packet in Fathom: `workspace/tmp/appkit-css-pin-repair.KvsMu3K7/`.
Each phase records exact source, runner, tool/environment identities and raw outputs.
The pinned checker remains `004b35b7dde5ebb3c31540f74a35ed60dd6f7fa8fdb2356aa09969077c249aaa`.

| Phase / `result.json` | Outcome | SHA-256 |
|---|---|---|
| `before` | Expected missing-helper fixture failure and stale-script pin refusal | `0ae6e2dba468eba39cf6e88fa2b8906281da65980d4b9c6abe19a2af0f47b772` |
| `after` | Unexpected synthetic source-read failure, retained | `3a9d6bac7a8abd944e470ea8303c2d4cbb40816e7af7892f3dde79e8fa6de085` |
| `after-trace` | Same source passes9/9 with shell tracing; artifact/refusal checks pass | `5c25f47b51891126213b8eb863f6d3e32994a97b802d48e51cb6f476f61f86bb` |
| `after-replay` | Ordinary no-trace replay passes9/9 and all scoped checks | `f8a7fcb10f8e9e5cde509c5b036225323b969945461ba5a1776e8c9b19a15d04` |

The first post-repair failure reports unreadable synthetic
`cmd/fathom/ui/../observatory_series.go`. Its cause is unknown; the unchanged traced
and ordinary passes do not explain it or erase that failure.

The final ordinary run separately syntax-checks fixture, builder and helper, passes
the active checkout's read-only source contract, and passes22 adoption checks with one
not applicable. A copied control passes; modifying or hiding its selected helper each
causes exactly the helper-path refusal, and both restorations pass. The runner confirms
only the authorized manifest delta and unchanged selected sources throughout the run.
Fresh per-phase private caches, an offline synthetic Go module and disabled pruning
keep real caches untouched. The CSS fixture uses a fake compiler output, not Tailwind.

Consumer `docs/app-kit.md` and `CHANGELOG.md` now hand off the repair. Existing
ADR-037/I39 and paired agent routing already govern it; no new decision/configuration
change is warranted. No production CSS/browser rerun, full Fathom gate, daemon restart,
commit, dependency change or deployment occurred. A subsequent read-only verification
matches all19 unchanged selected inputs; only the two expected handoff documents differ.
Both repositories pass `git diff --check`, and the kit MCP package tests pass.

## Acceptance audit: original checklist proven; extensions still open

The fixed original G4 checklist is supported by the following evidence. This audit
reads historical records and actual retained/current bytes; it does not rerun a host,
browser, consumer binary or compiler. It grants no release or whole-app certification.

| Original requirement | Evidence inspected | Boundary |
|---|---|---|
| Inventory identifies profiles/layers and revisions | Both current manifests, all selected artifact bytes, and exact shared nav/shell identities | Working-tree selection, not released application pins |
| Both agent harnesses resolve the same short contract | Existing host verifier replays the private Codex/Claude transcripts; every observation matches the public receipt and both retained executable hashes match | Historical actual synthetic sessions, not current-host or fleet registration |
| Missing selected artifacts cause failure | Fathom's exact ordinary repair receipt, changed/missing-helper controls and restorations; host missing-shell/profile controls | Explicit selected checks, not installed fleet automation |
| Clean consumer builds without sibling checkout | Exact candidate ZIP, both delivered1197-file module trees, both binaries/CSS, composite fixture digest and recorded isolated command coverage | Empty-cache source-snapshot rehearsal, not clean Git release |
| Improvement reaches a second consumer with reviewed feedback | Exact2094/1207 browser receipts, all tested HTML/CSS inputs, harnesses and current/frozen nav/shell bytes; `RAC-NAV-01` returned disposition/owner/revisit | Experimental RAC→kit→Fathom propagation, not permanent API promotion |
| Candidate and migration notes prepared; publication authorized separately | Content-identified candidate and maintained migration checklist, with no publish/tag | Private preparation only |

Audit packet: `/Volumes/m2ext/appkit-g4-acceptance.yrXRQX/`. The final audit records2536
file observations (including repeats), not2536 behavioral tests. It checks identity and
recorded coverage; hashes do not authenticate observations. The two failed audit scripts
and outputs remain separate and immutable:

| Receipt | Outcome | SHA-256 |
|---|---|---|
| `result.json` | Audit wrongly expected verifier output to contain the public receipt's supplemental build metadata | `51e3796e3e015e6a1868c924520ca00eca6874be74f822d444edbb01529bd023` |
| `replay-02/result.json` | Audit wrongly treated the three-file fixture digest as the main.go digest | `1c42c47255bdbedd031dce3735c2f1a670f488e17750c9dc72a666c5e6586320` |
| `replay-03/result.json` | Observation fields replay exactly, binaries checked separately, composite fixture digest independently reconstructed; all five grouped checks pass | `16f771c2ca50d4831d45d011812083b72a644b184acba7e5ce29332bc6ff71ef` |

No product, receipt, original verifier or acceptance threshold changed for those audit
repairs. Unrecorded supplemental build metadata is not upgraded to replay-proven data.
The decoded handler output is recorded by hash in the historical rehearsal, not retained
as a separate body; this audit verifies its recorded identity and fixture/binary coverage,
not a fresh handler execution. Intermittent workspace `EPERM` attempts remain failed
diagnostics; later successful reads do not establish their cause.

### Why the complete G4 goal is not retired

Scope's identity-workbench paragraph and ADR-0069 explicitly extend G1/G4 beyond that
original checklist. Navigation propagation cannot certify the newer guide or all starter
areas. The current CUE work inventory remains authoritative and is not promoted here:

- `core.themes-style-guide` still records partial delivery/integration, no adopted
  consumer row, and a required versioned second-consumer guide upgrade/returned improvement.
  RAC's existing working-tree guide and Gallery demonstrations are useful evidence,
  not that missing versioned propagation proof.
- The five starter areas still require their declared standards/gates and a tested
  just-in-time contribution path. A navigation handoff without the readiness work-item,
  applicable gate decisions and reviewed consumer receipt does not prove that whole path.
- Remaining visual, interaction, security, performance, lifecycle and consumer conditions
  are listed in `internal/qualityplan/inventory.cue`; none becomes passed through this audit.

Next: bind one selected real-consumer guide/JIT task to its exact readiness record and
applicable acceptance contract, reuse existing proof where identities match, then close
its missing delivery/consumer checks under scoped authority. Do not rerun navigation or
all long-tail conversions merely to increase check counts. Release and G6 remain separate.

### Documentation and final test verification

The scoped intent-doctor subset passes15 checks, including144-line scope, router and
Unreleased budgets, paired intent stanzas and184 relative file links (not anchors).
`docs-result.json` SHA-256:
`e76c13da45afdfd3c9f7a0489630b305fe061f3e5e42754fa578591a81afcab8`.
It does not check all ADRs, incidents, skill resolution, host versions or nested wiring.

The MCP package first passed after the repair notes, then a later post-audit run failed
on workspace-read `EPERM` errors. That failed tool-log diagnostic is not a passing test
or a diagnosed product regression. One unchanged logged retry passes, with618 selected
command-package/document/pack inputs retained and unchanged during execution; this is
not the entire compiler/dependency closure. Raw outputs and execution envelope live
beside `mcp-retry.json`, SHA-256
`47916d24e9afd65bbdc694687eed915e7b0e41485a373315ca21b1a0a554a32b`.
This verification paragraph postdates those frozen records. No oracle or implementation
was changed to obtain the retry's pass, and the operating-system denial's cause remains unknown.
