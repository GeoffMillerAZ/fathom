# Wails Gallery experiment

This unpublished G6 experiment inherits [root intent](../../docs/intent/README.md).
[ADR-0026](../../docs/adr/0026-isolated-http-desktop-experiment.md) admits the isolated
module and ordinary HTML command path; [ADR-0035](../../docs/adr/0035-reproducible-macos-http-only-experiment.md)
adds the explicit macOS HTTP-only patch; [ADR-0046](../../docs/adr/0046-native-media-denial-in-http-only-profile.md)
adds native media denial. [ADR-0047](../../docs/adr/0047-process-owned-http-and-native-recovery.md)
adds opt-in process-owned HTTP recovery. It is not `appkit/shell` or desktop support.
[Private bundle preparation](BUNDLE.md) implements ADR-0068's fixed parent/companion
entrypoint; [in-place lifecycle proof](../../docs/assessments/2026-09-16-private-bundle-runtime.md)
adds direct AX close/reopen/shutdown. The [normal-opening probe](LAUNCHSERVICES-PROBE.md)
now passes the same AX lifecycle; earlier failures, shared workflows and rollback remain separate.

## What exists

Wails `v3.0.0-beta.21` hosts the existing `apps/gallery` HTTP handler without changing
its rendering, synthetic session model, HTML forms or SSE handler. Private source inputs
are disabled. The native entry offers a second comparison window and an explicit
browser-opening action; [menu evidence](../../docs/assessments/2026-09-15-native-menu-input.md)
covers AX close/reopen/continuation, with physical Cmd+N acceptance still unresolved.
Ordinary native launch now refuses the unpatched profile with an explicit browser
recovery message. Root/server builds do not inherit the experimental patch.

One ephemeral IPv4 loopback listener owns all application traffic. The host mints up to
eight outstanding 256-bit launch tokens, each single-use with a 120-second lifetime.
Exchange sets an HttpOnly, SameSite=Strict, host-only process-session cookie, then
redirects to `/shell/profile`. Exact Host, loopback peer, origin, fetch metadata and
credential ambiguity checks precede application access. The health route exposes only
`ok`. Application requests still need the Gallery's own session, CSRF and version rules.
The host cancels active requests before bounded shutdown; the response writer is not
wrapped, preserving real transport flushing and deadlines.

