# Theme-guide browser Stop/retry: controlled acquisition

This is the missing cancellation lane of the fixed
[browser contract](fixtures/styleguide-performance/browser-contract.md), not a change
to application behavior, performance budgets or preview isolation. It uses actual
Gallery handlers behind a [one-shot delay fixture](fixtures/theme-cancellation/README.md).
Source files and consumer repositories stay unchanged by the browser workflow.

## Fixture and build proof

Private root: `/Volumes/m2ext/appkit-evidence/appkit-theme-cancel.F2rbr0oq/`.
`green/receipt.json` freezes repository-local Go/test/embed sources and selected tools
before race tests and compilation. Existing cached dependencies are used; this is not
a hermetic rebuild, released pin or installed application.

- Fixture, Gallery and styleguide race suites pass; build and input-identity checks pass.
- Build receipt SHA-256:
  `5c00db787591e23708c43d199f5c75ef151263d7d687db245e822bd6707f8ec3`.
- Executed fixture candidate SHA-256:
  `3db83cd377f0b79a8bed4e54f3db8c8eb556e1089455b367db6a4c121fbd3bc2`;
  the build retains308 repository-local source/test/embed files.
- Unit checks cover canceled non-delegation, byte-identical retry, untouched form state,
  ineligible/refused envelopes, busy-arm rejection, timeout and control authentication.
- The existing cooperative iframe-callback cancellation test remains separate: delaying
  an iframe after Preview commits cannot prove cancellation of an uncommitted POST.

## Retained setup failure

`/private/tmp/appkit-theme-cancel.r4uWv6Qe/cancellation-gallery-q1p6lj/` preserves the
first executed probe's pre-browser identity failure: `go version` inherited a local
Go environment containing relative `GOPATH` text. No probe receipt or browser task
was produced; the owner retains stderr and clean Chrome/server exits. The next probe
runs the exact Go executable with explicit `GOENV=off`, `GOWORK=off` and local toolchain;
no user configuration was changed. Pre-execution source snapshots remain separate.

## Escape diagnostic: failed

`/private/tmp/appkit-theme-cancel.r4uWv6Qe/cancellation-gallery-0gocJp/` retains the
actual first-cell task; receipt SHA-256:
`0dedef243c20d4dcfabfcb09c9060062a3345b0cebf15032b084b1cfb8eeec99`.
The12-cell requirement remains; one desktop/dashboard/light diagnostic was attempted
and11 cells are unexecuted. The initial unexecuted draft's review defects were repaired
before acquisition and established no application evidence.

The native POST starts at08:49:39.305Z; the fixture reports it held, and automated
Escape is sent at39.311Z. At49.321Z the fixture is still live and uncanceled, so the
ten-second cancellation poll fails. The fixture times out/delegates at54.307Z and the
POST returns200 at54.318Z, without a network failure. Context close follows at54.366Z.
An awaited CDP-detach interval is not separately timestamped; its causal role is unknown.

No download event/ZIP is observed; source hashes match. Those observations do not
replace the unreached cancellation/retry postconditions. Guard errors are absent.
Owner input identities, clean process exits and refused HTTP/debugger ports pass;
cleanup cannot turn this cancellation failure into a success. The failed packet remains
unchanged. Scoped preflight `probe-check-FmsXbO/` passes426 syntax/link/budget checks,
not browser acceptance or a full intent-doctor.

## Explicit browser-command diagnostic

A separate prospective `cdp-stop-loading` run uses `Page.stopLoading`, present in the
selected Playwright protocol definitions. It is not a fallback within the Escape run,
an application script or evidence of physical keyboard behavior. Its retained run is
`/private/tmp/appkit-theme-cancel.r4uWv6Qe/cancellation-gallery-Gn4eqa/`; probe SHA-256:
`a2a173b84bbb7024e3f61e48bc598a5ced986d37845f2593f176ee9dbb0416f9`.

The command runs at08:51:20.682Z; the server records cancellation at20.684825Z, with
no timeout/delegation. Playwright and raw CDP report the same aborted native POST.
The producer observes unchanged document/edits, no download, unchanged retry bytes,
candidate styles, verified ZIP members/digests and reset. All six task postcondition
groups pass, but neither the diagnostic nor the full gate passes.

Two unknown `other` targets attach at20.975Z; the guard refuses them and later refuses
to detach while they remain paused. This acquisition records the startup context ID
`41189737CFDAD02F6CCCB990830E172A` before closing its blank page, an empty list after,
then unknown targets with that same context ID after the task. It proves shared
context lineage for this run, not a target-creation cause or safe exclusion.

The owner exceeds its nominal180-second probe watchdog plus bounded cleanup because
the fallback awaits child `close` without a second deadline. Its terminal receipt
records probe SIGTERM and a Chrome forced-stop attempt; Chrome/server eventually exit0
and both ports refuse connections. Input identities are unchanged, but owner verification
is false. A separately frozen rescue refuses a now-missing server PID and sends no
signals (`rescue-6EN1fX/`); it does not repair normal cleanup proof.

`internal-evidence-QMlkTg/` byte-verifies90 files from all three acquisitions
(86,913,494 bytes), excluding ephemeral profiles only. Originals remain intact.
The original Escape failure remains independently visible; the full12-cell gate is open.

