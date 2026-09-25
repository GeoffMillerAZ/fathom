# Local theme-lab browser checks

Opt-in probes use an already-approved isolated Playwright harness and installed Chrome
with fresh temporary profiles. They never install dependencies, use a signed-in profile,
write source/preferences, contact the paid portal or modify a consumer.
The selected server must be the Gallery adapter using the Synthwave source below.

```sh
go run ./apps/gallery/cmd/gallery -addr 127.0.0.1:0 -design-assets= \
  -theme-source cue/tokens/geoff-synthwave/geoff-synthwave.cue
```

Use the actual printed address. Set APPKIT_PLAYWRIGHT_MODULE to the approved absolute
Playwright index.mjs path; this suite checks the tested1.63.0 entry/package identities.
An absent/different harness or missing route fails, never silently skips.

```sh
APPKIT_THEME_BROWSER_ORIGIN=http://127.0.0.1:56315 \
APPKIT_PLAYWRIGHT_MODULE=/tmp/appkit-browser-review.xBrDCp/node_modules/playwright/index.mjs \
node apps/styleguide/browser/native_preview.mjs
```

Repeat with APPKIT_THEME_BROWSER_SCRIPT_MODE=disabled for the no-script native canary.
Run usability.mjs and review_loop.mjs with the same origin/module environment for the
editor geometry/error path and complete source/candidate/export task respectively.
Use `node apps/styleguide/browser/usability.mjs --zoom=200` for the normal checks
plus an actual200% Chrome Appearance lane in a separate0700 temporary profile.
It checks native keyboard repair/correction/source/reset and expanded-state geometry;
captures are viewport-only, not full-page zoom images. Default invocation stays unchanged.
Use `node apps/styleguide/browser/components_reflow.mjs` for the exact17-component
narrow specimen inventory, figure/SVG bounds and selected computed paint across six
viewport/scheme cases. It disables page scripts; screenshots need separate review.
APPKIT_BROWSER_VARIANT labels a run; it never changes expected application behavior.
Browser executable selection follows the Gallery browser configuration's absolute-path
and loopback-origin checks. Do not point these probes at a real app/account.

Each probe prints a fresh output directory and writes a JSON receipt with exact named
assertions, response hashes, screenshots and limitations. review_loop also downloads
two synthetic ZIPs there and inspects their exact members with macOS /usr/bin/unzip.
It never extracts archive paths into a workspace or user Downloads. Only controlled
409/422 network console errors are expected; other console/page errors fail.

Containment: explicit loopback route/method allowlist, no WebSockets, fresh contexts,
document CSP and empty iframe sandbox, plus assertions that no worker appeared.
Do not use Playwright serviceWorkers:block here: its all-frame init script throws on
the opaque frame's serviceWorker getter. Do not relax sandbox or hide that exception.
Page evaluation is diagnostic only; no DOM mutation or CSP bypass is used.

These are synthetic Gallery tasks, not a universal browser runner or an accessibility,
font, performance, consumer-adoption or converted-component certification suite.
[Dated results and limits](../../../docs/assessments/2026-09-14-theme-lab-browser.md).
[Narrow-component repair and actual zoom follow-up](../../../docs/assessments/2026-09-15-theme-preview-reflow.md).

## Ordinary performance acquisition

`performance.mjs` implements the first/repeat subset of the prospective
[browser contract](../../../docs/assessments/fixtures/styleguide-performance/browser-contract.md).
It runs120 native journeys and retains bodies/ZIPs. It does not start the server,
install tools or prove warm-cache/degraded/memory/cancellation coverage.

In addition to the origin/module/browser selection above, explicitly set:

