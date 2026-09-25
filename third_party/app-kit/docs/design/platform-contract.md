# The Platform Contract

This is an earlier platform implementation sketch, not shipped API documentation.
[ADR-0021](../adr/0021-runtime-profiles-and-platform-proof.md) scopes runtime choices
and platform proof; [scope](../intent/scope.md) names what exists. Dynamic Go/HTTP
desktop traffic retains the loopback boundary. Portable and terminal consumers do
not inherit this platform; optional services are built only for an admitted need.
The [isolated native experiment](../../spikes/wails-gallery/README.md) is the current G6
implementation/proof boundary. ADR-0026 admits its module and ordinary HTML CSRF path;
the unimplemented lifecycle/capability sketches below are not its availability claims.

How a kit app boots, authenticates itself to itself, acquires a shell, and
degrades gracefully when the shell is missing. This is the detailed design
behind D5 (the Loopback Invariant,
[../adr/0005-loopback-invariant.md](../adr/0005-loopback-invariant.md)) and
D6 (Wails v3 shell,
[../adr/0006-wails-v3-desktop-shell.md](../adr/0006-wails-v3-desktop-shell.md)).
Security posture defers to D14
([../adr/0014-security-baseline.md](../adr/0014-security-baseline.md)).

The contract in one sentence: **an app is `func New() http.Handler` plus an
`app.cue` manifest; everything else in this document is the kit's job.**

Packages introduced here:

| Package | Responsibility |
|---|---|
| `appkit/app` | `Run()`, manifest loading, wiring |
| `appkit/web/server` | loopback listener, port acquisition, instance lock, idle shutdown |
| `appkit/auth` | token→cookie exchange |
| `appkit/shell` | capability interfaces + degradation levels |
| `appkit/shell/wails3` | Wails v3 implementation (build tag `!server`) |
| `appkit/shell/headless` | browser-tab / cloud implementation (build tag `server`) |

---

## 1. Lifecycle

### 1.1 Boot sequence (all shells)

```
main.go
  └─ appkit.Run(myapp.New)            // one line; everything below is the kit
       1. load + validate app.cue manifest (D7)
       2. acquire single-instance lock (or forward to live instance, exit)
       3. bind 127.0.0.1:0  ──────────── port now known, no race possible
       4. generate launch token (32B crypto/rand, base64url)
       5. start http.Server on the listener (handler tree fully mounted)
       6. start shell:
            wails3  → window at asset-served bootstrap page
                      → bootstrap polls /healthz, then location.replace(launchURL)
            headless→ open default browser at launchURL (--browser)
                      or print URL / bind configured addr (cloud)
       7. shell (or tab) hits GET /auth/launch?token=… → cookie → 303 /
       8. app runs; lifetime policy per §1.5
```

Order matters: the listener is bound and serving **before** any shell
exists, so "shell navigates to a dead port" cannot happen. Binding `:0`
means port conflicts cannot happen either — the OS picks a free port and
tells us:

```go
// appkit/web/server
func Start(ctx context.Context, h http.Handler, o Options) (*Server, error) {
    ln, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil {
        return nil, fmt.Errorf("loopback: bind: %w", err)
    }
    s := &Server{
        ln:    ln,
        port:  ln.Addr().(*net.TCPAddr).Port,
        token: newToken(), // 32 bytes crypto/rand, base64url, one-shot
    }
    s.http = &http.Server{Handler: s.guard(h)} // guard = auth + Host check
    go s.http.Serve(ln)
    return s, nil
}

func (s *Server) BaseURL() string   { return fmt.Sprintf("http://127.0.0.1:%d", s.port) }
func (s *Server) LaunchURL() string { return s.BaseURL() + "/auth/launch?token=" + s.token }
```

### 1.2 Token → cookie exchange

The launch token appears in exactly one URL, once. The exchange route
(served by `appkit/auth`):

- `GET /auth/launch?token=<t>` — constant-time compare against the stored
  token; on match, **consume it** (it never validates again), set
  `appkit_session=<fresh 32B random>` as `HttpOnly; SameSite=Strict; Path=/`
  (no `Secure` flag — the origin is plain-HTTP loopback, which browsers
  treat as a potentially-trustworthy origin), and reply `303 See Other → /`.
  The redirect strips the token from the address bar and history.
