# Native menu and window lifecycle

Date: 2026-09-15. G6 progress under [ADR-0047](../adr/0047-process-owned-http-and-native-recovery.md),
not native certification. [Run guide](../../spikes/wails-gallery/NATIVE-UI-PROBE.md).

## Defect and repair

The original native companion left **New comparison window** enabled after its
two-window limit was reached. The callback refused another window, but the menu
promised an unavailable action. `TestActualNativeWindowLifecycle` reproduces this
through an external macOS Accessibility (AX) driver, not a product callback.

`native.go` now derives menu availability from readiness, shutdown state and Wails'
authoritative window manager. New disables at two windows and re-enables after a
committed close. The observer attaches through `Window.OnCreate`, before native
display. No second window registry or premature capacity release is introduced.

Pinned beta.21 dispatches close listeners concurrently and can drop late
`WindowWillClose` events after manager removal. The new observer watches the manager
with a 10ms tick and two-second ceiling; cancellation/shutdown terminate it. A timeout
reports failure without releasing capacity early. Source references:
`pkg/application/window_manager.go` (`addAndRun`), `webview_window.go`
(`NewWindow`, `HandleWindowEvent`) and `application.go` (window-event dispatch).
Both prepared Wails patches and the root dependency graph remain unchanged.

## Actual observations

| Scenario | Original | Candidate |
|---|---|---|
| Two windows disable New | Failed, remained enabled | Passed |
| Close one, re-enable and reopen using AX menu | Not reached | Passed |
| Last AX close exits child and parent; listener refuses; no browser opener | Not reached | Passed |
| AX Open in browser, then last close retains HTTP until foreground shutdown | Passed | Passed |
| Cmd+N opens a replacement window after closing one | Not reached | **Failed**, stayed at one |
| Cmd+N opens the initial second window before any disable/re-enable cycle | Not tested | **Failed**, stayed at one |

The full four-scenario gate remains **failed**. The independent AX-only scenario
does not replace the original keyboard scenario. At-cap Cmd+N leaves two windows,
but without a working positive keyboard control that is not proof of key refusal.
Even with a positive control, window count would prove capacity enforcement, not
Cocoa's suppression of the callback: the callback independently enforces the cap.
Activation and permission preflight do not prove AppKit received a keyboard event.
An optional, separately labeled PID-only event diagnostic cannot rescue a failed gate.

The external driver verifies the staged executable path against the target PID,
reads only that app's bounded AX tree, and uses AXPress for menu/close actions.
Keyboard events go to the verified PID, never the global event stream. No permission
prompt, clipboard, signed-in browser, private assets or real user data is involved.

The parent owns a synthetic Gallery and the unchanged guarded HTTP boundary. For
browser continuation the injected opener exchanges a real one-shot launch using a
fresh Go HTTP cookie jar, then independently verifies the profile after native exit.
It does not launch an OS browser or transfer native cookies/drafts. Profile request
counts prove handler arrival, not complete paint, accessibility or new-window task parity.

## Reproducible evidence

Host: macOS 26.6.2 (25G83), arm64, SDK 27.0, Go 1.26.3; pinned Wails
`v3.0.0-beta.21` with the existing HTTP-only/media-denial preparation.

Private evidence directory:
`~/Library/Application Support/app-kit/native-evidence/2026-09-15-native-ui-M50MkM`.
Its README and hashes identify exact executables, source snapshots and logs.

- `ui-baseline.txt`: original native SHA
  `de52ac38826dc723de669b3f0f7d74862ecd9b7305067d69b77ee17cc4de6aae`;
  enabled-at-cap failure, separate browser-menu pass.
- `ui-candidate-1.txt`: superseded late-close-listener implementation; retained failure.
- `ui-candidate-2.txt` and `ui-candidate-3.txt`: manager-removal observer; keyboard
  failure retained. The latter adds the separate AX-only reopening/normal-close pass.
- `ui-diagnostic-1.txt`: pre-display listener attachment, native SHA
  `5865716610239efdf9656e2ceeea6b081bf977b098dbc479600dfb59eff141c2`;
  AX-only lifecycle and continuation pass; keyboard fails, first diagnostic unavailable.
- `ui-diagnostic-2.txt` / `ui-diagnostic-3.txt`: initial and reopened shortcuts fail.
  AX focus belongs to the app; exact menu character/modifiers match n/Command. A
  listen-only PID tap observes this driver's tagged key-down/up with the expected
  keycode/modifiers. AX virtual-key metadata is unavailable, not mismatched. These
  observations narrow the failure but do not prove AppKit handled the event.
