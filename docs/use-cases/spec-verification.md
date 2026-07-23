# Evidence-backed definition of done (spec verification)

Stop trusting "done!" — verify each task against actual code evidence before an agent or human declares the work complete.

## When to use this

- An agent or PR author says a feature is implemented and you want an honest verdict before merging.
- A plan or ticket lists discrete tasks with symbols or specs and you need per-task coverage status.
- You suspect a stub was committed — a function exists but no test reaches it and no runtime evidence has been ingested.
- A spec requirement is listed in a plan file and you want to know whether the codebase matches or contradicts it.

## Tools

| Name | Kind | What it gives you |
|---|---|---|
| `verify_plan` | MCP tool | Per-task verdict (`verified`, `claimed-but-absent`, `present-but-untested`, `edge-not-wired`, `contradicted`, `likely-unconfirmed`, `spec-uncovered`) with cited evidence; accepts `service`, optional `concern`, and `tasks[]` |
| `spec_coverage` | MCP tool | Coverage map: which spec IDs are addressed by evidence observations and at what tier |
| `spec_status` | MCP tool | High-level pass/fail/unknown rollup per spec ID or plan section |
| `traceability` | MCP tool | Traces a spec ID forward to symbols and backward to observations, showing the evidence chain |
| `spec_refs` | MCP tool | Lists every observation that cites a given spec ID, with tier and staleness |
| `contradiction_scan` | MCP tool | Finds observations that conflict with each other or with the spec within a concern scope |
| `fathom gen-status-cue` | CLI | Render a deterministic `STATUS.cue` from a plan file + the live index — the plan stays the source of truth; `STATUS.cue` is regenerated, never hand-edited |

## How it enhances Claude Code

Without fathom, an agent finishes a task and self-reports "done" based on what it wrote. Fathom replaces that with a structured evidence check:

1. `verify_plan` walks each task's declared symbol through the index. It checks whether the symbol exists (`claimed-but-absent` if not), whether a test reaches it via the call graph (`present-but-untested` if not), whether the edge specified by `reachable_from` is wired, and whether any observation contradicts the spec.
2. `contradiction_scan` with a `concern` argument surfaces conflicting evidence before the agent writes its summary — catching cases where two observations disagree about behavior.
3. `spec_coverage` and `spec_status` give the agent a compact rollup it can include in a PR description or status comment without reading every file again.

The payoff: the agent cannot hallucinate completion. Verdicts are derived from the index, not from the agent's recollection of what it wrote. The concern-scoping argument reduces token cost by ~77% by focusing the scan.

### Honesty boundary (critical)

Fathom is a **static** engine. `test_reaches: true` means a test reaches the symbol via the recorded call graph — it does **not** mean the test passes at runtime. `runtime_pass` is always `"unknown"` until CI or coverage evidence is explicitly ingested. Auto-pass to the `verified` tier requires evidence at the `verified` tier; heuristic pre-scan results (non-Go extractors) produce `likely-unconfirmed`. Do not report a task as `verified` to a human unless `verify_plan` returns exactly that verdict.

For Go services, `test_reaches` also consumes typed call edges (ADR-021 §3): a symbol reachable only through an interface method shared by several concrete implementations — e.g. two adapters both implementing `Complete`, or two backends both implementing a `Store` interface's `PutClaims` — used to be an unconditional refusal (the syntax-only graph can't disambiguate an overloaded method name across receivers), so the task verified `present-but-untested` even when a test genuinely exercised it through the interface. It now verifies, and the criterion's `detail` says so explicitly: `"2 test(s) statically reach it, e.g. TestRunAdapter (reached via interface dispatch, 2 implementer(s))"` — read "reached via interface dispatch" as strictly better evidence than a direct static reach, not worse; it means the compiler-level type resolver, not just AST pattern-matching, confirmed the call site.

## Worked example

An agent finishes four tasks in `my-service` against plan `plan.cue`. Before reporting done it runs:

**1. Call `verify_plan` for all four tasks:**

```json
{
  "service": "my-service",
  "concern": "payment-flow",
  "tasks": [
    {
      "id": "T-01",
      "title": "Charge card endpoint",
      "spec_ids": ["[US-101]"],
      "symbol": "ChargeHandler",
      "reachable_from": "TestChargeHandler"
    },
    {
      "id": "T-02",
      "title": "Refund endpoint",
      "spec_ids": ["[US-102]"],
      "symbol": "RefundHandler",
      "reachable_from": "TestRefundHandler"
    },
    {
      "id": "T-03",
      "title": "Idempotency key validation",
      "spec_ids": ["[US-103]"],
      "symbol": "validateIdempotencyKey",
      "reachable_from": "TestChargeHandler"
    },
    {
      "id": "T-04",
      "title": "Webhook signature check",
      "spec_ids": ["[US-104]"],
      "symbol": "verifyWebhookSignature",
      "reachable_from": "TestWebhookDispatch"
    }
  ]
}
```

