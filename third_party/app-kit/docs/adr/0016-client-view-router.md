# ADR-0016: Ship a vanilla client view-router for static-client surfaces

- **Status:** Superseded by [ADR-0017](0017-multi-target-layered-kit.md)
  (2026-08-07). The body below is unchanged and its hard boundary remains in
  force — see the closing section.
- **Date:** 2026-06-16
- **Decision drivers:** static hosts (GitHub Pages) have no server router; kit
  apps that ARE hypermedia apps already get routing for free from real URLs;
  D2 prohibits Node and build toolchains; the router must be auditable,
  vendorable, and zero-dependency

## Context

The kit's canonical UI pattern (ADR-0003, D3) routes via real HTTP URLs — the
server renders every path, back/forward work by construction, and there is no
client router because the browser already has one. This is correct and stays
correct for all kit apps that have a real Go HTTP server.

A second category of surface exists: **fully static clients** — sites deployed
to GitHub Pages, Cloudflare Pages in static mode, or any CDN with no server
router. These hosts serve exactly the files you upload and return HTTP 404 for
every unknown path. SPAs built on this substrate cannot use real server URLs for
client-side "pages" without special handling, because a hard reload of
`/resume` fetches a path the host does not know, and the CDN returns 404.

The Geoff Miller portfolio site (`dev.geoffmiller.cloud`, deployed to GitHub
Pages) is the first such surface in the kit ecosystem. It is a single-page
client app with multiple logical views (home, resume, blog index, post detail)
that need:

- In-app navigation without full-page reloads (visual continuity).
- Deep-linkable URLs that survive hard reloads (`/resume` re-renders the
  resume view, not a blank page).
- Browser back/forward that work without a server.
- `document.title` synchronization per view.
- Query-param pass-through for filter/search state.

No Go HTTP server exists in this deployment context; the Loopback Invariant
(ADR-0005) does not apply here — the invariant governs kit *apps*, and a static
CDN-deployed site is not a kit app in that sense. There is no NATS spine, no
templ, no SSE stream.

**The no-Node constraint (ADR-0002) applies here too.** A client router that
requires a build step (webpack, Vite, Rollup, esbuild) to produce the vendored
artifact would drag Node back in. The kit's pattern for JS is to vendor a
single, version-pinned, `go:embed`-ed file (see Datastar: one 34KB file,
ADR-0003). The router must be a single, self-contained `.js` file with no
dependencies and no build step.

A vanilla History-API router meeting all of the above was authored, proven in
production at `dev.geoffmiller.cloud`, and is now contributed to the kit as
`appkit/ui/router/router.js`.

## Decision

We ship `appkit/ui/router` — a package containing a single, dependency-free,
build-free JavaScript file (`router.js`) that implements a History-API client
router, embedded via `go:embed`. The Go package exposes two functions:

- `Script() string` — returns the JS source for inlining into a `<script>` tag.
- `Handler() http.Handler` — serves `router.js` with `Content-Type:
  application/javascript; charset=utf-8` and a long-lived cache header.

The JS file registers `window.AppKitRouter` with a single factory function
`AppKitRouter.create(config)` and returns a router object with `start`,
`navigate`, `onChange`, `current`, `url`, and `routes`. It supports:

- Exact-path route matching with a single trailing-segment param (e.g.
  `/resume/fcb` → route `resume`, `params.id = "fcb"`).
- Query-string parameter parsing into `state.params`.
- `document.title` sync via per-route `title` or a caller-supplied `titleFor`.
- A `base` prefix for sub-path deployments.
- `history.pushState` / `history.replaceState` with `popstate` for
  back/forward.

**The hard boundary:** this router exists ONLY for static-client surfaces with
no server router. It MUST NOT be used in kit apps that have a real HTTP server.
Those apps use real URL-based navigation (ADR-0003 §2.4, anti-pattern 5.4 in
`docs/design/ui-architecture-patterns.md`). A client router bolted on top of a
hypermedia app is always wrong.

## Alternatives considered

- **Hash-based routing (`#/resume`).** Attractive: works on any static host
  with zero configuration; hard reloads always hit the index. Lost because hash
  URLs are ugly, copy-paste-hostile, and excluded from some SEO contexts. The
  GitHub Pages `404.html` trick (pair a byte-copy of `index.html` as `404.html`
  so the CDN serves the SPA shell on every deep path, then rewrite in JS) is a
  well-established pattern that gives clean History-API URLs with acceptable
  trade-offs (deep paths return HTTP 404 status, which is noted in the design
  doc; for a personal portfolio behind no crawler SLA this is acceptable).
