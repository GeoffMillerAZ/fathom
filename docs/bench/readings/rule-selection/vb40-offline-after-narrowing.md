# RS-M2b: VB-40 offline after compile-time scope narrowing (2026-10-21, $0)

> **Disposition (owner, 2026-10-22).** This reading measured the RS-4 implementation on branch `rs4` (commit 946abe105) with the replay harness of commit a25affa87; neither is merged. RS-4 is rejected for now (docs/plans/rule-selection.md).

In plain terms: RS-4's narrowing makes rule selection worse, not better. It serves the governing rule at first touch on 1 of the 8 guards tasks, down from 3 at the base commit, and it makes the edit hook about 4 times slower. The cause is name resolution. Common words in a rule's text resolve as indexed names. "store", "docs", "main", "Never" and "two" all match a package, a directory or a symbol somewhere in the index. So 295 of the 317 repo-wide rules gained literal file scopes. Those scopes rank as exact-file matches and push the real governing rules out of the 6-rule cap. Do not keep RS-4 as written. The d2 order is not at fault: it ranked exactly what it was given.

Plan row `RS-M2b` in `docs/plans/rule-selection.md`, after `RS-4` (ADR-077 owner decision, code `narrowScopeFromMappings` in `internal/semantic/narrowscope.go`, called from `verifyDraft`). These bars are RS-M2's, unchanged: the governing rule served at first touch on at least 6 of the 8 guards tasks, fixture precision at least 0.90, at most 20 ms added p95 hook latency, and no whole-table load (I48). The d2 order is unchanged and nothing was tuned to the tasks.

## Verdict

RS-M2b verdict: FAIL

The governing rule is served at first touch on 1 of 8 tasks. That task is i2, which drops from position 1 to position 2. At the base commit `76f942a33`, against the same store copy, the count is 3 of 8 (i2, i28, i36), which reproduces RS-M2. Narrowing loses i28 and i36 and gains nothing.

Fixture precision is 46 of 46 (1.00) by RS-M2's definition. That definition is vacuous here, because a narrowed scope contains the path by construction. Narrowing-promoted rules hold 24 of the 48 served slots. By hand, 5 of those 24 are about the edited code's subject.

Added CPU per serve call is +69 to +72 ms at p50 and +85 to +97 ms at p95, against a 20 ms bar.

No new read is on the request path, and the serving code is not what got slower. HEAD's code reads the un-narrowed copy at the base cost (p95 32.1 ms) and serves the same 3 of 8. The cost comes from the narrowed scopes: 36,853 literal paths were added, and the largest scope has 1,345 entries. `fileScopeMatch` compiles one matcher per glob for each candidate on every call.

## Per task

| Task | Edited path | Base | HEAD | Candidates at HEAD (exact / package / parent / root) | Rules narrowed onto the path | First governing candidate at HEAD |
|---|---|---|---|---|---:|---|
| `guards-i1-packagedeps-sorted` | `internal/analysis/packagedeps/packagedeps.go` | miss | miss | 0 / 0 / 5 / 280 | 0 | rank 123, root, `AGENTS.md#L35` |
| `guards-i2-store-purego` | `internal/store/sqlite.go` | pos 1 | pos 2 | 25 / 55 / 27 / 255 | 26 | rank 2, exact file (narrowed), `ADR.md#L2945` |
| `guards-i11-mcp-tool-doc` | `internal/mcp/server.go` | miss | miss | 0 / 72 / 27 / 280 | 0 | rank 318, package, a plan binding |
| `guards-i16-raw-cp-deploy` | `CLAUDE.md` | miss | miss | 3 / 0 / 0 / 32 | 0 | rank 22, root, `AGENTS.md#L28-L29` |
| `guards-i19-org-agnostic-fixture` | `internal/connector/testdata/multiremote/README.md` | miss | miss | 0 / 0 / 6 / 35 | 0 | rank 18, root, `AGENTS.md#L15` |
| `guards-i28-duckdb-path` | `internal/store/backend.go` | pos 1 | miss | 26 / 55 / 27 / 254 | 27 | rank 28, package, `ADR.md#L754-L756` |
| `guards-i31-workspace-artifact` | `cmd/fathom/defaultpaths.go` | miss | miss | 6 / 332 / 4 / 274 | 7 | rank 289, root, `AGENTS.md#L10` |
| `guards-i36-usecase-maintained-line` | `docs/use-cases/incident-investigation.md` | pos 1 | miss | 7 / 1 / 9 / 24 | 7 | rank 8, package, `AGENTS.md#L77` |

The base's tiers have no exact-file candidate on any of the 8 paths. On i2, i28 and i31 the narrowed count is one higher than the exact-file count, because the in-force check withholds the one narrowed claim that cites superseded ADR-003 (`ADR.md#L52` or `#L53-L54`). On i16, HEAD has 3 exact-file candidates that come from the `classifyGlob` change in `governingscope.go`, not from narrowing. Three `self-healing-fleet.md` rules already scoped to the literal `CLAUDE.md` now rank as exact-file and move to positions 1 to 3. That reorder shows on the un-narrowed copy too.

## Every miss, traced

