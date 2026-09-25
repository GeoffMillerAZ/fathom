# Exact independent native delivery — 2026-09-16

G4/G6 runtime evidence under ADR-0026/0035/0047 and I9/I17/I19. The unchanged
[independently built pair](2026-09-15-independent-gallery-host.md) now has an actual
macOS launch/menu/normal-shutdown observation. No production support, signing,
installation, consumer adoption or full native acceptance is claimed.

## What ran

The new [bounded external harness](../../spikes/wails-gallery/NATIVE-DELIVERY-PROBE.md)
copied and pinned the actual `host-run-1/gallery-server` and `gallery-native` outputs.
It did not rebuild a Go test host or substitute an HTTP fixture. The real cgo-free
server launched its ordinary hash-verified native companion and owned the listener.
The existing external Swift Accessibility driver targeted only that direct child's
verified PID/path. Both app binaries and the driver remained unchanged.

Before execution, all58 frozen host-source files matched the original build receipt.
The driver source matched the frozen source, current source and its retained historical
source archive. Its compiled executable was reused, not freshly recompiled.
Copies were set0500 and rehashed. This is private local identity/provenance evidence,
not signature authenticity or protection from a hostile same-user process.

The test-only sandbox denies both fleet-source roots and `/usr/bin/open` execution.
Both refusal canaries passed before app startup. This prevents a failed native startup
from launching an OS-selected browser; it is not a production containment policy.
No keyboard stimulus, permission grant, browser-menu action or real user data was used.

## Observed outcomes

| Requirement | Observation |
|---|---|
| Delivered artifacts | Exact server/native hashes match both earlier cold-build roots |
| Native readiness | Parent reports native navigation acknowledgement; not task completion |
| Initial state | One window, New enabled, browser action available |
| Capacity | AX New reaches two windows; New becomes disabled |
| Reopening | Close reaches one/New enabled; reopen reaches two/New disabled |
| Normal shutdown | Close to one, then last close; parent exits0 without signal |
| No cleanup rescue | Native PID absent before cleanup; no cleanup error or forced shutdown needed |
| Listener shutdown | Observed loopback port refuses TCP with `ECONNREFUSED` |

Twelve retained driver observations include the asynchronous intermediate state:
New can already be disabled while AX still reports one visible window. The harness
waits for the complete expected state tuple instead of accepting that intermediate
observation. No expected outcome or product code changed to obtain the result.

Observed host: macOS26.6.2 build25G83, arm64, Node26.3.0. The native load command is
minos14.0/SDK27.0; this does not establish runtime support on macOS14. The scenario
took3.64seconds including orchestration, not a UI responsiveness benchmark.
Both owned processes were absent in a separate post-run process check.

## Refusal and review

Eight Node checks pass on this host with zero skips. Seven cover argument/hash,
diagnostic/state and cleanup-result contracts. The eighth runs an inert, explicitly
pinned `/usr/bin/true` as a deliberately invalid server: exit0 is correctly refused
as `server-exited`, with a failed retained receipt. No native UI is launched by it.

Preflight review hardened staged-file permissions, startup/error observations,
last-close races, exact process ownership and cleanup. Ownership is retained before
native policy/digest assertions; signaling rechecks the observed executable identity.
An exhausted task deadline cannot suppress the separate bounded cleanup phase.
Historical source snapshots and refusal receipts remain; production binaries were
never edited. The rehearsal preserves original Cmd+N/media failures, not replaces them.

## Retained evidence

Private parent: `/Volumes/m2ext/appkit-native-delivery-rehearsal.7gjy3tWo/`.
`INPUTS.md` and copied prior receipt/driver sources bind the input provenance.
Successful run: `appkit-native-delivery-sC1wc9/`; it retains all three executables,
executed harness, sandbox/configuration, exact driver observations and server logs.

| Artifact | SHA-256 |
|---|---|
| Original independent-build receipt | `640e557e06dde2591c39e9ad4030f1b034396fc56dd27fd8a82cd42f75fcd309` |
| Actual server | `95cdc86056847a3beb18b9a9d7fa327041c39ad81adb9140b6f702a8b39e4e71` |
| Actual native | `98cf0b52661e82d0a51ca2230b7f0c3289252e487b1b798fe4c4583da3350f1b` |
| External driver | `b0417f2e08a973439702f9e2767ed3aa5600a01ffd6d175c751bfea9f20c705a` |
| Executed harness | `e76deefa7fb5ce8d8161e783b10ae977cc34486edafd1066999a8b7e91f609cc` |
| Runtime `receipt.json` | `e26db3883aed1ff037fa1c7803b6909927d77ffc53312afecc143ea4fe3851bb` |
| `driver-receipts.json` | `8162e0bd975d12a5089f7b985b0d92dc893d73c44dcaa95eef1cc67c38ffb118` |

Parent zero exit plus child absence supports the parent's normal-closure protocol;
the external Node process cannot directly reap the grandchild's status. AX window
and menu observations are not rendered-content, keyboard/IME, pointer, accessibility,
form, font/pixel, HTTP-authorization, media or OS-browser handoff proof. The default
WebView data store is unchanged, not an ephemeral per-window store. Earlier browser,
HTTP and instrumented-native task evidence remains separately scoped.
Full G4/G6 gates, authorized distribution/update recovery and human Cmd+N remain open;
Windows/mobile remain deferred. No release, installed host refresh or consumer writes.
