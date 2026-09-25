# Theme-guide memory v2: native instrument controls

## Prospective resume contract

- Packet: R3, qualifying the just-landed independent lifecycle join for the actual
  Gallery Synthwave workload. Inputs start at app-kit main `84cfdbb`.
- Deliverable: one tiny synthetic guide-shaped native-navigation control, then one
  separate deliberately retained8MiB backing-store control if the first qualifies.
  These are instrument controls, not a substitute Gallery workload or runtime fix.
- Reuse the unchanged guard, lifecycle collector/join, memory sampler/metrics and
  bounded owned-process helper. Native CDP input drives the fixture's actual form;
  DOM queries only observe it. No Playwright, page-script injection or sandbox bypass.
- Context identity comes from the retained `Target.createBrowserContext` response.
  The page has an empty-sandbox preview. A fixture-owned external script can retain
  and touch8MiB in its parent page; allocation does not disappear on iframe navigation.
- Freeze source, contract, Node/browser identities and Chrome flags before execution.
  One fresh private Chrome profile per control; loopback fixture, no consumer or
  signed-in browser, real data, dependency, global configuration, daemon or deployment.
- Acceptance: one warm native form action, baseline, twenty native form actions and
  final endpoint; five-second quiet collection, full raw process/heap/lifecycle data,
  independently derived relationships and numeric results. Positive growth must stay
  within the existing limits; retention must exceed the unchanged backing-store limit.
  Failed sampling, unknown ownership/containment or ambiguous identity stops the run.
- Bounds: one positive and one retention acquisition maximum; no automatic retries
  or full twelve-cell matrix. Each owner has a120s deadline and separately bounded
  cleanup. About45minutes implementation/verification; one economical fixture builder
  and one focused review. Preserve failed code/logs before any repair.
- Claims: structural and numeric controls cannot establish detached/native/GPU
  reclamation or full memory acceptance. Attribution stays unverified. Existing v1
  failures, v2 budgets, human A/B and G1/G3/G4 acceptance remain unchanged.
- Return: maintained synthetic fixture/owner, exact run evidence and next action in
  this assessment plus the owning memory-v2/resumption guidance. No readiness promotion.

## Result

**The one positive attempt failed during browser bootstrap, before the fixture was
loaded. Retention was not run.** The fresh Chrome153.0.8010.53 inventory contained
the owned `about:blank`, two explicitly admitted `browser_ui` targets and an
unadmitted `background_page` titled `Google Hangouts`, with URL
`chrome-extension://nkeimhogjdpnpccoofpliimaahmaaome/background.html`.
The owner refused it, rather than silently closing/excluding it or changing policy.

The retained raw inventory identifies that target and its context. It does not prove
why the extension loaded, what requests it made, or the origin of historical `other`
targets. No guard, application context, fixture navigation, checkpoint, heap/RSS
measurement or native cycle was reached. This is a failed startup qualification,
not a guide-memory failure, leak diagnosis or passed positive control.

Chrome exited0 without SIGKILL; process output drained completely. Both the fixture
and DevTools ports refused connections after cleanup. Input identities matched
before and after. The failure and clean cleanup were independently reviewed.

## Implemented machinery and verification

- `memory-control-fixture.mjs` serves only three synthetic no-store routes. Its native
  parent form targets an empty-sandbox preview, with a visible generation counter.
  Retention mode alone serves a parent-owned external script that keeps and touches
  an idempotent8MiB allocation. No runtime injection or consumer code is involved.
- `memory-control.mjs` owns one fresh Chrome and the tiny Node fixture, closes only
  the initial known blank page, creates an explicitly recorded context through the
  guard, and drives form submission with native CDP mouse input. Full lifecycle and
  metric checks gate the intended cycles; those paths remain browser-unverified.
  It reuses existing process, guard, lifecycle and memory modules without edits.
- `prepare-memory-control.mjs` freezes15 local source/test/contract files and records
  Node, Chrome launcher, actual Chrome framework and Info.plist hashes. The launcher
  hash alone did not distinguish the new Chrome153 from the prior Chrome152 record.
  This is not a full Chrome-bundle archive. Select and verify the trusted complete
  manifest before launch: ESM imports run before the owner's in-process hash checks.
