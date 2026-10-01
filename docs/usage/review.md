### `review` — the change-review verb

```sh
fathom review --service <id> (--since <git-ref> | --manifest FILE) [--store DB] [--mode report|persist] [--no-persist] [--idempotency-key K] [--expect-index-watermark W] [--budget-seconds N] [--call-log P] [--json] [--format text|github] [--out FILE]
```

CLI mirror of the `review_diff` MCP tool: diff impact + obligations/findings
+ architecture conformance + spec coverage in one cited verdict, followed by
the test-impact block (BD-5, `docs/plans/blast-depth.md`) — the `Test*`
functions that statically reach a changed symbol (`run …`, cited file:line,
with the symbols each reaches) and the changed symbols no test reaches
(`gap …`):

```
[x] test_impact: 2 affected test(s), 1 changed symbol(s) with no test
  - run svc::pkg.TestFoo (pkg/a_test.go#L1-L1) reaches svc::pkg.A, svc::pkg.B
  - gap svc::pkg.Orphan (pkg/orphan.go#L1-L1): no test reaches it
```

`--json` carries the same block as `test_impact` (`{checked, reason?,
affected_tests, test_gap}`), identical to `review_diff`'s, folded into a
versioned `ReviewResponse` envelope (FL-16, `docs/plans/findings-loop.md`):
`schema_version`, `producer{name,version,build}`, `inputs{manifest_digest,
source_watermark, index_watermark, rules_digest, plans_digest,
extractor_versions}`, the unchanged `dimensions[]`/`summary`/
`findings_persisted`, `coverage{complete, unknown}` (false the moment any
dimension is unchecked, or the run is truncated/stale/cancelled),
`truncated`/`stale_index`/`cancelled`, and `unanchored_paths` — a changed
path the index carries no symbol for (a doc, a workflow file) is accepted
as a valid process subject and listed here rather than given an invented
anchor.

Exactly one of `--since <ref>` or `--manifest FILE` (a JSON
`[{"path","sha256"}]` array) selects the changed-file set; with `--manifest`
every path's CURRENT content must hash to its declared sha256 or the run
refuses with `manifest mismatch: <path>` and exit 2 — never a silent
re-read.

`--since` (ADR-044 §1.6, CV-13) additionally resolves `<ref>` to a commit
sha and carries it, plus the service's own corpus root, on the review
engine's internal Input (`Base`/`Root`). This feeds the **`integrity`**
dimension (ADR-044 §1, CV-12): without `--since`, `[ ] integrity` prints
with reason `needs a base ref`; with it, `fathom review` runs
internal/conserve's `integrity` plane over base..head and reports any
guarded-test loss (a certifying or invariant-enforcing test that was
deleted, renamed, weakened, newly skipped, had its referenced golden
fixture change, or lost/moved its `// certifies:` marker) or a hand edit
to a generated file whose generator inputs did not change. The
**`conservation`** dimension (ADR-044 §2, CV-24) reads the same
`Base`/`Root` and reports the same "needs a base ref" reason without
`--since`; with it, `fathom review` (and `review_diff`) build a real
`internal/conserve.Input` via `conserve.NewInput` and run it through
`conserve.Run` over every registered plane (`conserve.Planes()`) — the
same composition `fathom conserve` itself uses — so `[x] conservation`
(`checked: true`) reports the same planes/findings `fathom conserve
--since <ref>` would over the identical range, with no separate command
needed. A ref that does not resolve fails open (every
dimension still runs over the changed-file set `--since` already
computed); `--manifest` runs never populate `Base` at all, since they
carry no ref.

`--mode persist` (the default) writes the deterministic-oracle findings to
the claims store; `--mode report` (`--no-persist` is an alias) never
writes anything — pinned by a store-state equality test including the
error path. `--idempotency-key K` (persist mode only) records the response
in an additive `review_runs` table: a repeated key over the SAME request
(service + mode + changed-file set) replays the recorded response and
writes nothing else; the same key over a DIFFERENT request refuses with
`idempotency key reused with a different payload` and exit 2.
`--expect-index-watermark W` compares W against the index's own watermark;
a mismatch sets `stale_index: true` (every dimension still runs — a report,
never a hard refusal) and always refuses persist regardless of `--mode`.
`--budget-seconds` is advisory — on excess it prints a named skip and exits
0; it requires `--mode report`/`--no-persist` (a bounded run must never be
a store writer).

`--format github` (VB-22, `docs/plans/verbs.md`) renders ONE deterministic
Markdown PR comment instead of the human summary or `--json`: changed
symbols with their callers out to depth 2 (review-recall-loop mechanism
(a), `docs/bench/readings/review-recall-loop-2026-09-18.md`) and each
caller's resolution tier plus depth (`[resolved, depth 1]`/`[heuristic,
depth 2]`), call-site completeness (DIRECT callers found vs. how many live
in this same diff — never inflated by a depth-2 row), governing rules with
cited lines, a
`### Verify` line naming `fathom verify --service … --plan …` whenever the
diff touches a `docs/plans/*.md` row's anchored symbol, affected tests and
the test gap, and a coverage block when a profile was supplied — every
finding line carries a stable claim id, and NO model prose appears anywhere
on this path (`internal/analysis/review/github.go`'s `renderGitHubReview`).
Opens with the fixed marker `<!-- fathom-review -->` so a caller can find
and update its own prior comment. `--out FILE` writes the report (any
format) to `FILE` instead of stdout — stdout then carries only a one-line
confirmation — for a CI step that captures the comment body and hands it to
a separate upsert step; see `docs/adoption/github-action.md` (VB-23) for the
full GitHub Action and the required-check wiring.
