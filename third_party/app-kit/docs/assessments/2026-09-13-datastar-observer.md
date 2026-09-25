# Pinned Datastar Activity observer — 2026-09-13

G6 now has an experimental client integration, not just a compatible-looking SSE event.
Scope: signed-in Gallery `/shell/activity`, synthetic local sessions, explicit Connect.
No consumer migration, production identity/persistence, native support or release is implied.
The [shell guide](../design/application-shell.md) owns behavior; [ADR-0021](../adr/0021-runtime-profiles-and-platform-proof.md)
owns runtime selection and actual-target promotion. Existing forms remain ordinary HTML.

## Exact dependency and source findings

Upstream Datastar **v1.0.3**, released 2026-08-27, resolves to commit
`73ab00e7c06d8c2bad030fdddafba800fcccbde2`. Unchanged `bundles/datastar.js`:
33,538 bytes, SHA-256 `5d6b7794a50a83d82da962aec5e382f5ae83ac7afbc751f903f7a9c6bd433c65`.
The commit-addressed download matched the tag-addressed download exactly.
[Vendored provenance](../../apps/gallery/assets/datastar/v1.0.3/README.md) and the full
upstream license are retained. The built binary embeds both, serves the notice as text,
and links it from the script response. All assets are same-origin with SHA-256 ETags,
GET/HEAD, nosniff and revalidation; no runtime CDN or package installation is required.

Source review found contract traps worth keeping as regression tests:

- Default JSON GET adds signal query data even for an empty selection. This server's
  strict query-free stream instead uses an empty, observer-only form with no named controls.
- `retry: 'never'` alone does not stop network/read-error retries. `retryMaxCount: 0`
  prevents these; manual reconnect supplies a fresh AbortController and full snapshot.
- Default visibility behavior can automatically reopen GET. `openWhenHidden: true`
  delegates lifetime to our controller, which aborts on hidden/pagehide and never auto-resumes.
- Fetch events identify the initiating element in `detail.el`; HTTP errors use
  `detail.argsRaw.status`. The signal watcher applies before our listener. Root null
  deletes the namespace, so a changed-leaves event alone is not a sufficient redaction hook.
- The exact pin supports `html[data-nonce]` expression compilation using a nonce-bearing
  temporary script. Only Activity adds this nonce and same-origin `connect-src`; neither
  unsafe-eval nor unsafe-inline is enabled. This source path is **not browser CSP proof**.

Primary references: [release](https://github.com/starfederation/datastar/releases/tag/v1.0.3),
[pinned fetch action](https://github.com/starfederation/datastar/blob/73ab00e7c06d8c2bad030fdddafba800fcccbde2/library/src/plugins/actions/fetch.ts),
[pinned CSP engine](https://github.com/starfederation/datastar/blob/73ab00e7c06d8c2bad030fdddafba800fcccbde2/library/src/engine/csp.ts),
[current security guidance](https://data-star.dev/reference/security).

## Behavior and deliberate limits

SSR activity stays useful without scripts; Connect remains disabled if enhancement
fails to load, with ordinary Reload activity available. Connecting is not an assertion
of freshness. The label distinguishes connecting, last observed, paused, closed,
interrupted and revoked states; comment heartbeats do not become an invented freshness clock.
One connection must settle before the next begins. Hidden/pagehide, Stop and disposal
cancel; return/back-cache events only restore controls. No automatic retry or repost.

Only the read-only activity region is updated with text nodes. A closed allowlist bounds
fields, ten activities and canonical decimal uint64 versions without Number coercion.
Ordering is monotonic within one connection, reset on reconnect because a session may
have expired and been recreated. The pinned signal store must agree with the full frame.
Null, 401/403 or invalid data clears observed content and its signal namespace; late
buffered signal patches cannot repopulate a revoked namespace. Non-auth transport
failures retain explicitly stale content. Clearing is not an ability to recall bytes
already delivered, a whole-page SSR identity refresh, or a new authorization boundary.

The adapter never selects profile/settings forms, changes their fields, baselines,
hidden versions or retained drafts. Server authorization, CSRF, optimistic conflicts,
fixed expiry, stream capacity and bounded writes remain the existing HTTP contract.
The observer is not a durable event log or an offline synchronization service.

## Reproducible evidence

The repository tests distinguish three levels:

1. **HTTP/HTML and assets:** `go test ./apps/gallery` checks Activity-only loading,
   nonce/CSP agreement, empty form and fixed action options, escaping, no-script fallback,
   script routes/license/pin/hash/revalidation, existing save/recovery/transport contracts.
   Observer JS has declared ceilings of **48 KiB raw / 20 KiB independently gzipped**,
   including the vendor bundle. Ordinary/rejected HTML and draft-script ceilings remain.
2. **Adapter state machine:** `node --test apps/gallery/shell_live_test.mjs` uses small
   DOM doubles: one request, fresh cancellation, paused/failed/revoked states, cleared
   signal namespace, late events, exact versions, ordering, malformed data and disposal.
3. **Actual pinned transport/signal code:** `node --test apps/gallery/shell_live_datastar_test.mjs`
   imports unchanged hash-verified bundle bytes with narrow DOM/FormData stubs and Node
   streams. It exercises the internal action seam, real SSE parser and signal store,
   exact request shape, null deletion, no retry, abort and authorization error shape.
   It does **not** compile the page's declarative action or enforce browser CSP.

`task check` includes both new Node suites. Tailwind CSS was regenerated with the pinned
binary. Package/race tests, Node results and payload measurements are reported separately
from graphical acceptance; none of these tests is a browser substitute.

Observed locally with Go 1.26.3 darwin/arm64, Node 26.3.0 and Tailwind 4.3.0:
13 adapter tests and four nested pinned-transport cases pass (Node counts the parent as
a fifth test); the nine existing draft-controller tests also pass. Script sizes are
observer 7,336 + entry 545 + vendor 33,538 = **41,419 raw bytes**; independently gzip
compressed responses sum to **16,316 bytes**. Gzip is a reproducible size measurement,
not a claim that the current static handler negotiates compression or a latency result.

## Browser follow-through and remaining native work

Chrome returned **Browser is not available: chrome**. The owner subsequently authorized
the in-app browser for local tests only; selection returned **Browser is not available: iab**,
and supported discovery returned no connected browsers. Those historical attempts failed.
The owner then authorized an isolated Playwright harness using installed Chrome and fresh
temporary profiles. The [repo-owned observer probe](../../apps/gallery/browser/shell_observer.mjs)
now passes **26 checks**: actual nonce/declarative execution, empty-form request shape,
initial/second-tab snapshots, unchanged draft, bounded stop/manual reconnect and sign-out
terminal redaction. The browser also exposed INC-16's native POST privacy-header conflict;
its fix preserves strict Origin refusal. [Exact receipt](2026-09-13-shell-browser.md).

Remaining browser work: hide/show/back without automatic reconnect; fixed expiry, network interruption,
capacity refusal and malformed-frame failures. Repeat observer keyboard, 390/1440 px, 200% zoom and
light/dark with a named device/browser and measured payload/latency. Preserve the ordinary
no-script workflow. Then repeat applicable ADR-0021 cases in an actual macOS host.
The owner deferred Windows under [ADR-0027](../adr/0027-web-and-macos-now-windows-on-demand.md);
preserve its future option without current Windows testing or support claims.
Do not promote platform support, accessibility or performance from build/Node success.