- **i28, lost:** 27 rules were narrowed onto `internal/store/backend.go`. "store" is a Go package name in the index, and these rules' text uses the word: "any second store" in ADR-049's refused list (`ADR.md#L2946`) and "a scratch store" (`ADR.md#L1517`). `BackendForPath` is a unique symbol, named in passing in a K4 incident note (`ADR.md#L3573`). Each narrowed rule now carries `backend.go` as a literal scope entry, which ranks as exact-file. That puts it ahead of the governing rule's `internal/store/**` package glob, so the governing rule falls from rank 2 to rank 28.
- **i36, lost:** 7 rules were narrowed onto `docs/use-cases/incident-investigation.md` through the bare word "docs", because every indexed directory is registered under its bare name too. They outrank the governing rule's `docs/use-cases/**` glob, which falls from rank 1 to rank 8.
- **i2, kept at position 2:** the 6 served slots all go to rules narrowed through "store". One of them, `ADR.md#L2945` ("pure-Go SQLite"), matches the task, so it counts as served. The base's position-1 rule (`ADR.md#L751-L756`, package glob) is displaced.
- **i1:** the governing rule (`AGENTS.md#L35`, determinism) was narrowed, but onto the wrong files. Its cited bullet names `cmd/fathom` ("cmd/fathom's two proc readers"), and "two", "contract" and "Determinism" resolve as symbols. It gained 1,055 entries, none under `internal/analysis/`, and still ranks 123rd at root tier. The rule names no path of the edited package, so narrowing has nothing correct to resolve.
- **i11:** no governs-role claim names `docs/mcp-tools.md`. This is RS-M2's construction gap, and narrowing cannot reach it.
- **i16:** the raw-cp rule (`AGENTS.md#L28-L29`) gained 757 entries through "Never", "ask", "branch", "budget", "config", "daemon", "main" and "tag". None of them is `CLAUDE.md`, a root file the rule never names.
- **i19:** the org-agnostic rule (`AGENTS.md#L15`) gained 594 entries through "docs", "tests", "fixtures", "configuration", "zero" and `ADR.md`, none under `internal/connector/testdata/`. It stays at rank 18, root tier.
- **i31:** the governing rule (`AGENTS.md#L10`) is not repo-wide. Its scope is `**/*.go`, `Taskfile.yml` and `scripts/**`, so `repoWideScope` skips it. `workspace/` is gitignored and so is not an indexed directory. The 6 served slots go to the `AGENTS.md#L35` and ADR rules narrowed onto `cmd/fathom/` files.

One cause covers all three regressions and the new served sets: a common word resolves as a concrete name. The sources are bare directory names, short Go package names (`main` covers 1,291 files, `store` 64), and unique heading or fixture symbols ("Never" is a heading in `docs/intent/autonomy.md`, and "two" and "zero" are Python symbols in bench scripts). Directory and package targets then expand into literal file entries, which rank as exact-file rather than as the package or directory the text named.

## What narrowing re-scoped

Of 778 served-status claims in narrowable layers, 317 are repo-wide, and narrowing re-scoped 295 of them. All 295 are `obligations` claims. The `adr-rules` layer has no repo-wide claim. Narrowing added 36,853 scope entries, and the largest scope has 1,345 entries. The `spec-bindings` layer (claims role) is not narrowed.

## Method (re-computable)

1. Data: a `sqlite3 -readonly .backup` copy of the shared daemon's store (`workspace/db/fleet.sqlite`, 2026-10-21 21:59), service `fathom`, with `service_meta.path` pointed at this branch's worktree. Nothing was written to, ingested into or loaded from the shared store or the daemon, and no model was called. Two copies were made: one stays as compiled (base), and the other has narrowing replayed into it (HEAD).
2. Replay, a deviation from RS-M2 that had to be made. RS-M2 measured the daemon's last compile and recompiled nothing. RS-4 runs only inside `verifyDraft`, after a model call, so a real recompile is model spend. `TestRSM2bNarrowStoreCopy` (`internal/semantic/vb40narrow_test.go`, run with `FATHOM_RS_NARROW_COPY=<copy>`; it refuses any path under `workspace/db/`) replays the stored drafts at $0. Each served-status claim of a layer `Compile` would narrow gets `compileScopeNarrower`'s answer, the same function `Compile` builds. The inputs are the claim's stored rule, its cited text (the cites link's anchor text) and its citation, and `IndexedFiles` is built the way `pkg/client`'s `CompileLayer` builds it. The scope column and the scopes links are rewritten together. Claim ids are kept, where a recompile would re-mint them from the new scope. That only affects the last tie-break.
3. Harness: RS-M2's, unchanged: `FATHOM_RS_STORE=<copy> go test ./internal/semantic -run TestVB40Offline_GuardsFirstTouch -v` (`vb40offline_test.go`), with the same matchers. HEAD is this branch's code on the replayed copy. Base is a worktree of `76f942a33` on the as-compiled copy.
4. Fixture precision: RS-M2's definition, unchanged. Of the governs-role rules served on the 8 paths, count those whose stored scope matches the path. The supplementary hand count covers the served rows whose scope gained the edited path through narrowing.
5. Latency: user plus system CPU of the serve call (`getrusage`), 320 calls per run (8 paths, 40 rounds), from prebuilt test binaries, with HEAD and base interleaved twice. Base: p50 30.7 and 29.7 ms, p95 33.5 and 35.0 ms. HEAD: p50 101.3 and 99.6 ms, p95 130.1 and 119.7 ms. Control, HEAD's code on the as-compiled copy: p50 28.6 ms, p95 32.1 ms, 3 of 8.
6. The trace lines (which names a governing rule resolved) were printed after the replay and never fed back.
