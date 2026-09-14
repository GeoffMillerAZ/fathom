# `service_ref` precision/recall (VB-31)

Row VB-31 (`docs/plans/verbs.md`, `needs: VB-30`): M8 is "precision ≥ 0.90 and
recall ≥ 0.90 on a hand-verified route table over the fixture fleet and one
real pair, else it stays a finding" — currently `skip_test: true`, disposition
`deferred`, because the "one real pair" half of the ground truth is not yet
populated (see below). This note is the measurement plan the row's `skip:`
reason points at, plus the seed corpus, so filling in the real pair is the
only step left before VB-31 can run for real.

## Ground truth

`testdata/serviceref/route-table.json` is the hand-verified table: each row
names a client call or a route registration, whether it *should* join into a
`service_ref` edge (`should_join`), and — for a true pair — the exact
normalized `(method, template)` a correct extractor must produce. A human
read the fixture source and wrote these rows independently of
`internal/analysis/serviceref`'s own code, which is what makes them ground
truth rather than a restatement of the implementation.

The current rows cover the `checkout-service` / `catalog-service` fixture
pair added under `internal/connector/testdata/fauxtown/` alongside VB-30:

- **`fixture-1-match`** — `checkout-service.FetchItem`'s `GET
  http://catalog-service/items/%s` normalizes to `GET /items/{}`, which
  matches `catalog-service`'s registered `GET /items/:id` (`getItemHandler`).
  The one true positive.
- **`fixture-2-decoy-call`** — `checkout-service.HoldItem`'s `DELETE
  .../items/%s/hold` matches no route anywhere in the fleet: a true negative
  on the client side (an unmatched call must not manufacture a hop).
- **`fixture-3-decoy-route`** — `catalog-service`'s `POST /items`
  (`createItemHandler`) has no client caller anywhere in the fleet: a true
  negative on the route side (an unmatched route must not appear as a hop
  either).

## Measurement procedure

1. Run `internal/analysis/serviceref.BuildFromStore` (or `ServiceRefEdges`
   directly, as `internal/analysis/serviceref/serviceref_test.go`'s
   `TestServiceRefEdges_FixtureJoin` already does) over the fixture fleet.
2. For every row in `route-table.json` with `should_join: true`, confirm the
   extractor produced exactly one hop matching its `from_service`,
   `to_service`, `method` and `normalized_template` — a miss is a false
   negative.
3. For every hop the extractor actually produced, confirm a `should_join:
   true` row exists for it — a hop with no matching row is a false positive
   (this catches `fixture-2-decoy-call` / `fixture-3-decoy-route` style
   over-matching).
4. `precision = true_positives / (true_positives + false_positives)`;
   `recall = true_positives / (true_positives + false_negatives)`. M8 passes
   at precision ≥ 0.90 **and** recall ≥ 0.90.
5. Record the run as a `docs/bench/ledger.jsonl` row (`docs/plans/README.md`'s
   `ledger` oracle) so VB-31 can flip from `skip_test` to a verified box —
   never by claim.

## Why "one real pair" is still open

The fixture proves the extractor's *mechanics* (normalization, cross-repo
join, decoy rejection) but not its *external validity* — whether the same
(method, template) matching heuristic holds on a route table nobody wrote to
make the extractor look good. VB-31 asks for one hand-verified pair from a
real (non-fixture) service-to-service HTTP call, sourced org-agnostically
(ADR-011: no org-specific names, hosts, or catalogs in code, docs, tests, or
fixtures) — for example a public open-source microservice pair, not an
internal repo. `route-table.json`'s `real_pairs` array is the seed for it;
until it holds at least one row, M8 measures only the fixture half and VB-31
stays a documented finding rather than a verified box.
