# Exact-artifact native delivery probe

Opt-in local G4/G6 rehearsal under [ADR-0047](../../docs/adr/0047-process-owned-http-and-native-recovery.md).
This runs an already-built independent server/native pair, not a rebuilt test host.
It is separate from [keyboard acceptance](NATIVE-UI-PROBE.md), Gallery DOM tasks,
permission denial, visual review and signing/distribution evidence.

## Inputs and safety boundary

Use exact private server/native binaries from an independently reviewed build receipt,
and the existing external [Swift AX driver](native_ui_probe.swift). Supply an absolute
path and lowercase SHA-256 for each. Hash agreement is identity, not authenticity:
review the sources/build provenance before authorizing execution. The harness neither
downloads dependencies nor compiles, installs, signs or modifies these executables.

The output parent must already be a private writable directory. Each run creates a
new private subdirectory and retains its executed source, policy, configuration,
bounded observations and failure evidence. Standalone mode copies all three inputs;
bundle mode copies only the AX driver and executes the pinned bundle in place. Use
synthetic Gallery only; its ordinary host disables private design corpora.

The app may open/focus two windows. Run native probes serially and do not interact
with their windows during the test. Existing Accessibility permission is required;
the driver never requests a grant or inspects another application's UI. Only AX
inspect, New and close operations are used: no keyboard events or browser-menu action.

The host normally attempts OS-browser recovery after native failure. This probe runs
it under a test-only sandbox that denies execution of `/usr/bin/open`, plus reads of
the two owner fleet-source roots. Positive refusal canaries run before the host.
This policy is a lab constraint, not a production sandbox or OS-opener success test.
Neither failure nor timeout may become a skip or a successful lifecycle result.

## Run

From the kit root, replace every placeholder with the reviewed exact input:

```sh
node --test spikes/wails-gallery/native_delivery_probe_test.mjs
node spikes/wails-gallery/native_delivery_probe.mjs \
  --server /ABSOLUTE/INDEPENDENT/gallery-server \
  --server-sha256 EXACT_SERVER_SHA256 \
  --native /ABSOLUTE/INDEPENDENT/gallery-native \
  --native-sha256 EXACT_NATIVE_SHA256 \
  --driver /ABSOLUTE/REVIEWED/native-ui-driver \
  --driver-sha256 EXACT_DRIVER_SHA256 \
  --output-parent /ABSOLUTE/PRIVATE/EXISTING_DIRECTORY
```

To exercise the same AX lifecycle through an existing private bundle, keep the
server/native flags above and point them at that bundle's fixed `Resources` paths,
then add this all-or-none quartet:

```sh
  --bundle-launcher /ABSOLUTE/Gallery.app/Contents/MacOS/gallery-launcher \
  --bundle-launcher-sha256 EXACT_LAUNCHER_SHA256 \
  --bundle-manifest-sha256 EXACT_BUNDLE_JSON_SHA256 \
  --bundle-plist-sha256 EXACT_INFO_PLIST_SHA256
```

Bundle mode refuses path, mode, quarantine, metadata, plan or identity disagreement.
It first runs the launcher with only `-check`, rechecks the unchanged bundle bytes,
then runs that same in-place launcher with no arguments. The launcher must replace
its PID with the fixed HTTP parent before native-child discovery. It does not copy,
extract or rebuild the bundle's server/native executables.

No global environment or user-profile changes are needed. The standalone copied
server or bundle's in-place server owns the listener and launches its own
hash-verified private native companion. The external harness verifies the direct child
PID, executable path and content identity before targeting it. The Swift driver
independently checks the running app's path.
Launching either file alone does not establish this paired-delivery result.

## Fixed acceptance and limits

Require one ready window with New enabled, two windows with New disabled, close to
one with New enabled, reopen to two, then close both. The normal last close must end
the parent successfully, leave no verified native process, and close the observed
loopback listener. Forced cleanup cannot satisfy that normal-shutdown requirement.
Retain failed observations and whether cleanup itself completed.

This is the unchanged external AX lifecycle oracle applied to the bundle's direct
executable entrypoint. It is not Finder or LaunchServices launch evidence, does not
replay the shared browser/native DOM-task suite, and does not exercise candidate
replacement, failed-start recovery or rollback. Those remain separate matrix gates.

The parent's successful exit is evidence that its own protocol accepted normal
native closure; the external harness does not directly reap that grandchild's status.
Window/menu facts do not prove rendered content, form input, HTTP authorization,
keyboard/IME, pointer, accessibility, fonts, pixels, media, browser recovery or updates.
The existing WebView data-store behavior is unchanged; no ephemeral-store isolation
is claimed. Actual platform support still requires the full
[web/native matrix](../../docs/design/web-desktop-parity.md). Windows/mobile remain deferred.
