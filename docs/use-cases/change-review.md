# Change & PR Review

Review a diff or pull request with automated impact analysis and conformance gates before merging.

---

## When to use this

- You have a PR open and want to know which callers, tests, and files are affected before merging.
- You need to catch public API breakage (added/removed/changed exported symbols) introduced by the diff.
- You want to enforce architectural layering rules or proto naming conventions as a pre-merge gate.
- You want to identify lockfile version divergence between two services that share a dependency.

---

## Tools

| Name | Kind | What it gives you |
|---|---|---|
| `review_diff` | MCP tool (investigation) | **The composed verb** (C1-1, `docs/plans/review-verb.md`): diff impact + obligations/findings + architecture conformance + spec coverage in ONE cited verdict, over an explicit `changed_files` list |
| `diff_impact` | MCP tool (investigation) | Blast radius of changed files: affected symbols, transitive callers, and related tests |
| `api_diff` | MCP tool (coding) | Added, removed, and changed exported symbols between two services |
| `arch_conformance` | MCP tool (coding) | Call-graph edges that violate declared layering rules |
| `dep_drift` | MCP tool (coding) | Dependency version mismatches between two services' lockfiles |
| `proto_conformance` | MCP tool (coding) | Naming convention violations in `.proto` files |
| `doc_coverage` | MCP tool (coding) | Exported symbols with missing documentation |
| `stale_docs` | MCP tool (coding) | Markdown files with inline-code or link references that no longer match known symbols or files |
| `review_change` | MCP prompt | Calls `review_diff` for a structured, one-call review of a code change |
| `ingest` | MCP tool (shared) | Re-ingest a service in-session to refresh the store before querying |
| `staleness_report` | MCP tool (shared) | Check whether the indexed state has drifted from the current HEAD |
| `fathom review` | CLI | Store-direct CLI mirror of `review_diff`: resolves changed files via `git diff --name-only <ref>`, runs the same engine (used by the non-blocking pre-push summary, `docs/plans/review-verb.md` C3-1) |
| `fathom ingest-diff` | CLI | Ingest only files changed since a given git ref, then exit (pre-session or CI use) |

---

## How it enhances Claude Code

Without fathom, an agent reviewing a PR must read changed files and mentally trace impact — it misses callers defined in files it did not read, cannot compare API surfaces across services, and has no conformance gate. With fathom:

