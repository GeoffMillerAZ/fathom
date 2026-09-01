# ADR-0005: Serve every app from a real loopback HTTP listener; shells are chrome (the Loopback Invariant)

- **Status:** Accepted
- **Date:** 2026-06-10
- **Decision drivers:** hard webview transport facts that no amount of code
  can route around; the one-binary-everywhere thesis (VISION.md, question 4);
  shell mortality — every shell we might pick is younger than our horizon;
  Datastar's SSE-first transport (D3) must work identically on every target

## Context

The kit's UI layer is Datastar v1.0.2 over SSE: one long-lived GET stream
per tab, short command POSTs, fetch()-parsed `text/event-stream` (D3,
[adr/0003-hypermedia-first-datastar.md](0003-hypermedia-first-datastar.md)).
Whatever transport we choose must deliver streamed responses incrementally
and deliver POST bodies intact, on every platform we will ever ship to.

The obvious desktop transport — serving the app through the webview's
request-interception/asset-handler layer — fails these requirements on
hard, dated, upstream facts:

- **Windows/WebView2 buffers entire responses.** Wails v3's Windows
  response writer accumulates all writes in a `bytes.Buffer` and only
  delivers on `Finish()` — a long-lived SSE response never emits a single
  event (https://github.com/wailsapp/wails/blob/master/v3/internal/assetserver/webview/responsewriter_windows.go,
  verified 2026-06). This is not a Wails bug: the Wails maintainer confirmed
  on 2026-05-02 that it "is exactly the upstream bug in WebView2"
  (https://github.com/wailsapp/wails/issues/5047), referring to
  MicrosoftEdge/WebView2Feedback#3519 — a streaming-response feature request
  open since 2023-05-28, still open as of 2026-05-12
  (https://github.com/MicrosoftEdge/WebView2Feedback/issues/3519).
- **Android WebView drops intercepted POST bodies.** Wails' experimental
  mobile PR notes that Android's `shouldInterceptRequest` does not deliver
  `fetch()` POST bodies; the PR works around it with a JavascriptInterface
  transport (https://github.com/wailsapp/wails/pull/5562, opened
  2026-06-10). Datastar commands are POSTs; interception eats them.
- **The Wails asset server rejects WebSocket upgrades with 501** on all
  platforms (https://github.com/wailsapp/wails/blob/master/v3/internal/assetserver/assetserver.go,
  verified 2026-06). Even the escape hatch we don't want is closed.
- **WKWebView custom-scheme streaming is undocumented.** macOS happens to
  stream (Wails calls `didReceiveData` per write), but progressive delivery
  to `fetch()` over custom schemes is not documented behavior, and custom
  schemes are treated as insecure contexts with mixed-content quirks
  (https://forums.developer.apple.com/forums/thread/725916). Working today
  by accident is not a foundation.

Meanwhile, the alternative is sitting right there: a Go process can serve a
real `net/http` listener on `127.0.0.1` and point the webview at it, and
then SSE, POSTs, HTTP/2-someday, caching, cookies, and CSP all behave
exactly as in a browser — because it *is* a browser hitting a real server.
This is proven prior art for Datastar-in-a-webview (stukennedy/irgo,
https://github.com/stukennedy/irgo) and the community-consensus workaround
in both the Wails and Tauri ecosystems
(https://github.com/orgs/tauri-apps/discussions/14552). Wails v3's own
experimental server mode (`-tags server`, pure HTTP server, no CGO,
https://github.com/wailsapp/wails/tree/master/v3/examples/server) confirms
that "the app is an HTTP server" is the shape Wails apps already have.

## Decision

Every kit app is **one Go `http.Handler` tree, served by a real `net/http`
listener bound to `127.0.0.1:0`** (ephemeral port chosen by the OS). All
app traffic — HTML documents, the Datastar SSE stream, command POSTs, auth,
vendored static assets — flows through that listener. Shells are chrome
pointed at that URL: a Wails v3 window (D6), a browser tab, a cloud
deployment behind real auth, a future mobile webview.

Authentication of the loopback origin: at launch the app generates a random
per-launch bearer token; the shell navigates to a one-shot exchange route
that consumes the token and sets an HttpOnly session cookie; every other
route requires the cookie. Non-loopback binds are refused unless explicitly
configured for cloud deployment, where real auth replaces the token dance
(D14, [adr/0014-security-baseline.md](0014-security-baseline.md)).

**No app code may depend on shell-specific JS bindings, IPC, or events.**
If a native capability is needed (tray, dialogs, secrets, updates), it goes
through the kit's capability interface with per-shell implementations
([design/platform-contract.md](../design/platform-contract.md)) — never
through `window.wails`, Tauri `invoke`, or any equivalent.

Boundaries of this decision:

- It does not pick the desktop shell — that is
  [adr/0006-wails-v3-desktop-shell.md](0006-wails-v3-desktop-shell.md).
- It permits exactly one use of the webview asset handler: a static
  bootstrap/splash page and error chrome that contains no app logic and
  never streams. The split is specified in the platform contract.
- Cloud deployments keep the same handler tree but bind a configured
  address with real authentication; the Loopback Invariant governs the
  *shape* (one handler tree, HTTP as the only app transport), not the
  literal bind address in every environment.

## Alternatives considered

- **Serve the app through the webview asset handler (the "native" Wails
  path).** Attractive: zero open ports, zero token lifecycle, the
  documented happy path of every webview framework. Lost on facts, not
  taste: WebView2 full-response buffering makes SSE impossible on Windows
  (upstream #3519, open since 2023, maintainer-confirmed 2026-05); Android
  interception drops POST bodies (PR #5562); WebSockets 501; WKWebView
  streaming undocumented. Three of our four targets break the core
  transport. Unrecoverable.
- **Per-platform transports** — asset handler on macOS/Linux where
  streaming happens to work, long-polling or chunk-batching shims on
  Windows, JavascriptInterface bridge on Android. Attractive: each platform
  uses its "native" path. Lost because it multiplies every UI feature by
  the number of transports: reconnection, compression, auth, and the
  Datastar contract would all need per-platform variants, and the web tier
  would still be different again. The kit exists to make one codebase; this
  is the complexity explosion the kit exists to prevent. It also keeps the
  worst transport (Windows shims) on our highest-priority desktop target.
- **WebSockets over the loopback listener.** Attractive: bidirectional,
  fashionable. Lost because Datastar v1 is SSE-native (D3) and the
  surrounding ecosystem (MCP's streamable HTTP, the Tao-of-Datastar
  patterns) has converged on plain HTTP + SSE; WebSockets would buy us a
  second framing protocol, a second auth story, and zero capability we
  need. Inside Wails' asset transport they are 501-rejected anyway.
- **Shell JS bindings as the app transport** (Wails services/bindings,
  call-by-ID). Attractive: typed calls, no port, framework-blessed. Lost
  because it welds app code to one shell's API — the exact dependency that
  makes shell failure a rewrite instead of a swap — and it still doesn't
  stream on Windows (IPC payloads were being chunked around a 2MB WebView2
  limit as recently as alpha.89, Wails PR #5369). VISION.md's survival rule
  ("everything else is a shell") dies the day we call `window.wails` from
  app code.

## Consequences

- **Easier:** web, desktop, and cloud become literally one binary with one
  transport; the browser-tab tier is free forever (zero-install, ultimate
  shell hedge); shell swap (Wails → Tauri-as-chrome → owned webview) is a
  chrome change, not an app change; everything we know about HTTP —
  middleware, compression, cookies, CSP, testing with httptest — applies
  unmodified on every target.
- **Harder, and what we do about it:**
  - *An open port other local processes can probe.* Handled: bind strictly
    `127.0.0.1` (never wildcard), per-launch random bearer token exchanged
    one-shot for an HttpOnly SameSite=Strict cookie, Host-header check
    against DNS rebinding, constant-time token compare. Specified in
    [design/platform-contract.md](../design/platform-contract.md) and D14.
  - *Windows firewall prompts.* Loopback-only binds avoid prompts in the
    common case; some endpoint-security configurations may still flag a
    listening socket. Accepted; we never bind non-loopback on desktop, and
    the packaging docs note the known-good configuration.
  - *Token lifecycle.* One-shot, short TTL, consumed on first exchange,
    never logged, stripped from history by an immediate 303 redirect.
  - *Port races and stale instances.* `127.0.0.1:0` makes bind conflicts
    structurally impossible; single-instance lock + liveness probe handles
    stale lockfiles; the shell only navigates after the listener is bound.
- **Risks accepted, with tripwires:**
  - *iOS App Store review of an in-app loopback listener* (a known review
    consideration). Tripwire: if a kit app is rejected for the listener,
    revisit the mobile transport — the candidate fallback is the PR #5562
    JavascriptInterface-style bridge behind the same `http.Handler`.
  - *A long-lived SSE stream per tab is held by `net/http`* — trivial at
    personal scale; cloud deployments should run HTTP/2 to dodge the
    browser's ~6-connection HTTP/1.1 per-origin cap. Tripwire: any kit app
    that regularly exceeds hundreds of concurrent streams.

## Revisit when

- WebView2 ships streaming interception (MicrosoftEdge/WebView2Feedback#3519
  closes) **and** Android interception delivers POST bodies **and** WKWebView
  documents custom-scheme streaming — all three, since the invariant must
  hold everywhere. Even then, the one-binary and shell-swap payoffs likely
  keep the loopback design; only the *forced* framing would soften.
- An OS or store policy materially restricts loopback listeners on a target
  we ship to.
- Datastar abandons SSE as its primary transport (e.g., "Project Darkstar"
  WebTransport becomes the default, https://data-star.dev/essays/v1_and_beyond).