- Frozen fixture/owned-process tests pass **19/19**, no skips. Syntax checks pass.
  The fixture's allocation test checks served source, not observed browser retention.
  Independent preflight review verified cleanup and the separated acceptance checks;
  no retained-allocation or native-cycle qualification follows from unit tests.

Private packet: `/Volumes/m2ext/appkit-evidence/appkit-memory-controls.mtL8llPl`.
`src/` retains the exact pre-execution code and prospective contract; later result
documentation does not modify that snapshot. `positive/report.json` retains raw
bootstrap commands/responses, launch flags, errors and owner cleanup. The separate
`unit-inputs.json`, `unit-result.json`, `unit.stdout` and `run-unit.mjs` bind the tests.
The positive process exited1 on2026-09-23 at04:42UTC (September22 locally).

| Retained item | SHA-256 |
|---|---|
| `manifest.json` | `b9090ead895ecfb4b02a87b987a94e410f44ed85c66eeb572cf05b2b9e9809d1` |
| `positive/report.json` | `d37c657596b7deaa7e402cd4f6ffa6bf614644321be7f012e6f8d8d3c58fac97` |
| `unit-result.json` | `20078839805b5cb3abb2db583c98b8387457c67876a046d47726a67bc8c6ab43` |
| `unit.stdout` | `9820846ea5f650cd9689b0028f1784d99edda4d3c0474173bbf5f36e1ae55f3e` |
| Owner `memory-control.mjs` | `8e25a785c42dcee5fa0558d6431331ea42cccf079f0e642a34264b908e93d932` |
| Fixture | `21fa1eaf3b7da57e097c197cd64143a0d34e8aea809262de963ecfbe620cb902` |
| Fixture tests | `e5892728e457a2f0efe4a7970de79cc22106bac4d90b8db743ae43ea1ce6dd8f` |
| Preparer | `47b065963aff492a7c1f1f5eec384acd56c2aff20b760dc939cdb405e60b8e8b` |
| Node26.3.0 | `56694c81b093cc8da273fa017cf91765b3653e5f64f16727976ffaa87b2b6b31` |
| Chrome153 framework | `7cb599c60d365f74036f4e6c17880b24318098ebbc7d9d7ad6ecc664778047c5` |

## Next bounded hypothesis, not a retry

The locally installed Playwright primary source at
`/Users/geoff/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright-core/lib/coreBundle.js:34649`
includes `--disable-component-extensions-with-background-pages` in its default
Chromium switches. This direct-CDP launch omitted it. That establishes a candidate
startup-profile distinction, not the cause of this target or proof that the switch
will suppress it on the selected Chrome revision.
Inspected local source SHA-256:
`9393fa79e1c67c74edc26b610d65a4f7ed73d345a762465cc88340a33a2454ac`.

Before another acquisition, select a separate R2 startup hypothesis and freeze a new
profile/contract with the one proposed switch, preserving this failed run and all
target refusals. Bound the next observation to initial target inventory and cleanup;
do not also retry twenty cycles or run retention. If the initial inventory qualifies,
record what changed and independently resume the existing positive/retention controls.
Unknown targets remain failures; do not add `background_page` to the ignored types,
force-close it or infer successful guide ownership from a blank browser.

No app/consumer files, dependencies, live data, default appearance, installed tools,
daemon or global configuration changed. No quality status or weighted completion
credit is awarded. The Gallery cell and full acceptance remain open under ADR-0073.
INC-64 records the platform facts in `docs/intent/domain.md`. One closed chart
incident was compressed from its prior layout in `d75ff9b` to preserve the150-line
incident-log budget; its evidence, outcome and remaining limits are unchanged.

## Separate startup-profile diagnostic: prospective contract

- Input: main `4bfab1b`, unchanged failed packet above, same selected Chrome153/Node
  identities. Preserve its source/report; this is a new manifest and fresh profile.