- `APPKIT_THEME_PERF_OUTPUT_DIR`: fresh existing private output directory.
- `APPKIT_THEME_SERVER_BINARY`: exact selected Gallery executable, not `go run`.
- `APPKIT_THEME_SOURCE`: exact immutable Synthwave CUE file supplied to the server.
- `APPKIT_THEME_PERF_SNAPSHOT`: pre-acquisition JSON manifest, schema
  `theme-guide-browser-inputs-v1`; `files` contains `probe`, `config`, `contract`,
  `source`, `server`, `playwright_entry`, `playwright_package`, `browser_executable`
  and `node`, each with absolute `path` and `sha256`. Include Node `version` in
  the top-level `node` identity. `server_build_receipt:{path,sha256}` selects a
  retained successful build receipt containing `passed:true`, exact
  `server:{path,sha256}` and `go:{path,sha256,version}`. Keep the matching source,
  executable and complete selected asset snapshots before launching.

Run `node apps/styleguide/browser/performance.mjs`. Verify its
`theme-guide-browser-v1-receipt.json` with the [independent scoped checker](../../../docs/assessments/fixtures/styleguide-performance/README.md).
The acquisition's `ok` means ordinary tasks/closure completed, not that budgets or
the full performance gate passed. The independent checker applies unchanged budgets.
Retain failed runs before repairs; runtime changes require fresh evidence. A repeated
structural harness error is not a reason to change an application or weaken its policy.

## Cache-preserving and degraded transport acquisition

`transport.mjs` connects to an externally owned, fresh temporary Chrome through
`APPKIT_THEME_BROWSER_WS` (exact loopback browser CDP URL). The owner launches and
cleans up Chrome, its profile and the synthetic Gallery server. The probe closes
only its contexts and connections; it cannot claim process cleanup for the owner.

Use `APPKIT_THEME_TRANSPORT_MODE=cache|degraded`, with the same explicit origin,
source, server, browser and Playwright selections as above. Set a fresh existing
`APPKIT_THEME_TRANSPORT_OUTPUT_DIR` and `APPKIT_THEME_TRANSPORT_SNAPSHOT`. Its schema
is `appkit-theme-guide-transport-inputs-v1`, with the ordinary identity fields plus
`files.guard` for `cdp_guard.mjs` and `files.phase_wait` for `phase_wait.mjs`.
Freeze the runner, guard and phase barrier before acquisition.
Run `node apps/styleguide/browser/transport.mjs` only after the launcher is ready.

Cache mode runs120 first/repeat tasks; degraded mode runs60 fresh tasks with the
contract's fixed100ms/200,000-byte/s download/64,000-byte/s upload conditions.
Raw per-session CDP events, phase boundaries, bodies, ZIPs and context cleanup are
retained. No HTTP routing is used. Application-target guarding is not an OS-wide
network sandbox. A repeat without actual cache provenance fails cache evidence.

`theme-guide-transport-<mode>-v1-receipt.json` uses schema
`appkit-theme-guide-transport-v1`. Producer `ok` is scoped acquisition evidence only;
mode/full gate completion stays false pending independent checks. Do not feed this
different schema to the ordinary checker or relabel it as an ordinary run.
The current producer waits for all required response bodies and records per-phase
start/completion clocks, then rechecks the visible frame state. Historical producers
without this barrier cannot establish that stronger timing claim (INC-52).

```sh
node --test apps/styleguide/browser/phase_wait.test.mjs
node --test docs/assessments/fixtures/styleguide-performance/transport-check.test.mjs
node docs/assessments/fixtures/styleguide-performance/transport-check.mjs \
  /absolute/receipt.json /absolute/snapshot.json --scoped
```

The transport checker reopens retained bodies/ZIPs, joins actual per-session CDP
events and requires shaping/cache lineage rather than trusting producer summaries.
`cache_repeat_revalidations` separately records raw304 observations; it is not a
synonym for a served-cache event. The checker derives both lists and requires a
causally earlier, same-context/same-URL, retained identical stylesheet body plus
exact byte-bound conditional/response validators. First-task or degraded-task
reuse is allowed only with that lineage; it cannot substitute for repeat evidence.
The v1 body contract remains strict: a body-free304 alone cannot manufacture a
newly read representation. [Ordinary Gallery controls](../../../docs/assessments/2026-09-17-gallery-css-revalidation.md)
therefore report their separate CSSOM/cold-body/304 scope, not a v1 theme-cache pass.
It does not verify external process ownership: separately bind the owner's receipt
to the probe/snapshot hashes, endpoints, clean exits and refused ports. Missing owner
cleanup keeps overall acceptance incomplete even if scoped artifact checks pass.
[Results, retained failures and limits](../../../docs/assessments/2026-09-16-theme-guide-transport.md).