- Unconsumed tokens expire 120 seconds after boot — long enough for a slow
  WebView2 cold start, short enough to be useless in a log scrape. If the
  token expires before exchange (shell crashed mid-launch), the kit
  regenerates and relaunches the shell rather than extending the window.
- Every other route requires the session cookie. Exceptions: `/healthz`
  (returns only `ok`, no app data) and `/instance/*` (separate secret, §1.4).

The `guard` middleware also rejects any request whose `Host` header is not
exactly `127.0.0.1:<port>` — the standard DNS-rebinding defense for
loopback servers (D14). `localhost` is rejected; shells always open
`127.0.0.1`, never `localhost`.

### 1.3 Browser-tab mode: heartbeat and self-termination

In headless mode nothing owns the process lifetime — the tab can be closed,
and a zombie server on a random port is unacceptable. The Datastar
architecture already gives us the liveness signal for free: every kit page
holds one long-lived SSE stream (D3). The kit's SSE helper increments a
connection gauge on open and decrements on close:

- **Alive:** ≥1 open kit SSE stream.
- **Grace:** 0 streams → start a 90-second timer (covers reloads,
  navigation, and laptop wake). Any new stream cancels it.
- **Dead:** timer fires → graceful `http.Server.Shutdown`, flush state,
  release instance lock, exit 0.