- Hypothesis: the candidate standard Chromium switch
  `--disable-component-extensions-with-background-pages` permits the initial owned
  browser to reach an admitted startup inventory. It does not explain old `other`
  targets or establish absence of arbitrary extension activity.
- Only browser-argument change: that single switch. Default preparation remains
  unchanged. A named opt-in profile supplies fixed arguments, never arbitrary flags.
- Owner adds a `startup` mode that cannot enter guard/app/context/workload code. Its
  manifest admits that mode only; no fixture server starts. After closing only the
  known blank page, retain initial inventory and timestamped target samples spanning
  at least5000ms (at most5500ms), with at least10 samples and no completion-to-completion
  gap over500ms (including first completion from window start). Requested cadence100ms.
  Each sample retains start/completion and raw target inventory. Every observed
  non-internal target remains a failure, including one during blank-page teardown.
- Cost: one acquisition,120s owner limit plus existing bounded cleanup; no retry,
  native cycles, retention control, metric budgets or Gallery work in this packet.
  Freeze and unit-test profile selection and coverage refusal before launch; retain
  all raw observations. The earlier prepared packet was not executed and stays intact.
- Acceptance: exact source/tools/one-flag identity, admitted bounded samples,
  known blank-page closure only, unforced complete Chrome shutdown and refused
  DevTools port. A pass means no noninternal target observed in those samples;
  polling cannot exclude transient targets between samples or qualify app memory.
- No consumer, browser installation, signed-in profile, daemon, global settings,
  dependencies, runtime code or source/quality gates change. Do not promote readiness.

### Startup-only acquisition result

One new run passed the bounded startup contract. The frozen packet is
`/Volumes/m2ext/appkit-evidence/appkit-startup-bounded.GZm0wrWU/packet`; its parent retains
`run-unit.mjs`. The prior unexecuted preparation at
`/Volumes/m2ext/appkit-evidence/appkit-startup-profile.HlFZnBN6` remains unchanged. No browser retry
or positive/retention workload was run under either preparation.

| Observation | Result |
|---|---|
| Source/tool preflight |19 frozen sources; same Node/Chrome launcher/framework/plist as the failed control; exactly one additional launch switch |
| Frozen unit checks |31/31 pass, zero skips; includes fixed profiles, sample-coverage refusals, fixture and owned-process cleanup |
| Initial raw targets |One `about:blank` page and two `browser_ui` Omnibox Popup targets; no background page observed |
| Target closure |Exactly the known blank page was closed; no foreign target was ignored or closed |
| Bounded sampling |50 raw inventories over5,041.284ms; maximum completion gap103.560ms; all50 inventories empty |
| Protocol scope |54 commands: one version read,52 target inventories and one known-blank close; no guard, fixture, app context, navigation or memory calls |
| Cleanup |Chrome exit0, unforced; both output streams reached EOF/close without truncation; owner68324 and Chrome68325 absent on independent inspection; DevTools56615 independently refused |
| Input integrity |Frozen inputs matched before/after; prior failed control remains unchanged |

The owner's pass was checked against raw command/response pairs and sample times,
not treated as its own oracle. Independent review confirms those observations.
The permitted inference is **no noninternal target observed in these bounded
startup samples under the one-switch profile**. This does not establish continuous
absence, prove the switch is the sole cause, explain historical `other` targets,
qualify application ownership/retention, or promote any quality status.

| Retained artifact | SHA-256 |
|---|---|
| `manifest.json` | `81ba29025208b17735941577e26c170aa8bce33cac9d18e4d5f71300b53bd713` |
| `startup/report.json` | `bd46e88593155047dc7afe129842f15758af7e20debc195189ea0f6aea78be61` |
| `unit-result.json` | `aa1ce6972dcaec70515b713d374d44bb4274fe6196cf3234f4cc1d6f0c7acad6` |
| `unit.stdout` | `6a595b91bd10e4cc81c8bbbc1625e937937e58b8255e7a3e0e2795eaff5fd1c1` |
| Executed owner | `4de48bd4d1493df35e464e56eb025618086b78da757008fe5b517a744dc4f06c` |
| Executed preparer | `78da672de60dd422379ebc59e73af36a06992b731048acea520a4dd5d0bf9e00` |
| Pure startup validator | `550dfbb8f9bf377818ba7cb75755175e2afd431791ee285d32c22abec4a36e55` |
| Validator tests | `1a3dc473469342974c58e94eda09751f8978aa2986acc04d73945657cfdf80f7` |

