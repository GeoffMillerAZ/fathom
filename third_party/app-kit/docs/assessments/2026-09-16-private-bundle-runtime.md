# Private macOS bundle runtime probe

G6 work under [ADR-0068](../adr/0068-prepare-private-unsigned-macos-delivery-first.md),
[ADR-0047](../adr/0047-process-owned-http-and-native-recovery.md) and I8/I9/I17/I19.
This record separates synthetic harness checks from actual delivered-app execution.
Windows, consumer migration, installation, signing and OS-policy changes are out of scope.

Later [unchanged-bundle Profile/Activity AX evidence](2026-09-16-native-profile-workflow.md)
records scoped user-task outcomes and remaining failures separately. It does not rewrite
the historical harness/lifecycle receipts below or certify the full delivery matrix.

## Implemented boundary

The [existing delivery probe](../../spikes/wails-gallery/NATIVE-DELIVERY-PROBE.md)
now accepts an explicit all-or-none bundle selection: launcher path and launcher,
manifest and plist hashes, in addition to the existing server/native/AX-driver pins.
It validates private canonical fixed paths and bounded metadata, inspects helper
quarantine without changing it, runs exactly `-check`, then rechecks unchanged bytes.
Only then may it execute the same in-place launcher with no arguments. The launched
PID must become the exact pinned HTTP parent before normal native-child discovery.
The harness does not extract the server/native resources. The parent's existing
private companion staging remains part of ADR-0047.

The AX oracle is unchanged: one window/New enabled; two/New disabled; close/reopen;
close both; parent exit0, native absence and loopback refusal before forced cleanup.
The test-only sandbox denies the browser opener and both owner source roots, with
positive refusal canaries. Existing Accessibility permission only; no physical keys,
browser-menu action, permissions grant or other application's UI is targeted.

