# External native menu and input probe

Opt-in synthetic macOS test under [ADR-0047](../../docs/adr/0047-process-owned-http-and-native-recovery.md).
Current [evidence and failures](../../docs/assessments/2026-09-15-native-menu-input.md).
The driver acts through Accessibility and PID-targeted keyboard events, not DOM
evaluation or product-only callbacks. This is automated OS input, not human input proof.

## Build and run

Use the existing explicit two-patch preparation from [README](README.md). Replace
`PREPARED` below with that preparation's absolute directory. Run from this directory.

```sh
go build -modfile=PREPARED/go.mod -tags appkit_contained -trimpath -buildvcs=false -o PREPARED/gallery-native .
xcrun swiftc native_ui_probe.swift -o PREPARED/native-ui-driver
CGO_ENABLED=0 go test -c -tags server,recoveryprobe,inputprobe -trimpath -buildvcs=false -o PREPARED/ui-check .
shasum -a 256 PREPARED/gallery-native PREPARED/native-ui-driver
```

Provide exact absolute paths and hashes via `APPKIT_RECOVERY_NATIVE`,
`APPKIT_RECOVERY_SHA256`, `APPKIT_NATIVE_UI_DRIVER` and
`APPKIT_NATIVE_UI_DRIVER_SHA256`. Then run:

```sh
PREPARED/ui-check -test.run '^TestActualNativeWindowLifecycle$' -test.v -test.timeout 150s
```

Missing identity or existing Accessibility/event-posting permission is a failure,
never a skip. The tool checks existing permission only and never requests a grant.
It activates only the exact privately staged Gallery process; do not run concurrent
native probes or interact with their windows during an automated scenario.

## Fixed acceptance

1. AX New creates the second window; New becomes disabled. Cmd+N at the cap does not
   add a window. Close re-enables New; Cmd+N must create a replacement. Final native
   closes exit the child and parent without browser opening or a remaining listener.
2. A separate AX-only create/close/reopen scenario checks the same menu/lifecycle
   outcomes. Its pass never substitutes for the keyboard scenario.
3. AX Open in browser invokes the injected HTTP opener exactly once; final native
   close preserves that session until explicit foreground shutdown.
4. Cmd+N must also create the initial second window before any menu state cycle,
   then pass the same close/reopen/lifecycle checks.

The injected opener is a fresh Go HTTP cookie jar, not LaunchServices or Chrome.
Each scenario verifies exact child selection, bounded shutdown and listener refusal.
Expected synthetic profile is Alex Morgan; private sources remain disabled.

## Optional failure diagnosis

`APPKIT_NATIVE_UI_KEY_MODE=chord` selects a separately labeled complete Command-down,
N-down, N-up, Command-up sequence using a private event source and PID-only posting.
Default uses the retained key-down/up stimulus. Both modes keep the same acceptance
outcomes; neither currently passes the keyboard cases. Other mode values fail.

`APPKIT_NATIVE_UI_DIAGNOSTIC=1` adds a separate diagnostic only after a failed window
observation. The test still fails. The driver checks owned AX focus and shortcut
metadata, then, only with existing listen permission, observes its own tagged
PID-posted Cmd+N through a listen-only per-process event tap. Unknown observations
are not passes. No global event posting, permission prompts or app code hooks occur.

Keep failed logs as well as successful subsets; retain binary/source hashes and
exact OS/SDK versions. Do not infer physical mouse/keyboard/IME, full accessibility,
visual fidelity, actual browser handoff or packaging from this probe.

## Independent AppKit control

Compile `native_ui_control.swift` with `xcrun swiftc` into a fresh private directory.
Supply its absolute path/hash as `APPKIT_NATIVE_UI_CONTROL` and
`APPKIT_NATIVE_UI_CONTROL_SHA256`, plus the same driver identity above. The same
checker exposes `-test.run '^TestNativeKeyboardAppKitControl$' -test.v -test.timeout 30s`.
It stages and starts only this standalone AppKit fixture, not Wails or an HTTP server.
No native-companion input is required for this selected test.

Default sends the same tagged PID keyboard stimulus and requires a second window,
followed by normal close/exit. `APPKIT_NATIVE_UI_CONTROL_MODE=ax` separately validates
the fixture's direct menu action and close behavior; unknown modes fail. A passing
AX control never becomes a keyboard or Wails pass. The fixture forwards AppKit
events unchanged and emits only fixed lifecycle categories and tagged-key booleans.
Its activation path matches the pinned Wails regular-app behavior; setup failures
remain separate from valid active/focused shortcut observations.
