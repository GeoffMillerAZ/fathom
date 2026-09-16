## VB-31: does `service_ref` clear M8 (P ≥ 0.90, R ≥ 0.90)?

### Verdict, plainly

No. `service_ref` is precise (1.00 on both real corpora — it never manufactures
a cross-repo edge that isn't there) but it is not close to complete: overall
recall is 0.20, because on real-world code its client-side detector only
catches the narrowest shape of Go HTTP call (a string literal or an INLINE
`fmt.Sprintf(...)` right there in the argument list), and its reused
server-side detector (`internal/analysis/routes`) is a line-oriented regex
that gets confused by two very ordinary things: a route path built by string
concatenation, and a method registered via gorilla/mux's chained
`.Methods(...)` call. On the crafted fixture fleet, where nobody wrote either
of those shapes, it is perfect (1.00/1.00). On one real pair of open-source
Go services that talk to each other over HTTP, it found none of the four
genuine edges a human reading the code can verify in five minutes. The row
stays a finding, not a shipped guarantee, and the two causes above are the
concrete next step if anyone picks it back up.

### Inputs

- **Contract read (order-preserving step 1):** `internal/analysis/serviceref/serviceref.go` only, before running anything or reading `serviceref_test.go`'s expected outputs. Confirmed: client side is Go-only, AST-walked (not regex), and only recognizes `http.NewRequest`/`NewRequestWithContext`/`Get`/`Head`/`Post`/`PostForm` calls whose URL argument is a string literal or an **inline** `fmt.Sprintf(...)` call — not a variable holding either. Server side reuses `internal/analysis/routes.Extract` unchanged (line-oriented regex over `net/http`/chi/gin/echo-shaped Go, plus JS/Python patterns).
- **Hand table written before any extractor run this session:** `testdata/serviceref/route-table.json` (`written_before_extractor_run: true`, `written_at_utc: 2026-09-14T19:31:49Z`). First extractor run this session: `2026-09-14T19:33:04Z` (`go test -run TestGoldenCorpus`), ~75 seconds after the table was saved.
- **Fixture fleet (corpus `fauxtown`):** `internal/connector/testdata/fauxtown/` — all six services inspected by hand; only `catalog-service` (server) and `checkout-service` (client) contain any HTTP route or client call. `ledger-service`, `payment-service`, `polyglot-service`, `worktree-service` contribute nothing. `testdata/indexbench/` was checked and has no HTTP routes (JSON baseline files only). `internal/analysis/serviceref/testdata/` does not exist.
- **Extra precision control (corpus `reachfix-fp-control`, not scored):** a broad `grep` across every `testdata/` directory in the repo for route-registration/client-call shapes turned up one more hit outside `fauxtown`: `internal/fusion/testdata/reachfix` (a VS-10 TestImpact fixture, unrelated in provenance to this plan). It has genuine `net/http.ServeMux` route registrations and zero client calls anywhere in its module, so it can never contribute a true cross-repo edge — it is folded into the same join as a false-positive control only, asserted to contribute exactly zero hops.
- **One real pair (corpus `real_pair`):** `openfaas/faas` (gateway subtree, client side) calling `openfaas/faas-provider` (server side), shallow-cloned into the scratchpad (never committed to this repo, never touching `/Volumes/m2ext`). `faas-provider` @ `f53fe63` (MIT), `faas` @ `8d803bf` (dual: MIT for third-party contributions, OpenFaaS CE EULA for OpenFaaS Ltd's own contributions — noted for the record; used here only to measure extraction accuracy on public source, not redistributed). Non-vendor, non-`_test.go` Go: gateway subtree ~3-4k LOC, faas-provider ~3k LOC — both well under the 50k ceiling. This was the task's own first-named candidate pair and was used as offered; two other candidates (`microservices-demo/{orders,shipping}` — Java, not Go; `nicholasjackson/building-microservices-youtube` — no Go-to-Go `net/http` calls, only gRPC) were evaluated and rejected before it.

### Table size

22 rows total: 3 `fauxtown` (1 match, 1 client-only, 1 server-only), 14 `real_pair` (4 match, 1 client-only, 8 server-only, 1 excluded), 5 `reachfix-fp-control` (5 server-only, unscored). Full table with file:line citations and per-row reasoning: `testdata/serviceref/route-table.json`.

One `real_pair` row is marked `excluded` rather than scored: the gateway's request-forwarding proxy (`gateway/handlers/forwarding_proxy.go:72`, used for both `/system/logs` and `/function/{name}/...`) calls `http.NewRequest(r.Method, url, nil)` — the method is the *incoming* request's method, forwarded verbatim, not a fixed value. No single `(method, template)` pair exists to hand-verify, so it is not counted as a ground-truth edge in either direction, applied consistently to both call sites that share the helper.

### Precision / recall

| Corpus | TP | FP | FN | Precision | Recall |
|---|---:|---:|---:|---:|---:|
| `fauxtown` | 1 | 0 | 0 | 1.00 | 1.00 |
| `real_pair` | 0 | 0 | 4 | n/a (0 predictions made) | 0.00 |
| **Overall** | **1** | **0** | **4** | **1.00** | **0.20** |

(`reachfix-fp-control`: 0 hops produced, as asserted — contributes 0/0/0 to every cell above; kept out of the table because it cannot add a true edge by construction, folding it in would not move any number.)

Overall precision counts the empty `real_pair` prediction set as contributing 0 to both the numerator and the denominator (it made no false claims — it just made almost no claims). Recall is the number that matters here: **1 of 5** genuine cross-repo edges across both corpora were found.

### False positives

None, in any corpus. `fauxtown`: `TestServiceRefEdges_FixtureJoin` (pre-existing) and the new `TestGoldenCorpus_Fauxtown` both confirm exactly the one true hop, zero extra. `real_pair`: the extractor produced **zero hops** — see below, not because it correctly abstained on four hard cases, but because its client-side detector found only one call in the whole gateway repo (see below), and that one call doesn't key-match anything faas-provider registers. `reachfix-fp-control`: hard-asserted zero.

### False negatives

**`real_pair` — all 4 hand-verified matches, all missed:**

1. `faas-gateway` `GET /system/functions` → `faas-provider` `handlers.FunctionLister` (`gateway/metrics/exporter.go:152`, symbol `getFunctions`). **Cause:** the URL is built as `endpointURL.Path = path.Join(endpointURL.Path, "/system/functions")` then `http.NewRequest(http.MethodGet, endpointURL.String(), nil)` — the URL argument is `endpointURL.String()`, a method call on a variable, not a string literal or an inline `fmt.Sprintf`. `ExtractClientCalls`'s `templateFromExpr` only recognizes those two shapes; this call is invisible to it, full stop — it never appears in the extracted `ClientCall` list at all.
2. `faas-gateway` `GET /system/namespaces` → `faas-provider` `handlers.ListNamespaces` (`gateway/metrics/exporter.go:190`, symbol `getNamespaces`). **Cause:** identical to #1 (`endpointURL.String()`).
3. `faas-gateway` `GET /system/function/{}` → `faas-provider` `handlers.FunctionStatus` (`gateway/plugin/external.go:76`, symbol `GetReplicas`). **Cause (client side):** `urlPath := fmt.Sprintf("%ssystem/function/%s?...", s.URL.String(), ...)` is assigned to a local variable on one line, then `urlPath` is passed to `http.NewRequest` on a later line — the `fmt.Sprintf` call is not *inline* at the argument position, so `templateFromExpr`'s `fmt.Sprintf` branch never fires. **Compounding cause (server side, would still block a fixed client detector):** faas-provider registers this route as `"/system/function/{name:["+NameExpression+"]+}"` — Go string concatenation, not one string literal. `routes.Extract`'s line-oriented regex captures only the first quoted segment, truncating the path to the literal, malformed string `/system/function/{name:[` (confirmed by running the extractor: see the raw output below). Separately, the real method (`GET`) is registered via gorilla/mux's chained `.Methods(http.MethodGet)`, which sits *outside* the parentheses the `HandleFunc(...)` regex captures — every faas-provider route in this repo extracts with `Method: "ANY"`, which can never key-match a client call's concrete method.
4. `faas-gateway` `POST /system/scale-function/{}` → `faas-provider` `handlers.ScaleFunction` (`gateway/plugin/external.go:156`, symbol `SetReplicas`). **Cause:** same dual cause as #3 (Sprintf-into-a-variable on the client side; string-concatenation + lost-`.Methods()`-chain on the server side).

No `fauxtown` false negatives.

### What the real-pair extractor actually found (raw)

Run via a throwaway program (`workspace/tmp/vb31realpair/main.go`, gitignored, not product code) over the same `routes.Extract` / `ExtractClientCalls` / `ServiceRefEdges` functions:

- `faas-gateway`: 23 routes (the gateway's own inbound `/system/...` API, not scored — see route-table.json's scoping note), **1 client call**: `GET /api/v1/query` (`metrics/prometheus_query.go:35`, symbol `Fetch`) — this is the one *inline*-`Sprintf` call in the whole repo, and it targets Prometheus, a third service outside this pair's fleet, so it correctly produces no hop.
- `faas-provider`: 19 routes, **every single one extracted with `Method: "ANY"`** (the `.Methods(...)` chain is invisible to the regex), and the four routes registered via `"literal[" + NameExpression + "]literal"` string concatenation extract with the path truncated at the first embedded quote (e.g. `/system/function/{name:[`, `/system/scale-function/{name:[`, `/function/{name:[`, `/system/namespace/{name:[`) — permanently unmatchable regardless of the client side.
- **Result: 0 `service_ref` hops.**

This is not a fixture artifact — it is `internal/analysis/routes` and `ExtractClientCalls` behaving exactly as documented, on ordinary, unremarkable Go code that simply wasn't written to be extraction-friendly. The fixture fleet's clean 1.00/1.00 is real but narrow: it proves the join algorithm (`ServiceRefEdges`) and the normalization (`NormalizeTemplate`) are correct, not that the two extraction halves generalize.

### Verdict against the row

Overall precision 1.00 ≥ 0.90 (met). Overall recall 0.20 < 0.90 (not met — the gate is `P ≥ 0.90 AND R ≥ 0.90`). **The row stays unchecked.** Per the row's own text ("else it stays a finding"), `docs/plans/verbs.md` VB-31 is edited to `[ ]` with a one-line measured note; it does not flip to `[x]`.

The concrete, scoped next step if this is picked back up: (a) client side — resolve a single-assignment local variable back to its `fmt.Sprintf`/literal definition before giving up (a small, bounded dataflow step, not a general SSA pass); (b) server side — teach `routes.Extract` to see a chained `.Methods(...)` call after `HandleFunc(...)`/`Handle(...)`, and to concatenate adjacent quoted segments joined by `+` before treating the result as one path. Both are `internal/analysis/routes` changes, outside VB-30/31's stated scope, and neither was attempted here — this reading only measures and reports.

## Rerun after wave I2 (2026-09-16)

Wave I2 picked up exactly the two next-steps named above and implemented
both as real `go/ast` walks (no line regexes anywhere in the Go path of
either package). The hand-verified table (`testdata/serviceref/
route-table.json`) was **not edited** — same file, same 22 rows, same
`written_before_extractor_run: true` provenance from 2026-09-14. Only the
extractor changed; the table was re-read against it.

### What changed

- **Client side** (`internal/analysis/serviceref/serviceref.go`,
  `templateFromExpr` + new `clientResolver`): a URL argument that is a local
  identifier now resolves back to its own most recent assignment within the
  enclosing function (`findLastAssign`) — bounded, single-hop, no general
  SSA pass. A `*url.URL.String()` call resolves by finding the last
  assignment to that receiver's `.Path` field earlier in the same function
  (`findLastFieldAssign`) — including through `path.Join`/`url.JoinPath`,
  which fold their arguments and specifically DROP a self-referential
  `X.Path` argument (`path.Join(endpointURL.Path, "/system/functions")`)
  rather than wildcarding it, since that argument is the pre-existing
  accumulator, not a substituted value. A `url.URL{Path: "…"}` composite
  literal is also recognized as an initializer. Separately, a `fmt.Sprintf`
  format string that begins with a printf verb whose paired argument is
  itself a `.String()` call (`fmt.Sprintf("%ssystem/function/%s", s.URL.
  String(), name)`) is now recognized as an interpolated host/base and
  stripped to a path-root marker rather than becoming an ordinary `"{}"`
  path-segment placeholder — without this, `"%ssystem/function/%s"` would
  normalize to `"/{}system/function/{}"` (wrong: no separator before
  "system") instead of the correct `"/system/function/{}"`. A bare
  concatenation (`a + b`) at the top level now requires at least one side to
  resolve to genuine literal content — two unresolved placeholders glued
  together (an `httptest`-style `ts.URL + path`, found by this exact change
  surfacing it in `internal/fusion/testdata/reachfix/server_test.go`) is not
  distinguishable from arbitrary string arithmetic and produces no
  `ClientCall`, preserving precision.
- **Server side** (`internal/analysis/routes/routes.go`, new `extractGo` +
  `goRouteExtractor`): the Go path no longer uses `regexp` at all — JS/TS and
  Python are unchanged. A call chain is decomposed into its full sequence of
  `.Name(args)` links regardless of order, so `r.HandleFunc(path,
  handler).Methods(http.MethodGet)` and the reversed `r.Methods("POST").
  Path(path).HandlerFunc(handler)` both resolve; a `.Methods(a, b)` call
  fans out to one `Route` per method instead of losing every method to the
  regex-invisible chain (the 2026-09-14 reading's "every extracted route
  reads `Method: "ANY"`" failure mode). `PathPrefix(prefix).Subrouter()`
  assigned to a variable propagates its prefix to every route later
  registered through that variable (tracked across statements in one
  top-to-bottom pass). A concatenated path
  (`"/system/function/{name:["+NameExpression+"]+}"`) folds through
  package-level string consts the way `go/constant` would evaluate the
  expression, instead of the old regex truncating at the first embedded
  quote; a genuinely non-constant fragment normalizes to `"{}"` rather than
  either guessing or abandoning the route.

### Rerun results

| Corpus | TP | FP | FN | Precision | Recall |
|---|---:|---:|---:|---:|---:|
| `fauxtown` | 1 | 0 | 0 | 1.00 | 1.00 |
| `real_pair` | 4 | 0 | 0 | 1.00 | 1.00 |
| **Overall** | **5** | **0** | **0** | **1.00** | **1.00** |

`fauxtown` was re-measured via the same in-repo golden test
(`TestGoldenCorpus_Fauxtown`, unchanged): `tp=1 fp=0 fn=0`. `real_pair` was
re-measured via the same throwaway program
(`workspace/tmp/vb31realpair/main.go`, unchanged) over the same two shallow
clones at the same commits (`faas-provider@f53fe63`, `faas@8d803bf`): the
extractor now produces exactly the 4 hops the table's `match` rows name —

```
faas-gateway GET /system/function/{} -> faas-provider     (plugin/external.go:76  -> serve.go:71)
faas-gateway GET /system/functions -> faas-provider        (metrics/exporter.go:152 -> serve.go:66)
faas-gateway GET /system/namespaces -> faas-provider        (metrics/exporter.go:190 -> serve.go:85)
faas-gateway POST /system/scale-function/{} -> faas-provider (plugin/external.go:156 -> serve.go:73)
```

— and no others: the Prometheus call (`client_only`, targets a third service
outside the pair) still correctly produces no hop, and the `excluded` row
(`buildUpstreamRequest`, method forwarded verbatim from the incoming
request) still correctly cannot resolve a method and is skipped.
`reachfix-fp-control` still contributes zero hops (`TestGoldenCorpus_
Fauxtown`'s hard assertion continues to pass) — its one previously-documented
gap (`mux.Handle(r.path, ...)`, a struct-field path with no literal content)
now extracts as a wildcarded `Method: "ANY", Path: "{}"` route rather than
nothing at all, which cannot manufacture a match since no client call in
either corpus ever carries method `"ANY"`.

### Remaining known misses

None against this table. The two out-of-scope-for-VB-30/31 gaps named in the
original reading (self-referential local dataflow, chained `.Methods()`, and
concatenated paths) are exactly what this wave closed. What is still NOT
handled, named honestly rather than claimed clean: a route path or client URL
built across more than one hop of indirection (e.g. a variable assigned from
a function call whose own return value is itself indirect), branch-sensitive
resolution (an `if`/`else` assigning two different values to the same `.Path`
— the LAST textual assignment wins, which is a heuristic, not control-flow
analysis), and any construct outside the specific shapes enumerated above.
All of these still resolve to "no edge" (client side) or "no route" (server
side, for an unrecognized path expression shape) rather than a guess, so they
cost recall on some hypothetical future corpus, never precision on this one.

### Verdict against the row (rerun)

Overall precision 1.00 ≥ 0.90 (met). Overall recall 1.00 ≥ 0.90 (met). **The
row is checked.** `docs/plans/verbs.md` VB-31 is edited to `[x]` with a
one-line note pointing here.
