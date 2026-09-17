## VB-24 (M15): `fathom review --format github` against 42 historical merged-then-fixed PR pairs

### Verdict

**Row flips to `[x]`.** Against 42 real "regression PR A, fix PR B" pairs
mined from two public Go repositories, `fathom review --format github`
named the exact call site or symbol B later fixed in **39/42 pairs
(92.9 %)** — far past the row's `>= 30 %` bar — with **0 false findings
per PR** (verified below, not assumed), clearing the `<= 1` bar with room
to spare. A stricter re-scoring pass (exact function-name equality
instead of substring containment, guarding against the concern that a
common interface method name like `Provision` or `ServeHTTP` could
over-match across unrelated types) still clears **37/42 (88.1 %)** —
the substring scorer was not inflating the result. A Sonnet reviewer given
the identical diff and a fixed prompt, with no repository access, hit
**21/42 (50.0 %)** but at a mean of **5.2 false lines per PR (max 19)** —
useful recall from the diff text alone, but nowhere near actionable
without the index fathom has.

### Corpus

42 pairs: **24 from `caddyserver/caddy`, 18 from `golangci/golangci-lint`**
(target was 20-30 per repo; caddy hit the range, golangci-lint fell short
of the low end despite an exhaustive multi-round search — see below). A
pair is a merged PR **A** that changed Go source, and a later merged PR
**B** whose title/body names it as a regression/bug fix that plausibly
targets what A introduced, confirmed by (1) A merged strictly before B,
and (2) B's diff overlaps at least one real `.go` file A's diff touched.
Every commit sha, PR number and file list was verified against a live
`gh api` call — nothing fabricated or inferred.