Next: separately admit the tested launch profile for one positive then one retention
instrument control, freeze a new packet and retain all lifecycle/guard refusals. The
current candidate manifest deliberately admits `startup` only. Neither20 native
cycles nor a4MiB budget may be claimed from this run; the actual Gallery cell remains
unrun. Do not repeat the startup diagnostic or launch a full matrix as a substitute.

## Separate instrument controls with the tested profile: prospective contract

- Start at main `445fe69`. Keep both the baseline failure and startup-only packet
  unchanged. Freeze a new trusted manifest/source closure before tests or acquisition.
- The explicit preparer option `--component-backgrounds-off` selects
  `no-background-components-controls-v1`. Its launch arguments exactly match the
  observed startup profile; it admits `positive`/`retention`, never `startup`.
  Existing default and startup-only flags/mode contracts remain unchanged.
- Every new-profile control must pass the same bounded5s startup samples and strict
  known-blank teardown before guard/context/app work. No new ignored target type.
- Reuse the unchanged fixture, native input, lifecycle/join, sampler, metrics and
  cleanup. Original control workload/limits above apply: warm action, baseline,
  twenty native actions, final; parent-owned8MiB retention must trip the4MiB limit.
- Cap: one positive run; only after independent qualification, one retention run.
  Each has120s plus bounded cleanup. No retry on failure, full matrix, Gallery run,
  app-script injection, consumer/global/runtime changes or new dependencies.
- Success qualifies only these structural/numeric instrument controls. It cannot
  exclude detached/native/GPU retention, prove app attribution or complete memory-v2.
  Record raw failure phase and next decision-changing hypothesis if any gate fails.

### New-profile positive result: task reached, qualification failed

The one new-profile positive acquisition exited1. Retention was **not run**. Its
private packet is `/Volumes/m2ext/appkit-evidence/appkit-profile-controls.4jkwWj99`;33 frozen unit
checks passed before acquisition. All19 source identities and selected tools matched
before/after, and launch flags exactly match the startup-only candidate. Admission
of retention after independent positive review is a session procedure, not an
automatically enforced dependency between the two CLI invocations.

The new run passed50 startup samples, advanced its warm preview1→2 and completed
twenty native form cycles2→22. It acquired baseline/final endpoints,111 process
samples and four lifecycle checkpoints. Replaying the frozen independent join over
631 raw events gives valid structural evidence and complete live membership; the
iframe loader/execution-context replacement is explained. These are scoped observed
task/structure results, not full instrument or app-memory qualification.

The unchanged metric checker rejects five renderer classifications in samples18–20:
CDP identifies PIDs71150,71154,71155,71158 and71159 as renderers while `ps` records
only `(Google Chrome He)`, not a usable argument vector. The raw process rows remain
retained; neither their cause nor a browser leak is established. The frozen evaluator
replays exactly the same five errors; metrics/budgets remain null. A sixth placeholder,
PID71151, has CDP type `unzip.mojom.Unzipper` and exposes an asymmetric checker gap:
absence of the renderer flag alone falsely supplies affirmative non-renderer evidence.
The prospective repair below refuses unavailable command/type evidence for either kind.

Shutdown independently failed: SIGTERM did not finish within2s, SIGKILL ended Chrome,
and stderr EOF/close did not finish within the separate2s drain budget. Its local
stderr read handle was closed by the owner; capture is incomplete. Later current-state
inspection finds all sampled test PIDs absent and both ports57000/57001 refusing;
that supplemental cleanup state does not repair the failed shutdown/capture receipt.
No larger timeout, process omission or guard exception is admitted.