Direct bundle-executable execution is not Finder/LaunchServices acceptance, DOM-task
parity, Gatekeeper assurance, manual update/rollback or resolution of Cmd+N/media failures.
The [full delivery matrix](../design/web-desktop-parity.md#private-unsigned-macos-delivery)
remains binding. Normal close must pass independently of emergency cleanup.

## Retained synthetic observations

Initial root `/Volumes/m2ext/appkit-evidence/appkit-bundle-runtime.uP1jLHLQ/` retains the executed
source/configuration and a13/13 synthetic pass, with no skips or native launch.
That suite includes an inert check-only refusal: no no-argument launch and no bundle
executable extraction. Its same-PID helper assertion is not actual exec evidence.

Review tightened the private outer runner to require a matching test-mode receipt
with exit0 and null error/signal before native execution. The old wrapper remains
in `tests/run.mjs`. A second review identified an early-discovery cleanup gap, not
an observed Gallery leak. The synthetic parent-only cleanup canary retains an orphan
failure under `/Volumes/m2ext/appkit-evidence/appkit-native-cleanup.KhPtH8/red/`.

The cleanup helper now binds a still-live parent, private direct-child path/hash and
confirmed ancestry before signaling. Parent exit before any child observation records
native absence as unknown, not success. Unknown identities fail closed; a provisional
`sandbox-exec` identity is not silently authorized. Test teardown also binds identity.

Root's frozen replay under `/Volumes/m2ext/appkit-evidence/appkit-bundle-runtime-replay.uhwFzg1N/tests/`
failed14/15: the correct-digest cleanup call received `ps failed` after the earlier
wrong-digest refusal passed. Both fixture processes had been observed alive. Exact
sources, stdout and report are preserved. No native app was launched; the prerequisite
failure withheld native launch. The cause remains unknown, not attributed to the app
or an OS defect. Earlier successful runs do not override it.

A subsequent agent revision split refusal and success onto separate fixtures and
reported stabilization without establishing a cause. Root rejected that inference,
restored the original refusal-then-success sequence on the same fixture, and retained
the independent refusal case as additional coverage. A test teardown's empty `ps`
result now also requires actual process absence. Bounded process-inspection diagnostics
retain PID, exit, signal and sanitized stderr if this fails again; they do not silently
retry wrong identities. This is [INC-48](../intent/incidents.md), not a resolved OS race.

The resulting frozen suite passes16/16, zero skips. Its exact source then runs the
native probe below. That pass is separate evidence, not a rewritten historical result
or a general repeatability claim.

## Exact app inputs and current status

Selected bundle: `/Volumes/m2ext/appkit-evidence/appkit-bundle-seam.tHv3fmlT/actual-artifacts/Relocated.app`.
The [assembly assessment](2026-09-16-private-bundle-seam.md) identifies the unchanged
launcher and independently built historical server/native pair. These are not today's
entire working-tree UI. The manifest declares macOS14.0; no runtime-on14 claim follows.
Observed host: macOS26.6.2 build25G83, Darwin25.6.0 arm64, Node26.3.0.

## Actual in-place bundle lifecycle

The bounded run completed on2026-09-16 at18:51:49–18:51:51 UTC. Seventeen read-only
quarantine inspections and both sandbox refusal canaries precede the launcher's
successful check-only result. In-place launcher execution with zero arguments becomes
the pinned HTTP parent at the same PID2024. Its verified private native child is PID2026.

Twelve bounded AX receipts cover initial one-window readiness, the two-window cap,
close/reopen and final close. Two inspect observations see transitional one-window/
New-disabled states; bounded readiness waits subsequently observe two windows. The
fixed expected final states are unchanged. Parent exits0 without a signal; native
absence and listener50593 refusal are established before emergency cleanup. A separate
post-run process check finds no Gallery launcher/server/native or AX driver process.
Original bundle resources, driver and executed source identities match afterward.

Private evidence root: `/Volumes/m2ext/appkit-evidence/appkit-bundle-runtime-diagnostic.BYetZkUs/`.
`tests/` and `native/` retain exclusive source/configuration/wrapper snapshots;
`native/appkit-native-delivery-bF6GZI/` retains the policy, check receipt, PID transition,
AX receipts and bounded server output. The standalone driver retains its earlier
source/build provenance; no Swift recompilation is claimed.

| Artifact | SHA-256 |
|---|---|
| Earlier14/15 replay report | `7a1ad69ea360a3a57b9810c89c0b3354b02c4f37fcbdc5b4649a7051f9c1a3ab` |
| Final16/16 test report | `2475f6e1020f7c52de383e20dc987bd1657491d7e409fd6498695fef969df175` |
| Native outer report | `55d14bd9675d17c9484fa9ddb5e790e1df8db46f45034a7b053c7ee04626f4bc` |
| Native lifecycle receipt | `d8055bfc29b08abbdb18f7f6cd1b663a669a86643bcfc119b59587a7dc429f86` |
| Executed probe | `8562d04f8b73d6878822728c0c95ab6ffc3752f074cb6eb3cc4ede826af35fc3` |
| Executed test oracle | `2ea680f364d4f410295b12b10dcf09bbebe30b67c5561ceddf938f8b3363684b` |

## Normal LaunchServices controls

The owner separately authorized normal local opening of this same staged bundle,
including its existing local synthetic browser recovery, without installation,
signing or policy changes. These controls are not the sandboxed direct run above.

The first exact request used `open -n -a BUNDLE` with private stdout/stderr files,
`--stdin /dev/null` and an explicit private `TMPDIR`. Both preflight and input pins
passed. At19:00:08–19:00:09 UTC, `open` exited1 with LaunchServices error `-10810`;
app logs were empty. Retained OS logs show launchd's `posix_spawn` of the launcher
failed with `EPERM` before execution. The narrow security-process log query adds no
cause. Post-run exact-path process checks found no surviving test process.

Read-only `codesign --verify --strict --verbose=4 BUNDLE` separately exits1:
`code has no resources but signature indicates they must be present`. Signature
description reports the existing linker/ad-hoc identity `a.out`, unbound plist and
no resource seal. No signing or attribute removal was performed. This consistency
failure does **not** establish why launchd refused that request.

The minimal control at19:09:29–19:09:31 UTC omitted the redirections and environment
override: `open -n BUNDLE` exited0, and the exact pinned HTTP parent PID22082 started
with native child PID22105. Thus the first refusal is not proof that the bundle cannot
start through LaunchServices or that signing is necessary. The changed request options
have not been isolated individually; no specific option is established as the cause.

The minimal harness then failed: it required a canonical executable spelling, while
macOS supplied `/var/folders/.../appkit-native-companion-2610381809/gallery-native`
resolving to `/private/var/...`. Its cleanup repeated that refusal; the original failed
receipt is retained. Its later, unexecuted listener regexp also has an escaping defect;
this frozen runner is not ready for reuse. Neither defect is attributed to Gallery.

A separately snapshotted owned-cleanup script verified the explicit system alias,
private directory/file ownership and modes, both executable hashes, PID paths and
direct ancestry. It observed one loopback listener and AX `windows:0` (not readiness),
then signaled only those bound processes. Both PIDs were absent and the listener refused
afterward; a separate `ps` check confirmed absence. This is emergency cleanup evidence,
not a successful normal-close lifecycle or a reason to relabel the control as passed.

Private roots retain source/configuration and original results:
`/Volumes/m2ext/appkit-evidence/appkit-launch-services.t7dyxdyA/` (`preflight/`, `run/`, `diagnosis/`),
and `/Volumes/m2ext/appkit-evidence/appkit-launch-services-minimal.9YVSyElK/` (`preflight/`, `run/`,
`owned-cleanup/`). Original app bytes and driver pins matched after both launch attempts;
attribute names were observed, not a proof of unchanged attribute values.

| Artifact | SHA-256 |
|---|---|
| Instrumented request failure | `f5c138b3293ed3a9cfd2d96d86207a0f29ebd9691d81a8eb1e6d888d87a5ae3c` |
| Read-only diagnosis report | `52fbe9a2b37e98081e4ae6be85a08cc438f121d7fad0109b4621457e6d604c61` |
| Minimal-control failed receipt | `ea5f378c2fc5af81deb3c648957c0ecd88a676832fae6a80f728f9f80cb6d356` |
| Minimal-control cleanup receipt | `ae4f1d984ff82fcb7f40289c085491105ece45fb81d2fe8d3f3323135c0211d6` |

ADR-0068's host-refusal trigger was reviewed: retain the failed exact request, but do
not infer blanket OS refusal or change HTTP/native topology from it. This established
the need for an alias-aware, regression-tested normal-launch harness with independent
cleanup; the follow-up below records that work. Strict signature verification remains
failed; any signing experiment still needs separate authority.
The matrix explicitly requires normal-environment path and cleanup evidence (INC-49).
No consumer, dependency, real data, deployment, commit, installed binary, signing
credential or global configuration was changed. Finder interaction was not tested.

## Maintained LaunchServices lifecycle follow-up

[The maintained probe](../../spikes/wails-gallery/LAUNCHSERVICES-PROBE.md) now accepts the
same explicit input pins and required bundle quartet. Its minimal `open -n BUNDLE`
request changes no app environment, redirects no app output and uses no test sandbox.
Sources, helper, driver, config and preflight are snapshotted before launch; bundle
plan/metadata, source hashes and selected identities are checked independently.

Native discovery retains the observed `/var` spelling for the AX driver while checking
its exact `/private/var` resolution, private ownership/modes, executable digest and
direct ancestry. Arbitrary aliases remain refused. Process path, digest and recorded
start stamp are rechecked before signaling; this is local ownership evidence, not a
hostile same-user race-proof security mechanism. Cleanup can rediscover a launch-owned
parent independently, attempts separately bound processes despite a parent-stop error,
and retains unknown native absence when authorization fails. A scenario or late identity
error cannot produce a passing outcome. These are harness changes, not app changes.

The initial outer wrapper retained six passing helper tests but refused its own summary
because Node26 selected the spec reporter rather than TAP. No native run followed that
failed prerequisite. Its snapshots remain at `/Volumes/m2ext/appkit-evidence/appkit-launchservices-proof.DUTUB66q/tests/`.
The next frozen wrapper explicitly selects TAP: eleven tests and the exact normal
lifecycle passed at19:26:12–19:26:15 UTC in `/Volumes/m2ext/appkit-evidence/appkit-launchservices-replay.WYpqCeAg/`.
Two safety edits landed after that runtime finished; its pass belongs to runner
`0f6c798919dbd57b8be78395bfbf2fe4feaad9bb405a3155313c3a05e7b7c7fc`, not the later file.

Root froze further edits and added the same-oracle absence-inspection failure case:
the earlier source fails11/12, while final source passes12/12, with no skips. The final
source also binds reads to opened-file identity and preserves known-parent cleanup
when an absence observation fails. Final exact-source lifecycle replay completes at
19:28:47.734–19:28:50.428 UTC on the previously named macOS26.6.2 arm64 host.

| Final normal-opening requirement | Observation |
|---|---|
| Request | Exact unchanged bundle; `open -n` exits0, application exit status unobserved |
| Bound processes | HTTP parent31651, native child31653; pinned bytes and standard path alias verified |
| Initial readiness | First AX observation is zero windows/disabled controls; a later inspect reaches one/enabled before any New action |
| Capacity and reopening | Two windows/New disabled; close to one/enabled; reopen to two; close both |
| Normal shutdown | Both processes absent and listener56340 refuses before emergency cleanup; no forced signal required |
| Retained limits | Thirteen AX observations, no shared DOM tasks, physical keyboard input, browser-menu action or signing |

Final private root: `/Volumes/m2ext/appkit-evidence/appkit-launchservices-final.rIF8XrZU/`.
`red/tests/` retains the old-source failure; `tests/` the unchanged12-case pass;
`native/appkit-launchservices-0ij0HR/` the actual preflight, source/configuration and
AX/process/normal-close observations. App, driver and harness identities match after
execution; the direct-executable helper remains unchanged. Separate exact-process
checks find no remaining selected bundle parent or driver. No lifetime status was
inferred from `open`'s exit code.

| Final artifact | SHA-256 |
|---|---|
| Old-source12-case report | `b1a43418caf4218bb735b2412376cc1a6d276af425e14864236983da124d7b57` |
| Final12-case report | `3463815a8e0b0c357332afefbc077db84542db83ca0c6cd8b83a2179d2c6000a` |
| Final native outer report | `71d46c4bd31d71fab9a6aa69d4f38f4c19e0fff9b96502946c6081ee5219c524` |
| Final native lifecycle receipt | `dfd6d4edcb4431450cb93583849369acb10e573b58a4a96709fb868db43af053` |
| Executed maintained harness | `81afc7d76176d8b180d10273d59acd3db5b19b889e1e4f9de8a3b23b66336833` |
| Executed test oracle | `c88b2ca675dd4bc122ba61dc79577d9dd691e19042c24ba1f48763691b4f40a4` |

INC-49 now points to maintained regressions and actual normal-opening lifecycle proof.
Earlier instrumented-opening refusal, strict signature failure, one-off harness failure
and intermittent direct-harness observation remain retained with their original limits.
G6 next requires exact-bundle shared workflows and authorized failed-start/manual
replacement/rollback, alongside the separately open native input/media gates. No
installation, signing, dependency change, consumer edit, commit or policy change occurred.