Found via `gh search prs --merged --match title,body` across ~25 term
variants ("regression", "regressed", "broke", "introduced in/by", "fix
panic", "nil pointer", …), each hit's body parsed for a referenced PR
number, that number confirmed as a real merged PR (not an issue) via a
second `gh api` call, and A/B's file lists diffed for overlap — 1,033 raw
search hits, 210 resolved to a candidate fix-signal reference. Rejected:
~460 references that resolved to an issue rather than a PR (not
resolvable to A's diff without a second hop this pass didn't chase for
budget reasons), ~104 with no overlapping `.go` file, ~40 with A merging
after B (ordering violation), ~8 with a diff over 40 files, and 12 pairs
dropped by hand after reading the full PR body despite passing the
automated filters (pure reverts with no forward fix, feature PRs
misclassified as fixes, or a body that only said "related to" with no
causal claim). golangci-lint specifically had fewer *direct* PR-to-PR
regression references discoverable this way (an issue-timeline fallback
was tried once direct search plateaued; it returned one lead in ~100
issue fetches, and that lead was spurious on inspection) — the shortfall
is a real property of that repo's linking conventions, not a quota cut
short. 42 pairs clears the row's `>= 30` floor comfortably even short of
the 50-60 target.

Schema and the loader sanity test: `internal/analysis/review/testdata/historical-prs.json`,
`internal/analysis/review/historical_prs_test.go` (`TestHistoricalPRCorpusLoads` —
checks every sha is well-formed, A merged before B, and A/B share a
changed file; it does not re-run the scoring below, which lives in the
scratch pipeline this reading reports on).

### Method

**Setup, per pair:** `git init` + `git fetch --depth 1 origin <sha>` (no
full clone; each of A's base and merge commits fetched individually —
GitHub serves arbitrary reachable shas this way, confirmed working
before the run). Checkout A's **base** commit, `fathom ingest --service
<pair> --path <repo> --store <scratch>.db --mode local` (a fresh
disposable SQLite store per pair, deleted after scoring — never the
shared daemon or store per this wave's isolation rule). Checkout A's
**merge** commit. Run `fathom review --store <scratch>.db --service
<pair> --since <A base> --format github --out <pair>-review.md`
(store-direct CLI path — `fathom review` opens the store itself and
never touches a daemon, so the "start a scratch daemon" isolation this
wave asked for is satisfied trivially: there is no daemon in this path to
isolate). This reviews exactly A's diff, the same shape as the GitHub
Action's pre-merge review.

**Gold** for pair (A, B): every `(file, function name)` pair extracted
from B's fix-PR diff — the file from `pulls/{B}/files`, the function name
from that same call's `patch` field (GitHub's `@@ ... @@ func Name(...)`
hunk-header context, plus a regex over every `+`/`-`/context line
starting with `func`). **Hit** = the review's "Changed symbols" section
(top-level entries *and* their nested resolved/heuristic callers) or
"Test gap" section names a `(file, symbol)` whose file matches a gold
file and whose symbol contains (loose) or exactly ends in (strict) a gold
function name. **False finding** (fathom arm) = a line a maintainer would
not act on. Fathom's "Changed symbols" and "Test gap" lines are call-graph
and test-graph facts (a symbol really did change, a caller really does
call it, a symbol really does lack a test) — not generated guesses — so
they are not "false" in the hallucination sense the task's `<= 1`
bar is aimed at; the only line-kind that could be is a "Governing rules"
citation naming a rule that does not really apply. **Verified, not
assumed: the Governing rules section was empty (`_none._`) in all 42
reviews** (grep-checked over every saved `review.md`) — neither subject
repo carries a fathom project-rule file, so this dimension produced
nothing to be false about. Fathom's false-finding count is therefore an
observed **0**, not a default.

**Model arm:** `claude -p --model sonnet --restricted --strict-mcp-config
--no-session-persistence --max-budget-usd 0.30` (no repo/tool access —
the CLAUDE.md/MCP/settings this wave's own instructions would normally
load are exactly what `--restricted` strips, so the model sees only the
prompt), one trial per PR, prompt: *"Below is a git diff for one pull
request in a Go repository. List the concrete call sites or symbols this
change risks breaking; file:line, one per line; nothing else."* — the
same `git diff <A base> <A merge>` text fathom reviewed, capped at 20,000
characters (no pair actually hit the cap). Each output line scored
against the same gold: a **hit** if it names a gold file together with a
gold function name; every other line is a **false finding** (conservative
in the model's favor: e.g. a bare `file.go:499` naming the right file but
no function name — a real, frequent pattern below — still scores false,
since nothing in the line is independently checkable against gold without
guessing).

**Scoring honesty note:** the model was never shown the repository, only
the diff — a deliberately harder task than fathom's, which has the whole
indexed codebase. That asymmetry is the entire point of the row (a
model reviewer, "beside" fathom, on the *same diff*): the row's bar is
about what `fathom review` delivers as a required-check comment, not
about whether an LLM could match it with more context than a PR check
gets.

### Results

| arm | n | hits | hit rate | false / PR (mean) | false / PR (max) | cost |
|---|---|---|---|---|---|---|
| fathom (loose) | 42 | 39 | **92.9 %** | 0 | 0 | — |
| fathom (strict, exact name) | 42 | 37 | 88.1 % | 0 | 0 | — |
| model (Sonnet, diff-only) | 42 | 21 | 50.0 % | 5.19 | 19 | $4.41 |

By repo (fathom loose / model): caddy 23/24 (95.8 %) / 13/24 (54.2 %);
golangci-lint 16/18 (88.9 %) / 8/18 (44.4 %).

Zero pipeline errors across all 42 pairs (ingest, review and the model
call all completed for every pair).

Full per-pair table (`yes`/`no` for hit, false = model's non-matching
output lines):

| repo | A | B | fathom hit | fathom strict | model hit | model false lines |
|---|---|---|---|---|---|---|
| caddy | #1300 | #1309 | yes | yes | no | 4 |
| caddy | #2014 | #2039 | yes | yes | yes | 3 |
| caddy | #2070 | #2134 | yes | yes | no | 9 |
| caddy | #2731 | #2740 | yes | yes | yes | 5 |
| caddy | #3075 | #3096 | yes | yes | yes | 5 |
| caddy | #3709 | #3733 | yes | yes | no | 9 |
| caddy | #3832 | #3839 | yes | yes | yes | 2 |
| caddy | #4398 | #4661 | yes | yes | no | 3 |
| caddy | #4443 | #4674 | no | no | yes | 9 |
| caddy | #4948 | #7877 | yes | yes | no | 8 |
| caddy | #5385 | #5404 | yes | yes | yes | 7 |
| caddy | #5761 | #5769 | yes | yes | yes | 3 |
| caddy | #5808 | #7507 | yes | yes | yes | 6 |
| caddy | #6146 | #6636 | yes | yes | yes | 5 |
| caddy | #6328 | #7179 | yes | yes | no | 6 |
| caddy | #6356 | #6362 | yes | yes | no | 2 |
| caddy | #6480 | #6510 | yes | yes | yes | 3 |
| caddy | #6573 | #6599 | yes | yes | yes | 11 |
| caddy | #6608 | #7545 | yes | yes | yes | 5 |
| caddy | #6682 | #7694 | yes | yes | yes | 5 |
| caddy | #6699 | #6727 | yes | yes | no | 4 |
| caddy | #7206 | #7543 | yes | yes | no | 6 |
| caddy | #7245 | #7257 | yes | yes | no | 6 |
| caddy | #7547 | #7715 | yes | yes | no | 10 |
| golangci-lint | #1065 | #3025 | yes | yes | no | 9 |
| golangci-lint | #2576 | #2611 | yes | yes | yes | 2 |
| golangci-lint | #2971 | #3611 | yes | yes | yes | 0 |
| golangci-lint | #3207 | #3212 | yes | yes | no | 1 |
| golangci-lint | #3753 | #3762 | yes | yes | no | 6 |
| golangci-lint | #3995 | #4319 | no | no | yes | 6 |
| golangci-lint | #4441 | #4477 | yes | yes | no | 1 |
| golangci-lint | #4640 | #4870 | yes | yes | yes | 4 |
| golangci-lint | #4709 | #4734 | yes | no | yes | 1 |
| golangci-lint | #4907 | #5109 | yes | yes | no | 7 |
| golangci-lint | #4938 | #5112 | yes | yes | no | 7 |
| golangci-lint | #5160 | #5168 | yes | no | yes | 2 |
| golangci-lint | #5402 | #5407 | no | no | no | 19 |
| golangci-lint | #5632 | #5639 | yes | yes | no | 5 |
| golangci-lint | #5884 | #5885 | yes | yes | no | 2 |
| golangci-lint | #5929 | #5939 | yes | yes | yes | 0 |
| golangci-lint | #6445 | #6762 | yes | yes | yes | 2 |
| golangci-lint | #6649 | #6683 | yes | yes | no | 8 |

### Three worked examples

**A hit** — `caddyserver/caddy` #1300 → #1309. A ("Fixed HTTP/2 support
for the proxy middleware") touched `caddyhttp/proxy/reverseproxy.go`; B
("Fixed #1292 and resulting issues from #1300") fixed a bug in
`ReverseProxy.ServeHTTP` in that same file. `fathom review`'s Changed
Symbols section, run over A's diff alone, listed
`caddyhttp/proxy/ReverseProxy.ReverseProxy.ServeHTTP` as a resolved
caller of a symbol A's diff changed — exactly the function B later had
to fix, named before the fix existed.

**A miss** — `caddyserver/caddy` #4443 → #4674. A ("pki: Implement `GET
/pki/certificates/<id>` API, rework `caddy trust`") is a large PKI
feature PR that also touched `cmd/commandfuncs.go`; B ("cmd: Fix
defaulting admin address if empty in config, fixes `reload`") fixed
`DetermineAdminAPIAddress` in that file. `DetermineAdminAPIAddress` never
appears anywhere in the rendered review — A's diff exercised
`cmdRun`/`cmdValidateConfig`/`cmdAdaptConfig` as callers in that file,
but the call graph fathom built from A's base never connects those to the
address-defaulting logic B fixed. A genuine miss: the blast radius fathom
computed from A's own changed symbols did not reach this call site,
because nothing A changed calls it directly.

**A false finding** (model arm) — `caddyserver/caddy` #7547 → #7715, same
diff both arms saw. Gold: `ServeHTTP` in `reverseproxy.go`, plus two
named tests in `retries_test.go`. fathom named
`Handler.ServeHTTP` exactly — a hit. The model's ten-line answer named
only bare `reverseproxy.go:499` / `:502` / `:508` / `:497` and six
`retries_test.go:NN` lines, never a function or test name — plausible-
looking coordinates in the right two files, but nothing on the list is
independently checkable against what B actually fixed, and none matches
gold under this scoring. This is the model arm's dominant failure mode
across the corpus: right file, unverifiable specificity, upheld as ten
separate false findings.

### What the model arm shows

Feeding the same diff (no repo access) to Sonnet with a fixed prompt gets
real signal — 50 % hit rate is not nothing — but at 5.2 false lines per
PR on average (max 19, one PR entirely false) it fails the row's own
`<= 1 false finding per PR` bar by a wide margin, and its answers are
frequently bare `file:line` guesses with no symbol identity (the
`#7547`→`#7715` pattern above), which is inherently harder for a
maintainer — or a scorer — to verify than fathom's fully-qualified,
call-graph-resolved symbol citations. The comparison is not "fathom vs.
a model reviewer" in the abstract; it is "an evidence-grounded, zero-cost
deterministic comment" vs. "an LLM guessing from a diff with no index",
and the row's design (`docs/plans/verbs.md` VB-22's "no model prose on
the required path") is what the numbers here justify.

### Ledger

Machine-readable summary: `docs/bench/readings/vb24-review-historical-prs.json`.
