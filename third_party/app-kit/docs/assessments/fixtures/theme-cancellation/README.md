# Controlled-delay theme-guide cancellation fixture

Synthetic assessment infrastructure, not a runtime feature or consumer integration.
It mounts the actual Gallery `NewThemeLab`/`NewWithOptions` with licensed references
disabled, immutable explicit Synthwave source and the executable's actual SHA-256.
It listens only on an ephemeral127.0.0.1 port and writes one private ready JSON line.

The wrapper delays exactly the next armed native preview transport envelope before
the real handler runs. It reads at most64KiB+1, restores examined body bytes and leaves
`Form`/`PostForm` unpopulated. Candidate semantics still belong to the real handler.
Wrong origin/host/peer/fetch metadata, oversized bodies and other actions bypass the
delay and retain the real handler's refusal behavior. Cancellation ends the delayed
request without delegation; a15-second timeout is recorded and delegates unchanged.
An unchanged retry is not delayed. No specimen callback or production handler changes.

Node-side controls are authenticated by the per-process ready token in an
`Authorization: Bearer ...` header: `POST /__probe/arm`, `GET /__probe/state`.
They require exact Host and a loopback peer. Never put the token in page HTML, URLs,
browser JS or a real application. There is no release endpoint or persistent state.
Graceful termination cancels requests and bounds server shutdown.

Freeze the fixture, complete selected Go/test/embed closure, browser probe, tools,
source/CSS and build receipt before execution. Build to an explicit private path;
run that binary with `-source ABSOLUTE_CUE_PATH`, not `go run`, for identity proof.
The browser owner must launch/clean up its own fresh Chrome profile and this server.
Bind cleanup to distinct exit and output-drain deadlines; retain failed/unresolved
children rather than equating a timeout or closed socket with process death. The
maintained `apps/styleguide/browser/owned_process.mjs` helper and synthetic canaries
cover this owner boundary, separately from task and browser-guard acceptance.

`apps/styleguide/browser/cancellation.mjs` consumes the standard explicit Gallery
origin/WS/source/server/Playwright selections plus:

- `APPKIT_THEME_CANCEL_CONTROL_URL`: exactly the ready loopback origin.
- `APPKIT_THEME_CANCEL_CONTROL_TOKEN`: ready token, Node-side only.
- `APPKIT_THEME_CANCEL_OUTPUT_DIR`: fresh existing private directory.
- `APPKIT_THEME_CANCEL_SNAPSHOT`: frozen `appkit-theme-guide-transport-inputs-v1`
  manifest and successful build receipt; see the producer's exact selected fields.

`APPKIT_THEME_CANCEL_METHOD` selects `escape` (default) or a separately recorded
`cdp-stop-loading` diagnostic using `Page.stopLoading`. No silent fallback occurs.
The initial probe selects one desktop cell to establish method viability; the fixed
lane still requires twelve cells. CDP Stop is not physical keyboard proof.
A request canceled by later context
cleanup is not a successful Stop. The first Stop failure ends acquisition and marks
remaining cells unexecuted. A passing producer summary would still require independent
receipt/ownership checks; `complete` stays false. Do not silently substitute route
aborts, `window.stop`, programmatic DOM mutation or a different browser-stop command.
This does not prove physical keyboard input, arbitrary Go callback preemption, timing,
memory budgets or consumer adoption. [Results](../../2026-09-17-theme-guide-cancellation.md).
