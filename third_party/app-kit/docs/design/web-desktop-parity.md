# Web-first workflows and native enhancements

Contract: [ADR-0033](../adr/0033-web-first-workflows-and-native-enhancements.md).
This specifies G6 and selected adoption, not shipped platform support.
[Runtime selection](runtime-selection.md) retains static and client-heavy choices.

## Product boundary

Core tasks work in an ordinary browser without a Wails window or bridge. Equivalent
outcomes matter, not identical chrome. Navigation, data exploration, forms, validation,
recovery and export use shared application contracts. Domain authorization, durable
state and business calculations have one authority. Native adapters add OS integration.

An OS-essential task records its requirement, useful web subset, unavailable-state
explanation and revisit trigger. No fake success, dead controls or indefinite loading.
Do not prebuild a capability registry, remote relay, offline store or sync engine.

## Select the topology

| Target | Contract | Not implied |
|---|---|---|
| Browser + local server | Independent command; loopback binding, explicit local authentication, origin/CSRF checks; no native process required | Remote hosting, OS access or offline editing |
| Browser + remote HTTPS service | Separately selected deployment; real identity/auth, secure sessions, bounded proxy trust and app-owned persistence | Proven by local HTTP or launch-token tests |
| Wails on macOS | Thin host around the same application contract; tested native boundary and lifecycle | Browser tests prove native containment, input or packaging |

Web-capable does not mean static or serverless: a browser may need a running server.
One source tree may have separate server/native entrypoints. Native dependencies
must not enter portable/server builds just to expose common workflows. ADR-0026's
isolated Wails module stays experimental.

## App-owned adoption record

Record the selected kit/runtime revision, topology and independent build/launch
commands; core task IDs/routes, synthetic fixtures and expected outcomes; native
enhancements, permissions and browser recovery; state/trust boundaries; app-specific
latency/payload budgets; and exact per-target evidence with unresolved failures.
Use existing adoption IDs. The local Gallery probe has a strict task receipt; no
cross-consumer parity schema or authenticated evidence service exists.

## Acceptance matrix

For each selected target record `passed`, `failed`, `not tested`, or `not applicable:
reason`. Remote HTTPS is required only if selected; Windows remains deferred.

| Case | Required evidence |
|---|---|
| Independent delivery | Server builds/launches without Wails; browser completes a core task with native process absent |
| Workflow parity | Same fixture and expected domain outcome through browser/native UI; deep links/back/reload preserve context |
| Commands | Real POST bodies, validation, CSRF/origin/auth refusal, stale versions and duplicate-side-effect handling where applicable |
| Session lifecycle | Expiry/logout/reauth remove privileged projections; retained drafts follow explicit app policy |
| Streams, if selected | Real flushing, bounded updates, cancellation and reconnect/resnapshot; selected proxy path tested separately |
| Multiple views | Two tabs/windows converge on saved state; conflicts preserve drafts and require explicit recovery |
| Input/accessibility | Keyboard/focus/IME, pointer, 200% zoom, 390/1440 CSS px and supported themes; non-color data alternatives |
| Failure states | Network loss, stopped server, rejected command and unavailable enhancement give truthful next actions |
| Native boundary | External navigation, new windows, bridge exposure and capability grant/denial tested in the actual macOS WebView |
| Delivery/lifecycle | Bounded shutdown; startup recovery; linked binary target agrees with the selected build profile; actual runtime evidence for claimed OS versions; signing/update/rollback tested for claimed distributed delivery |

Evidence names source/binary hashes, runtime/OS/browser, topology, fixture, input
method, expected/observed outcomes and measured budgets. A compile, HTTP 200,
screenshot or call graph proves only its own observation. No inherited target passes.

## Sequence and current evidence

