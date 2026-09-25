# Native disclosure: one advertised scroll experiment

G6 diagnostic under [ADR-0026](../adr/0026-isolated-http-desktop-experiment.md),
[ADR-0033](../adr/0033-web-first-workflows-and-native-enhancements.md) and I8/I17/I19.
Status: the exact-bundle scroll workflow failed. A subsequent read-only structure
inspection succeeded without opening the disclosure; failure/retry remains unproven.

## Question and unchanged target

The [bounded-read native workflow](2026-09-16-native-profile-workflow.md#bounded-read-follow-up-profile-outcomes-pass-disclosure-blocks-retry)
stops at the exact laboratory disclosure: `AXDisclosureTriangle`, enabled, collapsed,
not expansion-settable, advertising only `AXScrollToVisible` and `AXShowMenu`.
No fixture child was captured. Its driver never scrolls or recaptures that control.
The receipt does not record geometry; it does not prove the control was offscreen.

Target remains
`/Volumes/m2ext/appkit-evidence/appkit-bundle-seam.tHv3fmlT/actual-artifacts/Relocated.app`, with the
[historical exact inputs](2026-09-16-private-bundle-runtime.md). The unchanged native
binary is `98cf0b52661e82d0a51ca2230b7f0c3289252e487b1b798fe4c4583da3350f1b`.
The retained failed driver source is
`d5100dbd8be5f71c78b3cb616d82ca414a15ad892e42c44f63783dc0137a7b37`.
Neither the app nor that original evidence is modified by this experiment.

## Bounded action and oracle

In a fresh private driver, permit one advertised `AXScrollToVisible` attempt on the
exact enabled disclosure bound to the original process/window. Retain before/result/
after observations, recheck process/window identity, and require the recaptured
element to match. Proceed through the existing `AXPress` path only if now advertised.
An already-open state still needs exact enabled actionable fixture controls; scrolling
or AX return code 0 is never an opening/task pass. Unknown/disabled/changed controls
remain refusals. Original validation, save, conflict, retry, Activity and lifecycle
outcomes are unchanged; missing prerequisites cannot become skips or successes.

No unadvertised action, input injection, HTTP/DOM substitute, permission request,
signing, installation, dependency, app-state hook or consumer change is introduced.
Existing owner authority covers the unchanged local synthetic bundle and real AX
controls, including its existing browser recovery. A new native run requires retained
source/configuration, passing preparation checks and review before execution.

This cannot establish physical keyboard/pointer behavior, general accessibility,
native media support, another bundle's behavior or completion of G6. Windows remains
owner-deferred. Preserve a failed result even if scrolling changes some metadata.

## Retained preparation and execution handoff

The three final first-party diagnostic sources are now preserved in the
[frozen source fixture](fixtures/native-disclosure-scroll/README.md); this 2026-09-18
preservation does not rerun or change the historical failed result below.

Private preparation: `/Volumes/m2ext/appkit-evidence/appkit-launchservices-scroll-prep.O8b0xs`.
The final driver passes36 pure selector/refusal/oracle checks. The existing lifecycle
suite passes12/12. An initial compile failure used an SDK constant unavailable on this
host; its source/stderr remain in `failed-compile/`. The final driver instead uses the
exact advertised `AXScrollToVisible` string. Neither result is native workflow proof.

The first prepared lifecycle copy still pinned the old workflow inputs and expected
them beside itself. Prelaunch review caught that incomplete handoff before execution.
A new private runtime root, `/Volumes/m2ext/appkit-evidence/appkit-launchservices-scroll-runtime.HSndzK`,
retains the originals and places the final inputs together. Only five workflow hash
constants change in its lifecycle probe; config changes only identify the runtime
handoff/authority. Limits, workflow outcomes and lifecycle oracle remain unchanged.
Static syntax and the same12 pure lifecycle checks pass; Sol independently reviewed
the driver semantics and the final integration. No app rebuild or modification occurred.

| Exact input | SHA-256 |
|---|---|
| Preparation identities, `frozen-inputs.json` | `60d8b0e434fc4031c42531d626d3d2e4592cb3d8d87011463fc3d32cb3ace39c` |
| Final Swift source | `f91b08749630de5122966f4d88c5ab7378cfd539825597beba2b121386ac5fbb` |
| Compiled workflow driver | `c3c8fdea26158ac2400537c91b1159745cbc7b332c8d51ea430bae7cd7873e10` |
| Runtime lifecycle probe | `d9614dcb7c734d0b3821a1b2bdb45cf6118fecaaa96c5abbfbba241b2644d032` |
| Runtime workflow config | `e0f6ed967f1465686ee7a89cc91ddb8870736efeab4f1772a14493f8dd1eeecf` |
| Exact invocation, `runtime-command.json` | `172cd147bf662e0ba26a9cf1378fbfbe9e11a0cd65f647e38aaab955b9a198f7` |
| Prepared handoff receipt | `c489918c0c087680fb543c8b4c239553781790063db6699a4c074f8d407e70b0` |

## Actual result: scroll is not an opening path

Root executed that invocation once, at22:50:19–22:50:28 UTC on2026-09-16. The retained
run is `runs/appkit-launchservices-7ti3ce/` under the runtime root above. Its prelaunch
receipt records exact host, source, bundle and command identities; the launch remains
normal `open -n BUNDLE`, without application arguments or environment overrides.

The visible Profile outcomes pass again:81-character invalid draft retained with its
validation error; corrected `Newer Workflow Casey` save; stale second-window
`Reviewed Workflow Avery` draft with the newer saved value disclosed; reviewed-draft
apply/save with Save disabled. Two transient invalid-element reads are retained before
complete successful observations, not counted as passed captures.

The one advertised scroll returns AX code0. After same-window/element recapture, the
disclosure remains enabled, `expanded=false`, `value=0`, not expansion-settable, with
only `AXScrollToVisible` and `AXShowMenu`. The original strict fixture wait fails:
`wait:fixtures-exposed:missing:Fail the next valid save once`. No fixture is armed;
native save-failure/retry and Activity are unperformed in this run. The independent
earlier Activity evidence is unchanged, not credited as part of this failed workflow.

The original close/reopen lifecycle completes. Before emergency cleanup, parent57238
and native57241 are absent and loopback port56181 refuses connections. Cleanup confirms
both absent; a later root PID/socket check independently observes absence/ECONNREFUSED.
All pinned inputs remain unchanged. LaunchServices parent exit status is unobserved.

| Retained runtime evidence | SHA-256 |
|---|---|
| `report.json` (failed) | `047bf5d01e8c9f78b15d637a07812a5d6453c4dee18587372f3b57c8d02117c0` |
| `workflow-report.json` (failed) | `4bcb71fbe1e36bbd3c2dc8d20a0968cc9490101ba4923dc0aa46018ac5eca6aa` |

Sol independently reviewed these receipts. Do not repeat this no-op scroll path or
claim an offscreen cause, unsupported opening action, general accessibility failure,
physical-input behavior or G6 completion. The next diagnostic is the exact disclosure's
accessibility structure and source contract; any app repair needs its own new artifact
and runtime evidence. The original bundle and every failed receipt remain intact.

## Structure diagnostic prepared; no new native run

The maintained `native_workflow_discovery.swift` now adds `laboratory_disclosure`
to its bounded read-only report. It retains the exact window/node ordinal, role and
subrole, enabled/expanded/settable state, API-advertised allowlisted actions and known
attributes, readable child count and geometry, parent/window equality and a bounded
same-window recapture. It rechecks the process/executable identity after capture.
Unknown actions, missing required attributes, incomplete traversal and unreadable or
mismatched identity cannot establish an observed disclosure. Unknown optional
attributes are omitted, not interpreted; this is not a general accessibility dump.

Private preparation: `/Volumes/m2ext/appkit-evidence/appkit-native-disclosure-structure.pkqSXV/`.
The original red self-test and subsequent compiler failures remain retained.
`final-2/` freezes the source, tests, Swift6.4 arm64 binaries and configuration before
executing the offline fixture. Its44 checks pass; stderr is empty. No GUI, native
target, bundle or live AX traversal ran. This does not change the failed result above.

| Prepared artifact | SHA-256 |
|---|---|
| Inspector source | `ddad2009177745034a1a36ca680a5ce60aff2999d1d4254537be201eddece2bd` |
| Fixture source | `743d96e3155f2c884154ec936406d78e9065b0ef295d46f4185eda0f516749b5` |
| Inspector executable | `948eec9505427bbeee796d2654bbbbed2b86b0d0ac064ea87b3ed60466dded93` |
| Frozen input manifest | `e9adba3785cac885acf2d3d3be73f529040397b673e66d9f5c13ce7d6d350497` |

A future run must independently pin the unchanged historical bundle/native hashes,
PID and ancestry, use the maintained LaunchServices wrapper and existing AX trust,
enforce an outer timeout, require complete observations and true identity facts,
and retain process/listener cleanup. A new app candidate still needs its own launch
authority. No opening action is inferred from this synthetic preparation.

## Actual read-only structure inspection

The unchanged historical bundle was inspected at 04:28:37–04:28:41 UTC on
2026-09-17 (September 16 locally). Private preparation and execution root:
`/Volumes/m2ext/appkit-evidence/appkit-native-disclosure-runtime.QXnsMm`;
run: `runs/appkit-launchservices-a7WNXV/`.

A private adaptation of the maintained LaunchServices wrapper pins and retains the
inspector closure, runs it after initial one-window readiness and before lifecycle
actions, and preserves every bounded observation. Root review caught missing
inspector-returned identity validation and late diagnostic retention; both were
corrected before launch. Earlier preparation packets remain retained. The final
15 offline tests pass: the original 12 lifecycle tests plus three classifier cases
covering multiple invalid inputs. These are not 15 native task cases.

The first inspection traversed seven nodes and found no named controls. It remained
`not-observed`, despite a complete traversal, and was not accepted as readiness.
The second traversed 91 nodes and produced this exact observation:

| Disclosure fact | Observed |
|---|---|
| Role / subrole | `AXDisclosureTriangle` / `AXSummary` |
| State | Enabled, collapsed; expansion not settable |
| Advertised actions | `AXScrollToVisible`, `AXShowMenu`; no `AXPress` |
| Geometry | Position `[1216,766]`; size `[879,20]` |
| Children | One reported child; neither failure checkbox nor Apply fixture exposed |
| Identity | Same traversal parent, window and recaptured element; exact PID/path rechecked |

Window/screen bounds were not acquired, so geometry does not establish whether the
control was visible or offscreen. The child count does not identify an opening action.
No scroll, disclosure action, input injection, HTTP/DOM workaround or Profile save
was attempted. This is a successful diagnostic, not successful failure/retry, native
workflow acceptance, physical-input proof or evidence of a Wails-specific defect.

The original New/close/reopen lifecycle then passed. HTTP parent 22177 and native
child 22179 disappeared, and loopback port 50977 refused connections before emergency
cleanup; cleanup again confirmed absence. Root's later check also found no selected
PIDs and `ECONNREFUSED`. Pinned inputs remained unchanged; LaunchServices application
exit status remains unobserved. No bundle, app or consumer source was changed.

| Exact retained artifact | SHA-256 |
|---|---|
| Resolved Node 26.3.0 executable | `56694c81b093cc8da273fa017cf91765b3653e5f64f16727976ffaa87b2b6b31` |
| Private lifecycle/diagnostic harness | `c386014f6db9043e68600f72d619442c5f86b2066038e97d7aaaa5f93f21b393` |
| `runtime-command-v2.json` | `29d4506154eb9fd47ab96a73fa78b17da757dba06e1316f46758869a2a236430` |
| Runtime `report.json` | `78be1deb7675d0a9949bb575890f39316fc5354fc8c8c1fac7967ec1e9c76751` |
| Runtime `disclosure-diagnostic.json` | `9b1e9f42b37813da4ddfe497111841c6ff5f57dafcc09f7c541122ba8b9fde36` |

Next evidence must distinguish physical/native activation from an AX-driver
limitation. Do not replace working native HTML semantics merely to satisfy the
driver, infer an unsupported opening action, or repeat the already-unproductive
scroll. A new app artifact needs separately authorized runtime evidence. The earlier
workflow failure, input/media gaps and Windows deferral are unchanged.
