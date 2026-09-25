# Theme-guide transport and cooperative cancellation follow-up

The corrected degraded-network acquisition passes all60 fixed journeys and independent
artifact, request-shaping and owner-cleanup checks. A corrected ordinary replay passes
all120 task/timing/body checks, but its cache-preserving acquisition still records
**zero cache hits** and correctly fails its cache-evidence requirement. This is not
a warm-cache pass. The separate local transport canary proves its bounded iframe
guard and a real cache hit; fixture success does not replace Gallery evidence.
The full theme-guide browser-performance gate remains partial.

## Cache-preserving application-target guard

`apps/styleguide/browser/cdp_guard.mjs` attaches through flattened browser-level CDP,
pauses new application targets and recursively configures page/iframe targets before
resuming them. Each application session gets the exact loopback-origin allow rule
and deny fallback. It does not use Playwright routing, Fetch interception or disable
HTTP caching. Unknown application target types fail closed. Browser, tab and internal
browser-UI targets are explicitly outside this application-target scope; this is
not OS-wide network containment. Only a fresh, temporary Chrome profile is used.

The private two-loopback-server canary at
`/Volumes/m2ext/appkit-theme-transport.du8Ja1dS/canary-EiH5UF/` records:

- Two forbidden-image requests in the deliberately unguarded control, none in the
  guarded cache or degraded lanes; both parent and opaque iframe are exercised.
- One real `Network.requestServedFromCache` event, with expected computed CSS.
- Seven actual request-extra-info rule IDs spanning page, iframe, CSS and native POST.
- A65,544-byte native form upload received over1,033.8ms under64,000-byte/s shaping.
- Owned Chrome exit0 and unchanged selected inputs.

This narrow canary is not the fixed Gallery task or its performance acceptance.
The earlier `canary-XdTaJX` fails on Chrome-internal targets before application
navigation. Its logs remain. The later launcher disables extensions only in its
temporary profile and records the explicit internal-UI exclusion; application workers
are not silently excluded. Earlier parent-only/non-flattened failures remain in the
[ordinary assessment](2026-09-16-theme-guide-browser-cost.md).

## Actual Gallery cache acquisition

Packet: `/Volumes/m2ext/appkit-theme-transport.du8Ja1dS/cache-gallery-ZoBkOm/`.
The frozen `transport.mjs` uses the same source, server, native forms, empty sandbox,
no-store documents, task matrix and ZIP checks as the prospective
[contract](fixtures/styleguide-performance/browser-contract.md).

All120 tasks complete, producing1,560 request/response records in68.524s. Every
observed response comes from the network: no request-served-from-cache event or
disk/service-worker/prefetch-cache response flag. The shared `/static/app.css` response
has no Cache-Control, ETag or Last-Modified header and transfers again on each view.
This observation does not isolate every possible cause of cache nonuse, including
opaque-frame partitioning. No cache policy, sandbox or budget was changed to get a pass.

The raw CDP packet includes421 sessions; the sole unpaused attachment is the owner's
initial `about:blank` page. Actual application targets are paused and configured;
there are no guard errors. The producer separately records complete task states and
failed cache evidence. Both owned processes exit0, their ports refuse afterward,
and selected inputs remain unchanged. Independent transport verification is required;
producer summaries are not performance certification.

Task receipt SHA-256:
`104b745a8358846e4965c878e1975a4701aa8cc529ac26deb51e2024c5363927`.
Executed probe SHA-256:
`771bfb8d7ae2c6a8e426479fd33d4793c96bdeaab01b6f8e8159fd43e35b6427`.
Guard SHA-256:
`7b6dcdbc6c32a08a318b22475dc51d6ab556bb22a0d7d937e8d823573b0f7952`.

## Degraded acquisition and phase-wait defect

Packet: `/Volumes/m2ext/appkit-theme-transport.du8Ja1dS/degraded-gallery-QPGUR5/`.
The complete60-task attempt takes578.103s and reports only2 producer task passes.
It retains53 stylesheet-body deadline errors, two closed-target body errors, file-read
EPERM failures and three canceled downloads. Both owned processes exit0 and both ports
refuse afterward; no process was restarted during an observation timeout.
Task receipt SHA-256:
`3df56d47000da3ca2fa68e1849e9d2df604935a1b4e03d1b7da3805e73234bc1`.