The root module and ordinary kit commands do not inherit Wails. This module's local
replace deliberately uses the current checkout, so default builds are not released pins or
a clean-consumer adoption rehearsal. `go.sum` pins the external module inputs. The opt-in
[independent host rehearsal](../../tools/releasecandidate/README.md#opt-in-real-gallery-host-rehearsal)
stages an exact verified private kit candidate and calls `prepare-wails -appkit-version VERSION`
without sibling reads. [Two cold roots](../../docs/assessments/2026-09-15-independent-gallery-host.md)
produce identical server/native binaries and pass selected real HTTP tasks. The
[exact-artifact native probe](NATIVE-DELIVERY-PROBE.md) adds AX window/normal-shutdown
[runtime evidence](../../docs/assessments/2026-09-16-exact-native-delivery.md); signing,
installation, browser visuals and full native acceptance remain separate.

## Run and verify

From this directory, with the existing platform build tools:

```sh
go test -race ./... -count=1
go test -tags server -race ./... -count=1
go run ./cmd/prepare-wails
```

Preparation prints a fresh directory and `modFile`. Substitute that exact path below;
keep outputs in the new directory. It verifies Wails' module/checksum identity, copies
its source and replays both pinned patches in order without changing the cache or dependency pin.
The temporary module file selects that copy and this app-kit checkout explicitly.

```sh
go test -modfile=PREPARED/go.mod -tags appkit_contained -race ./... -count=1
go build -modfile=PREPARED/go.mod -tags appkit_contained -trimpath -buildvcs=false -o PREPARED/gallery-host .
PREPARED/gallery-host
```

`./gallery-host -browser` explicitly opens the same guarded HTTP app through the OS's
selected browser. The launch capability goes directly to the opener, never stdout or
diagnostics. Ctrl-C owns browser-mode shutdown; tab-based idle detection is not built.
Direct-native startup/profile failure reports a manual `-browser` retry, not automatic
same-process recovery. The separate opt-in parent below preserves HTTP across native exit.

For a cgo-free browser/server binary:

```sh
CGO_ENABLED=0 go build -tags server -trimpath -buildvcs=false -o gallery-host-server .
```

For opt-in macOS recovery, first build the contained native binary above and compute
its exact lowercase SHA256 (`shasum -a 256 PREPARED/gallery-host`). Run the independent
server binary with both explicit inputs; substitute an absolute path and that hash:

```sh
./gallery-host-server -native-host /ABSOLUTE/PREPARED/gallery-host -native-host-sha256 EXACT_SHA256
```

The parent verifies and stages a private executable copy before starting HTTP; a bad
identity fails without browser fallback. It launches one native companion over a private
inherited socket, with a 15-second initial-navigation deadline. Unexpected exit, broken
control or missing readiness stops/reaps that child and attempts one OS-browser recovery.
Ready is a native navigation acknowledgement, not application usability certification.
Normal close requires PostShutdown acknowledgement plus zero exit within two seconds;
it ends the server unless **Open in browser** selected continuation. A stalled exit is failure.
Continuation remains a foreground Ctrl-C lifetime; if browser opening fails, existing
HTTP sessions remain available and stopping returns a sanitized failure. The OS opener
is bounded to five seconds. No automatic native restart, tab-idle detector or daemon exists.

Parent shutdown closes control, signals the exact child, waits two seconds, then kills
and waits two more; a failure to reap is an error. A child observes parent EOF and has a
four-second cancellation watchdog even before Wails starts. Private protocol frames
are canonical, at most 2 KiB, and partially received frames have a three-second bound.
No control HTTP endpoint, domain RPC or companion token argument/environment is added.
Browser launch still passes the one-use URL directly to the OS opener, never logs it.
Saved server state and existing browser sessions can survive; cookies and unsaved native
drafts are not transferred to a different browser. See [recovery proof](RECOVERY-PROBE.md).

Windows is deferred under [ADR-0027](../../docs/adr/0027-web-and-macos-now-windows-on-demand.md).
These historical cross-build commands are retained for future use, not a current gate:

```sh
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -buildvcs=false -o gallery-host-amd64.exe .
GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -buildvcs=false -o gallery-host-arm64.exe .
```

The dependency-closure test fails if the server build imports Wails or `runtime/cgo`.
Root `go test ./...` does not traverse this nested module; run its explicit gate.
Do not install or repin global tools to make it pass. No Wails CLI is required here.
The legacy `Info.plist` names the direct-native entrypoint, not the selected paired
topology. [BUNDLE.md](BUNDLE.md) generates its own metadata; neither path is an installer.

## Evidence on 2026-09-13

- Go 1.26.3 on darwin/arm64; Wails beta.21 module hash
  `h1:2jc9E7eT4g3vNQIaEoRP19Uvm4LCuL9OzYPD+Uf+yYk=`.
- Native macOS, Windows amd64/arm64 and cgo-free server builds pass. Windows results
  are cross-compilation, not Windows runtime or WebView2 evidence.
- Race tests pass for launch/replay/expiry/concurrency/refusal and the actual HTTP
  Gallery journey: profile load, failed CSRF, save, two converging streams, stale-write
  refusal, resnapshot and cancellation/EOF/listener shutdown.
- The native menu callback is asynchronous at this pin; window creation is serialized.
  Cancellation monitoring starts after `ApplicationStarted`, not before the event loop.
  Those are source-level mitigations, not native lifecycle proof.
- macOS bundle metadata passes `plutil -lint`. Native app-control failed twice with
  `Sky Computer Use native pipe startup failed` before launch. No experiment process
  was running at the follow-up process check. No window, screenshot or keyboard task
  has been observed; no browser was substituted for the requested Chrome surface.

Beta.21 was selected after comparing beta.20: it adds relevant lifecycle/menu/packaging
fixes, but not the missing navigation/bridge controls. Source: [release comparison](https://github.com/wailsapp/wails/compare/v3.0.0-beta.20...v3.0.0-beta.21).

## Explicit production blockers

1. **Native containment beyond the bounded probe.** The unchanged beta.21 pin lacks a
   public native navigation veto/bridge-disable switch. The opt-in patch installs an
   exact-origin main-frame policy in the existing delegate before loading, omits the
   `external` and `wails` handlers, and suppresses runtime injection. Separate asset
   middleware denies framework HTTP routing. The [final native pair](../../docs/assessments/2026-09-14-native-containment.md)
   changes eight failures to passes: 14/14 scoped cases pass, not Gallery/platform
   certification. Follow the [probe runbook](NATIVE-PROBE.md); strict frame refusal also
   excludes private iframe comparisons from this native profile. Remaining cases belong
   in the [parity matrix](../../docs/design/web-desktop-parity.md).
2. **Cookie boundary.** Port-specific names prevent accidental acceptance by this guard;
   cookies themselves are host/path scoped, not port-confidential. Another loopback
   service visited by the browser may receive them. No hostile-local-process isolation
   or per-window privacy is claimed. Wails' default WebView data stores are not ephemeral
   per-window containers; actual sharing/persistence needs native testing.
3. **Client and native proof.** The [shared real Gallery workflow](../../docs/assessments/2026-09-14-native-gallery-workflow.md)
   passes 14 DOM-driven checks in each client for POST validation/conflict/retry,
   two-view state, streams/resnapshot, revocation and shutdown. Follow the [runbook](WORKFLOW-PROBE.md).
   Physical input/focus, broader authority and failure states remain open. Beta.21 Darwin
   ignores the configured `Permissions` map. The second patch adds a macOS 12+ native
   media-denial callback; the [capability probe](CAPABILITY-PROBE.md) requires both native
   and page evidence. Geolocation, notifications and clipboard-read remain unproven;
   their API presence is not policy enforcement. This experiment grants no device access.
   [Actual media evidence](../../docs/assessments/2026-09-15-native-media-denial.md) proves
   camera denial only: microphone/combined time out before the callback, so the full gate fails.
4. **Delivery/recovery.** Opt-in process recovery has [bounded native/HTTP evidence](../../docs/assessments/2026-09-15-native-process-recovery.md),
   not graphical browser handoff, menu/last-window-close or full startup-failure certification.
   No signing, installer, update/rollback, OS dependency installer or tab-idle policy exists.
   Native control must reconnect for physical-input/visual testing. Windows work is deferred, not waived.

ADR-0035 addresses the fired ADR-0021/0026 containment trigger without granting
production promotion. ADR-0027 narrows current G6 targets without weakening their matrix.
The paired workflow uses test-only DOM instrumentation, not a generic capability
framework. Neither that task nor the separate refusal probe certifies platform support.
