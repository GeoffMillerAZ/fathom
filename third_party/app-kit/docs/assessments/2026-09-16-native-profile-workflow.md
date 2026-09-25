# Unchanged private bundle: native Profile/Activity evidence

Current result: **partial native workflow proof**, not a complete workflow pass.
Validation, corrected save, stale-window conflict and reviewed-draft recovery pass in
the bounded-read follow-up below. The independent Activity journey also passes live
updates, pause/reconnect and unsaved-draft retention. Failure/retry remains blocked by
the laboratory disclosure's missing advertised opening action. A later
[single advertised-scroll experiment](2026-09-16-native-disclosure-scroll.md) preserves
that failure: scroll returns AX success but exposes neither an opening action nor
fixture controls. It adds no native failure/retry pass.

Owner-authorized local test of the historical private bundle, through real macOS
Accessibility controls using existing trust. No installation, signing, policy change,
external site, real data or application modification. This is separate from the
[new candidate's cold builds](2026-09-16-current-gallery-candidate.md).

Target: `/Volumes/m2ext/appkit-evidence/appkit-bundle-seam.tHv3fmlT/actual-artifacts/Relocated.app`.
Its exact launcher/server/native/manifest/plist hashes remain the
[historical runtime identities](2026-09-16-private-bundle-runtime.md).
The native executable remains SHA-256
`98cf0b52661e82d0a51ca2230b7f0c3289252e487b1b798fe4c4583da3350f1b`.
No result here transfers to a new build, full accessibility conformance or physical
keyboard/pointer acceptance. Cmd+N and media failures remain separate.

## Initial discovery and readiness follow-up

First private root: `/Volumes/m2ext/appkit-evidence/appkit-launchservices-workflow.x6HCQWiK/`.
The original 12-test lifecycle suite passed against the private variant. Root then
executed its frozen wrapper at 20:12:53–20:12:57 UTC on 2026-09-16.
The inspector saw one window and seven nodes; none of its ten named controls matched.
That is a completed bounded observation, **not a successful workflow or proof that
the controls are absent from the application**. No field or form action occurred.

Second private root: `/Volumes/m2ext/appkit-evidence/appkit-launchservices-workflow-v2.k8zXsfgJ/`.
Only the private runner changed: retain the initial observation, wait a bounded
500 ms and observe again before any menu mutation, then observe after the existing
first New/Close cycle returns to one window. No extra menu action, AX attribute write
or app hook was added. The unchanged 12-test suite passed before actual execution at
20:19:10–20:19:16 UTC.

| Observation | Nodes | Selected actual result |
|---|---:|---|
| Initial one-window readiness | 7 | No named controls matched |
| After 500 ms, before menu action | 91 | Display name = Alex Morgan, settable; Activity link actionable |
| After existing New/Close cycle | 91 | Same selected control states |

In both populated observations, Save changes is disabled, consistent with an unchanged
draft. The Profile name matches both a heading and a link: this conservative discovery
classifier reports ambiguity rather than selecting one. The laboratory disclosure is
an enabled `AXDisclosureTriangle` but does not advertise `AXPress`; hidden fixture and
Activity-page controls are not expected to be present yet. No workflow action is
credited by these metadata observations.

The timing result supports waiting for required UI readiness instead of treating a
native window's existence as form readiness. It does not identify a WebKit/Wails
cause. `complete` describes this bounded `AXChildren` traversal; it does not certify
all accessibility relationships or the application's full tree. Names are exact
AXTitle/AXDescription matches, not a general accessible-name computation.

Both runs preserve the original lifecycle oracle and normal-close checks. First run:
parent 50375/native 50377 and port 59503 absent/refused. Second run: parent 61188/native
61207 and port 60113 absent/refused. These were observed before cleanup; cleanup also
reports both processes absent. A later exact-PID check found none of the four PIDs.
LaunchServices owns the parent, so its process exit status remains unobserved.

| Retained artifact | SHA-256 |
|---|---|
| First wrapper | `47be95b7828bde3b5eda9222fdf4b2b684d49464ba5322baa0b3354ccb559732` |
| First private runner | `c15ab85482940442278aaecce21b99ef0087ccee6891ea2449fb2bd5ed690007` |
| First inner receipt, `runs/native/appkit-launchservices-6t19qc/report.json` | `0d9e78529b65b4c61ad76d03e9e1b09c682d59ab57e6a66b558f12a5b69f83d8` |
| Second wrapper | `21ce2063cfa6d4da863dd2b463cbd65bb4e9b2ccf12b4acb37737c83a55fd187` |
| Second private runner | `5f266716f8c7248d545e2cdcb86ad4b2d19f485d1d92b036dc0af9b1f72efdf3` |
| Second reviewed config | `fad37db23bb1aaeb40da68b37cfc3e184b35391632a66698d6ce5e982ce1e2ad` |
| Second synthetic receipt | `5fce3ecf783715a510dafb086abaedb40b7cb7f278fa2ac55416810844c36cab` |
| Second inner receipt, `runs/native/appkit-launchservices-shZprG/report.json` | `cc0a2025a1217814a8e86310ac50881378e6ad081c408e4c855b4ace28047022` |
| Same compiled inspector in both runs | `37afa1bbae1d871e63c1a57f3395a47357c1299e6b1c398f129aafe9ca3153dd` |

The first preparation handoff's statement that `runs/native` was absent became stale
when root executed the frozen runner. Runtime claims use the retained preexecution,
command and actual-result receipts, not that later preparation summary. Original
sources and historical receipts are preserved, including this limitation.

## Actual workflow attempt: value setter did not change the field

Private root: `/Volumes/m2ext/appkit-evidence/appkit-launchservices-workflow-actual.UvtkUpfB/`.
The frozen workflow driver has 20 passing pure selector/refusal/oracle checks; the
unchanged enclosing lifecycle suite passes 12/12. Those are preparation checks, not
live user-task passes. The driver retains two distinct AX window references, resolves
exact control names and roles, checks identity before mutations, records command
attempts/results and requires actual visible state after each action. No app hook,
HTTP/DOM shortcut, keyboard/pointer injection or permission request is present.

Root executed the normal LaunchServices wrapper at 20:40:30–20:40:39 UTC. The baseline
observed 182 nodes across two Profile windows: both showed Alex Morgan and a disabled
Save changes button. The first 81-character `AXSetValue` command returned AX success
(code 0), but after the bounded wait **both fields still showed Alex Morgan and both
Save buttons remained disabled**. The workflow correctly failed at `invalid-input`;
it never pressed Save or reached validation, conflict, retry or Activity.

This is an observed automation-path failure, not evidence that a person cannot edit
the app or an identified Wails/WebKit cause. API success is not a UI outcome. The last
capture also shows the laboratory disclosure advertises only `AXScrollToVisible` and
`AXShowMenu`, with `expanded=false` and `expanded_settable=false`; no disclosure action
was attempted. These metadata do not prove an alternate activation path works.

The wrapper retained raw stdout/stderr before schema validation, the parsed failed
workflow and full command/source/configuration identities. It completed the original
window-close/reopen lifecycle despite the valid workflow failure, then returned exit1.
Parent3348/native3351 were absent and port62689 refused before cleanup; cleanup also
reported both absent. Input hashes remained unchanged. The failed receipt is preserved.

| Actual workflow artifact | SHA-256 |
|---|---|
| Frozen wrapper | `e6f3052ba02c9b74a8e63e435c0eb15945a1e5a83d15b3074c909463715f42b1` |
| Private lifecycle runner | `6132e827d21a22a966ae517e8396230debda837e17043d40e0874dc429900e8f` |
| Swift driver source | `0757d3b3f8eae07e771eae778aabfdc9b2cbfb35344aeefa4c1e5c5af1adab85` |
| Swift driver executable | `1c6fb95b85daa1e1dcda883526f902caa0e3b931afd8ac142d73fe03061a021f` |
| Reviewed native configuration | `ed95fddc9da6a820cf1f09786a2bc2fca9b109ae9b893732fa564bb9319d0160` |
| Inner `appkit-launchservices-MtrMAO/report.json` | `65d36247dcc931efa783a3bfba9f77442328aa4ce2a6079b811c52f32ad2fdd0` |
| Its `workflow-report.json` | `f027298bb4cee22f5e1b23499ca4a423ffbb930ee2e4d212acc67c5af90945af` |
| Raw driver stdout | `a05ab6f18fbf86dec316119d44b63d31c3ed4589d76eeca4dac58606e6527ebd` |

## Advertised field-press follow-up

Fresh root: `/Volumes/m2ext/appkit-evidence/appkit-launchservices-workflow-focus.w2QDKi9o/`.
The only semantic changes are a required advertised `AXPress` on the exact field
before the existing setter, reselecting the same field/window afterward, and read-only
`AXFocused` diagnostics. The original workflow outcomes remain. The driver passes21
pure checks, including an added field-press refusal; the unchanged harness passes12/12.

At 20:46:09–20:46:14 UTC, the actual run observed the field change from unfocused to
focused after AXPress. AXSetValue then produced the exact81-character value and an
enabled Save button. The runner pressed Save and recorded AX code0, but its subsequent
capture failed with `actions-unreadable` before observing the validation outcome.
This demonstrates the edit path in this run, not successful invalid submission, a
general focus-causation rule or a known explanation for the unreadable action list.

The first failure remains intact. The follow-up also remains failed, with raw command
output and attempted/completed steps retained. Parent5922/native5929 and port63153
were absent/refused during normal close, before cleanup; all input hashes matched.

| Field-press follow-up artifact | SHA-256 |
|---|---|
| Wrapper | `116a2c6351b97659a42a17e392455447a600f7af5155da2c0257ebc7a6d97358` |
| Swift source | `efe399bf20c44fad6afd306f19086d0f2edff4276260d9318ea1425135e5ac52` |
| Swift executable | `85fb73a77fd756d27e1362e3119019ca787369a74c57a8e0300ad55cf0c87b14` |
| Native configuration | `214dd710f9653634ebee651207d1b6134b61e25ececea7f70287848abd7956f5` |
| Inner `appkit-launchservices-S0jTb7/report.json` | `12892fef2417d731419d34a9d4df88e2f11891d62f22345af3821c347319ac7b` |
| Its `workflow-report.json` | `9d2754954a795824c47dbfe964a0b25f69e59de3a1274e3bd84d11c332fdeec3` |

## Bounded-read follow-up: Profile outcomes pass, disclosure blocks retry

Fresh root: `/Volumes/m2ext/appkit-evidence/appkit-launchservices-workflow-transient.dz1eEIAA/`.
Only observation handling changes: typed AX read failures retain operation, attribute,
code and available window/node context. During an existing wait, selected incomplete
reads may be retried within the original deadlines, with fewer than64 retained
diagnostics. Identity, type, limit and non-wait failures remain strict. No action is
retried and no unreadable capture becomes a passed state. All original visible workflow
conditions remain;28 pure helper checks and the unchanged12-test lifecycle suite pass.

The actual run at20:53:37–20:53:42 UTC observed these outcomes through native AX controls:

| Task | Actual visible result |
|---|---|
| Invalid submission |81-character draft retained, field-length error and Changes need attention |
| Corrected save |Newer Workflow Casey, success notice, Save disabled |
| Stale second window |Reviewed Workflow Avery draft retained, newer saved Casey values disclosed, Apply reviewed draft enabled |
| Reviewed save |Reviewed Workflow Avery, success notice, Save disabled |

Two intermediate reads returned `invalidUIElement` (-25202): `AXChildren` while
awaiting invalid feedback and `AXRole` while awaiting the corrected save. The runner
retained them and then obtained complete captures satisfying the unchanged outcomes.
This does not identify the cause/code of the earlier generic `actions-unreadable` error.

The laboratory disclosure then exposed only `AXScrollToVisible` and `AXShowMenu`,
with `expanded=false` and `expanded_settable=false`. The driver refused to invent an
opening action. It did not open the disclosure, arm a fixture or execute failure/retry
or Activity steps. The **complete workflow still returns failed**, preserving the
partial task outcomes rather than treating a skipped prerequisite as passed.
Normal close and cleanup observed parent9209/native9211 absent and port63687 refused;
all source/app hashes remained unchanged.

| Bounded-read artifact | SHA-256 |
|---|---|
| Wrapper | `1b2d98428e4ae54f2e3a9613f0059c2f785fda580c7bc8d2100e4e0c6eb9e245` |
| Swift source | `d5100dbd8be5f71c78b3cb616d82ca414a15ad892e42c44f63783dc0137a7b37` |
| Swift executable | `d423295f9b27a510bdfb8fa7a74a41db5587f5a5f8b56c5f97fb0ef91dfc7170` |
| Native configuration | `98f74cd196c3fad00f61052bde50a78911faa9fcda2c6ca4e5ce97e00e43da44` |
| Inner `appkit-launchservices-xg5qt6/report.json` | `0e6b98e40c1dfcb9d2741d6e49ee40fd0f8a72d2115ee523033935e24a8eb37b` |
| Its `workflow-report.json` | `c7b2d2db0e158074de2a168307db17b8b27874036faafa56d3ec9f858514d654` |

## Independent Activity-only journey

The separately frozen `activity-only-v1` journey ran at21:06:05–21:06:10 UTC on
2026-09-16 against the same unchanged historical bundle. Private root:
`/Volumes/m2ext/appkit-evidence/appkit-launchservices-activity-only.4DwaQjnI/`.
Its preparation passed33 pure driver checks and the unchanged12 lifecycle tests.
Root and a separate reviewer checked the new oracle before launch; root rehashed
the harness, driver, test/configuration and all five selected bundle inputs.
This was a fresh synthetic session, not continuation or repair of the failed full case.

| Native task | Observed result |
|---|---|
| Baseline | Both Profile windows show Alex Morgan with Save disabled |
| Open Activity | Page-load version1, not watching; Connect enabled, Stop disabled |
| Connect | Watching version1; Connect disabled, Stop enabled |
| Save in the other window | Live Workflow Avery saved with success notice/Save disabled; Activity observes version2 |
| Establish draft, then Stop | Unsubmitted workflow draft retained with Save enabled; paused status, Connect enabled, Stop disabled |
| Reconnect | Watching version2, Connect disabled/Stop enabled; unsaved draft and Save-enabled state retained |

The driver retained one transient AXChildren read failure (`-25202`, window0/node12)
while waiting for Activity, then required a complete matching observation. No action
was retried and an incomplete capture was not counted as success. All interaction used
the reviewed public AX controls; no HTTP, DOM, keyboard/pointer injection or private
accessibility action substituted for a user task. The eight declared journey steps
and explicit `activity-live-only:not-complete-profile-workflow` scope are in the receipt.

Normal close observed parent12339/native12346 absent and loopback port64391 refused;
cleanup and a separate exact-PID check agreed. All selected app/harness inputs remained
unchanged. LaunchServices owns the parent, so its exit status remains unobserved.

| Activity-only artifact | SHA-256 |
|---|---|
| Wrapper | `e18c31829523792152bc9ecc9f249eca3632948acbdc64ec74c4633d89435849` |
| Swift source | `29a47f09e29bd4b8fdbd0b9cbbe42901e4b0a56135d78e7def49286fabb41f42` |
| Swift tests | `c4ebf7174ecb17052afd041cd5470cbf23d2416af3ccff0cb22d6408546e62f5` |
| Swift executable | `ced56bb5149aadc91b44b3504fded5cb39fc7deab6d22229a8b5e6f4e49d3c21` |
| Native configuration | `fe1ae50562c629cf9d81077c6de45c0c8494afbe5457e3320566a89296e6ca29` |
| Twelve-test lifecycle receipt | `d8a14ac497f04cc0e0df87b8bcf84b32bddf37f710215c78dad76c04ffb82fd2` |
| Outer native receipt | `facd82e4f3148e400b2d6900f6f94f6d37df0e407751d9a7e5afa082e6071dd4` |
| Inner `appkit-launchservices-cjkDEP/report.json` | `e086c0d62ef1a594212af1a1de385ed7b4eeabc23aa792fd19cd5c2f72358c63` |
| Its `workflow-report.json` | `dc868665f5d047373f95b0a28442b190df528345227f99bef174125dc6396388` |

The earlier full-workflow source, tests and failed receipt remain frozen separately
under `baseline-full-workflow/`, as well as at their original root. This Activity pass
does not prove fixture access, failed-save retry, a new candidate, or general native
accessibility/input, media, packaging/rollback or performance acceptance.

## Remaining acceptance

The original full workflow remains failed at its laboratory-control prerequisite.
The separately named Activity-only journey passes updates and reconnect independently,
but cannot substitute for failure/retry or change the original expected results. A
different input method or app change must be reviewed separately, not silently used to
turn this failed AX-control workflow into a pass.
Unsupported controls remain failures/limitations, never substitute HTTP or DOM
successes. No application changes are authorized by this test.
