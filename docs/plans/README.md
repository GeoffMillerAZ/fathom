# docs/plans — contract-bearing plan files

Plan files here are Markdown checklists where `[x]` claims are gated at
pre-push and CI. The grammar is specified in
`docs/design/coding-agent-excellence.md` §4.2; parsing is
`internal/verify/markdown.go` (`ParseMarkdownPlan`).

## Checkbox semantics

| Checkbox | Meaning | Gated? |
|----------|---------|--------|
| `[ ]`    | Open — work not yet claimed done | Never (fails `--require-all`) |
| `[x]`    | Claimed done — MUST verify against the index | Always (--strict in CI) |
| `[~]`    | In progress — not done, not claimed | Never (fails `--require-all`) |
| `[-]`    | Skipped or abandoned — exempt; refine with `disposition:` | Never |

`fathom verify --require-all` is the **wave gate** (ADR-018 §2): it
additionally fails on any open/in-progress task, so a wave advances only when
everything is claimed-and-verified or explicitly skipped.

## Line format

```
- [ ] Title text [SPEC-ID] {symbol: Sym, package: dir/of/sym, reachable_from: Entry, min_tier: verified}
```

(The example is deliberately unchecked: the parser is line-based and does not
skip code fences, so a `[x]` example in any `docs/plans/*.md` would gate. The
hooks also exclude this README — plans claim, READMEs describe.)

- **Title**: human description of the work.
- **Spec IDs**: bracketed references (`[US-042]`) linked to spec documents.
- **Annotation block** `{…}`: the acceptance contract, comma-separated
  `key: value` pairs.

A line with no annotation and no spec IDs is `Unknown` (`no-contract`) —
reported honestly, never a false pass. Under `--strict` (CI), Unknown claimed
tasks block.

## Annotation keys

| Key | Meaning | Default |
|-----|---------|---------|
| `symbol` | Claimed symbol that must exist in the index | — (required for a symbol contract) |
| `package` | Repo-relative directory the symbol's file must live under. Disambiguates a bare name that exists in several packages: matches are filtered to that directory, and when exactly one canonical symbol remains, `test_reaches`/`edge_wired` query its canonical key instead of the bare name. If the filter empties the match set, `symbol_present` fails with `claimed-but-absent`. If the directory still contains several distinct symbols with that name, the name stays ambiguous and resolves to nothing — never first-match-wins. | unset (no filter) |
| `reachable_from` | An entrypoint that must statically reach `symbol` | unset |
| `min_tier` | Minimum evidence tier (`verified`, `pre-scan`, `doc-claimed`, `sme-stated`, `assumed`) | `verified` |
| `skip_test` | `true` exempts the symbol from the test-reaches requirement | `false` (a `Test*` must statically reach it) |
| `allow_contradiction` | `true` lets contradictions on the symbol pass | `false` |
| `id` | Author-owned stable task ID, overriding positional `md-NNN` (which renumbers when lines are inserted). Use in every gated plan that will be diffed across edits — the Stop hook matches tasks by ID. | positional |
| `disposition` | `refused` \| `deferred` — types WHY a `[-]` line is skipped; requires `reason` | unset |
| `reason` | Free-text justification for a disposition or skip. **Double-quote a value containing commas** (`reason: "kit-side done, donor NOT deleted"`); unquoted values must be comma-free. | unset |
| `trigger` | Revival condition for a `deferred` task — recorded and surfaced, never machine-evaluated in v0 | unset |
| `skip` | Justification for `skip_test: true` (machine-visible; linted when absent) | unset |
| `oracle` | `lexical` — a non-code row's contract (ADR-030): `path` must exist in the working tree `fathom verify` reads the plan from (never `git show HEAD:`), be non-empty, and match `pattern` at least once. `ledger` — a published-bench-reading contract (ADR-034 decision 2, RC-34): see `run_id`/`task`/`arm`/`min_trials` below. Either class verifies and leaves U2 — it IS a contract the gate can refute. **One contract per row**: `symbol` and `oracle` together are refused by lint and fail verification. A `skip:` reason never lifts U2 — only a symbol or an oracle does. | unset |
| `path` | Repo-relative file the oracle reads (the lexical file, or a committed JSONL bench ledger for `oracle: ledger`). Never absolute, never `..`. | unset |
| `pattern` | Go regexp (line-anchored: `^`/`$` match at line bounds) the lexical oracle must find in `path`. **Required when `path` is `.md`** — a document that merely exists proves nothing about a claim. Double-quote it (regexps carry commas and braces). | unset |
| `run_id` | `oracle: ledger` only: the run whose row to check in `path`. Required. | unset |
| `task` | `oracle: ledger` only: the task id half of the (run_id, task, arm) key. Leave unset (with `arm`) to match a row keyed by `run_id` alone instead — the home bench's one-row-per-run ledger (`docs/bench/ledger.jsonl`), whose own rows carry no task/arm either. | unset |
| `arm` | `oracle: ledger` only: the arm half of the (run_id, task, arm) key. Setting `arm` without `task` is refused by lint. | unset |
| `min_trials` | `oracle: ledger` only: the trial floor the matched row's `trials` (or, for a home-bench row, `treatment_trials`) must meet or exceed. | `1` |
| `needs` | Comma-separated row ids this row waits on (eligibility, never gating). `<sibling>.md#<id>` names a row in another plan under the same directory: lint checks the shape, `fathom verify` and `plans next` read the sibling's authored state (claimed or dispositioned satisfies; its verdict is not recomputed). | unset |

