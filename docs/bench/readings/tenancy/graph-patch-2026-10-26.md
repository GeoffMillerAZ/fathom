# Graph patch vs rebuild across 24 real commits, 2026-10-26

**Verdict, in plain words.** Almost all of a cold call-graph build on this repository is one function. 96–97% of
`callgraph.Build` is `resolveCtorScoped`, which scans every node for each of about 3,850 constructor-scoped call
strings. With an index over those lookups, the same graph builds in 0.56–0.93 s instead of 17.1–25.3 s, identical
on all 25 snapshots. An exact row-level patch, which re-resolves only the callers a commit can affect, was also
identical to a rebuild on all 24 commits. It took 0.82–1.53 s, which is more than the indexed rebuild: rebuilding
and diffing the name indexes is O(nodes), and that already costs more than resolving every edge with an index. The
existing single-file patch (`Graph.ReplacePackage`, SU-4) differed from a rebuild on 13 of 24 commits, and it cost
up to 54 s. Read by `docs/design/responsible-tenant.md` §18.

## Method

- **Store.** This lane's own scratch store (never the 7575 daemon). First, one `fathom ingest --store <scratch>
  --service fathom` of a worktree at 8502a744, which took 174 s. Then 24 incremental ingests of the same store, one
  at each later first-parent commit of `main` up to f2264add (76–113 s each). The binary was built from f2264add.
- **Dumps.** After each ingest, a scratch program (not committed) saved `Client.ListAllSymbols` and
  `Client.TypedEdges` for the service, which are the loads the snapshot and `typed_edges` groups run. It also timed
  both loads.
- **Timings.** A scratch test in package `callgraph` (not committed) timed the following on each dump, one at a
  time, on an otherwise idle 4-core Linux VM: `Build`, `BuildBoth`, `BuildResolved`, `ProductEntryReach` and an
  indexed variant of `Build`. For each consecutive pair of dumps it also timed two patches of the previous syntax
  graph:
  - the scoped patch below;
  - `ReplacePackage` applied to every package a changed file belongs to, with that package's new nodes. This is
    what `memo.PatchPackage` does for a one-file refresh.

  It then compared each patched graph with `Build` over the new dump: nodes, every callee list, every caller list
  and the typed edge kinds.
- **Indexed variant.** Before the resolve loop, it builds one map from directory to `.Method` suffix to keys. A
  constructor-scoped string `<import-path>#.Method` then checks only the import path's `/`-boundary suffixes
  (`importPath == dir` or `importPath` ends in `"/"+dir`), not every node. It follows the same rule as the scan:
  exactly one match gives an edge, and none or more than one give nothing.
