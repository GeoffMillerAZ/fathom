# Theme-guide memory: retained failures, not acceptance

Scope: the separate memory lane of the fixed
[browser contract](fixtures/styleguide-performance/browser-contract.md), using the
actual Gallery theme guide. No production renderer, preview sandbox, token default,
consumer, dependency or performance budget is changed by this work.

## Acquisition and ownership

`apps/styleguide/browser/transport.mjs` reuses its native task function: navigate,
edit, preview, acknowledge/export and reset. Each of twelve viewport/specimen/scheme
cells has one warm task and twenty further tasks. Settled endpoints collect each
unique JS isolate once, wait five idle seconds and record heap, DOM and process RSS.
The producer records observations; the independent checker decides scoped acceptance.
Instrumented memory durations do not qualify as timing-lane evidence.

`process_rss.mjs` queries the owned browser's CDP process list, then `/bin/ps` for only
those PIDs and the owned server. Raw rows, start identities, PPIDs, process types and
sampling intervals remain available. Browser-reported non-renderer overhead and server
RSS are separate; **every renderer, including Chrome's internal UI renderer, remains
in the reported renderer sum**. Missing/disappearing PIDs are failures, not zero RSS.
Summed process RSS is not unique physical memory or an attribution of all cost to app JS.

Private root: `/Volumes/m2ext/appkit-evidence/appkit-theme-memory.KZKiDOw5/`.
`green/receipt.json` retains a fresh build and passing full race suites for Gallery,
styleguide and Gallery command, with selected source/tool identities unchanged.
This uses existing module caches, not a hermetic dependency closure.

- Build receipt SHA-256:
  `cde9d5dbd38644faa74fc41dbdcd4e4717c8d375037a2b07ccf87e28d67a3550`.
- Executed Gallery SHA-256:
  `613e956d4617c10bb2b97ed6c91c5cd8287832535525d424b87be4c709dfc50a`.
- Chrome152.0.7977.84, Node26.3.0 and Playwright1.63; Apple M4,32GiB, Darwin25.6 arm64.
- Fresh private Chrome/profile, loopback synthetic data and application-target CDP
  containment. No signed-in profile, real app data or OS-wide containment claim.

## First acquisition: failed

Original packet:
`/private/tmp/appkit-theme-memory.iyZV6SpO/memory-gallery-xxTDqN/`.
Its probe SHA-256 is
`a8b74fd634c1fdffa1d52e3b926a5fd9bfe733145f95e479bc463fe55ae596f4`.

| Observation | Result and limit |
|---|---|
| Native tasks |232/252 complete;20 unexecuted after one baseline failure |
| Endpoint pairs |11/12 cells finish twenty cycles and both settled endpoints |
| Maximum sampled renderer RSS during work |658.31–751.30MiB across completed cells; exceeds256MiB |
| Settled renderer RSS growth |Ten cells grow39.53–51.69MiB; one shrinks50.14MiB;32MiB ceiling fails |
| Retained JS heap growth |1,112–4,016 bytes across completed cells; not evidence of a growing JS leak |
| Process sampling |One missing renderer PID; sampling gaps remain independently checked |
| First-cell topology |`Unexpected live memory target: other`; old producer did not retain its raw target list |
| Final input identities |External-volume reads fail with `EPERM`; unchanged-input proof fails |
| Cleanup |Owned Chrome/Gallery exit0; both listening ports refuse connections |

Clean cleanup does not repair input-identity failure. Small JS growth does not excuse
RSS or sampling failures. The original packet cannot be upgraded by later successful
reads, corrected tests or a new acquisition. These are diagnostic observations from a
failed packet, not accepted memory measurements or an app-leak diagnosis.

`internal-evidence-jilD0N/` holds byte-identical copies of278 files
(136,496,739 bytes), excluding only the ephemeral Chrome profile. Originals remain.
Backup receipt SHA-256:
`82be6436c6094adff4438ca3da9e9fc2e195c44bac6152218cb85764d88f5318`.

## Acquisition and checker repairs

The next producer revision retains partial endpoint/GC failures, exact raw
`Target.getTargets` observations, guarded-session snapshots and wall-clock endpoint
boundaries. Exact page and iframe paths replace the permissive `/theme-lab` prefix;
unknown target types still fail closed. No isolation exception was added.
`rss-check-CVI8ln/` freezes and passes thirteen sampler tests and four mock-only
endpoint tests, plus producer syntax checks. Mocks perform no host/process reads.
Earlier `rss-check-aDupTf/` retains four assertion-harness failures; the repaired
sampler's first thirteen-test pass is separately retained in `rss-check-ymmxuv/`.

`transport-check-81jnnU/` retains the initial independent-checker revision and its
failed positive fixture (mismatched origin). Review also found missing raw-heap and
temporal frame-lifecycle bindings. A final detached session alone cannot distinguish
an invalid measurement from normal context cleanup after a valid endpoint. Repairs
must bind attachment/policy/detachment to endpoint observations, not remove the check.
The historical packet lacks the new temporal observations and must remain unqualified.
The repaired checker passes85 pure/integration canaries in `transport-check-Ulneel/`.
This is checker validation, not application acceptance.

## Retained replay and browser-overhead control

`/private/tmp/appkit-theme-memory.iyZV6SpO/memory-gallery-mWDqcy/` repeats the same
workload with the new observation fields. Probe SHA-256:
`f0c24e5156193aa9dce5a815d707119b785ffbf68bbbc277d3aa5a2ec77517d1`.
Input identities and owner cleanup pass;232 tasks complete and20 remain unexecuted.
The exact target list identifies two attached `other` targets in a context separate
from the live guide context. This assessment previously called it the closing startup
context without recorded ID lineage; that attribution was unsupported and is corrected
by INC-53 and the full replay below. They also trigger two guard setup failures;
ignoring them or clearing the guard error would not be valid proof.