One Datastar detail makes this honest: GET streams default to
`openWhenHidden: false`, meaning a backgrounded tab drops its stream
(https://data-star.dev/reference/actions, Datastar v1.0.2, 2026-06). The
kit's main stream therefore sets `openWhenHidden: true` in headless mode so
a backgrounded-but-open tab still counts as alive. Desktop and cloud builds
disable idle shutdown entirely (window lifetime and the deploy platform own
the process, respectively).

### 1.4 Single instance and deep-link forwarding

One app, one process, one port. The lock lives at
`os.UserConfigDir()/<app-id>/instance.json`, mode 0600:

```json
{"pid": 4242, "port": 52731, "secret": "<32B base64url>"}
```

Second-launch protocol: read the lock → `POST /instance/forward` on the
recorded port with `X-Instance-Secret` and a body carrying argv + any
deep-link URL → on `200`, exit 0 (the live instance focuses its window via
`shell.Windows()` and publishes the deep link on the bus as
`app.deeplink.received`, D8) → on connection-refused or secret mismatch,
the lock is stale: delete it, take it, proceed with boot. The secret is
file-permission-protected, same-user-only — exactly the trust boundary we
want, and it never travels off-box.

OS-level deep-link *registration* (custom URL scheme from the manifest's
identity block in `app.cue`) is a shell capability (`shell.DeepLinks()`);
forwarding is the kit's, and works identically whichever shell registered
the scheme.

---

## 2. The capability interface — `appkit/shell`

App code never imports a shell. It asks the kit for capabilities and gets
truthful answers about what the current shell can do.

```go
package shell

type Level int

const (
    Absent    Level = iota // no-op implementation; calls succeed and do nothing
    Degraded               // works with reduced semantics (documented per feature)
    Supported              // full native behavior
)

type Feature string

const (
    FeatWindows       Feature = "windows"
    FeatDialogs       Feature = "dialogs"
    FeatTray          Feature = "tray"
    FeatNotifications Feature = "notifications"
    FeatClipboard     Feature = "clipboard"
    FeatSecrets       Feature = "secrets"
    FeatFileScopes    Feature = "filescopes"
    FeatDeepLinks     Feature = "deeplinks"
    FeatUpdater       Feature = "updater"
    FeatPower         Feature = "power"
)

// Shell is what appkit.Run gives the app. Getters never return nil:
// an Absent feature returns a no-op implementation, so app code never
// nil-checks — it Level-checks when behavior matters.
type Shell interface {
    Level(Feature) Level

    Windows() Windows
    Dialogs() Dialogs
    Tray() Tray
    Notifications() Notifications
    Clipboard() Clipboard
    Secrets() SecretStore
    Files() FileScopes
    DeepLinks() DeepLinks
    Updater() Updater
    Power() PowerEvents
}
```

Representative interfaces (full set lives in the package; these set the
tone — small, blocking, error-honest):

```go
type Dialogs interface {
    OpenFile(ctx context.Context, o OpenOptions) (path string, err error)
    SaveFile(ctx context.Context, o SaveOptions) (path string, err error)
    Message(ctx context.Context, o MessageOptions) (choice string, err error)
}

type SecretStore interface { // OS keychain on desktop; backend per matrix
    Get(service, key string) ([]byte, error)
    Set(service, key string, value []byte) error
    Delete(service, key string) error
}

type Updater interface {
    Check(ctx context.Context) (*Release, error)
    Apply(ctx context.Context, r *Release) error // Supported: verified atomic swap
    // Degraded: Check works, Apply returns ErrManualUpdate with instructions.
}

type PowerEvents interface {
    Events() <-chan PowerEvent // Suspend, Resume, ShutdownImminent; Absent: never sends
}
```

### 2.1 Implementation matrix

**Column semantics:** the matrix describes what the kit's shell *package*
delivers on that target — whether the mechanism underneath is Wails, our
own OS-API code, or a web API is an implementation detail. (Example:
`PowerEvents` is not a Wails feature; the wails3 shell package implements
it directly against the OS.)

| Feature | wails3 (desktop) | headless: browser tab | headless: cloud | future-mobile (tracked, PR #5562) |
|---|---|---|---|---|
| Windows | **S** — multi-window, focus, fullscreen | **A** — one tab; focus/open are no-ops | **A** | **D** — single fullscreen surface |
| Dialogs | **S** — native pickers | **D** — kit HTML modals; file open = upload picker (copies, no path) | **D** — kit HTML modals | **D** — PR notes save/open incomplete |
| Tray | **S** — systray + window attach | **A** | **A** | **A** |
| Notifications | **S** planned | **D** — Web Notifications, permission-gated | **D** — in-app toasts only | **D** — TBD by experiment |
| Clipboard | **S** planned | **D** — async clipboard API, user-gesture-gated | **D** — same | **D** |
| SecretStore | **S** — OS keychain | **S** — same Go process, same keychain | **D** — env/mounted secrets | **S** planned — Keychain/Keystore |
| FileScopes | **S** planned — picker-granted paths | **D** — configured dirs only, no grants | **D** — server volume only | **D** — sandbox container only |
| DeepLinks | **S** planned — OS scheme registration + forward | **D** — receives forwards; no OS registration | **D** — https URLs *are* the links | **S** planned — universal links |
| Updater | **S** — verified atomic swap (§5) | **D** — check + notify, manual replace | **A** — deploys own it | **A** — stores own it |
| PowerEvents | **S** planned — own OS-API impl | **A** — cgo-free build, no OS hooks | **A** | **D** — app lifecycle events |

S = Supported, D = Degraded, A = Absent. "Planned" entries in the wails3
column (Notifications, Clipboard, FileScopes, DeepLinks, PowerEvents) are
committed but not yet built — each lands with its first consumer. The
future-mobile column is provisional until Wails PR #5562 (opened
2026-06-10, unmerged) or the gomobile plan-B ships; it exists so app
authors see today which features they should gate.

### 2.2 How app code consumes the matrix

Two mechanisms, both mandatory:

1. **Manifest declaration.** `app.cue` declares capability needs:

   ```cue
   capabilities: {
       required: ["secrets"]            // refuse to start without these
       optional: ["tray", "deeplinks"]  // app adapts at runtime
   }
   ```

   At boot, `appkit.Run` fails fast — with the matrix row in the error —
   if a required feature is `Absent` on the current shell. CI can check
   "does this app run on cloud?" by evaluating the manifest against the
   matrix without booting anything (D7: the matrix itself is CUE data).

2. **Runtime queries** for optional adaptation:

   ```go
   if sh.Level(shell.FeatTray) >= shell.Degraded {
       sh.Tray().SetMenu(trayMenu())
   }
   ```

   Templates get the same answer via the kit's view context, so nav items
   for shell-gated screens render only where they work.

---

## 3. Static assets vs. app traffic: the split

**Everything that is the app goes over loopback. The Wails asset handler
serves only the bootstrap and error chrome.** Precisely:

Over the loopback listener (every shell, identically):

- HTML documents, the per-tab SSE stream, command POSTs, auth routes
- All static app assets: vendored `datastar.js` (~34KB, go:embed, D3),
  compiled Tailwind CSS, fonts, icons

Via the Wails asset handler (`application.Options.Assets`, wails3 build
only):

- `bootstrap.html` — paints instantly when the window opens, polls
  `/healthz` on the loopback port (passed via query parameter), then
  `location.replace(launchURL)`. Pure inline HTML/CSS, no app data, no
  Datastar, nothing that streams.
- `error.html` — shown if `/healthz` never answers; renders diagnostics
  and a "copy support bundle path" affordance.

Why not serve app statics through the asset handler on desktop? Three
reasons, in order:

1. **One origin per app.** If statics came from `wails://` and documents
   from `127.0.0.1`, every page spans two origins: CSP `script-src 'self'`
   (D14) stops covering our own JS, cookies don't flow to asset requests,
   and caching semantics fork between desktop and web. The Invariant's
   whole value is that desktop *is* the web build.
2. **The asset transport is the thing D5 exists to avoid.** It buffers
   whole responses on Windows and 501s WebSockets
   ([ADR-0005](../adr/0005-loopback-invariant.md) has the receipts); the
   fewer request classes touch it, the smaller the platform-quirk surface.
3. **Uniform testing.** `httptest` against the one handler tree covers
   statics, SSE, and commands on every tier; asset-handler-served files
   would be the only untested path.

The bootstrap page is the exception because it must paint *before* the
loopback server is provably up, and because a "server failed to start"
error page by definition cannot come from the server.

---

## 4. Build matrix

One `main.go` per app. The variation lives in the kit's boot layer behind
build tags, mirroring Wails' own convention (`-tags server` is the Wails v3
experimental server-mode tag — our headless tag is deliberately the same
word, so the two toolchains agree about what a "server" build means;
https://github.com/wailsapp/wails/tree/master/v3/examples/server, verified
2026-06).