| Retained artifact | SHA-256 |
|---|---|
| `manifest.json` | `fd66b16d4ac3f1cfb1024f0f4a2f36fda502f174e0df4804ee988137c1467daa` |
| `positive/report.json` | `f4a74d5d0180ebf8058e723f63401214333e137c76c4bdf07f14461ced3ca406` |
| `unit-result.json` | `1b87e2033491f59208365e755b12982b8679e308ee152bf5a63646972b611b84` |
| `unit.stdout` | `c3440dcdfb9b312f9028f60b8f6f736550dc30cc0be9d4c50357bd0431bb770c` |

### Prospective incomplete-process-evidence repair

Preserve the browser packet and its evaluator bytes. Freeze regression tests before
running them against the old checker, then freeze the repaired checker before GREEN.
Reject parenthesized unavailable-command placeholders for browser/server/children,
and require one explicit child `--type` argument before renderer/non-renderer
agreement. Known full renderer and utility commands remain legal. Do not remove any
PID, trust CDP alone, fill unknown costs with zero, relax budgets, or alter v1 checks.
This is a pure checker repair; no further browser acquisition is part of this packet.

The pure repair is now verified under
`/Volumes/m2ext/appkit-evidence/appkit-process-classification.YY7W9qiT`. Each phase retains pre-run
checker/test/helper bytes, Node/runner identity, output and exit status:

| Phase | Result |
|---|---|
| `red` |52 tests,46 pass/6 fail: five false passes plus the missing unavailable-command diagnostic for the already-refused renderer |
| `green` |Same52 tests pass after unavailable-command and explicit-type admission repair |
| `token-red` |55 tests,52 pass/3 fail: review catches two substring-driven false passes and one false refusal |
| `token-green` |Same55 tests pass; classification uses the parsed exact type value, not a substring elsewhere in the command |

The final checker SHA is `524a1048341e096d6deb59baf2bc2a6838901c9af71b9299b980916e6b4969d0`;
test SHA `082ac5dd7faee2cd36f1f8599c9f7f45d09ef3fefb27a47a471a5f289b74d50a`.
`token-green/result.json` SHA:
`3cf534c89c25f0d1ea80ef5ed27ccd0794ddf81f147ba7ce68fc33c0ce85a40f`.
Independent review found no remaining blocking regression in this scoped repair.
Supplemental evaluation of the retained browser observations remains invalid and
identifies nine unavailable commands across samples18/19/20/68/98, including utility
rows the old checker admitted. This is a new diagnostic, not an edited historical
receipt or a new acquisition. V1 checker bytes, every numeric limit and all processes
remain unchanged. INC-65 records the defect and mechanical outcome; closed INC-16
was compressed from `445fe69` without changing its outcome to keep intent in budget.

Next runtime hypotheses remain separate: recover independently classifiable process
arguments without losing sampled processes, and dispose the exact owned app context
before closing the guard connection to test orderly shutdown. The current guard closes
only its socket; that is a concrete difference to investigate, not an established
cause. Freeze independent controls before a new acquisition; do not retry unchanged
or run retention/Gallery until positive qualification passes.

## Separate owned-context shutdown diagnostic: prospective contract

- Packet: R3 instrument qualification, starting at `a09cefd`. Named need: the
  retained positive control required SIGKILL and lost stderr EOF. `disposeOnDetach`
  was already enabled; explicit disposal is a hypothesis, not a confirmed cause.
- Freeze a new private source/tool/contract packet before tests and acquisition.
  `--shutdown-only` selects `no-background-components-shutdown-v1`, with the same
  tested launch flags and startup window, admitting only owner mode `shutdown`.
  Existing profiles and their cleanup paths stay unchanged.
- Use the unchanged benign fixture and guard: one warm native preview action,
  twenty native actions and two independently joined lifecycle checkpoints. No
  memory sampler, heap collection, retention allocation, Gallery or real data.
- Before guard disconnect, query context/target inventories, dispose only the exact
  context returned by this run's create command, then require that context and its
  targets absent in fresh inventories. Retain every command/result. Missing,
  malformed or failed evidence fails; do not dispose unrelated contexts or send
  `Browser.close`. The guard retains its existing5s command timeout (five commands,
  26s outer bound); guard close and Chrome2s grace/2s kill/2s drain remain unchanged.