Source review identifies a collector race: `drain` awaited only responses already
observed, not outstanding requests whose headers had yet to arrive. The initial
stylesheet could therefore be omitted from the closure at the phase boundary and
interrupted by the next navigation. Recorded useful-state timings cannot certify
complete styled delivery. INC-52 qualifies the earlier ordinary timing claim too;
its historical body/ZIP evidence remains intact. The new private phase barrier requires
the exact request/response set and completed body acquisition before advancing, with
failure deadlines and synthetic delayed-header/body regressions. Corrected browser
execution must still earn acceptance; no timeout, application policy or budget is relaxed.

The independent checker also exposed a real CDP shape its first fixture omitted:
an opaque-frame Document begins on its parent session and finishes on the child.
Acceptance requires a unique child whose parent session and target ID match the
request's session/frame ID, plus the same document request/loader identity. An
unscoped request-ID search is not sufficient. The initial18/19 checker test result
is retained in `transport-check-1VIvCp`; its missing-cache fixture shifted boundaries
and left a disk-cache flag, so that fixture is repaired without weakening cache proof.
The degraded run overlapped a short local Go verification; no timing is corrected
for host load or promoted despite the acquisition failures.

The separately frozen checker revision passes24 unit canaries in
`transport-check-WafRqs/`. It rejects the cache acquisition solely for absent cache
provenance, and rejects the degraded acquisition's incomplete bodies/closures.
Its owner-receipt check binds both probe hashes, snapshots and endpoints and verifies
clean exits/refused ports. Receipt SHA-256:
`edc75c2d49ef34f6f638881d3fa26fb03349acbd37bc2b783224af5210d794d6`.

The corrected producer uses a separately pinned `phase_wait.mjs`, per-phase monotonic
start/completion clocks and fresh frame-state checks after complete body acquisition.
Its10 barrier regressions and full Go race/quality/MCP checks pass in `verify-4fDYqD/`
(receipt `6fe8b99d6f90e8ec79d0e79318c731b9509ed68e7e2d154f091890f6cac77e02`).
Its external-volume replay, `degraded-gallery-bi3OiV/`, reports52/60 complete tasks
in150.447s, with no stylesheet-body deadline. All eight failed tasks retain EPERM
body-file errors; the run is not passed. Receipt SHA-256:
`41a53d3d86d4b2d5ea6ca3651c35f379ea12c85cd582587bb7850f97008cdb10`.
The probe/phase-helper hashes are respectively
`86f74d375248a9cca3bc6ac7146b3eb96112fe3b6ed736a026b018b7ff96f34e` and
`bc53923671c9718fd4f49b1c387c3945bbc498935b60b620ddaaac029603c637`.

## Corrected degraded-network acceptance

The unchanged corrected probe, selected Gallery binary/source and fixed budgets were
replayed with output/profile storage on the internal disk, after observing9.7GiB free.
This changes only the private test-artifact location, not app/server/cache policy.
Packet: `/private/tmp/appkit-theme-transport-4c59pO/degraded-gallery-NMAYAx/`.
All60 tasks pass in163.060s with780 complete request/response bodies and60 verified ZIPs.
Each of the780 actual requests has the expected session-bound shaping rule, including
iframe documents/assets and native POSTs. The29-case independent checker passes, and
the owner receipt proves clean Chrome/server exits, refused ports and unchanged inputs.

| Useful task endpoint | Largest observed | Fixed degraded ceiling |
|---|---:|---:|
| Navigation | 756.036ms | 3,000ms |
| Preview | 787.222ms | 2,500ms |
| Verified ZIP export | 266.367ms | 2,500ms |
| Reset | 756.585ms | 3,000ms |

The largest decoded view is141,623 bytes against256KiB; ZIPs are16,340 bytes against1MiB.
These are observed sample maxima, not population percentiles. Per-phase clocks require
complete body acquisition and stable rendered state; file read/retention time is included
conservatively. The actual machine/tool identities match the earlier ordinary assessment.
No Go verification from this session ran alongside this successful acquisition.

