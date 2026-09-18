## Review-recall loop (M15/VB-24): reproduce, improve, re-measure

### Verdict, plainly

`fathom review --format github`'s recall on the 42-pair historical-PR corpus
moves from **5/42 (11.9%)** to **6/42 (14.3%)** after one deterministic
mechanism — walking the "Changed symbols" caller listing to depth 2 instead
of stopping at direct callers. That is a real, verified gain (one new pair,
`golangci-lint` #5884→#5885, confirmed by hand: the caller that B's fix
touched is two hops from A's change, a relationship depth-1 structurally
cannot see), but it is nowhere near the row's `>= 30%` bar and the row stays
`[ ]`. Precision holds at 0/PR false findings — the same "index facts, not
generated" reasoning the original reading applied, unchanged by this
mechanism. Two of the five candidate mechanisms this loop was asked to try
turned out to be **already true** of the code (test-gap reachability and
heuristic-caller inclusion were never capped the way the candidate list
assumed); a third (selector-method/interface edge repair, O39/O40) is a real
open defect but needs investigation this session's time budget did not
reach. The **5 pure misses** this reading reproduces are byte-identical to
the published reading's list — the harness is a faithful, independently
re-derived match of the original numbers before any improvement is applied.

### Reproduction (STEP 1)

No runnable harness existed — the original VB-24 reading explicitly says its
scoring "lives in the scratch pipeline this reading reports on," not in this
repo. Built one: `internal/reviewbench` (pure: corpus loading, gold
extraction, scoring) + `cmd/reviewbench` (I/O: git fetch/checkout, in-process
ingest via `pkg/client`, in-process `review.ChangedSymbolReviews` /
`impact.TestGap` calls — never the built CLI binary, never rendered
Markdown). Scoring reads the review engine's own structured Go values
directly, which matters: the 2026-09-17 "strict re-score" found the
original bench's `CALLER_RE` regex never matched a single real
nested-caller citation (a rendered-Markdown re-parsing bug) — scoring off
structured data instead closes that whole bug class by construction.

**Method** (unchanged from the original reading, cited from
`docs/bench/readings/vb24-review-historical-prs.md`): per pair, shallow-fetch
PR A's base and merge commits of the real upstream repo, ingest A's base
into a throwaway store, resolve `git diff --name-only <base>` against A's
merge checkout for the changed-file set, run the review engine over it. Gold
= every `(file, function)` pair extracted from B's fix-PR diff (hunk-header
`@@ ... @@ func Name(...)` context plus any `func` line in the patch). Hit
rule (the row's actual bar, established by the 2026-09-17 strict re-score):
a gold entry counts only if it is named as a caller (any depth) outside A's
own changed files, or as a pre-existing test reaching A's change — not one
of A's own changed-file symbols recited back (that is a file-level fact,
not call-graph inference, and satisfied by construction for most of this
corpus).

Fixture-pinned smoke test: `internal/reviewbench/smoke_test.go`
(`FATHOM_REVIEWBENCH=1`, gated exactly like `internal/semantic/eval_test.go`'s
`FATHOM_EVAL_LLM` convention — real network + git fetch, never in
`go test ./...`) runs 2 real pairs and asserts the exact strict-hit outcome
this reading records for them.