- Acceptance: twenty observed actions, structural checkpoints, exact-context
  disappearance, healthy guard, unforced Chrome exit with complete output, both
  owned ports refused and all frozen inputs unchanged. Process exit alone is not
  complete capture. Failed cleanup remains failed even if later PIDs disappear.
- Cap: one fresh-profile acquisition,120s workload plus bounded cleanup. No rerun,
  timeout extension, new ignored target, global/consumer/config/runtime change.
  A pass demonstrates this narrower synthetic cleanup only, not the cause of the
  prior failure or positive-memory qualification. Record outcome and next action;
  no retention/full matrix follows automatically. Unit checks cover exact ownership,
  malformed evidence and still-live context/target refusals before runtime work.

### Shutdown-only result: scoped pass, memory control still unqualified

The single acquisition passed in
`/Volumes/m2ext/appkit-evidence/appkit-context-shutdown.1wLjN5BD/packet`. All56 frozen unit tests passed
before execution. The owner and independent post-run review verify all21 frozen
sources and the same Node26.3.0/Chrome153.0.8010.53 tool identities. No old packet
was overwritten; baseline/positive/retention cleanup paths remain unchanged.

The warm action advanced1→2, then20 native actions advanced2→22. Fifty startup
samples span5,020.343ms;564 lifecycle events and two checkpoints are retained.
Independent replay through the last pre-cleanup checkpoint confirms valid structural
evidence and complete live membership. No heap/RSS sampler or retention allocation ran.

The five cleanup commands inventory, dispose and re-inventory exactly the created
context. All four targets (page, iframe and two browser-internal UI targets) disappear;
the default context identity stays unchanged. The command sequence takes31ms. Guard
coverage ends healthy/closed with no errors. The exact spawned Chrome receives only
SIGTERM, exits0, and supplies both pipe EOFs plus close without truncation or local
read destruction. Exit and complete capture take175ms, within the original deadlines.
Independent read-only checks find owner77624/Chrome77625 absent and both owned ports
58047/58052 refusing. These supplement rather than replace the retained cleanup events.

| Retained artifact | SHA-256 |
|---|---|
| `manifest.json` | `ebde6508dffb0bbb478f3f4d61784bda3386fdc6aedb6949d271d907675c1406` |
| `shutdown/report.json` | `9935059e05fd901c1a92a587fac69f3fcdeb48b71d9203108a3fbc2c8284c222` |
| `unit-result.json` | `626966f0250b5ddf230810d2166a9806dc4528b9526d8dd3130abad87ea4c69c` |
| Executed owner | `80d4ae40cf2beb066e9004985cbac10f9f96af9559b56c4d9f21ddcf0dc6c010` |
| Parent `review-shutdown.mjs` | `d7dabf50cf8f818930e7567063067ce0df08418337d3a8d550be4760634d39ab` |

The read-only review retains `review-result.json`. It checks raw actions, exact
creation/disposal identity, fresh inventories, unchanged sources/tools, final guard,
raw process/pipe events and independent port/PID observations; it does not trust the
producer's pass bit. Independent source/result review found no blocking discrepancy.
Review also added guard-health checks during context cleanup and after disconnect,
before the candidate was frozen, so asynchronous guard failure cannot supply a pass.

This proves cleanup only for this shorter no-memory workload. No A/B causal claim is
made: it omits sampling and two settled endpoints from the failed positive. Context
disposal does not establish memory reclamation, and no timeout/type/budget was relaxed.
Next selected runtime packet must first address unavailable process-command evidence,
then freeze a separate positive-control candidate incorporating explicit cleanup and
independently qualify it before retention. Do not rerun this passed shutdown diagnostic
or repeat the known-invalid positive unchanged. Gallery/memory/full gates stay open.

## Process-argument investigation and component-update control

