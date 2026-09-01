# View router

The operational guide for `appkit/ui/router`: when to use it, the full API
reference, and the static SPA recipe for GitHub Pages. This is the companion
to ADR-0016 (`../adr/0016-client-view-router.md`): the ADR says *why* the
router exists and where its boundary is; this document says *how* to use it.

---

## 1. When to use (and when not to)

**Use the view router only for fully static client apps** — surfaces deployed
to a static host (GitHub Pages, Cloudflare Pages static mode, S3 + CloudFront)
where there is no server that can serve `GET /resume` with a 200.

| Surface | Routing strategy |
|---|---|
| Kit app with a Go HTTP server | Real URL navigation — every path is a server render. ADR-0003 §2.4. Nothing to add here. |
| Static SPA (GitHub Pages, CDN) | `AppKitRouter` — this document. |

**Never use the client router inside a kit hypermedia app.** That is
anti-pattern 5.4 in `ui-architecture-patterns.md`. The server already routes;
adding a client router duplicates it, breaks server-side rendering of deep
links, and creates two sources of truth for "what page am I on."

---

## 2. Serving the script

The Go package exposes two ways to deliver `router.js` to the browser.

### 2.1 As a served file

Mount the handler at a stable path and reference it with a `<script>` tag:

```go
import "github.com/geoffmilleraz/app-kit/appkit/ui/router"

mux.Handle("/assets/router.js", router.Handler())
```

```html
<script src="/assets/router.js"></script>
```

The handler sets `Content-Type: application/javascript; charset=utf-8` and
`Cache-Control: public, max-age=31536000, immutable`. Use a cache-busting
path component (e.g. `/assets/router.v1.js`) if you need forced updates.

### 2.2 Inlined into the page

For sites that want a single-file output with no separate asset request:

```go
import "github.com/geoffmilleraz/app-kit/appkit/ui/router"

// Inside a template:
fmt.Fprintf(w, "<script>%s</script>", router.Script())
```

Both approaches serve the same embedded bytes.

---

## 3. API reference

`router.js` registers one global: `window.AppKitRouter`.

### `AppKitRouter.create(config) → router`

Creates a router instance. Call `.start()` to activate it.

**`config` fields:**

| Field | Type | Required | Description |
|---|---|---|---|
| `routes` | `Route[]` | yes | Ordered list of route descriptors. |
| `notFound` | `string` | no | Route ID to activate when no path matches. Defaults to the first route. |
| `onChange` | `function(state)` | no | Callback invoked on every navigation. Additional listeners can be added via `router.onChange()`. |
| `base` | `string` | no | Path prefix stripped before matching (e.g. `"/app"`). Trailing slashes are ignored. |
| `titleFor` | `function(state) → string` | no | Returns a `document.title` string. Overrides the route's built-in `title` field. |

**`Route` shape:**

```js
{ id: "resume", path: "/resume", title: "Résumé" }
```

| Field | Required | Description |
|---|---|---|
| `id` | yes | Unique identifier. Passed to `navigate()`. |
| `path` | yes | Absolute pathname starting with `/`. |
| `title` | no | Sets `document.title` on activation. |

### Router methods

#### `router.start() → router`

Attaches the `popstate` listener, reads the current URL, and emits the initial
`onChange` state. Returns the router for chaining:

```js
const router = AppKitRouter.create({ routes, onChange: render }).start();
```

If the current URL does not match any route, the fallback route is activated
and the URL is silently replaced (no extra history entry).

#### `router.navigate(idOrPath, opts?)`

Pushes a history entry and emits.

```js
router.navigate("resume");                             // by route ID
router.navigate("resume", { params: { role: "fcb" } }); // with query params → /resume?role=fcb
router.navigate("/resume", { replace: true });         // by absolute path, replace mode
```

`opts`:

| Key | Type | Description |
|---|---|---|
| `params` | `Record<string, string>` | Key/value pairs encoded as query string. Empty-string values are omitted. |
| `replace` | `boolean` | Use `replaceState` instead of `pushState` (no new history entry). |

#### `router.onChange(fn) → unlisten`

Adds an additional `onChange` listener. Returns an unlisten function:

```js
const off = router.onChange(state => updateBreadcrumb(state));
// later:
off(); // remove the listener
```

#### `router.current() → RouteState`

Returns the current `RouteState` without emitting. Useful for reading initial
state before `start()` fires.

#### `router.url(id, params?) → string`

Builds a URL for the given route ID without navigating:

```js
const link = router.url("resume", { role: "fcb" }); // → "/resume?role=fcb"
```

#### `router.routes`

