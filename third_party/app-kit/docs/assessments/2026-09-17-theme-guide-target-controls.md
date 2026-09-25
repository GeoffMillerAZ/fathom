# Theme-guide unknown targets: minimal and phase controls

These controls narrow the [retained cancellation failure](2026-09-17-theme-guide-cancellation.md).
They do not repair the guard, identify a browser-internal cause or certify a target.
No app policy, source CUE, consumer, browser installation or signed-in profile changes.

## No-app controls

`apps/styleguide/browser/target_control.mjs` uses a fresh externally owned Chrome with
`--no-startup-window`, the existing pinned Playwright and unchanged CDP guard. Variant
`empty` creates no page; `blank` creates one incognito `about:blank` page with the same
viewport, reduced-motion and download settings as the theme task. Neither runs a Go
server, navigates to Gallery or downloads anything. The guard's allowed loopback origin
is an unused control origin, not a live data endpoint; this is not OS-wide containment.

Each control prospectively selects target enumeration at500/1,000/1,500ms after setup.
Unknown targets remain paused and fail; later observations would be explicitly missing.
The private owner executes retained probe/guard copies, binds schema/variant/endpoint
and snapshot identities, caps captured output and keeps workload success separate from
owned-process cleanup. Other selected tools/entry points have before/after identities,
not a hermetic dependency closure or a claim against malicious change-and-restore races.

Private owner/preflight root: `/Volumes/m2ext/appkit-target-control.ORiayyOU/`.
`preflight-5hvABs/` retains source before syntax and16 synthetic lifecycle/guard tests;
receipt SHA-256: `1411198ea0c7f83dc799973574b2c34ead394780572c55e538c09e153563147b`.

Runs below are under `/private/tmp/appkit-target-control.k1xJaQK9/`:

| Run | Observations | Targets | Diagnostic / owner cleanup |
|---|---|---|---|
| `empty-8Js85B` | 3/3 | none | pass / pass |
| `blank-I2zhbu` | 3/3 | owned blank page and two declared `browser_ui` targets | pass / pass |
| `empty-L4Tem4` | 3/3 | none | pass / pass |
| `blank-NHc01S` | 3/3 | owned blank page and two declared `browser_ui` targets | pass / pass |

This fixed alternating four-run batch records zero page requests, downloads, page
errors or unexpected pages. Guard errors are absent; exact input checks, complete
capture, Chrome exit0 and debugger-port refusal pass in each2.2–2.7-second run.
It shows the failure did not reproduce in these controls, not that empty or blank
Chrome can never create unknown targets. No theme-performance evidence is inherited.

## Fail-fast observations within the actual theme task

The cancellation producer now accepts `APPKIT_THEME_CANCEL_PHASE_TARGETS=on|off`,
default `off`. Opt-in observations follow the complete-response barrier for initial
navigation, retry, export and reset. Full targets, command order and before/after guard
errors are retained. A failed observation stops the task and labels later phases
unexecuted; it cannot produce an ordinary timing or complete cancellation pass.

The owner explicitly selects CDP Stop, no startup window and phase observations.
Both runs use the previously race-tested `quality-cleanup/` fixture candidate:
server SHA-256 `312bc03aad110cc9af78bb41bff56a51d77fcbcfb77211da37ca7d29c9e83280`;
build receipt `8b3dfe82623f4f242810d9bbf49b0c50d34825248dcc07a82b1a671a0f976270`.
`guard-check-hHF9MR/` retains the new probe syntax and three guard canaries;
`owned-check-bccPbc/` retains the private owner syntax and13 lifecycle canaries.
Those are harness checks, not an independent task oracle.

Runs are under `/private/tmp/appkit-theme-cancel.r4uWv6Qe/`:

| Run suffix | Initial enumeration | Enumeration after retry | Export/reset | Cleanup |
|---|---|---|---|---|
| `ezRNg0` | clean | two unknown `other` targets; guard fails | unexecuted | pass |
| `l1J8am` | clean | two unknown `other` targets; guard fails | unexecuted | pass |

Both initial and canceled-request postconditions pass. The native retry response and
required assets complete, but guard refusal occurs before retry acceptance is assigned.
Do not call that a verified complete retry/export/reset task. There are no download
events or export requests. Owner input checks, capture, Chrome/server exits0 and port
refusal pass; elapsed times are2,581ms and1,439ms. Each still leaves11 matrix cells and
independent cancellation acceptance unexecuted.