- **Scoped patch.**
  1. Rebuild the five name indexes over the new node set.
  2. Diff them against the old graph's indexes.
  3. Mark as touched every key that was added or removed, or that lies in a file whose nodes changed, together with
     the `.Method` suffix of each such key.
  4. Re-resolve every caller that is in a changed file, or that has a call string whose resolution path (in
     `resolve`'s own order) reads a touched key or a changed index entry. Every other caller keeps its old callee
     list.
- **Changed file.** A file whose node set (key, name, kind, path, line, calls) differs between the two dumps.

## Readings

| # | commit | nodes | changed files | Build s | BuildBoth s | indexed Build s | +typed s | scoped patch s | re-resolved callers | patch == rebuild | ReplacePackage s | ReplacePackage == rebuild |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 0 | 8502a744 | 64295 | (base) | 19.9 | 18.7 | 0.67 | 0.45 | | | | | |
| 1 | 45aae169 | 64347 | 10 | 17.1 | 18.3 | 0.66 | 0.38 | 1.06 | 798 | yes | 11.22 | no (8 callers) |
| 2 | 740d3c9e | 64348 | 2 | 19.6 | 20.5 | 0.66 | 0.41 | 0.88 | 33 | yes | 0.18 | yes |
| 3 | d5038ee5 | 64684 | 52 | 20.7 | 19.4 | 0.57 | 0.42 | 0.95 | 2211 | yes | 17.04 | no (64 callers) |
| 4 | 0666ccc7 | 64684 | 1 | 20.7 | 22.1 | 0.60 | 0.44 | 0.84 | 19 | yes | 0.17 | yes |
| 5 | 5cf215cb | 64708 | 2 | 19.4 | 20.4 | 0.56 | 0.53 | 0.98 | 24 | yes | 0.24 | no (3 callers) |
| 6 | 8dd8003a | 64709 | 2 | 21.3 | 21.8 | 0.65 | 0.44 | 0.85 | 25 | yes | 0.20 | no (3 callers) |
| 7 | 6144a135 | 64745 | 16 | 21.2 | 21.5 | 0.78 | 0.43 | 1.10 | 391 | yes | 12.24 | no (9 callers) |
| 8 | 5efa49bf | 64746 | 2 | 22.2 | 22.2 | 0.71 | 0.46 | 1.09 | 58 | yes | 1.39 | yes |
| 9 | 68368af0 | 64746 | 1 | 20.9 | 20.9 | 0.59 | 0.50 | 0.89 | 17 | yes | 0.47 | no (1 callers) |
| 10 | a652c571 | 65663 | 140 | 20.9 | 20.4 | 0.75 | 0.46 | 1.01 | 6255 | yes | 34.12 | no (98 callers) |
| 11 | 636e517b | 66458 | 125 | 21.6 | 20.7 | 0.69 | 0.43 | 1.05 | 7105 | yes | 30.50 | no (163 callers) |
| 12 | a42e9ea6 | 66489 | 7 | 20.7 | 20.9 | 0.68 | 0.44 | 0.91 | 855 | yes | 1.23 | no (5 callers) |
| 13 | 6c1a17dd | 66491 | 2 | 21.3 | 29.7 | 0.72 | 0.64 | 1.53 | 135 | yes | 0.30 | yes |
| 14 | 1e89100f | 66478 | 18 | 24.9 | 20.4 | 0.65 | 0.47 | 0.95 | 57 | yes | 3.65 | no (65516 callers) |
| 15 | 387953b3 | 66478 | 0 | 20.6 | 20.3 | 0.72 | 0.44 | 0.93 | 0 | yes | 0.00 | yes |
| 16 | 9b0c3878 | 66479 | 1 | 21.8 | 22.4 | 0.64 | 0.49 | 0.86 | 1 | yes | 0.17 | yes |
| 17 | dfc2c4b3 | 66479 | 0 | 21.1 | 22.4 | 0.64 | 0.41 | 0.82 | 0 | yes | 0.00 | yes |
| 18 | d04992c5 | 66479 | 0 | 20.0 | 23.0 | 0.82 | 0.38 | 1.00 | 0 | yes | 0.00 | yes |
| 19 | d9ea9e31 | 66480 | 1 | 21.2 | 21.4 | 0.67 | 0.44 | 1.01 | 1 | yes | 0.22 | yes |
| 20 | 1bcd20de | 68678 | 241 | 22.6 | 23.0 | 0.75 | 0.63 | 1.41 | 7258 | yes | 53.99 | no (227 callers) |
| 21 | 89d8a0bc | 68681 | 12 | 23.7 | 25.5 | 0.80 | 0.47 | 1.05 | 235 | yes | 2.40 | yes |
| 22 | 6620dbab | 68683 | 2 | 22.4 | 23.0 | 0.77 | 0.51 | 1.01 | 137 | yes | 0.21 | yes |
| 23 | 9dc5ed7d | 68687 | 2 | 23.3 | 24.0 | 0.66 | 0.50 | 1.09 | 18 | yes | 9.90 | no (6 callers) |
| 24 | f2264add | 68711 | 7 | 25.3 | 24.7 | 0.93 | 0.45 | 1.23 | 545 | yes | 1.56 | no (6 callers) |

The "+typed" column is `AddTypedEdges` over all of the service's typed edges, applied to the indexed syntax graph.
With it, the indexed graph equaled `BuildBoth` on every dump. The scoped patch plus the same call also equaled
`BuildBoth` on every commit.

Ranges, as minimum–maximum (median):

| load or build | seconds |
|---|---|
| `ListAllSymbols` (snapshot load) | 0.37–0.48 (0.39) |
| `memo.NewSnapshot` indexing (dumps 0, 12, 24) | 0.16–0.20 |
| `TypedEdges` (typed_edges load) | 0.58–0.76 (0.64) |
| `Build` (syntax) | 17.10–25.27 (21.17) |
| `BuildBoth` | 18.31–29.65 (21.50) |
| `BuildResolved` | 0.61–1.06 (0.68) |
| indexed `Build` | 0.56–0.93 (0.67) |
| `AddTypedEdges`, all typed edges | 0.38–0.64 (0.45) |
| `ProductEntryReach` on the both graph | 0.12–0.19 (0.14) |
| scoped patch, syntax | 0.82–1.53 (1.01), of which index rebuild 0.33–0.75, index diff 0.28–0.54, edges 0.14–0.28 |
| `ReplacePackage`, per commit | 0.00–53.99 (1.23) |

Size of the subject:

| | dump 0 | dump 24 |
|---|---|---|
| nodes | 64,295 | 68,711 |
| raw call strings | 220,489 | 231,735 |
| constructor-scoped call strings | 3,845 | 3,882 |
| typed edges | 213,060 | 215,611 |
| syntax edges | 81,232 | 82,936 |
| `EstimatedBytes` of the both graph | 133.0 MB | 139.8 MB |

Changed files per commit were 0–241 (median 2), and three of the 24 ingests changed no symbol rows at all. The
scoped patch re-resolved 0–7,258 callers (median 58). A commit's reach is not bounded by its files or its package:
resolution goes through service-wide unambiguity indexes. A new function named like an existing one, anywhere,
turns every bare-name caller of that name in the service from resolved to unresolved.

## What `ReplacePackage` got wrong

The differing callers fall into three classes, read from the scratch test's per-caller diff:

1. **Duplicate edges.** `ReplacePackage` loops over the new nodes without deduplicating by key, so a key observed
   twice adds each of its edges twice. Examples: CUE fields and Go `init` functions, which share one key per
   package; `conserve.init` got six copies of one edge to `Register`.
2. **Edges a rebuild does not have.** Two kinds:
   - The package is selected by its `Package` name, which takes in every directory that declares `package main`.
   - Two same-key nodes from build-tagged files get their edges added under "first in the list", where `Build`
     uses "final observation wins". Example: `cmd/fathom` `processResidentMemoryBytes` kept edges the rebuilt
     graph does not have.
3. **Missed edges.** Other packages' callers are never re-resolved, which the file's own comment documents.

Its comment claims "never a WRONG edge". Classes 1 and 2 contradict that claim.

## Caveats

- The VM's absolute numbers are about 2.5× the operator machine's: O1260 reads a cold build there at 8.4–9.6 s,
  against 17.1–25.3 s here. The ratios are the reading. These are single runs, not repeated measurements; the
  ranges across 25 snapshots of one subject stand in for that.
- The indexed build and the scoped patch were scratch code. The rows in `docs/plans/responsible-tenant.md` §H turn
  each into a committed function, and its test re-proves equality.
