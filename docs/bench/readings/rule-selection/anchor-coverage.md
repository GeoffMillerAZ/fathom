# RS-M1: anchor coverage of the rules eligible on the VB-40 guards paths (2026-10-21, $0)

Plan row `RS-M1` in `docs/plans/rule-selection.md`, design ADR-077 d2 rank (1). Question: of the rules eligible on the 8 VB-40 target paths, how many cite a doc unit that has a `doc_links` row reaching the edited path or its package? KEEP (build the anchor rank) at 25% or more, else CUT (RS-1 starts with contract keys and path proximity).

## Verdict

RS-M1 verdict: CUT

23 of 1731 eligible rules (1.3%) are anchored by a `doc_links` row to the edited file or its package; 4 (0.2%) by a link to the exact file. The anchor rank would be empty for 5 of the 8 tasks and decide fewer than 20 of 350 eligible rules on any task. Rank (1) stays first in ADR-077's fixed order, but RS-1 starts with ranks (2) to (4): contract keys and path proximity.

## Method (re-computable)

1. Data: a read-only `.backup` copy of the shared daemon's store (`workspace/db/fleet.sqlite`, 2026-10-21 17:17), service `fathom`. Nothing was written to, ingested into or loaded from the shared store; no model call.
2. Tasks and paths: the 8 tasks of `testdata/scenariobench/guards-fathom.json` named in `docs/bench/readings/decision-port-ab-2026-09-23.md` (guards mechanism analysis). Each task names a target directory, so the edited path is one representative file in it (the tasks create or edit files there): the file column below.
3. Eligible rules: claims of the two governs-role layers (`obligations`, `adr-rules`) with a served status (fresh, stale, contradicted) and a scope glob that matches the edited path (`store.ClaimsForServe` with the serve prefixes, then `internal/filter` glob match, the same test `fileScopeMatch` applies). Raw sections are not counted (they carry no claim and no cite). The counts agree with the baseline reading's "N more" lines to within the cap (6 shown + `displaced`).
4. Cite to unit: each claim's `cites` link (`path#Lstart-Lend`) resolves to the narrowest `doc_units` row of that path containing `Lstart`.
5. Anchored: that unit has a live (`valid_to` = 0) `doc_links` row with `target_kind` `path` whose target is the edited file or a file in its directory, or `target_kind` `symbol` whose symbol (`symbol_centrality.file`) lives in that directory. For `CLAUDE.md` (repo root) only the exact file counts, since the root directory would match everything.

## Result

| Task | Edited path (representative) | Eligible rules | Cite resolved to a doc unit | Unit has `path` link to the file | Unit has `path` link to the file or its package | Unit has `symbol` link into the package | Anchored |
|---|---|---:|---:|---:|---:|---:|---:|
| `guards-i1-packagedeps-sorted` | `internal/analysis/packagedeps/packagedeps.go` | 289 | 281 | 0 | 0 | 0 | 0 |
| `guards-i2-store-purego` | `internal/store/sqlite.go` | 346 | 338 | 2 | 2 | 0 | 2 |
| `guards-i11-mcp-tool-doc` | `internal/mcp/server.go` | 318 | 310 | 0 | 4 | 0 | 4 |
| `guards-i16-raw-cp-deploy` | `CLAUDE.md` | 35 | 35 | 0 | 0 | 0 | 0 |
| `guards-i19-org-agnostic-fixture` | `internal/connector/testdata/multiremote/README.md` | 35 | 35 | 0 | 0 | 0 | 0 |
| `guards-i28-duckdb-path` | `internal/store/backend.go` | 346 | 338 | 2 | 2 | 0 | 2 |
| `guards-i31-workspace-artifact` | `cmd/fathom/defaultpaths.go` | 321 | 313 | 0 | 15 | 0 | 15 |
| `guards-i36-usecase-maintained-line` | `docs/use-cases/incident-investigation.md` | 41 | 41 | 0 | 0 | 0 | 0 |

Total: eligible 1731, resolved 1691, path link to the file 4, path link to the file or its package 23, anchored by any link 23 (1.3%).

## Notes

- The governing rule of each task (its `rule.source` line in `docs/intent/invariants.md`) lives in units of that file; only guards-i31's unit links the edited file and package (its unit carries 153 links, a fan wide enough that the link does not discriminate).
- `symbol` targets are bare names, so a name shared across packages resolves to one file in `symbol_centrality`; the symbol column is an upper bound and still reads 0.
- The store holds the daemon's last ingest, not a fresh one of the committed HEAD.