> **Before reaching for `skip_test`:** a test statically reaches every function the
> code it calls references as a *value* -- an HTTP handler registered through a mux,
> a route-table literal, `http.HandlerFunc(h)`, middleware, or a method value is
> reached by an httptest that builds the server (VS-6), and calls inside `t.Run`,
> `defer`, or goroutine closures count for the enclosing test (VS-7) -- the old
> "call it at the test's top level" rule is retired. A method name shared by two
> receivers needs `package:`; an edge into a file the connector never discovered is
> named by ingest as a discovery failure, never a clean 0 (VS-9). `skip_test` is for
> rows with no test at all, never for "the wiring is indirect".

The zero value of every key is the **strictest** contract; loosen explicitly.
`verify.LintPlan` reports v2 convention violations (duplicate IDs, dispositions
without reasons, unjustified skips) as **warnings** on stderr — they never
change exit codes in v0.

## Annotation placement (load-bearing)

The annotation block must sit **on the checkbox line itself**. The parser is
line-based: an annotation on a continuation (wrapped) line is silently
ignored, and the task verifies as `no-contract` instead of carrying its
intended contract. Keep the checkbox, title, spec IDs, and `{…}` block on one
line, however long.

## Honesty boundary

`test_reaches` means a `Test*` function **statically reaches** the symbol via
the call graph — never that the test passes or even runs. `runtime_pass` is
always Unknown until CI/coverage evidence is ingested as its own tier.

## Gate commands

```sh
# pre-push (fail-open on exit 2):
bin/fathom verify --service <svc> --plan docs/plans/<slug>.md

# CI (fail-closed, strict):
bin/fathom verify --strict --store <tmpstore> --service <svc> --plan docs/plans/<slug>.md
```

## Computed progress and archival (E3/E4, docs/plans/new-use-cases.md)

`fathom plans status` computes a per-plan rollup {proven, claimed-unproven,
open, deferred-with-trigger} by running the verify engine over every active
plan — never an asserted state. `fathom plans next` lists the ordered work
frontier (open rows plus claimed-but-failing rows) with their `{id, symbol,
package}` anchors, for an agent picking up work.

`fathom plans archive <plan>` moves a plan whose every row is proven or
dispositioned, and whose row IDs are lexically absorbed into CHANGELOG.md,
ADR.md, or the per-release narrative under `docs/changelog/*.md`, to
`docs/plans/archive/` — writing a tombstone (date, HEAD sha, verify
summary) into the archived file's header. `fathom plans restore <plan>` moves
it back. **`docs/plans/archive/` is out of gate scope by construction**: both
the pre-push `verify` command (`lefthook.yml`) and CI (`scripts/ci.sh`) walk
only the top level of `docs/plans/` (a non-recursive glob / `find -maxdepth
1`), so every archived plan removes exactly one `fathom verify` invocation
from each push and CI run — the gate's cost is O(active plans), not
O(all-plans-forever).

## Recorded demo (2026-07-07, first gated run)

The very first run of the gate against `h1-coding-agent-excellence.md` caught
the plan's author (an AI agent) over-claiming — the exact failure mode the gate
exists for:

```
✗ [md-001] Adoption instrument repaired: … {symbol: session_stats, skip_test: true}
    symbol_present: "session_stats" not found in the index
…
15 tasks total; 9 claimed: 6 verified, 3 failing        → exit 1, push blocked
```

`session_stats` was a Python function inside a bash heredoc — not an
index-provable symbol; the claim was rewritten honestly (non-code deliverables
are recorded in CHANGELOG, not claimed in gated plans). Two further failures
exposed a real recall gap (`test_reaches` misses method-call edges —
`docs/dogfood-observations.md` O6). After correcting the claims:
`8 claimed: 8 verified, 0 failing → exit 0`.

Lesson: contracts must name index-provable symbols; the gate is only as strong
as the honesty of its `skip_test` escapes, so every `skip_test` should cite why
(O6 tracks removing the method-edge class).