Keep failed Stop input distinct from cleanup-induced cancellation. Neither fixture
unit tests nor a prepared browser probe completes this gate. Unknown browser targets
continue to fail the guard; no isolation exception is authorized by this assessment.

## Bounded owner cleanup follow-up

The maintained `apps/styleguide/browser/owned_process.mjs` separates process exit from
output EOF/child close. Every grace, kill and drain wait has a deadline and retained
outcome. Signals target only the originally tracked ChildProcess, never a process group
or scanned PID. Forced termination, missing exit/close, signal errors and incomplete
output remain cleanup failures. If exit is still unobserved, the helper records the
unresolved PID, releases only local read handles and unrefs that exact child so the
owner can report failure; this does not prove the child died or authorize a later PID
kill without fresh identity verification. Spawn failure without a PID invents no child.

The private owner now notices a written probe receipt and, when the guard refuses
detachment, stops its exact owned Chrome before waiting for the probe's socket exit.
It retains connection-refusal details separately from workload success. `ok` on the
helper means lifecycle/capture cleanliness, not a successful task exit code.
`owned-check-9XdD5Y/` freezes the final helper/tests/owner and passes13 synthetic child
canaries plus owner syntax; receipt SHA-256:
`e6e471b602418d3fcb07e02c2c6df7e8f126696e6ec4042023b669e186ad0fb2`.
These fake events do not prove every real host failure path. INC-54 records the defect.

The new runs use `quality/receipt.json`, not the earlier green binary:

- Build receipt: `eb8586aa1b528334113997c68083a247aaf3b796a29e7b5f6a46d95b70e39ac2`.
- Server: `aef08dc9fd975e5f4a2a3edbf44e0c6250c32873206174a9c142a7376f1f050d`.
- Quality-plan/MCP/fixture race suites and selected input identities pass.

`cancellation-gallery-UbETZ8/` under the same internal temporary root retains the first
live bounded-owner replay. It finishes in2,378ms; Chrome/server exit0, probe exit1,
complete captured output and refused HTTP/debugger ports establish clean ownership.
All six task postconditions pass, but the unknown-target guard still fails.
Owner receipt: `423bae9db707bc9a4f7d60be11f9c67b32b4c52ecf4567eba9924ee21ee1542c`.
Probe receipt: `2dcf2dc0492b57956f6caa0822460bbe4cd50abb3d04328ea0b76ce2e5db0b8c`.
`internal-evidence-K4tYmV/` byte-verifies34 files/29,209,615 bytes; originals remain.

## Target-command lineage and retained repeatability failure

An opt-in guard trace now retains sequence-numbered Target commands/responses and raw
Target events with complete context IDs, plus probe markers before/after enumeration
and context close. It changes no target filter, network policy or resume decision.
`guard-check-gWLtMs/` passes three fake-WebSocket tests plus probe syntax, including
unknown-target refusal; receipt SHA-256:
`1dd86372490debfd1ce4c7f831a521536a37af6a65937abfc1314b05cc31f395`.

| Retained run suffix | Startup mode | One-cell diagnostic | Owner cleanup | Elapsed |
|---|---|---|---|---|
| `DFMapB` | blank page | passes; all six task groups and guard | passes | 4,741ms |
| `6gcoKE` | blank page | fails unknown-target guard | passes | 3,234ms |
| `4ISXuC` | blank page | fails unknown-target guard | passes | 2,667ms |
| `1pDGwc` | no startup window | fails unknown-target guard | passes | 2,687ms |

All use explicit CDP Stop, unchanged source and the same fixed task; each has11
unexecuted cells and no independent acceptance. The two blank-page repeats were a
bounded repeatability check, not retries until a pass. `DFMapB`'s probe receipt is
`9d8f59ef9a20508ac45fab9d5907992f4c9ff4899908e6bfb7d36ff57284584a`;
it neither fixes the intermittent issue nor completes the12-cell requirement.

In both failed repeats, protocol sequence20 sends `Target.getTargets`,21/22 receive
the two `other` attachments, and23 receives its response containing those targets.
They precede context close and use the recorded startup context, not the guide's.
This pins command/event ordering; it does not prove creation cause or safe exclusion.

The explicit no-startup-window control follows the launch choice present in the pinned
Playwright implementation. Its startup target observations are both empty, yet the
same sequence recurs after the task in a non-guide context. An explicitly closed
startup page is therefore not necessary to reproduce this failure. No startup-context
identity can be inferred for that empty-start control. All task postconditions still
pass; the guard correctly stays failed and never resumes unknown targets.
`internal-evidence-XcK7M8/` byte-verifies136 files/116,914,720 bytes from these four runs,
excluding only ephemeral profiles. Source snapshots and original packets remain intact.

## Remaining acceptance

The [minimal and phase controls](2026-09-17-theme-guide-target-controls.md) now retain
four clean no-app runs, an ordinary Preview failure and post-Stop failures before retry.
Cancellation, retry and downloads are not necessary; creation cause stays unknown.
Read-only unknown-target metadata remains inconclusive. Isolate editing/native submit
before a full matrix replay and independent checks. Cache/memory gates and propagation
remain open. G1/G4 and `core.themes-style-guide` are not complete. No keyboard, browser,
application or full-performance certification is claimed.
