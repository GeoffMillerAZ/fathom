# RS-M2: VB-40 offline, the governing rule at first touch with the scope-evidence selector (2026-10-21, $0)

Plan row `RS-M2` in `docs/plans/rule-selection.md`, design ADR-077 d2 (the order fixed before this reading), code `rankByScopeEvidence` (`internal/semantic/selector.go`) and `claimInForce` (`inforce.go`). Bar: the governing rule served at first touch on at least 6 of the 8 guards tasks, fixture precision at least 0.90, at most 20 ms added p95 hook latency, no whole-table load (I48).

## Verdict

RS-M2 verdict: FAIL

The governing rule is in the served set at first touch on 3 of 8 tasks at HEAD, the same 3 as at the base commit (i2, i28, i36; i36 moves from position 5 to 1). The other bars hold: fixture precision 1.00 (46 of 46 served governs-role rules are scoped to their path), added CPU per serve call +4 to +5 ms at p50 and +4.5 to +9.2 ms at p95, and the only new reads are one indexed point lookup of the edited file's own observation names and one bounded read of `ADR.md`. The order was not tuned to the tasks (ADR-077 R1). ADR-077's first revisit clause has fired: apply VB-40's own clause and reopen compile-time scope narrowing from mappings.

## Per task

| Task | Edited path | Base | HEAD | Candidates by scope (path globs / repo-wide) | Withheld superseded | First governing candidate at HEAD |
|---|---|---|---|---|---:|---|
| `guards-i1-packagedeps-sorted` | `internal/analysis/packagedeps/packagedeps.go` | miss | miss | 5 / 280 | 4 | rank 123, repo-wide, `AGENTS.md#L35` |
| `guards-i2-store-purego` | `internal/store/sqlite.go` | pos 1 | pos 1 | 82 / 280 | 6 | rank 1, package, `ADR.md#L751-L756` |
| `guards-i11-mcp-tool-doc` | `internal/mcp/server.go` | miss | miss | 99 / 280 | 4 | rank 318, package, a plan binding |
| `guards-i16-raw-cp-deploy` | `CLAUDE.md` | miss | miss | 0 / 35 | 0 | rank 22, repo-wide, `AGENTS.md#L28-L29` |
| `guards-i19-org-agnostic-fixture` | `internal/connector/testdata/multiremote/README.md` | miss | miss | 6 / 35 | 0 | rank 18, repo-wide, `AGENTS.md#L15` |
| `guards-i28-duckdb-path` | `internal/store/backend.go` | pos 1 | pos 1 | 82 / 280 | 6 | rank 2, package, `ADR.md#L754-L756` |
| `guards-i31-workspace-artifact` | `cmd/fathom/defaultpaths.go` | miss | miss | 336 / 280 | 4 | rank 289, repo-wide, `AGENTS.md#L10` |
| `guards-i36-usecase-maintained-line` | `docs/use-cases/incident-investigation.md` | pos 5 | pos 1 | 10 / 31 | 0 | rank 1, package, `AGENTS.md#L77` |

No task's edited path has a tier (1) or (2) fact: RS-M1 found no anchoring `doc_links` row for these rules, the Go files carry no log templates (TL-4 reads Python, Java, JS/TS and PHP loggers), and contract keys have no persisted reader (ADR-050 §6's tables are not built). So on these 8 tasks the selector reduces to tiers (3) and (4).

## Every miss, traced

- **i1:** the governing rule (determinism, sorted outputs) is compiled only from the root `AGENTS.md` with a repo-wide glob, so it sits in tier (4). Five path-glob rules (`internal/AGENTS.md`, `internal/**`) take five slots and the sixth goes to the best of 280 repo-wide rules by today's in-tier order (relevance, glob length, text); the determinism rule is 118th of them. `internal/AGENTS.md`'s own determinism contract compiled into no claim that names sorting or map order.
- **i11:** no governs-role claim for `internal/mcp` names `docs/mcp-tools.md`; the only candidate that does is a plan binding (claims role) scoped to the package, which BP-18 ranks after every governs-role rule, repo-wide ones included (rank 318 of 379). A construction gap: `internal/AGENTS.md`'s "Adding things > MCP tool" rule did not compile into a rule naming the doc.
- **i16:** the edited file is at the repository root, so every one of the 35 candidates is tier (4) and the deploy rule's rank (22nd) is today's in-tier order. Nothing in the index ties the raw-cp rule to `CLAUDE.md` more strongly than the other 34.
- **i19:** six `internal/**` path-glob rules fill the cap; the org-agnostic rule (`AGENTS.md#L15`, ADR-011) is compiled repo-wide, tier (4).
- **i31:** 336 path-glob rules (`cmd/fathom/**` and `cmd/**`) outrank the workspace/ rule, compiled repo-wide in tier (4).

All five misses have one cause: the governing rule is compiled with a repo-wide glob (or not compiled at all, i11) while at least six rules carry a path glob naming the edited path. A scope-evidence order cannot lift a rule the index ties to the edit no more strongly than any other repo-wide rule; that needs scope narrowed at compile time from mappings (ADR-077's revisit clause), not a different order.

## Method (re-computable)

1. Data: the read-only `.backup` copy of the shared daemon's store used by RS-M1 (`workspace/db/fleet.sqlite`, 2026-10-21), service `fathom`, `service_meta.path` pointed at this branch's clone so cited files are read at HEAD. Nothing was written to, ingested into or loaded from the shared store; no model call.
2. Harness: `FATHOM_RS_STORE=<copy> go test ./internal/semantic -run TestVB40Offline_GuardsFirstTouch -v` (`internal/semantic/vb40offline_test.go`, skipped without the variable). It asks `ObligationsFor` (the edit hook's compiled cap, 6 rules) for each task's representative path (RS-M1's file column) and scores a served row by keyword against its rule text and citation, as `h1governance_test.go` scores the what-governs lookups and the baseline reading matched the governing rule "by keyword against its own citation/topic". The keyword matchers were written from each task's `rule` block before any served list was read. `ExplainObligations` gives each candidate's rank and scope tier.
3. Base: the same harness on a worktree of the base commit (`764996011`) against the same store copy.
4. Fixture precision: of the rules served on the 8 paths, the governs-role ones whose stored scope glob matches the path (46 of 46); the 2 claims-role rules are scoped from their plan row's contract at serve time and are counted apart.
5. Latency: user+system CPU of the serve call (`getrusage`), 320 calls per run (8 paths, 40 rounds), base and HEAD interleaved twice: base p50 24.8 to 25.0 ms, p95 27.1 to 27.6 ms; HEAD p50 28.9 to 29.7 ms, p95 32.1 to 36.3 ms. The added cost is mostly parsing `ADR.md` (573 KB) once per call for the in-force check.
6. The store holds the daemon's last compile, not a fresh compile of the committed HEAD (a compile is model spend).