1. **One call composes the whole review.** `review_diff` returns diff impact, obligations/findings scoped to the changed files, architecture conformance, and spec coverage as one cited verdict — the agent does not have to remember (or narrate) an 8-step tool sequence.
2. **Impact is grounded in the call graph.** The verdict's `impact` dimension returns the exact set of symbols and tests that transitively depend on the changed files, so the agent knows what to re-test without guessing.
3. **Layering rules gate automatically, honestly.** The `arch_conformance` dimension produces zero findings on a clean diff, and reports `checked:false` with a reason — never a clean-shaped pass — when no rule pack is configured, so the agent can tell "passed" from "wasn't checked."
4. **Findings are durable.** A finding refuted once (`fathom claim refute` or the `obligations` tool's feedback action) stays refuted across every later re-review of the same diff — no re-litigating the same false positive on every push.
5. **Fewer tokens spent on context assembly.** A single `review_diff` call returns exactly the affected nodes and findings; the agent does not need to scan the whole repo or issue five separate calls to find them.

---

## Worked example

A PR modifies `internal/api/handler.go` in `payment-service`, which now calls into `internal/store` directly — a layering violation the service's own `.fathom/rules/archconformance.cue` forbids.

**1. Refresh the store for the changed service.**

```json
{ "name": "ingest", "arguments": { "service": "payment-service" } }
```

Returns `{ "service": "payment-service", "ok": true }`. The store now reflects HEAD.

**2. Call `review_diff` once with the changed files.**

```json
{
  "name": "review_diff",
  "arguments": {
    "service": "payment-service",
    "changed_files": ["internal/api/handler.go"]
  }
}
```

Returns one cited verdict:

```json
{
  "service": "payment-service",
  "changed_files": ["internal/api/handler.go"],
  "dimensions": [
    {
      "name": "impact",
      "checked": true,
      "count": 2,
      "findings": [
        { "kind": "impact", "symbol": "api.RouteHandler", "file": "internal/api/router.go", "message": "transitively depends on changed code", "citation": "internal/api/router.go#L12-L12", "severity": "info" },
        { "kind": "impact", "symbol": "api.TestHandler_Charge", "file": "internal/api/handler_test.go", "message": "transitively depends on changed code", "citation": "internal/api/handler_test.go#L4-L4", "severity": "info" }
      ],
      "tests": ["api.TestHandler_Charge"],
      "files": ["internal/api/handler.go", "internal/api/router.go", "internal/api/handler_test.go"]
    },
    { "name": "obligations", "checked": true, "count": 0, "findings": [] },
    {
      "name": "arch_conformance",
      "checked": true,
      "count": 1,
      "findings": [
        {
          "kind": "arch_violation", "symbol": "api.Handler",
          "file": "internal/api/handler.go", "line": 42,
          "message": "api.Handler → store.Query: package \"api\" must not import \"store\" (deny rule)",
          "citation": "internal/api/handler.go#L42-L42",
          "severity": "error",
          "claim_id": "3f9a…"
        }
      ]
    },
    { "name": "spec_coverage", "checked": true, "count": 0, "findings": [] }
  ],
  "summary": "service payment-service: 1 changed file(s); 4/4 dimensions checked; 3 active finding(s)",
  "findings_persisted": true
}
```

`arch_conformance` caught the layering violation — cited with its reason and rule-pack-derived source — and the two `impact` findings name exactly which caller and test are affected.

**`review_diff` writes to the claims store — deliberately, and only the oracle dimensions.** `findings_persisted:true` reports that the `arch_conformance` and `spec_coverage` findings (the deterministic-oracle dimensions — never `obligations`, which re-serves already-compiled claims, and never `impact` context noise) were persisted as content-addressed claims on the `findings` layer, each anchored with a hashed line citation and a scope fingerprint so the daemon's post-refresh citation sweep and remediation-drift pass verify them instead of falsifying them. The write is idempotent (an identical re-run upserts identical rows) and self-reconciling: a review-minted claim whose finding a later run over the same files no longer emits is flipped to `superseded` — absent = fixed. If the PR author disputes the violation, `fathom claim refute --claim 3f9a… --note "..."` (or the `obligations` tool's `action:"refute"`) suppresses it durably — a later `review_diff` call re-emitting the identical finding carries `status:"refuted"` (excluded from `count`, still listed) instead of re-flagging it.