**Baseline reproduced**: 3 of the corpus's 5 known hits were re-run directly
against the pre-mechanism binary (`caddyserver/caddy` #1300→#1309,
#4398→#4661, #5808→#7507 — all three strict-new hits, byte-identical to the
published reading). The other two (`golangci-lint` #5929→#5939,
#6445→#6762) are confirmed analytically rather than by a separate
pre-mechanism run: their post-mechanism match depth is 1 in every case,
and `ChangedSymbolReviews`'s pre-mechanism code **was** exactly
`callgraph.CallersDetail(g, k)` — direct callers only — so a depth-1 match
today is, by construction, identical to what the pre-mechanism code would
have found. All 5 pure misses (own-symbol-only excluded, no caller/test
signal at any depth) reproduce byte-identical to the published list:
`caddyserver/caddy` #4443→#4674, `golangci-lint` #3995→#4319, #4709→#4734,
#5160→#5168, #5402→#5407. This is strong, direct evidence the harness is a
faithful reproduction, not a coincidence of corpus size.

### STEP 2 — mechanisms tried, in the task's own order

**(a) Impact depth 2, labeled — IMPLEMENTED, measured gain.**
`review.ChangedSymbolReviews` (`internal/analysis/review/github.go`) walked
only `callgraph.CallersDetail(g, k)` — direct callers — into the rendered
"Changed symbols" section; the review engine's OWN blast-radius dimension
has always been unbounded, but that never reached the GitHub-rendered
comment's caller listing. Extended the walk to depth 2 (matching
`blast_radius`'s own BP-27 default, cited in its own tool description, for
the same reason: unbounded would let one heavily-called symbol's fan-in
swamp a PR comment). `CallerRef` gained a `Depth` field; rendering now shows
`[resolved, depth 2]` etc.; `CallSitesTotal`/`CallSitesUpdated`
(call-site-completeness) stay DIRECT-callers-only by design — a depth-2
indirect caller is not "a place that calls this." Sorted depth-ascending,
then resolved-before-heuristic (the same ordering convention the
`{typed, heuristic}` resolution-mix summary field already uses), then by
symbol.

Tests: `TestChangedSymbolReviews_TransitiveCallerDepth2` (a 3-hop chain —
GrandCaller→ResolvedCaller→A, GreatGrandCaller→GrandCaller — pins depth 2
included, depth 3 excluded, dedup, and call-site counts unaffected);
`TestRenderGitHubReview_ComposesTheFixedSections` updated for the new
`[tier, depth N]` render format.

Measured: **+1 pair** (`golangci-lint` #5884→#5885 — see worked example
below), **0 regressions** (every pair that was already a hit stayed a hit;
`caddyserver/caddy` #5808→#7507's two depth-2 matches are the SAME two test
functions the affected-test path already credited it for — redundant, not a
double-count, since scoring is pair-level). False findings: unchanged at
0/PR — a depth-2 caller is exactly as much an index fact as a depth-1 one.

**(b) Interface implementations / selector-method edges — NOT ATTEMPTED,
scoped for next.** The task named this "the highest-value fix" if the
misses are that class. `.fathom/observations.md` O39/O40 describe a real,
open defect (selector-method call edges disappearing, receiver names
duplicating in canonical targets) — but both are filed against a DIFFERENT
repo's corpus (`platform-engineering`/`review-kit`), root cause marked
`TODO`, and root-causing it for THIS corpus's 5 pure misses would mean
re-ingesting each miss pair with store access retained (this session
deleted every throwaway store immediately after scoring, by design, to
control disk) and inspecting whether the gold caller exists as an
unresolved/mis-canonicalized edge versus genuinely absent from the graph.
Not reached this session (see Deviations). Recommended next step, in
priority order the miss data itself supports (5/5 pure misses show ZERO
caller signal at any depth or tier, which is consistent with either a real
missing edge OR an O39/O40-class resolution bug — indistinguishable without
per-pair inspection).

**(c) Heuristic fallback when zero resolved callers — ALREADY TRUE, no
change needed.** The candidate assumed `ChangedSymbolReviews` might default
to resolved-only edges the way `find_callers`/`blast_radius` do (BP-26/
BP-27) and need an opt-in heuristic fallback. It does not: `CallersDetail`
walks `callgraph.BuildBoth(nodes, typed)` — ONE merged graph — and every
row it returns (resolved or heuristic) was already rendered, tier-labeled,
unconditionally. There is no gate to open. Confirmed by reading
`internal/analysis/review/github.go` before writing any code for this
mechanism; no commit.

**(d) Test-gap via typed test reach at depth 2 — ALREADY TRUE, no change
needed.** `impact.TestGap` → `callgraph.TestImpactDetail` is a full
reverse-reachability walk (BFS to "does ANY path exist", not a
shortest-hop-bounded one) — it was never depth-capped. The `caddyserver/
caddy` #5808→#7507 pair's two affected-test gold entries hit via this path
even in a from-scratch pre-mechanism run. No change needed; verified by
reading `internal/analysis/impact/testgap.go` and confirmed empirically (the
affected-test hit predates mechanism (a) in every run).