```
main.go                           // at the app root; identical for every target:
                                  //   func main() { appkit.Run(myapp.New) }
appkit/app/boot_wails3.go         // //go:build !server  → wails3 shell
appkit/app/boot_headless.go       // //go:build server   → browser-tab + cloud
```

| Target | Build | Notes |
|---|---|---|
| Desktop dev | `wails3 dev` (Taskfile) | hot reload via templ watch + wgo (D4) |
| Desktop release | `wails3 task {darwin,windows,linux}:package` | DMG / NSIS+MSIX / AppImage; signing per §5 |
| Browser-tab tier | `CGO_ENABLED=0 go build -tags server` → `./app --browser` | opens default browser at LaunchURL; idle shutdown on (§1.3) |
| Cloud | same binary → `./app --listen :8080` | non-loopback bind requires explicit config + real auth (D14); container `FROM scratch`-friendly (cgo-free) |
| Future mobile | tracked: PR #5562 same-`main.go` build when merged | plan B: gomobile c-archive + owned webview at the in-process loopback URL |

Rules with teeth:

- The `server` build **must not link Wails** — `appkit/shell/wails3` is
  `!server`-tagged all the way down. CI asserts this with
  `go list -deps -tags server` (no `wailsapp` in the closure). This keeps
  the cloud/tab binary cgo-free, scratch-deployable, and immune to alpha
  churn.
- The desktop binary degrades to headless behavior at *runtime* (`--browser`
  flag, or automatic fallback per §6) — degradation never requires a
  different build artifact in hand.
- `--listen` with a non-loopback address refuses to start unless the
  manifest's cloud block configures real authentication (D14). There is no
  "oops, bound 0.0.0.0 with the launch-token auth" failure mode.

---

## 5. Update and distribution flows