The `Route[]` array passed to `create()`. Read-only reference.

### `RouteState` shape

Every `onChange` callback and `router.current()` returns a `RouteState`:

```js
{
  id:      "resume",          // active route ID (or fallback ID)
  path:    "/resume",         // canonical path of the active route
  params:  { role: "fcb" },  // decoded query-string parameters
  hash:    "",                // fragment without leading #
  matched: true,              // false when the fallback activated
}
```

**Query params vs. path segment params:** a trailing path segment under a
known parent route becomes `params.id`. For example, `/resume/fcb` with a
route `{ id: "resume", path: "/resume" }` resolves to:

```js
{ id: "resume", path: "/resume", params: { id: "fcb" }, matched: true }
```

Query-string params take precedence — if `?id=bar` is also present, the
query-string value wins over the path segment.

### Deep-link query params

The router preserves query-string params on hard reload and emits them in
`state.params` on `start()`. Apps can use this for filter/search state that
should survive a page refresh:

```js
router.navigate("home", { params: { q: "platform engineering" } });
// URL: /?q=platform+engineering
// On reload: state.params.q === "platform engineering"
```

### Title sync

`document.title` is set automatically on every navigation if a `title` is
present on the active route or if `config.titleFor` returns a non-empty string.
`titleFor` takes precedence over `route.title`.

---

## 4. Static SPA on GitHub Pages recipe

GitHub Pages serves exactly the files you upload. A hard reload of `/resume`
returns HTTP 404 because there is no `resume/index.html`. The fix is simple:

### Step 1 — Ship `404.html` as a byte-copy of `index.html`

GitHub Pages serves `404.html` for any unknown path. If `404.html` is
identical to `index.html`, the SPA shell loads and the router reads the
current URL and activates the correct view.

```
dist/
  index.html    ← the SPA shell
  404.html      ← byte-copy of index.html (build-time cp)
  assets/
    router.js
    ...
```

In your build step:

```sh
cp dist/index.html dist/404.html
```

### Step 2 — Add `<base href="/">`

With a base tag, all relative asset references (`assets/router.js`,
`assets/styles.css`) resolve correctly regardless of which deep path the user
hard-loaded:

```html
<head>
  <base href="/">
  <link rel="stylesheet" href="assets/styles.css">
  <script src="assets/router.js"></script>
  ...
</head>
```

Without `<base href="/">`, a hard reload of `/resume` would resolve
`assets/router.js` as `/resume/assets/router.js` — a 404.

### Step 3 — Use relative asset paths

All asset references in HTML must be relative (no leading `/`). The `<base>`
tag converts them to root-relative at resolution time, so they always point
at the right origin path.

### Step 4 — Wire the router

```js
const router = AppKitRouter.create({
  routes: [
    { id: "home",   path: "/",       title: "Home" },
    { id: "resume", path: "/resume", title: "Résumé" },
    { id: "blog",   path: "/blog",   title: "Blog" },
  ],
  notFound: "home",
  onChange: render,
}).start();

function render(state) {
  // Swap visible section, update nav active state, etc.
  document.querySelectorAll("[data-view]").forEach(el => {
    el.hidden = el.dataset.view !== state.id;
  });
}
```

### Trade-offs acknowledged

- **Deep paths return HTTP 404 status** while still rendering the correct
  view. Web crawlers (Googlebot) follow JavaScript and index the content.
  HTTP-status-sensitive monitoring will flag these as errors — acceptable for
  a personal portfolio with no strict SEO SLA.
- **There is no server-side render** of path-specific content. All views must
  be in the client bundle or fetched via `fetch()` from static JSON endpoints.
- **If you need clean HTTP 200 on deep paths**, move to a host with a
  server-side rewrite rule (Cloudflare Pages `_redirects`, Netlify `_redirects`,
  Vercel `vercel.json` rewrites). The client router works identically — just
  remove the `404.html` trick.

---

## 5. Hypermedia apps: nothing to add

For kit apps with a real HTTP server, routing is real URL navigation — every
path is a `GET` that the server handles with a full-page render or a stream.
The browser's back/forward button works by construction. `AppKitRouter` is not
loaded, not referenced, not mounted.

The escape ladder in `ui-architecture-patterns.md` §3 (L2 islands) does not
change this. An island is a leaf that does not route; it dispatches `CustomEvent`
and the page turns events into commands.

---

*Cross-references:* ADR-0016 (the decision), ADR-0002 (no Node; JS as vendored
go:embed artifacts), ADR-0003 (hypermedia-first; anti-pattern 5.4 on client
routing), `ui-architecture-patterns.md` §2.4 (list+detail; real URL navigation).