**Result from `verify_plan`:**

```
T-01  verified             ChargeHandler found; TestChargeHandler reaches it; tier=verified
T-02  present-but-untested RefundHandler found in index; no test reaches it via call graph
T-03  claimed-but-absent   validateIdempotencyKey not found in my-service index
T-04  edge-not-wired       verifyWebhookSignature exists; TestWebhookDispatch does not call it
```

The agent does **not** report done. It takes corrective action:

**2. For T-02**, the agent checks `spec_refs` to confirm what spec coverage exists:

```json
{ "service": "my-service", "spec_id": "[US-102]" }
```

Result: one observation at tier `pre-scan` (heuristic), none at `verified`. Confirms the gap is real.

**3. For T-03**, the agent calls `spec_coverage`:

```json
{ "service": "my-service", "spec_ids": ["[US-103]"] }
```

Result: `[US-103]` shows no implementing symbols (uncovered). The symbol was never committed.

**4. Run `contradiction_scan` before any status report:**

```json
{ "service": "my-service", "concern": "payment-flow" }
```

Result: no contradictions found. Good — the two unfinished tasks are gaps, not conflicts.

**5. After fixing T-02 and T-03**, re-ingest via the MCP `ingest` tool (not the CLI, because the session holds the DuckDB lock):

```json
{ "service": "my-service" }
```

Then re-run `verify_plan`. All four tasks now return `verified`.

**6. Render a status artifact for the PR / commit:**

```sh
fathom gen-status-cue \
  --plan plan.cue \
  --service my-service \
  --store workspace/db/fleet.sqlite \
  --output workspace/artifacts/STATUS.cue
```

Writes a deterministic `STATUS.cue` with each task's verdict and cited evidence (`file:line`, tier). The plan file is the human-owned source of truth; `STATUS.cue` is regenerated, never hand-edited — commit it so reviewers see exactly what is and isn't backed by code.

## The `fathom verify` gate (done-means-proven)

`fathom verify` reads plan files in `docs/plans/*.md` and checks every `[x]`-marked checkbox against the index. A `[x]` line is a *claim* — the task is declared done. The gate verifies each claim has code evidence; unclaimed (`[ ]`) lines are skipped.

**Checkbox semantics:** Annotate a line with `{symbol: Foo}` to name what should exist; add `[US-042]` for spec coverage; add `{reachable_from: Bar}` to require a test edge. An unannotated `[x]` line verifies as `no-contract` (honest, not a false pass).

**Plan file convention:** `docs/plans/README.md` describes the format. `fathom verify` skips any file named `README.md` (it describes the convention, not a plan).

**Exit codes:**
- `0` — all claimed `[x]` tasks are backed by evidence, or no plan files found.
- `1` — at least one claimed task fails verification; push blocked.
- `2` — operational error (unreadable plan, daemon unreachable); fail-open, push allowed.

**Lefthook pre-push (fail-open):** `lefthook.yml` runs `fathom verify` over every `docs/plans/*.md` file at pre-push. Exit 1 blocks the push; exit 2 warns and allows (daemon may be down during local work):

```sh
bin/fathom verify --service <service> --plan docs/plans/my-plan.md
```

**CI (`--strict`, fail-closed):** `scripts/ci.sh` adds `--strict`, which treats operational-error exit 2 as failure — recommended when a daemon is guaranteed available:

```sh
bin/fathom verify --strict --service <service> --plan docs/plans/my-plan.md \
  --store workspace/db/fleet.sqlite   # or omit to use the HTTP daemon
```

**HTTP-first, store fallback:** by default `fathom verify` queries `http://localhost:7575/rpc` (the shared daemon). Pass `--store <path>` to bypass the daemon and query a DuckDB file directly — useful in CI environments without a running daemon.

## Computed progress and archival (`fathom plans`)

`fathom plans` is the CLI verb family for SDD state that is **computed by
the verify engine, never asserted** — the same honesty discipline as
`fathom verify`, applied to "how much of this plan is actually done" instead
of one checkbox at a time.

