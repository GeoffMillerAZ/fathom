# ADR-0047: Process-own HTTP state and isolate native recovery

- **Status:** Accepted
- **Type:** platform
- **Date:** 2026-09-15
- **Extends:** ADR-0026, ADR-0033 and ADR-0035; supersedes no record

## Context

Pinned Wails v3 beta.21 can take unrecoverable process paths: `New()` can call
`fatal()` and `os.Exit`, while macOS `NSApp` termination and `Run` may not return.
The native lifecycle revisit trigger has therefore fired. Running the guarded HTTP
server, app state and optional native shell in one process cannot promise recovery
to the browser after those paths.

## Decision

For the opt-in, unpublished macOS experiment, a cgo-free parent owns the existing
guarded HTTP server and app state. `-native-host` names a hash-verified absolute
native companion; `-native-host-sha256` selects its exact bytes. The parent copies
those bytes into a private temporary directory before execution. Root/server imports
remain native-free. Existing direct-native, manual-browser and `-browser` paths remain.

The private child inherits an anonymous full-duplex socket as fd 3. It exchanges only
bounded canonical control frames: `init` with origin/single-use launch, `ready`,
`launch-request`/`launch`, `browser`/`browser-ok`/`browser-failed` and `closed`.
Companion arguments/environment and all logs contain no launch secrets. Browser launch
passes a capability directly to the bounded OS opener, as direct browser mode does.
No HTTP control endpoint or domain RPC is introduced.

The parent allows 15 seconds for readiness. EOF, malformed protocol, missing ready or
unexpected exit recover to the OS browser at most once, without a restart loop.
Identity mismatch is a hard refusal, never fallback. Normal close requires `closed`
from PostShutdown plus zero exit within two seconds; a stalled exit is failure. Allow
three seconds to drain final control after process exit. Normal close ends the parent
unless browser continuation was selected; continuation remains Ctrl-C-owned.
Browser-opener failure keeps existing HTTP sessions usable until Ctrl-C, then returns
an explicit sanitized failure. The foreground process never silently becomes a daemon.

Parent shutdown uses bounded signal, kill and reap of the exact child. A parent crash
closes the socket so the child observes EOF. These rules still require actual lifecycle
evidence. Ready acknowledges native navigation only, not task success. Preserving the
browser session and server state does not transfer cookies or unsaved drafts.

This record adds no signing, installer, updater, remote authentication, general
capability framework, Windows/mobile work, dependency, consumer write or public API.

## Alternatives considered

- Recover in the Wails process: fatal exit and non-returning native lifecycle paths
  make that boundary unsafe.
- Restart the native child indefinitely: hides faults and risks a crash loop.
- Put control on HTTP: broadens a private lifecycle channel into the app protocol.

## Consequences

- HTTP state can survive a failed native companion and continue in the OS browser.
- A private temporary copy, companion identity and protocol require maintenance.
- Native acceptance remains incomplete until the lifecycle oracle passes.

## Revisit when

- A native fault closes or corrupts the guarded HTTP server.
- Credentials or control cross the declared private-channel boundary.
- Upstream supplies an isolated recoverable lifecycle.
- A named consumer promotion or signed packaging path is selected.
