# Synthetic Gallery streaming proof — 2026-09-13

## Result and boundary

Gallery now has an explicitly requested `/shell/events` snapshot stream over the same
synthetic profile/settings store and handler tree. This earns a narrow G6 prerequisite:
real HTTP delivery, bounded transport writes, isolation and local process shutdown.
It does **not** establish Datastar client compatibility, production auth or desktop support.
[ADR-0021](../adr/0021-runtime-profiles-and-platform-proof.md) retains the full target matrix.

No HTML, CSS or browser script changed for this specimen. No page opens a stream;
no Datastar client/SDK or Wails dependency was added and no CSP was loosened.
Ordinary saves still use CSRF-protected HTML POST, optimistic versions and redirects.
There is no new public runtime/session API, message bus, durable queue or telemetry.
The Gallery session is deliberately synthetic, not a consumer account backend.

## Observable transport contract

- Exact unencoded GET `/shell/events`, no query/body, exact single
  `Accept: text/event-stream`; loopback peer/Host and same-origin guards apply.
- A single existing signed-in session cookie is required. Missing/unknown/expired
  sessions return 403, signed-out 401, ambiguous cookies 400. Reads never allocate or
  extend a session. The original fixed 30-minute lifetime remains authoritative.
- Each connection starts with the current full snapshot. Optional `Last-Event-ID`
  accepts one canonical positive uint64 decimal string or empty; malformed cursors fail.
  Old/future valid cursors also resnapshot: no history retention or replay is promised.
- Frames use `datastar-patch-signals` with `data: signals` JSON in `appkitShellProbe`.
  Fields are `version`, `display_name`, `time_zone`, `start_page`, `digest`, `access`,
  `availability`, `activity`. Version is a decimal string, including above 2^53.
  No CSRF, cookie ID, nonce or one-shot failure control enters the projection.
- Subscription and initial snapshot are captured atomically; state-change notifications
  coalesce without a per-reader event queue. Intermediate versions may be skipped.
  JSON/write/flush do not hold the store mutex. Other sessions remain isolated.
- Limits: four streams per session, 32 total, 8 KiB per frame, five seconds per
  write/flush. Capacity refuses with 429 and `Retry-After: 1`; unsupported write-deadline
  hosts refuse with 500. A 15-second comment heartbeat is implemented, not timed here.
- Disconnect releases counters. Sign-out/expiry attempts a terminal null projection and
  closes; rapid sign-in never restores an old stream's authority. Redaction is best effort:
  already delivered/in-flight bytes cannot be recalled. Cancellation returns without it.
- A lost POST response leaves the outcome unknown. Read current saved state before an
  explicit version-checked retry; stale-write 409 is not an idempotent replay result.

