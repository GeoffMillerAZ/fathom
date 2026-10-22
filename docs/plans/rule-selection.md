# Rule selection: the plan (ADR-077)

Grammar v2 (`docs/plans/README.md`). A row flips to `[x]` only when
`fathom verify --service fathom --plan docs/plans/rule-selection.md` proves it. Design: ADR-077 (proposed 2026-10-22,
flips to accepted on RS-M2's reading), pointer in `docs/design/semantic-layers.md`. Amends ADR-016 (serving). Baseline:
`docs/bench/readings/decision-port-ab-2026-09-23.md` (VB-40: the governing rule at first touch on 3 of 8 guards tasks).

Every reading here is $0: offline replays and counts, no provider or agent spend. No row adds a table, a column, a cache
or a background job (ADR-049 decision 2) and none adds an MCP tool (P4). The selector ranks only: a heuristic-tier link
never denies an edit (I57). The 6-rule cap and the 320-token ceiling do not rise (I35). RS-M1 read CUT, so RS-1 starts
with ranks (2) to (4) of ADR-077 d2 (contract keys, path proximity); the `doc_links` rank (1) stays first in the order and
is built only if a later reading raises its coverage.

## Wave 1: selector and in force (d2, d3)

- [x] (op) RS-M1: anchor coverage of the rules eligible on the 8 VB-40 target paths, by `doc_links`. KEEP (25% or more) builds the anchor rank, CUT starts RS-1 with contract keys and path proximity. Read CUT at 23 of 1731 (1.3%) {id: RS-M1, oracle: lexical, path: docs/bench/readings/rule-selection/anchor-coverage.md, pattern: "^RS-M1 verdict: (KEEP|CUT)"}
- [x] One deterministic selector between the candidates (claims plus raw sections) and the 6-rule cap, ranking by scope evidence, strongest first: a `doc_links` row, a contract key or log template on the edited symbol, a path glob naming the edited path, a repo-wide glob; moved out of `serve.go` into its own file; rank only {id: RS-1, symbol: rankByScopeEvidence, package: internal/semantic, reachable_from: TestSelectorRanksByScopeEvidence, needs: "RS-M1"}
- [x] A claim citing a fully superseded ADR section is withheld from the cap with the withheld count disclosed; a partly superseded one stays, tagged; `internal/adr.ParseSections`, a section without a status line parses as accepted, `valid_to` stays empty {id: RS-2, symbol: claimInForce, package: internal/semantic, reachable_from: TestClaimCitingSupersededADRIsWithheldAndCounted}

## Wave 2: readings

- [x] (op) RS-M2: VB-40 offline. The governing rule at first touch on at least 6 of 8 guards tasks at HEAD, measured the way `internal/semantic/h1governance_test.go` measures; fixture precision at least 0.90; at most 20 ms added p95 hook latency; no whole-table load (I48). Read FAIL at 3 of 8 (precision 1.00, +4.5 to +9.2 ms p95 CPU): the five misses' governing rules are compiled repo-wide, so ADR-077's revisit clause fired {id: RS-M2, oracle: lexical, path: docs/bench/readings/rule-selection/vb40-offline.md, pattern: "^RS-M2 verdict: (PASS|FAIL)", needs: "RS-1"}
- [x] (op) RS-M3: in-force hand check of 20 claims, including those citing ADR-003, ADR-004 and ADR-013, with 0 wrong withholds. Read PASS: 10 withheld and 6 tagged of 526 ADR-citing claims, 0 wrong {id: RS-M3, oracle: lexical, path: docs/bench/readings/rule-selection/in-force-check.md, pattern: "^RS-M3 verdict: (PASS|FAIL)", needs: "RS-2"}
- [x] (op) RS-M4: raw share of served rows over 14 days of hook log ($0); above 50% is a revisit trigger of ADR-077. Read at most 38.9% (the call log's citations, bounded by 2 raw rows per call), not fired {id: RS-M4, oracle: lexical, path: docs/bench/readings/rule-selection/raw-share.md, pattern: "^RS-M4 raw share: [0-9.]+%"}

## Deferred

- [-] Producer-catalog widening: `schema/semantic/producers.cue` covers every claim writer (LC-10's revival) {id: RS-3, disposition: deferred, reason: "ADR-077 d5: waits for a catalogued writer's claim refuted at a tier above its method, or 2+ new writers with a named consumer by 2027-01-20"}
- [-] (op) RS-M5: served-rule behavior at n>=30 via PR-M2 {id: RS-M5, disposition: deferred, reason: "needs about $76 of bench spend, on hold"}