- **A minimal JS framework (Alpine.js, Preact, Svelte compiled output).**
  Attractive: more features, component ecosystem. Lost because (a) they require
  a Node build step to produce the vendored artifact (D2 violation) or are too
  large to vendor as a single readable file, and (b) routing is the only feature
  needed — a framework is the wrong shape for one module of behavior.
- **Import a third-party router from npm (Navigo, page.js, etc.).**
  Attractive: battle-tested, documented. Lost for two reasons: (a) taking a
  runtime `npm install` dependency violates the "JS enters only as vendored,
  version-pinned, go:embed-ed artifacts" rule (D2/D3), and (b) a vendored
  third-party router is an opaque blob that the kit cannot own. The kit's
  position is "own the code" — an 180-line vanilla file is readable, auditable,
  and forkable. It was proven in production before being contributed.
- **Do nothing; let each static site implement its own routing.** Attractive:
  zero kit scope creep. Lost because the kit is about propagation (VISION §3) —
  a solved, proven module belongs in the kit so it does not get re-invented
  per-project, and `go:embed` makes the Go-side serving trivially reusable.

## Consequences

- **Easier:** static SPA surfaces get a proven, tested, kit-standard routing
  primitive with a Go HTTP serving interface. No per-project re-invention. The
  `404.html` recipe is documented and canonical.
- **Easier:** the router's Go wrapper (`Script()` / `Handler()`) fits the kit's
  existing pattern for vendored JS assets — one embed, one handler — so any
  future kit HTTP shell serving a static frontend can use it with one import.
- **Harder:** the boundary between "use the client router" and "use real server
  URLs" must be actively enforced. ADR-0003 anti-pattern 5.4 already names
  "client-side routing" as forbidden inside hypermedia apps; this ADR adds the
  explicit carve-out and the explicit limit.
- **Risk: deep paths return HTTP 404.** The GitHub Pages `404.html` trick
  delivers the app shell but the CDN returns `404` HTTP status. Web crawlers
  index it correctly because Google follows JavaScript, but HTTP-status-sensitive
  monitoring or strict SEO audits will flag it. *Tripwire:* a static surface
  needs zero-404 deep links (e.g. requires Google News inclusion) → move to a
  host with a server-side rewrite rule (Cloudflare Pages, Netlify, Vercel),
  which retains the same client router with a `_redirects` file.
- **Risk: the router is a hand-authored file.** No upstream to pull patches
  from. *Tripwire:* a browser behavior changes that breaks the History API
  contract → the file is 180 lines and fully owned; patch it directly.

## Revisit when

- A static surface needs features the router does not provide (parameterized
  segments beyond trailing `/id`, nested routes, lazy route loading) → extend
  the file or swap it for a vendored third-party bundle under the same
  `go:embed` pattern, with a new ADR entry for the dependency.
- The "kit app" and "static client" categories converge (e.g. a kit app is
  deployed as a static export) — re-examine whether a client router and the
  Loopback Invariant can coexist, and write a narrowing ADR if they can.
- ADR-0002 (no Node) is revisited and relaxed — at that point a build-step
  framework becomes an option and the singleton-JS-file approach can be
  superseded.

## Superseded by ADR-0017 — the carve-out became a target class

Added 2026-08-07, when the successor landed. Read this before acting on
anything above.

This ADR authorised one artifact for one surface and called it an exception.
It was not an exception; it was the first member of a category the kit had
not named. `geoffmiller-cloud/spa/router.js` and `appkit/ui/router/router.js`
are byte-identical (md5 `ad73ef48ea3726d06946595ce0acf296`) — a React 18 CDN
site has been running kit code by copy for months with nothing detecting
drift, because this ADR gave the kit a consumer without giving it a
relationship with that consumer.
[ADR-0017](0017-multi-target-layered-kit.md) generalises the carve-out into
**L2 — portable browser JS**, a named layer any target may take, and gives it
the propagation mechanism this ADR lacked: `appkit vendor` with a `kit.lock`
and a `--check` that exits 1 on drift.

**What survives, verbatim and still binding:**

> this router exists ONLY for static-client surfaces with no server router.
> It MUST NOT be used in kit apps that have a real HTTP server.

So does this ADR's ruling that the Loopback Invariant (D5) governs kit *apps*
and that a static CDN-deployed site is not one. ADR-0017 restates both rather
than reopening them.

**What changed:** the router is no longer sui generis. `router.js`,
`overlay.js`, `chat.js` and `motion.js` are all L2 artifacts under one
contract — zero dependencies, no build step, one `window.*` global,
`go:embed` plus a `Script()`/`Handler()` pair — and the vendoring recipe is a
kit command rather than a copy-paste. This ADR's own revisit trigger ("write
a narrowing ADR if they can") is what fired.