Read-only review of the preserved failed positive finds all nine parenthesized-command
rows outside the twenty app-action interval: samples18–20 in baseline settling and
68/98 in final settling. All have nonzero RSS and parent71094. Eight PIDs appear once;
PID71267 first has a complete unzip-helper command (sample97,4,496KiB), then the same
PID/start identity has an unavailable command (sample98,74,976KiB) about102ms later.
Neither zero-cost exclusion, simple output truncation nor a proved zombie diagnosis
is supported. The original metric and cleanup failures remain unchanged.

Primary-source inspection narrows the next experiment:
- Apple's [published `ps` source](https://github.com/apple-oss-distributions/adv_cmds/blob/main/ps/print.c)
  uses parenthesized `p_comm` as the fallback after argument acquisition/parsing fails
  (`getproclline`, lines117–123,179–208,272–288); known zombies have a separate marker.
  Output-width handling comes later. This supports investigating missing arguments,
  not treating `-ww` as a repair, nor identifying which fallback caused this run.
- Chromium's [component registration guard](https://chromium.googlesource.com/chromium/src/+/9aa2f8951b0b5b6e76f0799d4dc3d799696e3791/chrome/browser/chrome_browser_main.cc)
  checks `--disable-component-update`. The locally inspected Playwright
  `coreBundle.js` line34650 also includes that switch (same source path/hash recorded
  in the startup investigation). These references motivate a fixed testing profile;
  they do not prove the installed Chrome's historical unzip work was component updates.

### Prospective contract: one component-update-disabled qualification

- Start at `8d86c5c`. New preparer option `--component-updates-off` admits only
  `positive`/`retention` under `no-component-updates-controls-v1`. Its sole launch-flag
  delta from the tested controls profile is `--disable-component-update`. Nothing
  changes in a signed-in browser, global updater configuration or consumer runtime.
- Incorporate the separately exercised exact-context cleanup for this new profile.
  Other profiles retain their existing behavior. This combination tests a candidate,
  not a single-variable causal claim about the old shutdown or argument failures.
- Freeze source, prospective contract, exact tools/flags and unit tests before use.
  Retain startup sampling, guard, positive fixture, warm action, two5s endpoints,
  twenty native actions, raw lifecycle joins and strict process/metric checker.
  Include every owned process: no PID/type exclusion, retry/substitution, blank
  subtraction, extra app scripts, unknown-as-zero or weakened evidence/budget.
- Cap: one positive acquisition, followed only after independent qualification by
  one deliberate8MiB retention acquisition. Stop on failure; no unchanged repeat,
  Gallery run or full matrix. Workload120s and existing per-operation/cleanup bounds
  stay fixed. A control pass qualifies only its structural/numeric instrument scope;
  detached/native/GPU retention and full app attribution remain unproven.
- Done for this packet: retained result, independent raw-evidence review, clean owned
  lifecycle or explicit unresolved failure, and one next action. No quality promotion,
  consumer changes, new dependencies or global configuration. Existing ADR-0073
  separation remains intact; no public contract or acceptance threshold changes.

### Component-update candidate result and bounded mechanical repair

The single candidate failed after20 successful actions. It remains frozen at
`/Volumes/m2ext/appkit-evidence/appkit-component-control.Xb46hw6J/packet`;58 tests passed first.
Eleven unavailable-command rows persist in samples17/18/19/65/66/67/82/99, again
outside the action interval. Disabling component updates is insufficient in this
run; no root cause is proved. Retention was not run. Its profile, flag and preparer
option were removed from the maintained checkout; the complete executed candidate
stays in its private packet for historical replay, not another default or a pass.

The independent metric checker also catches baseline idle4,999.730292ms after the
nominal5,000ms pause (final idle5,001.052125ms). The collector awaited a requested
timer delay but never rechecked its minimum monotonic deadline. Keep the5s gate;
fix acquisition rather than allowing timing tolerance in the oracle.

Explicit context disposal and process capture did pass under this longer workload:
four context targets disappear, the guard stays healthy, Chrome exits0 after only
SIGTERM, and both pipe EOF/close events arrive within the existing budgets (188ms).
Owner80664/Chrome80665 and every unavailable-row PID are independently absent;
recorded ports58560/58562 refuse. This does not qualify the failed metrics.