| Target | Distribution | Update |
|---|---|---|
| macOS | signed + notarized DMG (`wails3 setup signing`, `wails3 task darwin:sign:notarize`) | `app.Updater`, GitHub Releases provider, SHA256SUMS verification, atomic binary swap (Wails v3 ≥ alpha.99, 2026-06) |
| Windows | signed NSIS installer (default); MSIX where store-style install matters | same `app.Updater` flow |
| Linux desktop | AppImage | same `app.Updater` flow |
| Browser-tab tier | bare binary from GitHub Releases, or `go install` (private module, D13) | Degraded updater: checks Releases, notifies in-app, instructs; no self-swap |
| Cloud | container image (cgo-free `server` build) | redeploy; the binary never self-updates in a container |
| PWA (mobile tier-1) | the cloud deployment + manifest/service worker | server-side deploy *is* the update — instant, no store |
| Future mobile native | store pipelines (TestFlight / Play) | store-mediated; `Updater` is Absent by design |

Release mechanics (D13): a kit app release is one CI job — build all
targets, sign, generate `SHA256SUMS`, publish a GitHub Release. The
updater's verification chain (sha256 + signature, atomic swap, rollback on
failed verify) is Wails' built-in updater state machine
(https://github.com/wailsapp/wails/blob/master/docs/src/content/docs/guides/updater.mdx,
verified 2026-06); the kit wires the provider and surfaces update events
on the bus (`app.update.available` → a kit-standard toast pattern).

---

## 6. Failure modes

| Failure | Detection | Handling |
|---|---|---|
| Port conflict | impossible at bind (`127.0.0.1:0`) | only pinned dev ports (`--port`) can conflict; those fail fast with the owning PID in the error |
| Stale instance lock | `/instance/forward` probe gets connection-refused / bad secret | delete lock, take ownership, boot normally (§1.4) |
| Launch token leaked (logs, shoulder-surf) | n/a — designed-for | one-shot consumption + 120s TTL + 303 strip + never logged by the kit; a used or expired token grants nothing; session cookie is HttpOnly, SameSite=Strict |
| Other local process probes the port | guard middleware | no cookie → 401 with zero app bytes; Host-header check blocks DNS rebinding; bind is strictly 127.0.0.1, never wildcard |
| WebView2 runtime missing (Windows) | wails3 shell init fails / runtime probe | offer Microsoft's WebView2 runtime install; on decline or failure, **fall back to browser-tab mode in the same process** — relaunch the shell layer headless, open default browser, idle-shutdown on (§1.3) |
| Webview/shell crashes mid-session | window-gone event without app quit | server keeps state, relaunches the window at a fresh LaunchURL; the SSE fat-morph snapshot (D3) repaints exactly where the user was — this is the Invariant paying rent |
| Shell init fails on any platform | error from boot layer | same graceful shell-less mode: log loudly, open browser tab, keep working; "no chrome" must never mean "no app" |
| Tab closed in browser-tab mode | SSE gauge hits 0 | 90s grace, then clean shutdown (§1.3) |
| Backgrounded tab mistaken for closed | would break idle shutdown | main stream sets `openWhenHidden: true` (Datastar v1.0.2 action option) so hidden-but-open tabs hold their stream |
| Laptop suspend/resume | `PowerEvents` (desktop) + Datastar reconnect | Datastar's built-in retry (exponential, configurable) reconnects the stream; idempotent snapshots make the repaint correct by construction; headless builds rely on retry alone |
| Update applies while app running | updater state machine events | Wails updater performs verified atomic swap; the kit's standard pattern is notify → user-triggered restart, never silent mid-session swaps |

The unifying principle of this table: because the app is a real HTTP server
with idempotent snapshot rendering, almost every failure reduces to
"reconnect and re-render" — the recovery story is the architecture, not
bolted-on code.

---

## Spike checklist (ROADMAP Spike 0.1, the D5 empirical gate)

1. Datastar SSE round-trip on **Windows** via the loopback listener inside
   a wails3 window — the fact that forced D5; verify it with our own eyes.
2. Same round-trip on macOS via loopback (and once via `wails://`, to
   document the asset transport's behavior, not to use it).
3. Kill-the-tab and kill-the-window drills against §1.3 and §6 timings.
4. `go list -deps -tags server` purity check wired into CI.