**`fathom plans status`** — a per-plan rollup: `proven` (claimed and
verifies), `claimed_unproven` (claimed but fails or unknown),
`open` (`[ ]`/`[~]`, not yet claimed), `deferred_with_trigger` (`[-]` with
`disposition: deferred`), `skipped_other` (any other `[-]`). Defaults to
every active `docs/plans/*.md` file; pass `--plan` (repeatable) to scope it:

```sh
bin/fathom plans status --service <service> --store workspace/db/fleet.sqlite --json
```

**`fathom plans next`** — the ordered work frontier for an agent picking up
where a plan left off: open/in-progress rows plus claimed-but-unproven rows
(verify Fail as `claimed-failing`, verify Unknown — e.g. no-contract — as
`claimed-unknown`; the same set CI's `--strict` blocks on, so the frontier
never says "done" on a plan CI rejects), each with its `{id, symbol,
package}` anchors and plan file.
Ordering is (plan file, then the row's position in the file) — an
author-owned `id:` need not sort alphabetically the way it was written, so
this is never a re-sort by ID.

```sh
bin/fathom plans next --service <service> --store workspace/db/fleet.sqlite --json
```

**`fathom plans archive <plan>` / `fathom plans restore <plan>`** — the
governed artifact lifecycle (bounds the gate's cost as the plan count
grows). Archive refuses unless: every row is `[x]`-claimed-and-currently-
verifying or `[-]` skipped; every row's task ID appears somewhere in
CHANGELOG.md or ADR.md (the plan's claims must already be absorbed into the
permanent record); and the engine is re-run fresh at archive time, never
trusted from an earlier pass. On success the plan moves to
`docs/plans/archive/` with a tombstone (date, HEAD sha, verify summary)
written into its header; `restore` moves it back, leaving the tombstone in
place. `docs/plans/archive/` is out of `fathom verify`'s gate scope by
construction — see `docs/plans/README.md`.

```sh
bin/fathom plans archive docs/plans/my-plan.md --service <service> --store workspace/db/fleet.sqlite
bin/fathom plans restore docs/plans/archive/my-plan.md --root .
```

## Setup & notes

**Profile:** Set `investigation` before running spec-verification workflows:

```json
{ "profile": "investigation" }
```

This exposes the investigation tool group plus shared tools, keeping the visible surface focused.

**Concern is optional for `verify_plan` but strongly recommended.** Without a concern, the scan covers the full service index. With a concern it covers only the relevant slice, which is ~77% fewer tokens in practice.

**Re-ingest during a session:** Call the `ingest` MCP tool in-session. Do not use `fathom ingest` from the CLI while a session holds the single-writer DuckDB lock — it will fail. The MCP route is the correct path.

**Staleness:** If `serve-mcp` has been running since before the last commit, results may be stale. Check with `staleness_report` or call `ingest` proactively at session start.

**`gen-status-cue` is a CLI command, not an MCP tool.** It renders `STATUS.cue` to `workspace/artifacts/` by default (per the workspace convention). The plan file passed via `--plan` is the human-owned source of truth; never hand-edit the generated `STATUS.cue` — regenerate it. It accepts a `.cue` plan (matching `schema/plan/schema.cue`), a `.json` plan, or a `.md` Markdown checklist (each `- [ ]/[x] title [SPEC-ID] {symbol: Foo}` line becomes a task).

**In the loop.** `verify_plan` also accepts `plan_markdown` — the contents of a checklist like `TRACKING.md` — directly, so an agent can verify before reporting done. The `/verify` slash command automates this: read the plan → call `verify_plan` → surface every `[x]` item that is **not** `verified`. Annotate a checklist line with `{symbol: Foo}` (and optionally `[US-042]`, `{reachable_from: Bar}`) to give it a checkable contract; an unannotated line verifies as `no-contract` (honest, not a false pass).

**Tier thresholds:** `verified` verdict requires evidence at the `verified` tier. Evidence from heuristic extractors (non-Go files) arrives at lower tiers and produces `likely-unconfirmed`, not `verified`. Go sources processed by the AST extractor can reach `verified`. This is not a bug — it is an honest representation of evidence quality.

## Maintained

This doc covers: `verify_plan`, `spec_coverage`, `spec_status`, `traceability`, `spec_refs`, `contradiction_scan`, `fathom gen-status-cue`, `fathom verify`, `fathom plans status`, `fathom plans next`, `fathom plans archive`, `fathom plans restore`. Update it when these change.
