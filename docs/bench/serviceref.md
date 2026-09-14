# `service_ref` precision/recall (VB-31)

Row VB-31 (`docs/plans/verbs.md`, `needs: VB-30`): M8 is "precision ≥ 0.90 and
recall ≥ 0.90 on a hand-verified route table over the fixture fleet and one
real pair, else it stays a finding". **Measured 2026-09-14: P=1.00, R=0.20
overall — the row stays a finding.** Full numbers, the FP/FN list and causes:
`docs/bench/readings/vb31-service-ref-m8.md`. This note is now a pointer plus
the ground-truth summary, not a measurement plan — the plan described below
(§ Measurement procedure) is what was actually run.

## Ground truth

`testdata/serviceref/route-table.json` is the hand-verified table: each row
names a client call or a route registration and whether it joins into a
`service_ref` edge across two services (`kind: match`), or has no counterpart
in its corpus (`kind: client_only` / `server_only`), or cannot be reduced to
one static `(method, template)` pair to hand-verify at all (`kind: excluded`).
A human read the source and wrote these rows independently of
`internal/analysis/serviceref`'s own code — and, for the fixture fleet, was
written BEFORE the extractor was run this session (`written_before_extractor_
run: true`, `written_at_utc`) — which is what makes them ground truth rather
than a restatement of the implementation. (The schema here is the task's:
`client_repo`/`client_file`/`client_symbol`/`method`/`route_template`/
`server_repo`/`server_file`/`handler_symbol`, superseding this file's earlier
`should_join`/`from_service`/`real_pairs` draft schema from VB-30's time —
same three fixture facts, same conclusions, re-shaped.)

Three corpora:

- **`fauxtown`** — `checkout-service` / `catalog-service` under
  `internal/connector/testdata/fauxtown/` (the other four fauxtown services
  have no HTTP content). One match (`checkout-service.FetchItem`'s `GET
  http://catalog-service/items/%s` → `catalog-service`'s `GET /items/:id`,
  `getItemHandler`), one decoy call (`HoldItem`, unmatched by design), one
  decoy route (`POST /items`, unmatched by design). P=1.00, R=1.00.
- **`real_pair`** — `openfaas/faas` (gateway) calling `openfaas/faas-provider`
  over HTTP, the one hand-verified non-fixture pair M8 asks for, sourced
  org-agnostically (ADR-011) from a public project, shallow-cloned into a
  scratch directory (never committed here). 4 genuine cross-repo edges
  hand-verified; the extractor found 0 of them. P=n/a (zero predictions
  made), R=0.00.
- **`reachfix-fp-control`** (unscored) — `internal/fusion/testdata/reachfix`
  (a different plan's fixture, discovered by grepping every `testdata/` for
  route shapes) has route registrations but zero client calls; folded into
  the same join only to prove it contributes zero hops.

## Measurement procedure (as run)

1. Read `internal/analysis/serviceref/serviceref.go` for its contract only —
   not its tests' expected outputs, and without running it — then write the
   ground-truth table from the fixture and real-pair source by hand.
2. Run `internal/analysis/routes.Extract` and
   `internal/analysis/serviceref.ExtractClientCalls` /
   `ServiceRefEdges` over the same inputs: an in-repo golden test
   (`internal/analysis/serviceref/goldencorpus_test.go`,
   `TestGoldenCorpus_Fauxtown`) for `fauxtown` + `reachfix-fp-control`, and a
   throwaway program (`workspace/tmp/vb31realpair/main.go`, gitignored, not
   product code) for `real_pair` (its source lives outside this repo).
3. For every `kind: match` row, confirm the extractor produced a hop with the
   same `(client_repo/FromService, method, route_template/Template,
   server_repo/ToService)` key — a miss is a false negative. For every hop
   the extractor actually produced, confirm a `match` row exists for that key
   — an extra hop is a false positive.
4. `precision = TP / (TP + FP)`; `recall = TP / (TP + FN)`, per corpus and
   overall. M8 passes at precision ≥ 0.90 **and** recall ≥ 0.90.

The in-repo golden test reports precision/recall via `t.Logf` and does not
fail `go test` on the threshold itself (the plan row's checkbox carries the
verdict, not the test) — it does hard-assert `fauxtown`'s FP/FN stay at zero
and that `reachfix-fp-control` contributes zero hops, both of which are
correctness invariants, not threshold calls.

## Why the row stays a finding

The fixture proves the extractor's *mechanics* (normalization, cross-repo
join, decoy rejection) — P=1.00, R=1.00, cleanly. It does not prove *external
validity*: on `openfaas/faas` ↔ `openfaas/faas-provider`, ordinary
unremarkable Go code that nobody wrote to be extraction-friendly, the client
detector missed every real call because none of them passed a literal or an
*inline* `fmt.Sprintf(...)` straight to `http.NewRequest` (real code builds
the URL in a variable first), and the reused route detector
(`internal/analysis/routes`) lost every real method to gorilla/mux's chained
`.Methods(...)` (invisible to its regex, so every extracted route reads
`Method: "ANY"`) and truncated every route whose path is built by Go string
concatenation. Overall R=0.20 (1 of 5 true edges found across both corpora).
See `docs/bench/readings/vb31-service-ref-m8.md` for the full FP/FN list,
causes, and the two concrete (out-of-scope-for-VB-31) fixes that would move
this number: resolve a single local-variable assignment back to its
`fmt.Sprintf`/literal definition on the client side, and recognize a chained
`.Methods(...)` call plus adjacent `+`-joined string literals on the server
side.

VB-31's plan row still carries `skip_test: true` with a `skip:` reason rather
than an `oracle: ledger` binding (`docs/plans/README.md`) — this measurement
was recorded as a reading doc, not appended to `docs/bench/ledger.jsonl`.
Wiring `oracle: ledger` so `fathom verify` can check the row by contract
rather than by prose is a reasonable follow-up but was not done here.
