# ADR-0006: Adopt Wails v3 (pinned alpha) as the desktop shell

- **Status:** Accepted
- **Date:** 2026-06-10
- **Decision drivers:** Go must be the host process (D2 — no Node, no
  second toolchain); the shell is replaceable by construction (D5, the
  Loopback Invariant), so we can afford a young shell; we need the
  undifferentiated desktop 20% — chrome, packaging, signing, updates —
  done by someone else; mobile must stay a tracked option, not a bet

## Context

The Loopback Invariant ([adr/0005-loopback-invariant.md](0005-loopback-invariant.md))
makes the shell a commodity: chrome pointed at `http://127.0.0.1:<port>`,
plus native capabilities behind the kit's capability interface. What we
need from a shell is therefore narrow — window/tray/menu chrome, native
dialogs, packaging, signing, auto-update — and what we refuse from a shell
is equally narrow: it must never become the app's transport or API surface.

State of the field, verified 2026-06-10:

- **Wails v3** is at v3.0.0-alpha.99 (published 2026-06-10; near-daily
  releases from master — alpha.84 through .99 all shipped 2026-05-03 to
  2026-06-10, https://github.com/wailsapp/wails/releases). Officially
  alpha; the docs call the API "reasonably stable" with apps in production
  (https://v3.wails.io/status/). Maintainer Lea Anthony on timing: "almost
  in Beta. No fixed release date" (2025-07-25) and "When it's ready. And
  it's nearly ready." (2025-12-03)
  (https://github.com/wailsapp/wails/discussions/4447); branches named
  `agent/beta-release-manager` (visible 2026-06) show an active beta push.
  v3 gives Go-as-host-process: procedural multi-window API, systray with
  window attachment, namespaced dialogs, Taskfile-based builds, in-tree
  packaging for DMG/NSIS/MSIX/AppImage, a `wails3 setup signing` wizard +
  `darwin:sign:notarize` task
  (https://github.com/wailsapp/wails/blob/master/docs/src/content/docs/guides/build/signing.mdx),
  and a built-in cryptographically verified updater (`app.Updater`:
  pluggable providers incl. GitHub Releases, sha256/signature verification,
  atomic binary swap,
  https://github.com/wailsapp/wails/blob/master/docs/src/content/docs/guides/updater.mdx).
- **Alpha means churn**: alpha.91 (2026-05-12) was a breaking macOS
  coordinate-system change (Y-down normalization); alpha.89 changed IPC
  payload chunking (PR #5369)
  (https://github.com/wailsapp/wails/blob/master/docs/src/content/docs/changelog.mdx).
- **Wails mobile** went from aspiration (WEP PR #4078 closed unmerged
  2025-09-24) to a real experiment **today**: PR #5562, opened by the
  maintainer 2026-06-10, adds experimental iOS + Android from the same
  `main.go` (iOS: WKWebView + `wails://` scheme; Android: WebView +
  `libwails.so` via JNI). Unmerged, simulator/emulator-verified only,
  dialogs incomplete, and Android interception drops fetch() POST bodies
  (https://github.com/wailsapp/wails/pull/5562).
- **Tauri v2** is the most mature alternative shell: stable since
  2024-10-02 with mobile shipped, core at v2.11.2 (2026-05-16)
  (https://v2.tauri.app/blog/tauri-20/,
  https://github.com/tauri-apps/tauri/releases). Go enters only as a
  sidecar binary (https://v2.tauri.app/develop/sidecar/) — and sidecars do
  not work on mobile: "Not yet, no"
  (https://github.com/orgs/tauri-apps/discussions/11454), tracking issue
  open (https://github.com/tauri-apps/tauri/issues/9774); iOS forbids
  spawning child processes at the OS level, so Go-as-sidecar is
  structurally impossible there.
- Single-maintainer concentration on Wails (substantially Lea Anthony, now
  visibly using AI agents for release engineering) is a real bus-factor
  consideration; Linux/WebKitGTK is the shared weak rendering engine across
  Wails, Tauri, and plain webview.

## Decision

We adopt **Wails v3, pinned at ≥ v3.0.0-alpha.99**, as the kit's desktop
shell, upgraded deliberately (pin exact version; budget one upgrade pass
per kit minor release; read the changelog, run the Gallery's visual
baseline, then bump).

**What we use:**

- Window, menu, and tray chrome via the v3 procedural API (multi-window,
  systray with window attachment).
- Native dialogs (file open/save, message) behind the kit's capability
  interface — app code calls `appkit/shell`, never Wails directly.
- Packaging: DMG (macOS), NSIS and MSIX (Windows), AppImage (Linux) via
  the in-tree `wails3` Taskfile tasks.
- Signing: `wails3 setup signing` wizard (credentials in OS keychain),
  `wails3 task darwin:sign:notarize` for macOS notarization, Windows
  signing from any OS.
- Auto-update: `app.Updater` with the GitHub Releases provider +
  `SHA256SUMS` verification, wired by the kit
  ([design/platform-contract.md](../design/platform-contract.md), §5).

**What we deliberately do NOT use:**

- The asset-server transport for app traffic. The webview loads the
  loopback URL; the asset handler serves only the static bootstrap/error
  chrome (D5; the split is specified in the platform contract, §3).
- JS bindings, services-as-IPC, runtime events, `wails3 generate bindings`
  — none of it. App code speaks HTTP to itself; the moment `window.wails`
  appears in app code, the shell stops being swappable.

**Mobile** is a tracked experiment, not a commitment: we watch PR #5562
(same-`main.go` iOS/Android), keep the kit's boot layer shaped so that
build works when it merges, and ship **PWA as the mobile tier-1** in the
meantime (accepting iOS's manual add-to-home-screen and missing background
sync). Plan B for native mobile is gomobile-as-library + an owned
WKWebView/Android WebView pointed at the in-process loopback listener —
the Invariant makes that a shell-build, not a rewrite.

**Fallbacks, kept warm:** (1) Tauri v2.11.x + Go sidecar as the tested
desktop fallback — a minimal Tauri wrapper pointing wry at the same
loopback server lives in the repo and gets smoke-tested per release;
desktop-only by construction (sidecars impossible on iOS). (2) The browser
tab, always, for free (D5).

## Alternatives considered

| Shell | Why attractive | Why it lost |
|---|---|---|
| Tauri v2 primary | Stable 20 months, mobile shipped, tiny binaries (~2.5–10MB) | Permanent Rust toolchain tax (config, capabilities, plugins in Rust — violates the spirit of D2); dual-binary signing/notarization surface; sidecar supervision edge cases; and its one decisive advantage — mobile — evaporates for us because Go sidecars can't run there (#11454, #9774) |
| Electron | Healthiest project surveyed (v42.4.0, 2026-06-09; multiple releases/week); boring, mature sidecar pattern | ~85–120MB of Chromium+Node per app; Node as host process is a direct D2 violation; desktop-only; everything it adds over a webview we either don't need or get from Wails at 1/10 the weight |
| webview/webview_go | The minimal wrapper our architecture deserves on paper | Go binding dormant since 2024-08-31 (~22 months); no packaging, signing, or update tooling — we'd rebuild Wails ourselves on an unmaintained base. Fork-and-own is plan C, not a starting point |
| Gio / Fyne | Healthy, actively maintained pure-Go toolkits (Gio v0.10.0, 2026-05-26; Fyne v2.7.4, 2026-05-12); real mobile | Structurally cannot host HTML/CSS: the entire templ+Datastar+Tailwind investment (D3, D4) would be rewritten as Go draw code, and their "web" targets are canvas blobs — the inverse of our architecture |
| PWA-only | Zero shell code; the architecture already serves it | Capability ceiling (no tray, no real file scopes, no reliable background work on iOS); app identity is weak; hostage to Apple policy (the Feb-2024 EU DMA home-screen-web-app episode, reversed Mar 2024, shows iOS PWA capability can vanish in a point release). Kept as a *tier*, rejected as the *only* tier |
| Wails v2 | The stable Wails (v2.12.0, 2026-03-26) | Maintenance mode, single-window, opaque build system, no updater, and a v3 migration later anyway. Wrong base for a multi-decade bet |

The honest cost of the winner: we are pinning an **alpha** with no
committed beta date, from a substantially single-maintainer project, and
breaking changes are still landing (alpha.91's coordinate flip was
2026-05-12 — four weeks ago). We pay an explicit upgrade budget and keep
two fallbacks warm precisely because of this.

## Consequences

- **Easier:** Go is the host process — one language, one debugger, one
  binary; desktop chrome, packaging, signing, notarization, and verified
  auto-update are someone else's well-built code; the Gallery and every
  kit app get DMG/NSIS/MSIX targets from Taskfile invocations.
- **Harder, and what we do about it:** alpha churn — pinned exact
  versions, upgrades as deliberate passes with the Gallery as the visual
  regression gate (D12); Wails-specific build tags and Taskfiles live only
  in the kit's boot layer, so churn never touches app code.
- **Risks accepted, with tripwires:**
  - *v3 stalls before beta.* Tripwire: six months without releases, or a
    regression that blocks us with no upstream response — then promote the
    Tauri wrapper or browser-tab tier while we decide.
  - *Maintainer bus-factor.* Tripwire: maintainer steps back without a
    succession story — same response; the Invariant caps the blast radius
    at the boot layer.
  - *Mobile experiment dies (PR #5562 closed unmerged, like #4078).*
    Tripwire: PR closed or six months stale — PWA remains tier-1; schedule
    the gomobile + owned-webview spike.
  - *WebKitGTK weirdness on Linux.* Accepted; Linux desktop is our lowest
    tier, and the browser tab is the escape hatch.

## Revisit when

- Wails v3 ships beta/stable → re-pin, shrink the upgrade budget.
- PR #5562 (or a successor) merges and survives two releases → schedule
  the mobile shell build against real devices.
- Tauri ships supported mobile sidecars or in-process library embedding
  (#9774 closes in our favor) → re-score the fallback as a contender.
- The Wails release cadence stops or the maintainer steps back → execute
  the fallback ladder above.