| Retained artifact | SHA-256 |
|---|---|
| `manifest.json` | `6f66df59c5d039dbd6cea299bbc379c39764043440e2ea213468f0bcccfef021` |
| `positive/report.json` | `fba3b0e682cc01b3baa7a6835743621777ff4e7aefe14bf7056d03354ae36336` |
| `unit-result.json` | `7d3dab675cd70313a1e0ee8aeed8c9d6c97d67f4c901b873d111b3e6f846c6ff` |

Next mechanical change, before further browser work: freeze a fake-clock regression
reproducing the early wake against current source, then freeze the repaired collector
that waits again until the full monotonic5s interval elapses. Exact-deadline and
oversleep controls must still pass, and the metric/v1 checkers remain byte-identical.
Also use the exercised exact-context helper in every context-owning synthetic control,
not only shutdown mode; no launch flags/modes, guard policy or deadline change.
This is instrument cleanup/timing repair, not a public API or acceptance change.
Cap: one frozen RED and one GREEN unit run; no browser retry in this packet.

The initial pure phases are preserved at
`/Volumes/m2ext/appkit-evidence/appkit-minimum-idle.ftv8VNBF/{red,green}`. RED has113/114 passes and
the intended early-idle refusal. The initial GREEN also has113/114: the repaired
collector reaches its deadline, then the fake fixture mistakes the endpoint's normal
10ms fresh-sample catch-up wait for another idle pause. This is a distinct fixture
coordination defect, not evidence that the timing gate should change.

Bounded follow-up: preserve both executed phases, make the fixture distinguish
post-idle sampling only after a heap-read observation, and keep every early-wake,
exact-deadline and oversleep assertion. Freeze that same corrected fixture against
old and repaired collectors as `fixture-red`/`fixture-green`, one run each. No browser
acquisition, dropped sampler, changed numeric oracle or reuse of the old stage paths.
That follow-up retains the intended RED but GREEN exposes another fixture assumption:
the sampler subtracts acquisition time, so it requests fractional delays below100ms,
not always exactly100ms. The fake treated this as idle work and stopped its sampler.
Keep both failed GREEN phases. Correct the fixture's bounded sampler-delay recognition,
assert no sampler errors, and freeze the same final fixture against old/new collectors
once as `sampler-red`/`sampler-green`; all original deadline assertions remain unchanged.

The final same-fixture comparison is verified: `sampler-red` has113/114 passes with
only the intended minimum-idle assertion failing; `sampler-green` has114/114 passes.
Of the twelve frozen inputs only `memory.mjs` differs between those two phases.
Original early-wake assertions, exact/oversleep controls, sampler and numeric checker
tests are preserved; the fake sampler must also report no errors. Independent source
review found no blocking issue. INC-66 records the mechanical defect and I17 outcome.

| Final artifact | SHA-256 |
|---|---|
| Repaired `memory.mjs` | `a83efd446ce2cd906799d28fc42bfd14db8345fdb345bb829bb1b516bc053299` |
| Same regression fixture in both phases | `c1d42bfe03e7728ec77e921bcd9adb4ab3ac3225b2861c1fa69ed821b5db6c02` |
| `sampler-red/result.json` | `cbf430787b94cb9508c22d7a615713ea5a3b6188d9cfbd12e69f102d34c7c345` |
| `sampler-green/result.json` | `d2494289949697a270c5dffa44c19dc11fcfa2c6630e866c3ee7d6074a3e0512` |

The v2 metric checker remains `524a1048341e096d6deb59baf2bc2a6838901c9af71b9299b980916e6b4969d0`;
v1 checker remains `8b7c6e0a8ec1265982520ae8d0daa9a92c10e0e4d9f8fd99235271bd69180735`.
Default-profile browser requalification, retention and Gallery memory remain unrun.
Next investigation must supply independently usable process-identity/type evidence
without dropping transient PIDs or carrying stale argv across identities. A width
switch or disabled component updater alone is not a supported repair. Do not perform
another positive acquisition until the process-evidence method has a concrete change.