The probe receipt hashes are respectively
`19e18f727cbae1ef6581d62b7ce2055d5670ecb1cfa4e6e0cc223f54e7b1b1fb` and
`1e5410df2a5440fb799f2b2804021d3200b66c4a5b8a2bb62cde076c6d98b05b`.
Archive `/Volumes/m2ext/appkit-theme-cancel.F2rbr0oq/internal-evidence-xzYgHL/`
byte-verifies126 files/60,279,636 bytes across all six controls, excluding only ephemeral
profiles. Original packets remain intact.

## Ordinary Preview versus Stop/retry

A separate explicit `ordinary-preview` control now avoids fixture arm, Stop and state
requests. It uses a native preview, acknowledged export and reset with the same expected
candidate/ZIP/source assertions. Its12 cancellation cells remain unexecuted. The default
`stop-retry` path preserves cancellation/retry assertions and adds opt-in enumeration
immediately after verified cancellation, before retry. Neither is full-lane acceptance.

The private owner now verifies receipt schema, actual control selection, configured
and applicable Stop method, phase mode, origin/CDP endpoint, snapshot and selected file
identities before using a receipt. Missing/wrong-mode receipts fail. This is acquisition
binding, not an independent oracle. `owned-check-RZRpn2/` retains18 binding canaries,
13 lifecycle tests and owner syntax; `guard-check-fdzHRK/` retains3 guard canaries and
probe syntax. Both pass with pre-execution source snapshots.

The fixed alternating four-run batch uses the race-tested `quality-controls/` server:
SHA-256 `66464a7da898e336ade6ac7b8ca98037034d1fa358a4df76d5bfa50045a3a9eb`,
build receipt `9cc18ca61174c47c5406cf05a2f3227bd97f34028c29347a796b72f69a2411eb`.
The probe SHA is `70eaa6ac6121e65728392be06ccd5c94e711eed8d7053971970f8cbbaeb78f27`.
Chrome152.0.7977.84, Node26.3.0, no-startup-window and the guard are unchanged.

| Run suffix | Control | Observed task/guard boundary | Owner cleanup |
|---|---|---|---|
| `TXA52Q` | ordinary Preview | initial/preview/export pass; reset enumeration fails before reset acceptance | pass |
| `SEyzgt` | Stop/retry | cancellation passes; Stop enumeration fails before retry | pass |
| `ZiRaVU` | ordinary Preview | all five task groups and guard pass; separate diagnostic only | pass |
| `c8jogh` | Stop/retry | cancellation passes; Stop enumeration fails before retry | pass |

Runs remain under the same internal temporary root. Elapsed owner times are1,915,
1,175,1,509 and1,222ms. Exact inputs/source, complete capture, exits0 and both refused
ports pass in all four. `producer_complete` and full `complete` stay false, including
the ordinary diagnostic pass. One clean sample does not erase the other failure.

Probe receipt SHA-256s, in table order:

- `18ddcaa91fff86f64821d1395d7e61112b7dba2666028506b9493feaf63a42b5`
- `1f9f0fe6863ccc99fbc8a4023a98f922e706351d016d33df091ab3e7c0367eb8`
- `6486f0675d41bda586c2ec91ce68d28fe6acb1abd2ebd2b4e334c6396a7f71cc`
- `a6eba549df072de54411adc2bfa243d061b6e15cfa38211f7e590f19d3ad5a93`

Each failing pair is root-attached in a context distinct from the guide. The unknown
attachment events fall between the relevant `Target.getTargets` command and response:
seq26/27–28/29 for ordinary reset, seq12/13–14/15 for both Stop observations.
Both Stop failures precede retry/export/reset, with unchanged baseline iframe/edits.
The ordinary failure has no cancellation or Stop. Thus none of cancellation, successful
preview replacement, retry, download or reset is individually necessary. Enumeration
brackets the observed attachments, but also stays clean repeatedly; cause is not proven.

## Read-only inspection of paused unknown targets

An additional explicit diagnostic keeps those targets paused and fails unchanged. Its
four allowed metadata queries per target have5s bounds; at most two targets are inspected,
with extras and every command result/error retained. No policy, resume, navigation or
script-injection command is added. Owner binding now also checks the inspection option.
`owned-check-hsrFA6/` passes20 binding and13 lifecycle canaries; `guard-check-GHaTs4/`
passes3 guard canaries plus syntax. Probe SHA:
`c917e2fb7d9003b5442cd488dc17aec49c2062db63e033dc01c25dafa1c4308c`.

`LCtujr` again fails at the post-Stop observation and retains two unresumed `other`
targets in a non-guide context. Both `Target.getTargetInfo` replies still say `other`;
both window queries return `Browser window not found`. Both schemas list Page/Network
domains. One frame query returns opaque `about:blank` (`securityOrigin` is `://`); the
other times out at5s. Listed domains therefore do not prove responsive commands, an
internal-UI identity or applicable network containment. The guard remains failed.
The6,673ms run still has verified owner cleanup, exact inputs and source preservation.
Its probe receipt SHA is `ddb104d1acf05a0b6d2be03e4c622edcc45247adb071c748805b102096344b9f`.