Alternatively, invoke the `review_change` prompt to get the same call pre-structured:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "prompts/get",
  "params": {
    "name": "review_change",
    "arguments": {
      "service": "payment-service",
      "changed_files": "internal/api/handler.go",
      "change": "handler now calls store.Query directly"
    }
  }
}
```

The returned prompt instructs the agent to call `review_diff` with those arguments and how to read the resulting verdict.

**CLI equivalent:**

```sh
fathom review --store workspace/db/fleet.sqlite --service payment-service --since origin/main
```

Resolves changed files via `git diff --name-only origin/main`, runs the identical engine, and prints a short human-readable summary (`--json` for the same payload shape as above). Like the MCP tool, a plain `fathom review` run **persists** its oracle-dimension findings to the claims store; pass `--no-persist` for a pure report with no store write of any kind — that is what the non-blocking pre-push summary (`docs/plans/review-verb.md` C3-1) uses, because a hook must never be an undisclosed writer to the shared store.

---

### Appendix: manual tool-by-tool sequence

Before `review_diff` existed, reviewing a change meant calling each analyzer separately. This sequence still works — useful when you want only one dimension, or when composing your own workflow — and every tool it calls remains independently available.

**Compute the blast radius of the diff.**

```json
{
  "name": "diff_impact",
  "arguments": {
    "service": "payment-service",
    "changed_files": [
      "internal/billing/invoice.go",
      "internal/billing/retry.go",
      "internal/store/txn.go"
    ]
  }
}
```

Returns:
```json
{
  "symbols": ["billing.CreateInvoice", "billing.RetryCharge", "store.RunTxn", "store.CommitTxn"],
  "tests":   ["TestCreateInvoice_Idempotent", "TestRetryCharge_Backoff", "TestRunTxn_Rollback"],
  "files":   ["internal/billing/invoice.go", "internal/billing/retry.go", "internal/store/txn.go",
              "internal/api/handler.go"]
}
```

**Check for public API breakage.** `api_diff` compares the exported symbol surface of two independently-indexed service snapshots — here `payment-service-base` is the pre-PR snapshot and `payment-service` is the current one.

```json
{
  "name": "api_diff",
  "arguments": {
    "service_a": "payment-service-base",
    "service_b": "payment-service"
  }
}
```

Returns:
```json
{
  "added":   ["billing.InvoiceV2"],
  "removed": ["billing.Invoice"],
  "changed": ["billing.CreateInvoice"]
}
```

`billing.Invoice` was removed — a breaking public API change.

**Check architectural layering.**

```json
{
  "name": "arch_conformance",
  "arguments": {
    "service": "payment-service",
    "rules": [
      { "from_pkg": "internal/api",     "to_pkg": "internal/billing", "allow": true },
      { "from_pkg": "internal/billing", "to_pkg": "internal/store",   "allow": true },
      { "from_pkg": "internal/api",     "to_pkg": "internal/store",   "allow": false }
    ],
    "deny_by_default": false
  }
}
```

Returns `{ "violations": [], "count": 0 }` — layer rules pass (or a `rule_source`/`checked:false` honesty split when no rules were configured; see the `arch_conformance` row above).

**Check proto naming conventions (if applicable).**

```json
{ "name": "proto_conformance", "arguments": { "service": "payment-service" } }
```

Returns `{ "issues": [], "count": 0 }`. No violations.

**Check dependency version drift between services.**

```json
{
  "name": "dep_drift",
  "arguments": {
    "service_a": "auth-service",
    "service_b": "payment-service"
  }
}
```

Returns:
```json
{
  "drift": [
    { "name": "golang.org/x/crypto", "current": "v0.21.0", "baseline": "v0.18.0" }
  ],
  "count": 1
}
```

`payment-service` is ahead on `golang.org/x/crypto`.

**Check doc and stale-docs coverage on changed files (optional).**

```json
{ "name": "stale_docs", "arguments": { "service": "payment-service" } }
```

Returns any Markdown references that no longer resolve to known symbols or files.

---

## Setup & notes

**`review_diff` never accepts inline architecture rules.** Unlike calling `arch_conformance` directly, `review_diff`'s `arch_conformance` dimension always resolves through the service's own `.fathom/rules/archconformance.cue` project rule pack (or reports `checked:false` when none exists) — there is no `rules` argument on `review_diff` itself. If you need to test rules that aren't committed yet, call `arch_conformance` directly with inline `rules`.

**Profile.** `review_diff` and `diff_impact` are both in the `investigation` group; the appendix's remaining tools (`api_diff`, `arch_conformance`, `dep_drift`, `proto_conformance`, `doc_coverage`, `stale_docs`) are in `coding`. To access all of them in one session, either set no profile (all tools visible) or use the `batch` tool to call across groups. Recommended: `investigation` profile for `review_diff`/`diff_impact`, `coding` for the appendix's per-dimension tools.

On stdio `serve-mcp`:

```json
{ "name": "set_profile", "arguments": { "profile": "coding" } }
```

Over the shared HTTP daemon, `set_profile` is rejected — register against `POST /rpc/coding` or `POST /rpc/investigation` instead (or `POST /rpc` for the full surface).

**`api_diff` requires two indexed snapshots.** The tool compares the exported symbol stores of `service_a` and `service_b`. To compare before/after a PR, you need the baseline service indexed separately — either as a second service ID pointing at the base branch checkout, or by re-ingesting the current branch under a distinct service ID (e.g. `payment-service-base`).

**`diff_impact` does not require a `concern`.** It takes `changed_files` instead. Pass repo-relative paths as returned by `git diff --name-only <base>`.

**CLI alternative for CI.** When no MCP session is running, use the CLI to update the store before running queries:

```sh
fathom ingest-diff --service payment-service --path /path/to/payment-service --base origin/main --store workspace/db/fleet.sqlite
```

This runs `git diff --name-only origin/main`, ingests changed files, and exits. A subsequent MCP session will pick up the updated store.

**Staleness during long sessions.** A `serve-mcp` process holds an exclusive DuckDB write lock. CLI `ingest-diff` will fail while a session is active. Use the `ingest` MCP tool instead to refresh in-session. The `staleness_report` tool tells you whether a refresh is needed before you run impact queries.

**`arch_conformance` rules: inline or project rule pack.** Pass layering rules as JSON in the call (`rules: [{from_pkg, to_pkg, allow}]`), or pass none and the tool falls back to the service's own committed rule pack at `.fathom/rules/archconformance.cue` (validated against `schema/rules/archconformance/schema.cue`; loaded from the corpus root recorded at ingest). The response's `rule_source` field reports which source was used — `"inline"`, `"project"`, or `"none"` — and when no rules were available from either source the response carries `checked: false` plus a `reason` instead of a clean-shaped zero-violation answer. A malformed rule pack is a loud error, never silently treated as "no rules". Scope note: rules match function-origin call edges; method symbols carry receiver-typed package prefixes and are a disclosed gap (see the schema's `#Rule` doc).