**(e) Sibling-file `test_gap_package` hints — NOT ATTEMPTED.** Deferred:
after (a)–(d), the corpus's remaining misses are the 5 pure misses (b) would
address, not a test-file-adjacency gap this mechanism targets. Revisit if
(b) is picked up and the miss set changes shape.

### Worked example — the one new hit

`golangci-lint` #5884 ("fix: typecheck memory leak") touched only
`pkg/goanalysis/runner_checker.go`. #5885 ("fix: stop the analysis after the
first package analysis error") fixed `analyze` in
`pkg/goanalysis/runner_loadingpackage.go` — a **different file**, never in
A's diff. Pre-mechanism (direct callers only), nothing in `runner_checker.go`
listed `analyze` as a caller — miss. Post-mechanism, the depth-2 walk finds
it: `analyze` (runner_loadingpackage.go) → calls → the changed function in
runner_checker.go, resolved tier. Exactly the "caller two hops away"
relationship the task's own mechanism-(a) description named.

### Per-PR table

Recall table across the 36/42 pairs this session ran through the
post-mechanism-(a) binary (structured `PairScore` data, not rendered text).
`own_symbol` = at least one gold entry recited from A's own changed files
(trivial, not scored as a strict hit on its own); `pure_miss` = zero
own_symbol/caller/affected_test matches of any kind.

| repo | pairs run | strict hits | own_symbol only | pure miss |
|---|---:|---:|---:|---:|
| caddyserver/caddy | 18/24 | 3 | 14 | 1 |
| golangci-lint | 18/18 | 3 | 11 | 4 |
| **total** | **36/42** | **6** | **25** | **5** |

The 6 unrun `caddyserver/caddy` pairs (#6608→#7545, #6682→#7694,
#6699→#6727, #7206→#7543, #7245→#7257, #7547→#7715) were confirmed misses
under the pre-mechanism binary in this session's own run (a from-scratch
23/24-pair pass, killed by the disk emergency below one pair short of
complete — see Deviations) but not re-verified against the post-mechanism
binary; mechanism (a) is strictly additive (BFS only adds rows, never
removes or reclassifies an existing depth-1 match), and only 1 of the 36
directly-verified pairs gained a new hit, so these are very likely still
misses, but this is not a proven fact — `reviewbench -repo
caddyserver/caddy` closes the gap in under 10 minutes on an unloaded host.

Hits (strict-new, gold entry / kind / depth):

| repo | A | B | gold | kind | depth |
|---|---|---|---|---|---|
| caddy | #1300 | #1309 | `createUpstreamRequest` (caddyhttp/proxy/proxy.go) | caller | 1 |
| caddy | #4398 | #4661 | `serversFromPairings`, `buildTLSApp` | caller | 1 |
| caddy | #5808 | #7507 | `TestGlobalOptions`, `TestMatcherSyntax` | affected_test **+** caller | (test path) / 2 |
| golangci-lint | #5929 | #5939 | `analyze` (pkg/goanalysis/runner.go) | caller | 1 |
| golangci-lint | #6445 | #6762 | `loadIssuesFromCache`, `saveIssuesToCache` | caller | 1 |
| golangci-lint | **#5884** | **#5885** | `analyze` (pkg/goanalysis/runner_loadingpackage.go) | **caller** | **2 — NEW, mechanism (a)** |