## Separate memory acquisition

`APPKIT_THEME_TRANSPORT_MODE=memory` reuses the same native task function, with a
separate `appkit-theme-guide-memory-v1` receipt. It is not a latency acquisition:
forced collection and process sampling intentionally add diagnostic work.
Each of the twelve specimen/scheme/viewport cells has one warm task, a settled
baseline, twenty further tasks and a settled final endpoint. Each endpoint performs
per-isolate GC followed by five idle seconds, retaining per-target DOM counts and
deduplicated heap usage. Renderer RSS is sampled with a 100ms cadence target; the
checker must reject actual gaps beyond the contract's 200ms limit.

The frozen input snapshot additionally binds `memory.mjs` and `process_rss.mjs`.
The external launcher must pass its actual child PIDs as `APPKIT_THEME_BROWSER_PID`
and `APPKIT_THEME_SERVER_PID`. CDP identifies that browser's process set; `/bin/ps`
queries only those specific PIDs plus the server, never the full host process table.
Raw process lists/rows, start identities, sample intervals and read failures remain
in the receipt. Browser-reported non-renderer overhead and server RSS are separate
from renderer totals; this is not an operating-system-wide memory accounting claim.

Run the frozen `transport-check.mjs` with `--scoped` against this separate schema;
it delegates numeric/ownership/lifecycle acceptance to `memory-check.mjs` and still
checks native task/asset/ZIP evidence. Bind its result to the launcher's process
receipt separately. A producer `ok` only says acquisition completed; it does not
establish memory budgets, full performance, browser cancellation or adoption.

Settled endpoints retain wall-clock boundaries, partial failures, raw target lists
and guarded-session snapshots. A session detached during later context cleanup is
valid only when its attachment/policy/detach events prove it was live and guarded
during measurement. Unknown target types are never silently removed. A bounded
startup-teardown wait retains its polls before the workload, instead of treating a
closed Playwright page as proof that Chrome has already destroyed its targets.
[Current failed measurements and next steps](../../../docs/assessments/2026-09-17-theme-guide-memory.md)
remain separate from checker unit passes and from the earlier timing lane.

## Separate native Stop/retry acquisition

`cancellation.mjs` uses the [controlled-delay fixture](../../../docs/assessments/fixtures/theme-cancellation/README.md)
over actual Gallery handlers, with a fresh externally owned Chrome/profile and explicit
pre-execution identities. Only the next armed preview envelope is held before the real
handler; the control token stays outside the page. The explicit method is `escape`
or `cdp-stop-loading`; the latter is not physical keyboard proof. A one-cell diagnostic
does not replace the twelve-cell requirement or inherit another input method's result.
Observe cancellation before teardown, retained edits, unchanged retry bytes and the
ordinary candidate/export/reset workflow. Cleanup-induced cancellation is not a pass.
The first Stop failure leaves remaining cells unexecuted. No route abort, page script,
iframe permission change or silent substitute stop method is permitted. Producer
`complete` stays false pending independent checks; [actual status](../../../docs/assessments/2026-09-17-theme-guide-cancellation.md)
does not inherit fixture unit-test or timing/memory acceptance.

`owned_process.mjs` provides bounded exit/pipe-drain tracking for an exactly owned
ChildProcess. Freeze it with the private owner before launch; run its synthetic tests
only after retaining their bytes. Forced stop, truncated output or an unresolved child
cannot count as clean cleanup. An unresolved-child receipt permits owner reporting,
not a claim that the PID is dead; revalidate its identity before any later recovery.
Producer/guard failures remain independent of clean process ownership.