Wire syntax follows [Datastar's SSE reference](https://data-star.dev/reference/sse_events)
and [WHATWG event framing](https://html.spec.whatwg.org/dev/server-sent-events.html).
Formatting those events ourselves is not a substitute for testing a chosen client pin.
Manual test reconnects do not establish a browser's automatic reconnect/backoff behavior.

## Reproducible repository checks

Environment: Go 1.26.3, darwin/arm64; macOS 26.6.2 (25G83). Code/test identities and
the separate process observation are recorded in the [receipt](fixtures/streaming/receipt.json).
These hashes cover named touched files, not a complete build-input manifest or release.

```sh
task check
go test -race ./apps/gallery ./apps/gallery/cmd/gallery -run '^(TestShellEvent|TestServeGallery)' -count=1
```

Both gates passed for this working-tree revision. Full `task check` includes Go vet/tests,
CUE validation, browser-state JS mocks and deterministic token regeneration. The focused
race command includes actual blocked-transport tests; mocks still do not prove browser UX.

| Evidence | What ran and what it proves |
|---|---|
| Real TCP HTTP journey | Two same-session readers and a separate session; initial flush, successful profile/settings POST convergence, stale 409 without snapshot, isolation, old/future-cursor reconnect, sign-out redaction/EOF |
| Refusal matrix | Wrong method/path/query/body/cursor/Accept, cookie ambiguity, cross-origin/Host/Fetch Metadata and non-loopback peer are refused; forwarded headers cannot grant loopback authority |
| Lifetime/DTO canaries | Cancellation/expiry release slots without TTL extension; exact allowlist, high versions, escaping, bounded frames, coalescing and permanent old-stream revocation |
| Initial-write review canaries | Revoke or cancel during response preparation prevents a fresh initial projection; unknown-length GET body (`ContentLength=-1`) is rejected even without transfer encoding |
| Blocked transport | Actual `net/http` over `net.Pipe`: a non-reading peer blocks Flush, an independent POST commits, real write deadline returns `os.ErrDeadlineExceeded`, handler/counters clean up |
| Command lifecycle | Real listener flushes, cancellation reaches active handler before shutdown, listener closes; independent Serve failure is retained and an uncooperative handler triggers bounded forced close |

The pipe test delegates deadlines to the real transport; it does not fake timeout results
or tune OS socket buffers. Three observed race repetitions timed out at approximately
5.002 seconds. This proves the configured path, not browser/TCP/WebView backpressure
performance. Test cleanup releases deliberately uncooperative code; Shutdown cannot kill
arbitrary application goroutines. Neither test suite exercises a signed native application.

## One actual compiled-process observation

The [preserved helper](fixtures/streaming/gallery_process_probe.go.txt) started exactly one
owned Gallery child with an ephemeral loopback listener and private assets disabled:

```sh
go build -trimpath -buildvcs=false -o /tmp/appkit-stream-gallery.3X5D1A/gallery ./apps/gallery/cmd/gallery
go run /tmp/appkit-stream-gallery.3X5D1A/probe.go
```

These are the historical invocation paths, not an installation recipe. For another
rehearsal, create a private temporary directory, build the reviewed Gallery there and
adapt the helper's single explicit executable path. Never target an existing user's process.
The receipt identifies the observed binary; it is not committed or claimed reproducible.

Observed: profile 200; initial event ID/version 1 and expected synthetic name; SIGTERM
to owned PID 95120 returned exit 0, active stream reached clean EOF and the listener
refused another connection. Observed shutdown elapsed 1 ms once, **not a latency claim**.
The process has exited. This helper does not POST; actual POST behavior is exercised
by the separate real-HTTP package journey. No browser, vendor project or consumer ran.

## G6 proof matrix after this change

| Required property | Current evidence | Still needed |
|---|---|---|
| Shared representative workflow | Same Go handler/store supports ordinary POST and explicit SSE | Pinned Datastar integration, browser/native task execution |
| Stream/reconnect bounds | Wire snapshots, coalescing, fixed expiry, manual reconnect, blocked-pipe timeout | Actual client reconnect/backoff, pinned-client cancellation, host streaming semantics |
| Authority and window consistency | Synthetic session origin/cookie/CSRF rules and independent HTTP readers | Production bootstrap/auth isolation; actual browser windows and WebViews |
| Focus/input/CSP | Existing no-JS forms retained; no new client | Keyboard, focus preservation, interrupted input and nonce CSP at selected pin |
| Shutdown | HTTP handler tests and owned macOS CLI process SIGTERM | Native window-close/reopen lifecycle on macOS and Windows |
| Native capabilities | None introduced | Required host adapters and honest browser-unavailable behavior |
| Packaging/update recovery | None established | Selected Wails pin, build/signature tools, artifacts and failure/rollback tests |

Chrome 152.0.7977.84 was running; approved extension/native-host diagnostics passed,
but the browser tool could not bind `chrome` after documented recovery. No browser
task ran. Reconnect/reinstall the Browser plugin from the app UI, or obtain approval
for the already-offered in-app-browser switch. No workaround accessed profile/session data.

Next: bind the smallest explicit read-only activity observer to a chosen, verified
Datastar pin and exercise it in a working browser before claiming client compatibility;
then reuse the same handler in the pinned Wails conformance spike. G6 remains open.
NATS, native mobile and accelerated client rendering retain their existing promotion
conditions; this transport specimen does not promote or pre-build them.