Pure misses (identical to the published reading's list):

| repo | A | B | note |
|---|---|---|---|
| caddy | #4443 | #4674 | `DetermineAdminAPIAddress` — confirmed (published reading + this session): nothing A changed calls it, at ANY depth. A real call-graph gap, not a depth-boundedness issue. |
| golangci-lint | #3995 | #4319 | no caller/test signal at any depth or tier |
| golangci-lint | #4709 | #4734 | gold set is empty in this run (B's patch carries no extractable `func` declaration — see Deviations) |
| golangci-lint | #5160 | #5168 | no caller/test signal at any depth or tier |
| golangci-lint | #5402 | #5407 | no caller/test signal at any depth or tier (the model arm's worst PR in the original reading, 19 false lines) |

### Recall/precision summary

| stage | n | strict hits | recall | false/PR |
|---|---:|---:|---:|---:|
| baseline (pre-mechanism, published) | 42 | 5 | 11.9% | 0 |
| after mechanism (a) | 36 directly verified + 6 inferred | 6 | 14.3% | 0 |

Both below the row's `>= 30%` floor by a wide margin — this loop moves the
number, it does not clear the bar. `docs/plans/verbs.md`'s M15/VB-24 row
stays `[ ]`.

### Reproduce

```sh
cd <fathom repo root>
go build -o <scratchpad>/reviewbench ./cmd/reviewbench
mkdir -p <scratchpad>/repos <scratchpad>/stores
<scratchpad>/reviewbench \
  -workdir <scratchpad> \
  -out <scratchpad>/results.json \
  -jsonl <scratchpad>/results.jsonl
# single repo (faster, reuses warm git/module caches on a repeat run):
<scratchpad>/reviewbench -workdir <scratchpad> -repo golangci/golangci-lint -jsonl <scratchpad>/golangci.jsonl
```

Requires network (git fetch against the real upstream repos named in the
corpus) and a Go toolchain on PATH; spends no LLM tokens, opens no daemon
connection. `-jsonl` streams one scored pair per line as it completes —
survives an interrupted run, which this session needed (see below).

### Deviations

- **Full 42-pair corpus not completed end-to-end against one binary.** The
  shared host this session ran on hit severe, RECURRING disk exhaustion
  (`/private/tmp` down to low-single-digit-percent free, twice reaching
  literally 0 free) caused by OTHER concurrent sessions on the same
  machine — confirmed, not assumed: killing this session's own reviewbench
  process and deleting its throwaway stores did not stop the free-space
  drop either time; a wave-9-scratch footprint under 250MB throughout is not
  consistent with being the cause. Recovered on its own each time as the
  other load finished. Consequence: the 42-pair corpus was assembled from
  four separate runs (a killed from-scratch pre-mechanism pass, 23/24 caddy
  pairs; two post-mechanism caddy passes, union 18/24 unique pairs; one
  complete post-mechanism golangci-lint pass, 18/18) rather than one clean
  end-to-end run — the STEP-1 brief's letter, not fully its spirit. The
  numbers above are honest about exactly which pairs are directly verified
  post-mechanism (36/42) versus inferred (6/42, all but certain, not
  proven).
- **`main.go`'s own store-cleanup bug, found and fixed mid-session**: a
  `defer os.Remove(storePath)` inside the per-pair loop only fires when the
  whole run's `main` returns, not per iteration — every pair's throwaway
  SQLite store accumulated for the full run instead of being freed
  immediately. Fixed (immediate `os.Remove` right after each pair, plus the
  `-jsonl` streaming output) before this contributed meaningfully to the
  disk emergency, but it was live during the first (killed) baseline run.
- **`golangci-lint` #4709→#4734's gold set is empty** (0 entries) in this
  run — B's patch carries no line this session's `ExtractGold` regex
  recognizes as a `func` declaration (possibly a comment/doc-only diff, not
  independently confirmed against GitHub in this session). A 0-gold pair
  contributes 0 to every count either way, so it does not change any
  reported number, but it is a discrepancy from the original reading's
  gold set for this pair worth flagging rather than silently absorbing.
- **Mechanisms (b) and (e)** were not attempted (open next steps, not
  ruled out) — see STEP 2 above for why and what would be needed.

### Reading path

`internal/reviewbench/` (corpus.go, gold.go, git.go, runner.go, score.go +
tests), `cmd/reviewbench/main.go`,
`internal/analysis/review/github.go` (`ChangedSymbolReviews`,
`changedSymbolCallerDepth`), `internal/analysis/review/github_test.go`,
`docs/usage.md` (`--format github` section), this reading, `docs/plans/
verbs.md` M15/VB-24 row, `.fathom/observations.md`.
