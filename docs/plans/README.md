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
| `reachable_from` | An entrypoint that must statically reach `symbol`. Reachability is a call-graph edge, so point `symbol` at a **function the entrypoint calls**, never at a type: a same-package bare composite literal (`Decide{...}` inside `package config`) produces neither a typed nor a heuristic edge, and no test in that package can ever satisfy an `edge_wired` claim against the type itself (only a cross-package qualified reference such as `config.Decide{...}` does). The "config parses" claim therefore binds to `Load` (config-experience CF-1, decision-port DP-1), not to the struct. A method whose bare name collides with a type in the same package (`Graph` the method on `CachedCallGraph` versus `Graph` the struct) must be written qualified, `CachedCallGraph.Graph`, or the bare name resolves to the type and nothing reaches it (daemon-footprint DF-6). And a regression row never names its own test as the symbol with a `skip_test`: that is the self-skip the gate refuses (O56); it names the function under test, reached from the test. | unset |
| `min_tier` | Minimum evidence tier (`verified`, `pre-scan`, `doc-claimed`, `sme-stated`, `assumed`) | `verified` |
| `skip_test` | `true` exempts the symbol from the test-reaches requirement. **Never free (O56): requires a `skip:` reason** — a bare `skip_test: true` is a grammar error, reported by `fathom verify` and `verify_plan` as `skip-without-reason`, and counted unproven exactly like an absent contract. See "Self-skip" below for the second half of the check. | `false` (a `Test*` must statically reach it) |
| `allow_contradiction` | `true` lets contradictions on the symbol pass | `false` |
| `library` | `true` declares that the row's deliverable is the API itself, so its `symbol` need not be reached from a product entry point (see "Product wiring" below). **Never free:** it needs the row's `reason:` naming why the API is the deliverable — a split's library half names its open wiring row, a gate harness names the gate that runs it — and a `library: true` added by the same edit or commit that claims the row waives nothing (the self-skip check, applied to this waiver too). Lint names a bare one and one on a row with no `symbol`. | `false` (a product entry point must reach the symbol) |
| `id` | Author-owned stable task ID, overriding positional `md-NNN` (which renumbers when lines are inserted). Use in every gated plan that will be diffed across edits — the Stop hook matches tasks by ID. | positional |
| `disposition` | `refused` \| `deferred` — types WHY a `[-]` line is skipped; requires `reason` | unset |
| `reason` | Free-text justification for a disposition or skip. **Double-quote a value containing commas** (`reason: "kit-side done, donor NOT deleted"`); unquoted values must be comma-free. | unset |
| `trigger` | Revival condition for a `deferred` task — recorded and surfaced, never machine-evaluated in v0 | unset |
| `skip` | Justification for `skip_test: true` (machine-visible; **required, not merely linted — see O56 above**) | unset |
| `oracle` | `lexical` — a non-code row's contract (ADR-030): `path` must exist in the working tree `fathom verify` reads the plan from (never `git show HEAD:`), be non-empty, and match `pattern` at least once. `ledger` — a published-bench-reading contract (ADR-034 decision 2, RC-34): see `run_id`/`task`/`arm`/`min_trials` below. Either class verifies and leaves U2 — it IS a contract the gate can refute. **One contract per row**: `symbol` and `oracle` together are refused by lint and fail verification. A `skip:` reason never lifts U2 — only a symbol or an oracle does. | unset |
| `path` | Repo-relative file the oracle reads (the lexical file, or a committed JSONL bench ledger for `oracle: ledger`). Never absolute, never `..`. | unset |
| `pattern` | Go regexp (line-anchored: `^`/`$` match at line bounds) the lexical oracle must find in `path`. **Required when `path` is `.md`** — a document that merely exists proves nothing about a claim. Double-quote it (regexps carry commas and braces). | unset |
| `run_id` | `oracle: ledger` only: the run whose row to check in `path`. Required. | unset |
| `task` | `oracle: ledger` only: the task id half of the (run_id, task, arm) key. Leave unset (with `arm`) to match a row keyed by `run_id` alone instead — the home bench's one-row-per-run ledger (`docs/bench/ledger.jsonl`), whose own rows carry no task/arm either. | unset |
| `arm` | `oracle: ledger` only: the arm half of the (run_id, task, arm) key. Setting `arm` without `task` is refused by lint. | unset |
| `min_trials` | `oracle: ledger` only: the trial floor the matched row's own per-cell trial count must meet or exceed — a scenario row's `trials`, or a home row's own `min_trials` (never its run-total `treatment_trials`, which inflates with task count and proves nothing about replication). | `1` |
| `criterion` | `oracle: ledger` only, task/arm-less rows: disambiguates a `run_id` that matches more than one home row (the same run scored under two registered criteria) by naming the one row's `evaluation.criterion_id`. Refused as ambiguous, naming the criteria present, when omitted and more than one row matches. | unset |
| `accept_verdict` | `oracle: ledger` only: `any` certifies the claim even though the matched row's own `evaluation.verdict` is `fail` or `indeterminate` — otherwise such a row is refused, naming the reason. | unset (refuses a fail/indeterminate row) |
| `needs` | Comma-separated row ids this row waits on (eligibility, never gating). Only a claimed row satisfies (in the same plan, claimed and verified). **Deferred is not done:** a `[-]` target keeps the row blocked — a deferred one until it revives, and a refused one (or any disposition but deferred) names the row as needing re-planning in `plans next`, never silently eligible (G2-6). `<sibling>.md#<id>` names a row in another plan under the same directory: lint checks the shape, `fathom verify` and `plans next` read the sibling's authored state by the same rule (its verdict is not recomputed). | unset |
| `certified_by` | Test name(s) whose compiled certifications-layer claim (vision 33 / ADR-043 stage 1, `docs/design/test-certifications.md` §4) must be `live` and scoped to the row's `symbol` for the row to verify. Repeatable via a comma-separated value, same convention as `needs` — **double-quote a multi-test value** (`certified_by: "TestA,TestB"`); a single test needs no quoting. When set, it REPLACES the ordinary `test_reaches`/`skip_test` requirement: only `certified` proves the row — `certificate-unproven`, `certificate-broken`, `certificate-unreachable`, and `certificate-absent` never do (`internal/verify.EvaluateCertifiedBy`). | unset |
| `strength` | `certified_by` only: the minimum certification strength (design doc §1's closed, ordered ladder: `smoke` < `asserts` < `golden` < `property` < `conformance` < `mutation-killed`) every named test's certificate must meet or exceed. | `asserts` |
| `max` | `oracle: lexical` only (CV-29, ADR-044 §3.1): the pattern's match count must be at most this many. Refused combined with `eq`. Also evaluated (CV-27/CV-28, `internal/verify.EvaluateGone`) off a `gone: pattern` row as its ratchet ceiling — the SAME bound, against the pattern's total match count across every scanned file, not a single file's count; `fathom refactor status` (a later row) additionally compares the live count against the baseline for drift reporting. | unset (no ceiling; a lexical row with no bound at all keeps the original "at least one match" default; a `gone: pattern` row with no bound at all instead defaults to "exactly zero," since it proves absence) |
| `min` | `oracle: lexical` only: the pattern's match count must be at least this many. Refused combined with `eq`. May combine with `max` as a range. Also evaluated on a `gone: pattern` row, same as `max`. | unset |
| `eq` | `oracle: lexical` only: the pattern's match count must be exactly this many — `eq: 0` proves absence (a `gone: pattern` row's own use case). Refused combined with `max` or `min` — one bound shape per row. Also evaluated on a `gone: pattern` row, same as `max`/`min`. | unset (falls back to `min`/`max`, or the original "at least one match" default when neither is set either; a `gone: pattern` row with none of the three defaults to `eq: 0`) |
| `gone` | ADR-044 §3.1, CV-27/CV-28 (`internal/verify.EvaluateGone`, wired into `fathom verify`'s own row evaluation): an absence contract — `import <importpath>` (no Go file at head imports it), `module <path>` (`go.mod` at head has no `require` for it), `pattern "<regex>"` (with optional `paths:`; zero lexical matches across the scanned corpus, `max`/`min`/`eq` bound the count same as the lexical oracle), or `callers <Symbol>` (with `package:`; the symbol is kept but has zero referents at typed tier, test referents counted separately unless `tests: allowed`). One `gone:` per row; never combined with `symbol`/`oracle`. Proven only when the index is at HEAD, the typed pass has zero failed packages in scope, and no build-tag-excluded file references the target — otherwise `Unknown`, never falsely proven (I44). Unproven (honestly) on any `Evidence` that does not implement `internal/verify.GoneEvidence`; `fathom verify --store`'s CLI adapter (`cliEvidence`, cmd/fathom) implements it — a stale ingest or a dirty working tree still reads `Unknown`, never a fabricated pass. | unset |
| `tests` | `gone: callers` only: `allowed` permits nonzero TEST referents to the retired symbol; any other value (typically unset) requires zero test referents too, same as ordinary (non-test) callers. | unset |
| `paths` | `gone: pattern` only (CV-27, design §3.1's "with optional `paths:`"): repo-relative paths (an exact file, or a `"dir/"` prefix matching every file under it) scoping the row's regexp scan — repeatable via the same comma-separated-VALUE convention `retires`/`surface_change` use. Refused on any other `gone:` form. | unset (scans the whole corpus) |
| `retires` | Capability-inventory key(s) (ADR-044 §2.1) this row declares INTENTIONALLY REMOVED — exact match or a `"<prefix>*"` wildcard, repeatable via the same comma-separated-VALUE convention `needs`/`certified_by` use (quote a multi-key value). Read by `internal/conserve.Declarations`: present here at a change's BASE excuses the matching loss ("prior"); added only at head is self-declared (I45) and excuses nothing. | unset |
| `surface_change` | Capability-inventory key(s) this row declares INTENTIONALLY CHANGED (not removed) — same shape and the same `Declarations` matching rule as `retires`. | unset |
| `weight` | The row's share of a weighted progress rollup (ADR-049, `docs/design/spec-interop.md` §4): a positive integer. Never gating. Lint refuses zero, a negative, a fraction or a word, and the row then counts as weight 1. | `1` |
| `accept` | Acceptance text, EARS-shaped where the author likes (`accept: "WHEN <trigger> THE SYSTEM SHALL <response>"`) — double-quote it. Never gating by itself: a row whose only contract-shaped key is `accept` still verifies `no-contract`; it is proven only through `certified_by`. | unset |

A plan-level `Baseline: <sha>` header line (its own line, not inside any row's `{…}` block — conventionally near the top of the file) pins the base commit every row in the plan is proved against (ADR-044 §4): `fathom refactor plan` writes it, `fathom refactor status` and `fathom conserve` (later rows) read it as `Plan.Baseline`.

A plan-level `Proof: certified` header line (same shape as `Baseline:`, its own line, case-insensitive on the `Proof:` label) raises the bar for every `symbol:` row in the file (ADR-049, `docs/design/spec-interop.md` §4): each one must also carry `certified_by:`, or is refused `certificate-absent` — through the same `EvaluateCertifiedBy` path an ordinary `certified_by:` row already goes through, with **no `skip_test` escape** (the header is the strongest gate; it cannot be waived row by row). Unset (the default) is today's tiers, unchanged.

> **Before reaching for `skip_test`:** a test statically reaches every function the
> code it calls references as a *value* -- an HTTP handler registered through a mux,
> a route-table literal, `http.HandlerFunc(h)`, middleware, or a method value is
> reached by an httptest that builds the server (VS-6), and calls inside `t.Run`,
> `defer`, or goroutine closures count for the enclosing test (VS-7) -- the old
> "call it at the test's top level" rule is retired. A method name shared by two
> receivers needs `package:`; an edge into a file the connector never discovered is
> named by ingest as a discovery failure, never a clean 0 (VS-9). `skip_test` is for
> rows with no test at all, never for "the wiring is indirect".

### Self-skip (O56) — an agent may not grade its own row

A `skip_test` escape is gated TWICE, and both halves are gate failures, not
lint warnings:

1. **skip-without-reason.** `skip_test: true` with no `skip:` value is a
   grammar error. `fathom verify` and `verify_plan` report it as
   `skip-without-reason` and count the row unproven — the same treatment as
   a claimed symbol that doesn't exist.
2. **self-skipped.** Even with a reason, a claimed (`[x]`) row's
   `skip_test`/`skip` may not have been introduced by the SAME edit or
   commit that claims the row done — an agent may not grade its own row.
   `fathom verify` and `verify_plan` compare the plan file against git: the
   working tree against HEAD (catches a skip added in an uncommitted edit,
   right before the box is claimed), or — when the working tree already
   matches HEAD, the ordinary case at push time — HEAD against its own
   first parent (catches a skip added in the SAME commit that flipped the
   box to `[x]`). Either way it is a bounded, single git read, never a
   `git log -L` or `git blame` walk. A row flagged this way is reported
   `self-skipped` and counted unproven, whatever its `skip:` reason says.
   A skip that PREDATES the claim — already present at the comparison
   point — stands: refining an existing skip's wording, or claiming a row
   whose skip was decided in an earlier, separate commit, is not
   self-grading.

The PostToolUse edit hook also notes, at the moment of the edit and without
needing the daemon, when a row's `skip_test` was just introduced: `row <id>
gained skip_test in this edit — verify will count it unproven until the
skip predates the claim`. This is informational (it fires whether or not
the row is even claimed yet) — the two checks above are what actually gate.

The zero value of every key is the **strictest** contract; loosen explicitly.
`verify.LintPlan` reports v2 convention violations (duplicate IDs, dispositions
without reasons, unjustified skips) as **warnings** on stderr — they never
change exit codes in v0. skip-without-reason and self-skip are the two
exceptions: both are ALSO gate failures (`verify.VerifyPlan`/`Gate`), not
merely lint.

**A skip is a waiver, never a contract.** `skip_test`/`skip` only exempt a
`symbol:` from the test-reach requirement. A claimed row whose annotation
carries nothing else — no `symbol:`, `oracle:`, `gone:`, or spec ID — verifies
`no-contract` (Unknown): the default pre-push gate lets it through and only
`--strict` blocks it, however well its `skip:` text names the evidence. Lint
names that shape on every claimed row (`skip_test/skip is a waiver, not a
contract`), and a claimed row with no annotation beyond its id too (`claimed
with no contract`). The lint is advisory; in this repository what fails a push
on either shape is `TestRepoPlans_EveryClaimedRowCarriesAContract` under `task
validate`, over every top-level plan. An `(op)` measurement row closes with the evidence as its
contract: `oracle: lexical` on the reading (a distinctive `pattern:` in the
committed file — never the plan row itself), or `oracle: ledger` on the run.

### Product wiring — done means proven AND wired (ADR-018 amendment 2026-09-28)

A claimed `symbol:` row is proven only when a product entry point also
reaches its function: verify walks the same syntax+typed call graph
`test_reaches` walks, from every `func main` of a Go `package main` outside
`testdata/` (bench binaries included) and from the `init` functions and
package-level variable initializers of every package those link. MCP tool
handlers, CLI verbs, console routes and hook entries are reached through
the binary that registers them (a registered handler is a func-value edge,
I23). The walk does not count (EW-15): a bare reference assigned to the
blank identifier (`var _ = f`); a `main` in a file no default build compiles
(`//go:build ignore`); a syntax-only guess at a method call's receiver in a
file the typed pass resolved; or an interface dispatch into a type no reached
code instantiates. A func literal held in a package variable still counts
whether or not anything calls the variable (O133). The criterion is `wired`:

| Outcome | Status | When |
|---|---|---|
| `product entry point <key> reaches it` | pass | the product runs it |
| `library-only` (label) | unknown | a test reaches it and no entry point does; the default gate reports it, `--strict` (the pre-push gate) refuses it |
| `library` criterion | pass | not reached, but the row declares `library: true` with a `reason:` that predates the claim |
| n/a, named | — | a type, a non-Go symbol, test code, or a service with no binary at all (a library: nothing to wire into) |

A row anchored on a test-only wrapper (`Search`, when the product calls
`SearchWith`) is re-anchored on the function the product runs; a row whose
feature the product does not run is re-opened, or split into a library row
(`library: true`, reason naming the open wiring row) and a wiring row
anchored on the product entry that must call it.

## Annotation placement (load-bearing)

The annotation block sits **on the checkbox line**, or on the row's
**indented continuation lines** (G2-5). When the checkbox line carries no
`{…}` block, the parser reads every indented line under it, up to a blank
line, a list item, an HTML comment or a code fence, as part of the row: title
text, spec IDs and the block, which may itself span lines. A row whose
checkbox line carries its block reads that line alone, so a note indented
under it is never part of it. An annotation on an **unindented** line under a
row is not read (Markdown's lazy continuation is usually the next paragraph);
`LintPlan` names it, and the row verifies as `no-contract` until it is
indented or joined. One line stays the simplest form, and the only one a
fathom older than G2-5 reads.

## Honesty boundary

`test_reaches` means a `Test*` function **statically reaches** the symbol via
the call graph — never that the test passes or even runs. `runtime_pass` reads
the newest attested run of the row's certifying tests (a test-kit envelope
`fathom tests ingest` recorded): Pass or Fail with its commit and age, Unknown
when none exists. It is informational and never changes a row's status; verify
never runs a test (TC-29, I8).

## Gate commands

```sh
# pre-push (lefthook): a scratch ingest of HEAD, then strict per plan over the pushed range
# (a waiver added in the range is self-declared and fails, I45); fail-open on exit 2.
# <base> is the remote sha lefthook passes on stdin (the ref the push replaces); a new branch
# or a sha this clone lacks falls back to "$(git merge-base HEAD origin/main)" and says so:
bin/fathom verify --strict --since <base> --store <scratch-store> --service <svc> --plan docs/plans/<slug>.md --quiet

# against the daemon (default gate: claimed+Unknown rows, library-only included, are reported, not refused):
bin/fathom verify --service <svc> --plan docs/plans/<slug>.md
```

Exit codes: 0 every claimed row proven, 1 a claimed row unproven, 2 operational (fail-open).
A stale index (a dirty tree, or a commit after the ingest) is operational. When every unproven
claimed row is a `gone:` row held back only by that precondition (I44), `--strict` exits 2,
the rows stay Unknown, and one `index-stale:` line (also `index_stale` in `--json` and the
daemon's `verify_plan`) names the reason, the first dirty path, the row count and the remedy:
commit or stash, then re-ingest. Any other unproven row keeps exit 1.

## Computed progress and archival (E3/E4, docs/plans/archive/new-use-cases.md)

`fathom plans status` computes a per-plan rollup {proven, claimed-unproven,
open, deferred-with-trigger} by running the verify engine over every active
plan — never an asserted state. `fathom plans next` lists the ordered work
frontier (open rows plus claimed-but-failing rows) with their `{id, symbol,
package}` anchors, for an agent picking up work. A plan the daemon cannot
verify (a missed deadline, an RPC error) reads `not measured: <the error>`
in both verbs; every other plan still reports and the verb exits 2
(operational, O1161).

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