The cancellation probe opts into `cdp_guard.mjs`'s Target-only `protocolEvents` trace.
It retains full target/context identity and command/response order, with markers around
enumeration and context close. Unknown types still fail closed; tracing is not causal
proof or a reason to omit targets. Other guard callers retain their existing defaults.

`APPKIT_THEME_CANCEL_CONTROL=stop-retry|ordinary-preview|idle-only|edit-only|pending-preview|completed-preview` defaults to `stop-retry`.
The separate ordinary control skips fixture arm/Stop/state requests and uses a native
`preview` phase, never a relabeled retry/cancellation pass. It leaves all12 cancellation
cells unexecuted; `executed_diagnostic_tasks` counts its separate one-cell attempt.
Receipts distinguish configured, selected and actually executed Stop methods.

The no-submit `idle-only` and `edit-only` controls require phase observations `on`.
Idle waits200ms after verified navigation, observes targets, then waits1,500ms and
observes again. Edit observes native selection, typography, radius and accent changes,
then waits1,500ms for a final observation. Neither arms/polls the fixture, submits,
stops, exports or resets. Action times/values and later unexecuted phases are retained.
Success requires navigation-only traffic, no POST/download/worker, unchanged baseline
preview/navigation, expected form values, blank digest, disabled export and the same
opaque sandbox/no-store/guard requirements. All12 cancellation cells remain unexecuted.

The `pending-preview` and `completed-preview` controls also require phase observations
on. Both arm the existing15-second fixture delay and submit the edited form natively.
Pending observes targets before any POST response/navigation; completed records a
passive held boundary and waits for the actual candidate plus complete assets before
observing targets. Each adds one observation after the common initial navigation and
skips final enumeration. Neither Stops, retries, exports or resets; pending cleanup
cancellation is not Stop evidence. Unknown targets remain paused and fail. The fixed
[submission-boundary acquisition](../../../docs/assessments/2026-09-17-theme-guide-submission-boundaries.md)
keeps both controls separate from all12 unexecuted cancellation cells and full acceptance.

`APPKIT_THEME_CANCEL_PHASE_TARGETS=on` adds fail-fast enumeration after navigation,
verified Stop (before retry), preview/retry, export and reset response barriers.
Default `off` preserves the task sequence.
The receipt records the option, full observations and explicit unexecuted phases; this
diagnostic is not an ordinary timing run or a replacement for the full task matrix.

`APPKIT_THEME_CANCEL_INSPECT_UNKNOWN=on` enables bounded read-only inspection after a
task/guard failure, before context cleanup. At most two unresumed, attached unknown
sessions receive `Target.getTargetInfo`, `Browser.getWindowForTarget`, `Schema.getDomains`
and `Page.getFrameTree`; each command has a5s bound. All results/errors, identities,
times and uninspected extras are retained. Default `off` sends none of these commands.
Domain names are not proof of command support or containment. Unknown targets remain
unresumed and failed; the owner still terminates exactly its own browser.

`target_control.mjs` is a separate no-app control, with `APPKIT_TARGET_CONTROL_VARIANT`
set to `empty` or `blank`. The owner supplies `APPKIT_TARGET_CONTROL_WS`, a fresh private
`APPKIT_TARGET_CONTROL_OUTPUT_DIR`, and `APPKIT_TARGET_CONTROL_SNAPSHOT` using schema
`appkit-target-control-inputs-v1`, matching `variant`, Node version and exact
`files.{probe,guard,node,browser_executable,playwright_entry,playwright_package}` identities.
Execute retained adjacent probe/guard copies; no app/server/navigation/download is needed.
Three fixed observations and owned cleanup cannot certify a theme task or identify the
cause of unknown targets. [Results and next diagnostic](../../../docs/assessments/2026-09-17-theme-guide-target-controls.md).
