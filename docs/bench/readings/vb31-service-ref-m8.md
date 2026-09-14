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