- `ui-chord-v4.txt`: final native SHA
  `701cdf9889cb98c8245f06ee3b9a1bcbd87060f58645eab3d3f1d6dd573bfc61`,
  driver SHA `096159246bb8a36fc6d17e1d4e0a266646781631ed5a1fead31ab0a032f20ce3`;
  a separately labeled full modifier/key chord still fails both keyboard cases.
  AX-only lifecycle and continuation pass. Four scenarios, two passes/two failures,
  15.43 seconds; no diagnostic rescue or weaker assertion.

The final close observer rechecks shutdown after acquiring its lock and avoids
menu refresh with no remaining windows. Final contained-native race tests and vet
pass; server race tests pass. Receipt canaries pass ten race-enabled repetitions;
server/recovery/input vet passes. `process-regression-v4.txt` records the existing
actual native crash/HTTP/SSE, control EOF and SIGTERM lifecycle regression separately.

The checker runs outside Wails and checks child absence, context still active,
parent result, exactly zero/one opener as applicable and listener refusal. Native
exit zero alone cannot pass it. Driver output is bounded to 4KiB; task receipts reject
unknown, missing, duplicate, null or trailing fields and mismatched operation/PID.
`TestNativeUIReceiptRefusals` includes malformed receipt and output-limit canaries.

## Independent AppKit input control

`native_ui_control.swift` and `TestNativeKeyboardAppKitControl` isolate the same
external driver from Wails. The privately staged synthetic app has a standard
NSMenuItem Cmd+N action and two ordinary NSWindows, no HTTP or native bindings.
Its `NSApplication.sendEvent` override observes only the driver's fixed diagnostic
tag, reports booleans, and forwards every event unchanged. It never logs arbitrary
keys or text. The checker owns the exact child, bounded output and cancellation;
the fixture has a separate twenty-second failure watchdog.

- `control-v1/v2.txt`: activation/setup failures, not valid shortcut comparisons.
  Matching Wails' regular-app `activateIgnoringOtherApps` path makes the control
  active with an owned key window; no installed bundle or global setting is needed.
- `control-v3/v4.txt`: tagged Cmd+N reaches AppKit `sendEvent` but creates no window.
  Menu-delegate observations show AX inspection opens and closes the menu before
  key delivery. A separate explicit AXCancel trial (`control-v5.txt`) still fails;
  that speculative driver change was removed, not kept as a fix.
- `control-ax-v5.txt`: the direct AX New action and both normal closes pass (1.12s),
  confirming the fixture's action/target and lifecycle independently of shortcuts.
- `control-key-v5.txt`: the same control fails the tagged keyboard path (5.49s).
  At key delivery the app is active, owns its key window and menu bar, New is
  enabled, and both event character forms and Command modifier match.
- `control-key-v6.txt`: transparent window/main-menu `performKeyEquivalent`
  observers call their superclass exactly once and return its unchanged result.
  Both report not handled. This is observed matching refusal, not merely absence
  of an action callback; it does not identify the cause of the refusal.

The old `AXUIElementPostKeyboardEvent` alternative is unavailable to Swift in SDK27
(deprecated before macOS10.9). The compile-only attempt was removed; no legacy
bridge, global keyboard post or runtime result is claimed for that path.

The raw-binary control reproduces the keyboard problem outside Wails. This narrows
the investigation to shortcut matching/dispatch or the synthetic input path; it
does not prove a macOS defect, a working physical keyboard path or Wails support.
Driver errors now distinguish activation refusal from other unavailable UI facts.
No Wails or product code changed during this control investigation.
Final `control-ax-final-race.txt` passes the direct-action/normal-close control
under Go's race detector; `control-key-final.txt` retains the independent keyboard
failure. Input/server vet and ten receipt-refusal race repetitions pass. The private
README binds the selected control/driver/checker and four-file source snapshot.

Next discriminating observation: physical Cmd+N in the same isolated control,
compared with this recorded synthetic event path. Owner availability was requested
asynchronously; other kit work can proceed meanwhile. No global posting, runtime
workaround or relaxed keyboard acceptance is selected.

## Remaining acceptance

Resolve the positive native keyboard stimulus/dispatch failure; then retain full
native focus/input/IME, pointer, zoom/theme and workflow evidence. OS-selected browser
opening, app-owned cross-client policy, remaining startup faults, full capability
denial and applicable authorized delivery remain open. Microphone/combined failures
are unchanged. Windows/mobile and deferred consumers remain outside current work.

The unavailable-action defect has a durable regression (INC-38); it does not complete
G6, promote a desktop adapter or certify a converted catalog target.
The ADR-0021/0026/0033/0035 conformance triggers remain under review: the input gate
fails, but current evidence does not yet isolate host handling from test delivery.
No new shell, dependency pin, native key-binding workaround or scope waiver is selected.