Archive `/Volumes/m2ext/appkit-theme-cancel.F2rbr0oq/internal-evidence-RtvIvo/`
byte-verifies166 files/145,492,913 bytes for these five acquisitions, excluding only
ephemeral profiles; originals remain intact.

## Idle and edited-but-unsubmitted controls

The prospective five-run sequence is idle/edit/idle/edit/Stop, all with phase
enumeration on, metadata inspection off and no startup window. The two no-submit
modes require verified navigation and preserve a blank review digest, disabled export
and unchanged baseline iframe. Idle observes after200ms and another1,500ms; edit
observes native selection, typography, radius and accent, then another1,500ms.
Neither mode arms/polls the fixture, submits a form, stops, exports or resets.
Both assert exact navigation-only response bodies and unchanged isolation policy.

The new `quality-preview-controls/` build is distinct from the previous batch:
server SHA `0569eccc70afcf7d8d46ce1ae3df1059c27de3e12fce0356489b335991ba30e5`;
build receipt `1e111c61545e70f0ebe3ff3c1c2a12971a13557c318d7a4cbfbf6889deb4de70`.
The frozen probe SHA is `a10288ee85fa0000a8ede9a9c78128a53f52c17706f93b8afa5bde6722a1d2c3`.
`guard-check-sD07HK/` passes3 guard canaries and syntax; `owned-check-Md6uMG/`
passes22 receipt-binding and13 lifecycle canaries. These are not independent task acceptance.

| Run suffix | Control | Observed result | Owner cleanup |
|---|---|---|---|
| `WplmQd` | idle | all3 target observations and no-submit postconditions pass | pass |
| `qtGzqt` | edit | all6 observations and no-submit postconditions pass | pass |
| `9aqndm` | idle | all3 observations and no-submit postconditions pass | pass |
| `UolTbU` | edit | all6 observations and no-submit postconditions pass | pass |
| `GCL7hj` | Stop/retry reference | canceled-request checks pass; post-Stop target guard fails before retry | pass |

The first four retain zero POSTs, downloads, workers and guard errors; all12 cancellation
cells are unexecuted. The reference retains two paused `other` targets and explicitly
unexecuted retry/export/reset. All full producer/completion flags remain false.
Elapsed owner times are3,412/2,584/2,685/2,601/1,149ms. Original failures remain intact.

An independent cross-run audit confirms all17 selected file name/path/size/SHA tuples,
the retained owner script, server build receipt and runtime versions match across all5;
every selected retained/live byte check passes. Normalized selected-file digest:
`a83e6b5f882cf81ac6721908d987ea4738b129ce3e080a7bf9099a5a63ab7b5b`.
Per-run origins/profiles/IDs/timestamps appropriately differ. Stop's two unknown
attachments occur at protocol seq13/14 between enumeration command12 and response15,
in a context distinct from the guide. That context's owner and causal mechanism remain
unknown; clean editing samples do not establish a general exclusion.

Probe receipt SHA-256s in table order:

- `4cedbe5b52c48a5113434741dff45a6237ab8fc22e263a3b3851ee7a9abcb714`
- `561f7c1b317d6d2fa700cbcc9a87b44650805363366c5c390195413dced607fb`
- `13ed63eff7a5aaf8ba9b8f14cbdb0374c9fc50c659569c9d128e9687d93310fe`
- `4d3b690e16f2b2f7213026a85a1c479fe2fc5d0dcc920be223230251a3f6f397`
- `3b82a259f0be71fa36a72be4a0d85561b3b99ff3ec3176f49d115a5e2df13542`

Archive `/Volumes/m2ext/appkit-theme-cancel.F2rbr0oq/internal-evidence-nnzCRS/`
byte-verifies160 files/144,995,080 bytes, excluding only ephemeral profiles; originals remain.

## Next evidence needed

Do not keep rerunning full theme matrices while target classification remains unresolved.
The fixed no-submit samples do not reproduce the failure, while the same-batch Stop
reference does. This narrows the next diagnostic to submission/loading, not a proven
cause or proof that editing can never trigger it. Retain target/context/time lineage.
If pursuing protocol support,
prove required commands and denial behavior separately before admitting any new target
type. A reported domain, blank URL or different context cannot justify an exclusion.
Keep unknown-target refusal and original failures; no browser/app policy or historical
budget changes. Eleven cancellation cells, independent acceptance, cache/memory and
real-consumer propagation remain open; this diagnostic is not a core-quality promotion.