Task receipt: `35c1cbf3a35657f0c1ea56bcf22ef59bc25ee8b7bdc2c1c779df170c5ea352e9`.
Owner receipt: `322d39f5bb376a7bd11520d271d97228123729964351f687557c2e660ccc0157`.
Independent packet: `/Volumes/m2ext/appkit-theme-transport.du8Ja1dS/transport-check-4tl69m/`;
receipt: `527b8081af809fd8e405f69877180b3907848f8fc896c1b44d93db21abfb3bfc`.
This earns the degraded slice only, not cache, memory, cancellation, consumer adoption
or the whole browser-performance gate. Failed external-volume packets remain failed.

## Corrected ordinary first/repeat timing

Packet: `/private/tmp/appkit-theme-transport-4c59pO/cache-gallery-DXj2Vu/`.
The same corrected producer completes120 ordinary first/repeat tasks in63.969s,
with1,560 complete responses. The independent checker accepts every numeric, task,
body/ZIP, phase-clock and owner-cleanup check, but returns failure for the sole remaining
condition: `cache provenance absent`. Zero cache hits are not renamed a warm pass.
Observed maxima are182.515ms navigation,174.771ms preview,84.492ms export and336.489ms
reset, below the original1,000/500/500/1,000ms ceilings. Body/ZIP maxima match above.
This supplies corrected ordinary timing evidence without upgrading the historical
collector or claiming the entire cache-mode result passed.

Task receipt: `ddfacf081b2f3f203b4b8619099e25fac1d3ff2d36a4e7954253825f6c688a15`.
Owner receipt: `93156536022aa624cc8d0a8b3a1d32ed1ba528877ea7fc718187a6ab456b7fd9`.
The independently retained checker/29 tests and execution are in
`/Volumes/m2ext/appkit-theme-transport.du8Ja1dS/transport-check-y7HlB8/`.
Its verification receipt is
`cd190c627c34f8e6831d3419c9810b497858257f41f97e8fe74967d1cc70b5bf`.
An independently compared backup of both internal-disk packets is at
`/Volumes/m2ext/appkit-theme-transport.du8Ja1dS/internal-evidence-Y9OoDi/`:
268 files,121,566,379 bytes. Its manifest maps original paths to byte-identical copies;
only the ephemeral Chrome profiles are excluded. Originals are not moved or deleted.

## Cooperative callback boundary

`TestStyleGuidePreviewCancellationDoesNotLeakPartialSpecimen` exercises only synthetic
in-process requests, without a listener. A trusted callback writes a partial prefix,
then waits on the request context. The test cancels that request and requires its
marker and exact cancellation cause, HTTP500 without leaked partial output, and a
successful fresh request. All channel waits have failure deadlines, not timing-based
success assertions. This proves cooperative context propagation and atomic failure
output, not arbitrary callback CPU/I/O preemption or browser stop/retry behavior.

The original test is retained at `/Volumes/m2ext/appkit-cancel.Jtq9rl/`; review removed
an unnecessary listener, bounded its waits and moved the partial write before cancel.
The full style-guide race suite passes in
`/Volumes/m2ext/appkit-theme-transport.du8Ja1dS/verify-x5HamE/`, alongside quality-plan
and MCP package checks, relative links, scope budget and selected diff checks.
That packet retains the selected package files, token source, inventory, documentation,
runner, exact Go/Node executable bytes and command environments before execution;
before/after identities match. It is not a full transitive-dependency certificate.
Verification receipt SHA-256:
`4cb8e688d70eea75191ea721509a697dc3318e92b4a1ecda68cb16f2e104c6d0`.
Final source/doc reconciliation repeats the full style-guide race suite, phase-wait
regressions, quality/MCP tests, relative-link/scope-budget and selected diff checks in
`/Volumes/m2ext/appkit-theme-transport.du8Ja1dS/verify-vFGHqS/`; all pass with unchanged
selected inputs. It precedes only these receipt/backup pointers, not a runtime change.

## Remaining work

Complete cache acceptance; retain the negative Gallery cache result and both failed
degraded acquisitions. A separate
byte-derived [CSS-validator candidate](2026-09-17-gallery-css-revalidation.md) now
proves ordinary Gallery reuse, but its120 theme journeys still have no reuse. Preserve
the original result and investigate the opaque-frame boundary; editor/preview
no-store policy stays fixed.
Memory and browser stop/cancel/retry need their separate
fixed workloads. Real-consumer delivery and broader core quality gates remain open.
No runtime, dependency, consumer, installed browser, global configuration or deployment
change is made by this work.