[G6](../intent/scope.md#g6--prove-web-and-macos-delivery) is the canonical goal. First
use the existing synthetic Gallery save/recovery/stream workflow, independent server
and isolated macOS candidate. Resolve native containment before consumer promotion.
Apply the matrix to an existing Wails app only when the owner selects its migration.

The [Wails experiment](../../spikes/wails-gallery/README.md) records builds, guarded HTTP
and [14 actual native containment cases](../assessments/2026-09-14-native-containment.md).
The [shared workflow pair](../assessments/2026-09-14-native-gallery-workflow.md) adds 14
DOM-driven task checks per client: save/conflict/retry, two views, explicit streams and
sign-out redaction, with shutdown. Physical input, native permissions, network-loss UI,
startup and delivery recovery remain open. The [Gallery Chrome smoke](../assessments/2026-09-13-gallery-browser-smoke.md)
covers comparison/theme inspection, not this core-task/native parity matrix.
Resume-as-Code and Fathom are selected for G2/G3/G4 UI uplift, not Wails migration or
remote deployment. CA/GGB and Windows remain deferred.

The [media-denial experiment](../assessments/2026-09-15-native-media-denial.md) replays
native containment and Gallery tasks with a second pinned patch. Camera reaches the
native denial callback and rejects without a stream; microphone/combined remain pending
without reaching it, including a fresh-process microphone trial. Visible/focused readiness
does not resolve that failure. The full three-case gate fails; other permissions remain
presence-only. No device grants, consumer promotion or production support is implied.

[ADR-0047](../adr/0047-process-owned-http-and-native-recovery.md) adds an opt-in cgo-free
parent that retains guarded HTTP/app state while a pinned native companion can fail.
The [recovery probe](../../spikes/wails-gallery/RECOVERY-PROBE.md) separates synthetic
process faults from actual macOS exit/HTTP-session/SSE survival. A failed OS opener keeps
existing HTTP sessions available until foreground shutdown and returns failure on exit.
Navigation readiness is not task completion, and a new browser does not inherit native
cookies or drafts. Follow-up tests kill the actual separate parent, exercise native
profile refusal, and run all fourteen tasks after native failure in fresh-context Chrome
configured headed, with a reviewed PNG. The [external native menu probe](../assessments/2026-09-15-native-menu-input.md)
adds AX cap/close/reopen and browser-continuation lifetime proof; its positive Cmd+N
case still fails. OS-selected opening, cross-client state policy, physical input and
full delivery gates remain open; AX actions do not certify keyboard/pointer parity.

The [independent host rehearsal](../assessments/2026-09-15-independent-gallery-host.md)
adds exact private-candidate delivery: separate cold roots, source/network-denied builds,
identical server/native binaries, linked-target inspection and localhost-only actual-handler
POST/SSE/shutdown checks. [Exact-output browser tasks](../assessments/2026-09-16-exact-gallery-browser.md)
now pass176 assertions across script/no-script desktop/narrow sessions with retained
task-viewport captures and cleanup proof. A separate shared no-script appearance-status
repair passes143 working-tree browser checks; the frozen candidate is unchanged.
The [same frozen native pair](../assessments/2026-09-16-exact-native-delivery.md) now passes
actual AX window capacity/reopen/normal shutdown using both delivered executables.
Full target acceptance and authorized distribution recovery remain separate gates.

## Private unsigned macOS delivery

[ADR-0068](../adr/0068-prepare-private-unsigned-macos-delivery-first.md) records the owner's
first delivery path. The matrix remains acceptance criteria, not a platform-support claim.
Use the existing isolated Gallery topology, not a new installer/updater framework.

The [private preparer/launcher](../../spikes/wails-gallery/BUNDLE.md) now assembles fixed
resources, verifies bounded files/metadata/modes/quarantine and selects exact paired
server arguments relative to its own executable. [Retained proof](../assessments/2026-09-16-private-bundle-seam.md)
includes race/refusal checks and two identical actual-artifact bundles; relocated
`-check` runs no server/native process. A separate [in-place executable run](../assessments/2026-09-16-private-bundle-runtime.md)
passes exact launcher→parent transition, AX cap/close/reopen and normal shutdown on
the named macOS host, without extracting bundle executables. Earlier intermittent
synthetic inspection failure remains recorded. The [maintained normal-opening probe](../../spikes/wails-gallery/LAUNCHSERVICES-PROBE.md)
now passes initial readiness, AX cap/close/reopen and process/listener shutdown before
cleanup; earlier request/harness failures remain retained. Strict signature verification
still fails independently; shared workflow and rollback remain open. The legacy plist names the wrong direct-native entrypoint.

| Gate | Required evidence before claiming this private path works |
|---|---|
| Exact contents | Reviewed executable/resource inventory, hashes, runtime/module pins, bundle metadata and minimum-target declaration; no paid corpus or sibling checkout required |
| Reproducible assembly | Two fresh private outputs from identical retained inputs; explain variable metadata rather than comparing only executable hashes |
| Real launch | Relocated bundle launches on the named macOS/architecture under normal OS policy; retain exact request/options and normal-environment path-alias checks; independent browser entrypoint remains functional; no inferred support for untested hosts |
| Failure refusal | Missing/changed executable or invalid metadata produces an actionable failure before unsafe launch; content integrity is not publisher authentication |
| Workflow/lifetime | Existing synthetic save/conflict/recovery/stream task, actual native close/reopen and bounded process cleanup, including independent cleanup when discovery fails; preserve separate input/media failures |
| Manual update | Explicit old/candidate revisions, no overwrite of a running bundle, retained known-good copy, simulated failed startup and successful rollback |
| State and recovery | Declare app-state lifetime; preserve selected durable state where it exists. Gallery's process-local fixture cannot prove a consumer database migration or persistent draft recovery |
| Trust and scope | Record strict signature-verification output separately from quarantine/launch observations; no inferred causal link or automatic policy bypass, credential access, installation or public distribution |

Prepare and test in fresh private staging only within the applicable authorization.
An installation into an application directory, signing/notarization, or real-consumer
upgrade is a separate action. Preserve failed receipts and usable browser recovery;
never relabel an OS refusal as successful private distribution. Promote a reusable
packaging API only after actual consumer fit, not from one Gallery bundle.