---

## The `fathom verify` gate — and `fathom review`'s non-blocking summary

Impact analysis answers *what is affected*; the `fathom verify` gate answers *what is proven done*. Before pushing, `lefthook` runs `fathom verify` over any plan files in `docs/plans/*.md`, checking each `[x]`-claimed task against the index. A claimed task that lacks code evidence blocks the push (exit 1); an operational error (daemon down) warns and allows (exit 2, fail-open). In CI, `scripts/ci.sh` adds `--strict` to treat exit 2 as a failure. Run `fathom verify` manually with `--plan <file> --service <id>` or pass `--store` to bypass the daemon. See [spec-verification.md](spec-verification.md) for the full gate reference.

The same pre-push hook also runs `fathom review --service fathom --since origin/main --no-persist` (C3-1, `docs/plans/review-verb.md`) — unlike `verify`, this step is **advisory, never blocking**: it always exits 0 and prints the verdict summary so review findings are visible on every push, even when nothing forces you to read them. It is truthfully a *summary*: `--no-persist` means the hook never writes to the shared claims store (persisting findings is a deliberate CLI/MCP act, not a hook side effect). An operational error there degrades to a single line (fail-open, same doctrine as `verify`'s exit 2), never a raw error dump. Latency: measured ~12s on a pathological 1777-file diff; a normal `--since` diff is far smaller.

---

## Review findings as compiled claims (`fathom report` + `fathom claim`)

Review conclusions can outlive the review session as governed claims (the findings layer — one markdown file per finding under `findings/`, compiled with `fathom compile --layer findings` or the `compile_layer` MCP tool; kind `finding`, tier `sme-stated`). Two CLI verbs close the loop:

- **`fathom report --store DB --service S --analyzer security_rollup --out findings/`** writes one deterministic markdown finding document from the security analyzers' current output (per-category sections with `path:line` lists; secret values are redacted). Written into the corpus dir, it flows through the normal ingest → compile → claims path; the report's own output directory is excluded from the scan, and re-running against an unchanged tree reproduces identical bytes.
- **`fathom report --store DB --service S --analyzer reverse-drift --out findings/`** is the reverse-SDD-drift check: the service's public API surface (`internal/analysis/apisurface`, with `_test.go` symbols excluded — test helpers are not public API) minus what a spec-bindings claim covers (matched by file-path glob against gated plan rows' `{symbol, package}` contracts) = the surface no plan or spec governs. When the service has zero compiled spec-bindings claims the report banners that explicitly — the listing is then the whole surface, not measured drift. Read-only — it reports the current index/claims state and doesn't persist anything itself, but its output can be pointed at `findings/` the same way security_rollup's is if you want ungoverned-surface findings to join the compiled corpus.
- **`fathom claim refute|corroborate --store DB --service S --claim ID --note "..."`** is the triage channel: `refute` flips the claim to `refuted` (suppression with a note — durable across recompiles); `corroborate` records provenance without touching status or tier. Both are store-direct (`--store` required): stop the daemon first when triaging the fleet store.

Once compiled, findings scoped to the files under review surface through the same `obligations` tool (labeled `kind:"finding"`), and `code_drifted:true` flags findings whose scoped code changed since compile — the "did remediation happen?" query. `fathom layers show findings` prints the pack's resolved definition and prompt provenance; see `docs/layers.md` for the full catalog (`docs/use-cases/coding-agents.md`'s Maintained line owns `fathom layers` itself).

---

## Maintained

This doc covers: `review_diff` (the composed verb, C1-1/C2-1, `docs/plans/review-verb.md`) and its `fathom review` CLI mirror, `diff_impact`, `api_diff`, `arch_conformance` (incl. its `.fathom/rules/archconformance.cue` project rule packs and `rule_source`/`checked` response fields), `dep_drift`, `proto_conformance`, `doc_coverage`, `stale_docs`, `review_change` (MCP prompt), `ingest`, `staleness_report`, `fathom ingest-diff`, `fathom verify`, the pre-push `fathom review` summary step (C3-1), `fathom report`, `fathom claim` (refute/corroborate). Update it when these change.
