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
  --store workspace/db/fleet.duckdb \
  --output workspace/artifacts/STATUS.cue
```

Writes a deterministic `STATUS.cue` with each task's verdict and cited evidence (`file:line`, tier). The plan file is the human-owned source of truth; `STATUS.cue` is regenerated, never hand-edited — commit it so reviewers see exactly what is and isn't backed by code.

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

This doc covers: `verify_plan`, `spec_coverage`, `spec_status`, `traceability`, `spec_refs`, `contradiction_scan`, `fathom gen-status-cue`. Update it when these change.
