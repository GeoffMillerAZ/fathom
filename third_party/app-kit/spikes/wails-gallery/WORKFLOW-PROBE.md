# Actual Gallery workflow probe

Opt-in, synthetic, local only. Implements part of [ADR-0033's matrix](../../docs/design/web-desktop-parity.md),
not production desktop certification. Read [README.md](README.md) and [ADR-0035](../../docs/adr/0035-reproducible-macos-http-only-experiment.md)
before preparing a native build. Never use a signed-in browser profile or real account data.

## What runs

The same Go oracle drives `gallery.New()` through the real `startHost` authorization
guard, HTML forms, application CSRF/version rules and pinned Datastar observer.
The browser binary is cgo-free with `server,workflowprobe`; it needs no native process.
The native binary uses the exact prepared HTTP-only Wails patch and the host's shared
window options. Two views share a synthetic session. No cookie or CSRF is injected.

Fourteen checks cover validation and retained draft, corrected save, stale conflict and
explicit review, a one-shot failed-save fixture and retry, idle observer, explicit
Connect, second-view update, Stop without reconnect for 1.4 seconds, manual resnapshot
without destroying the other draft, and sign-out redaction. Expected POST statuses are
`422,303,409,303,303,503,303,303,303`; exactly two streams open and none remain at shutdown.
Post-response checks inspect the newly rendered state, not only response codes.

Only the test driver differs. Chrome uses an isolated Playwright context and an exact
local-request allowlist. Native uses public `WKWebView.evaluateJavaScript` on the main
thread through a bounded cgo callback, not Wails' disabled JS bridge or a replacement
delegate. Source-reviewed scripts set fields, dispatch input/change, and schedule the
real form's `requestSubmit()`; both renderers keep their normal response headers.
SSE response writers are never wrapped; test instrumentation records counts only.

## Run

From this directory, prepare Wails as described in README; replace `PREPARED` below
with that command's exact temporary directory. Use an existing approved Playwright
installation and installed Chrome. This harness installs nothing.

```sh
workflow_dir="$(mktemp -d /tmp/appkit-gallery-workflow.XXXXXX)"
CGO_ENABLED=0 go test -c -tags server,workflowprobe -trimpath -buildvcs=false -o "$workflow_dir/browser" .
APPKIT_WORKFLOW_PROFILE=browser APPKIT_PLAYWRIGHT_MODULE=/ABSOLUTE/APPROVED/playwright/index.mjs "$workflow_dir/browser" > "$workflow_dir/browser.json"
APPKIT_WORKFLOW_PROFILE=check APPKIT_WORKFLOW_RECEIPT="$workflow_dir/browser.json" "$workflow_dir/browser"
go test -c -modfile=PREPARED/go.mod -tags appkit_contained,workflowprobe -trimpath -buildvcs=false -o "$workflow_dir/native" .
APPKIT_WORKFLOW_PROFILE=native "$workflow_dir/native" > "$workflow_dir/native.json"
APPKIT_WORKFLOW_PROFILE=check APPKIT_WORKFLOW_RECEIPT="$workflow_dir/native.json" "$workflow_dir/browser"
```

Run browser first, with no native candidate running. Native may open/focus two local
windows; it closes both and emits its receipt in `OnShutdown`. Native exit zero alone
is **not** success: require the separate strict receipt check. Missing, malformed,
incomplete, extra or reordered cases fail. `APPKIT_WORKFLOW_PROFILE` unset runs ordinary
Go tests without launching either client. Run Node tests separately:

```sh
go test -tags server,workflowprobe -race ./...
go test -modfile=PREPARED/go.mod -tags appkit_contained,workflowprobe -race ./...
node --test workflow_browser_test.mjs
```

## Bounds and evidence limits

The oracle gives each task 12 seconds and the overall task 90 seconds; the native
process has a 95-second hard watchdog. Timings include driver and polling overhead,
and Stop deliberately includes 1.4 seconds. These are failure-detection budgets,
not responsiveness benchmarks or comparisons between engines.
Record source/binary/receipt hashes, exact run commands, actual OS and observed client
identity alongside receipts. The checker validates content consistency, **not** signed
provenance or proof that a native binary ran: a fabricated receipt can be consistent.

This is programmatic DOM input, not physical keyboard, IME, trusted pointer/user
activation, focus traversal, screenshot review, visual fidelity or accessibility proof.
Evaluation is privileged test instrumentation and cannot certify page CSP enforcement.
Native error/warning/bridge counters do not substitute for a Web Inspector console
audit. Native window policy matches the host, but the probe's explicit Quit and menu
are test orchestration: normal menu shortcuts and last-window-close behavior remain
separate. Cookie sharing here is not ephemeral storage or cross-process isolation.
No private comparisons, permissions, network-loss UI, native crash recovery, signing,
installer/update rollback, remote hosting, Windows or consumer adoption is certified.