The independent checker retains11 RSS ceiling/growth failures, one263.213ms sampling
gap, early renderer-command classification races and two missing iframe CSS
`requestWillBeSentExtraInfo` events. Completed-lane peaks are729.69–759.81MiB,
RSS growth38.69–48.58MiB and JS retained growth1,072–3,796 bytes.
Two additional current-file identity mismatches are expected because startup handling
was edited after acquisition; the executed original bytes remain in the packet.

The separate control at `/private/tmp/appkit-theme-memory-overhead-FlB95v/` uses
the same Chrome executable/flags, fresh profile,1440×1000 viewport and guard, but a
tiny synthetic Node HTTP page instead of Gallery. It closes the startup page and
proves target teardown before installing the application guard; no application page
is created until the guard is ready. No unknown-target guard failure occurs in this
control. That observation does not identify the cause of the full-workload failure.

After per-isolate GC and five idle seconds, ten samples per control observe:

| Control | Summed renderer RSS range |
|---|---:|
| Blank top-level page |398.52–398.61MiB |
| Blank page with one opaque sandbox iframe |527.13–528.41MiB |

The sum includes Chrome's internal UI and spare renderers, just as the fixed gate
does. The latter are not measured app allocations; these controls are not unique
physical-memory accounting, a general minimum or a license to subtract the baseline.
All twenty samples retain raw process lists/rows. Guard errors are absent, inputs
stay unchanged, Chrome exits0 and both ports refuse connections. Control receipt SHA:
`b9656f7d1b86a3dc668773481703447cd420a8437deebd2a31d05228265f14fa`.

This fires ADR-0069's measured-budget review trigger: the selected instrument/browser
already exceeds the fixed absolute ceiling without app-kit content. Keep v1 failed;
resolve the metric's intended product meaning before designing any prospective
replacement contract. Do not retrospectively recalibrate, weaken isolation or claim
an application optimization from the blank control.

A bounded five-second startup-quiescence helper now retains every poll and refuses
persistent unknown targets/read failures. Seven endpoint/startup mock tests and the
thirteen sampler tests pass in `rss-check-1LnWHu/`; a later transport-order change
applies the control's startup sequence to memory mode only; the full replay below fails.

`internal-evidence-oKdg2k/` retains byte-identical copies of the replay and blank
control (290 files,148,011,640 bytes), excluding their ephemeral profiles only.
Original paths and bytes remain intact; its manifest maps every retained copy.

`verify-TO0plM/` freezes the current selected sources/tools and passes115 Node
regressions, the styleguide race suite, qualityplan/MCP checks, local documentation
links and selected diff checks. Scope remains144 lines. Receipt SHA-256:
`e04e4e27f1caa75d3755660f06dc9245bb7c1c5cc3d591c4a7f0c90da00697be`.
The final evidence-pointer prose is later than this test snapshot. Bounded independent
review finds the native task and cache/degraded startup order unchanged.

## Fresh startup-sequence replay: still failed

Private root `/Volumes/m2ext/appkit-evidence/appkit-theme-startup.zlqzNEfd/` retains a fresh race/build
pass and unchanged source/tool identities. Build receipt SHA-256:
`f9bffbcee7e4748c7f7852aa5957cccffe0397c4b3f703be8b5960d989bd36f6`.
The selected Gallery binary is
`c35eba4ae2cf39033a7d928d44d143876d344859f528897ed8a488487a855ef5`.
Actual acquisition: `/private/tmp/appkit-theme-startup.Y9JGiq2Z/memory-gallery-yynrWp/`;
probe SHA-256 `c49211d82ce2b0ea3acb19b6223d2bdda234d334d19bd331130f7760c440cea1`.

Startup observation records an empty target list at265ms, before the application
context is created. At the first baseline,939ms after acquisition start, two attached
`other` targets appear in context `9A41D53D8363293A11063D370FFF6A57`, distinct from
guide context `7854FA4425D7F38654F49EC258C40560`. No recorded lineage identifies the
former as startup context. This disproves the proposed startup-quiescence fix as a
complete solution; target creation cause remains unknown, not a proven application bug.

The producer completes232/252 tasks; the first cell leaves20 unexecuted. The guard
correctly refuses both unknown targets. Input identities and owner cleanup pass;
Chrome/server exit0 and both ports refuse connections. `transport-check-erY1IJ/`
passes85 checker canaries but rejects the acquisition with2,152 errors, including
missing-task cascades. Passing checker tests are not a memory-gate pass. Its receipt
SHA-256 is `c19289055d2d17226a866f16fa52b3ff6db91d13bfd5d11a2fd280f3f49830d5`.

`internal-evidence-iBZkxX/` preserves byte-identical copies of278 acquisition files
(146,148,947 bytes), excluding only the ephemeral profile; originals stay intact.
Backup receipt SHA-256:
`23efba87ad8d5399d8aa58ca427a3c330a471885b468014fc84a3825b3755f5b`.

## Remaining work

Identify the unknown-target creation trigger before another full replay, resolve
renderer identity/CDP evidence races and investigate retained RSS growth separately from JS.
Do not subtract browser-internal renderer RSS from the fixed gate, change process
flags, infer a leak or optimize the application until the evidence supports it.
The blank control is diagnostic only and cannot replace the actual guide task.

The256MiB sampled renderer ceiling,4MiB heap-growth ceiling,32MiB RSS-growth ceiling,
twenty cycles and200ms maximum sampling interval remain unchanged. Browser stop/retry,
preview cache, consumer propagation and other core acceptance remain open. G1/G4 are
not complete; no catalog target or general performance certification is implied.
