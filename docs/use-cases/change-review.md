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
| `diff_impact` | MCP tool (investigation) | Blast radius of changed files: affected symbols, transitive callers, and related tests |
| `api_diff` | MCP tool (coding) | Added, removed, and changed exported symbols between two services |
| `arch_conformance` | MCP tool (coding) | Call-graph edges that violate declared layering rules |
| `dep_drift` | MCP tool (coding) | Dependency version mismatches between two services' lockfiles |
| `proto_conformance` | MCP tool (coding) | Naming convention violations in `.proto` files |
| `doc_coverage` | MCP tool (coding) | Exported symbols with missing documentation |
| `stale_docs` | MCP tool (coding) | Markdown files with inline-code or link references that no longer match known symbols or files |
| `review_change` | MCP prompt | Structured agent workflow for reviewing a code change: rename impact → test impact → API surface → complexity → call path |
| `ingest` | MCP tool (shared) | Re-ingest a service in-session to refresh the store before querying |
| `staleness_report` | MCP tool (shared) | Check whether the indexed state has drifted from the current HEAD |
| `fathom ingest-diff` | CLI | Ingest only files changed since a given git ref, then exit (pre-session or CI use) |

---

## How it enhances Claude Code

Without fathom, an agent reviewing a PR must read changed files and mentally trace impact — it misses callers defined in files it did not read, cannot compare API surfaces across services, and has no conformance gate. With fathom:

1. **Impact is grounded in the call graph.** `diff_impact` returns the exact set of symbols and tests that transitively depend on the changed files, so the agent knows what to re-test without guessing.
2. **API breakage is explicit.** `api_diff` names every added, removed, and changed exported symbol between the baseline and the changed version. The agent can block the merge rather than let a silent breaking change through.
3. **Layering and proto rules gate automatically.** `arch_conformance` and `proto_conformance` produce zero-output on a clean diff, so the agent only needs to act when there are findings.
4. **Staleness is surfaced, not hidden.** If the MCP server has been running across commits, `staleness_report` flags it and the agent can call `ingest` before querying — avoiding stale answers.
5. **Fewer tokens spent on context assembly.** A targeted `diff_impact` call returns exactly the affected nodes; the agent does not need to scan the whole repo to find them.

---

## Worked example

A PR modifies three files in `payment-service`. The agent works through the following sequence.

**1. Refresh the store for the changed service.**

```json
{ "name": "ingest", "arguments": { "service": "payment-service" } }
```

Returns `{ "service": "payment-service", "ok": true }`. The store now reflects HEAD.

**2. Check for staleness (optional guard).**

```json
{ "name": "staleness_report", "arguments": { "service": "payment-service" } }
```

Returns `{ "reingest_recommended": false, "indexed_commit": "a3f8c1d...", ... }`. Confirmed fresh.

**3. Compute the blast radius of the diff.**

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

The agent now knows which four tests to run and sees that `internal/api/handler.go` is indirectly affected.

**4. Check for public API breakage.**

`api_diff` compares the exported symbol surface of two independently-indexed service snapshots. Here, `payment-service-base` is the pre-PR snapshot and `payment-service` is the current one.

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

`billing.Invoice` was removed — a breaking public API change. The agent flags this for the PR author.

**5. Check architectural layering.**

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

Returns `{ "violations": [], "count": 0 }`. Layer rules pass.

**6. Check proto naming conventions (if applicable).**

```json
{ "name": "proto_conformance", "arguments": { "service": "payment-service" } }
```

Returns `{ "issues": [], "count": 0 }`. No violations.

**7. Check dependency version drift between services.**

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

`payment-service` is ahead on `golang.org/x/crypto`. The agent surfaces this for alignment.

**8. Check doc and stale-docs coverage on changed files (optional).**

```json
{ "name": "stale_docs", "arguments": { "service": "payment-service" } }
```

Returns any Markdown references that no longer resolve to known symbols or files.

---

Alternatively, invoke the `review_change` prompt to get a pre-structured agent workflow:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "prompts/get",
  "params": {
    "name": "review_change",
    "arguments": {
      "service": "payment-service",
      "change": "refactored billing.Invoice → billing.InvoiceV2, updated store.RunTxn signature"
    }
  }
}
```

The returned prompt guides the agent through rename impact, test impact, API surface, complexity, and call-path checks in a single structured sequence.

---

## Setup & notes

**Profile.** Most tools in this workflow (`api_diff`, `arch_conformance`, `dep_drift`, `proto_conformance`, `doc_coverage`, `stale_docs`) are in the `coding` group. `diff_impact` is in the `investigation` group. To access all of them in one session, either set no profile (all tools visible) or use the `batch` tool to call across groups. Recommended: set `coding` profile for the main review pass, then switch to `investigation` for `diff_impact`.

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

**`arch_conformance` rules are caller-supplied.** The tool does not read a config file; pass your layering rules as JSON in each call. If you want a persistent rule set, wrap the call in a project-level script or hook.

---

## The `fathom verify` gate as a pre-push complement

Impact analysis answers *what is affected*; the `fathom verify` gate answers *what is proven done*. Before pushing, `lefthook` runs `fathom verify` over any plan files in `docs/plans/*.md`, checking each `[x]`-claimed task against the index. A claimed task that lacks code evidence blocks the push (exit 1); an operational error (daemon down) warns and allows (exit 2, fail-open). In CI, `scripts/ci.sh` adds `--strict` to treat exit 2 as a failure. Run `fathom verify` manually with `--plan <file> --service <id>` or pass `--store` to bypass the daemon. See [spec-verification.md](spec-verification.md) for the full gate reference.

---

## Review findings as compiled claims (`fathom report` + `fathom claim`)

Review conclusions can outlive the review session as governed claims (the findings layer — one markdown file per finding under `findings/`, compiled with `fathom compile --layer findings` or the `compile_layer` MCP tool; kind `finding`, tier `sme-stated`). Two CLI verbs close the loop:

- **`fathom report --store DB --service S --analyzer security_rollup --out findings/`** writes one deterministic markdown finding document from the security analyzers' current output (per-category sections with `path:line` lists; secret values are redacted). Written into the corpus dir, it flows through the normal ingest → compile → claims path; the report's own output directory is excluded from the scan, and re-running against an unchanged tree reproduces identical bytes.
- **`fathom report --store DB --service S --analyzer reverse-drift --out findings/`** is the reverse-SDD-drift check: the service's public API surface (`internal/analysis/apisurface`) minus what a spec-bindings claim covers (matched by file-path glob against gated plan rows' `{symbol, package}` contracts) = the surface no plan or spec governs. Read-only — it reports the current index/claims state and doesn't persist anything itself, but its output can be pointed at `findings/` the same way security_rollup's is if you want ungoverned-surface findings to join the compiled corpus.
- **`fathom claim refute|corroborate --store DB --service S --claim ID --note "..."`** is the triage channel: `refute` flips the claim to `refuted` (suppression with a note — durable across recompiles); `corroborate` records provenance without touching status or tier. Both are store-direct (`--store` required): stop the daemon first when triaging the fleet store.

Once compiled, findings scoped to the files under review surface through the same `obligations` tool (labeled `kind:"finding"`), and `code_drifted:true` flags findings whose scoped code changed since compile — the "did remediation happen?" query. `fathom layers show findings` prints the pack's resolved definition and prompt provenance; see `docs/layers.md` for the full catalog (`docs/use-cases/coding-agents.md`'s Maintained line owns `fathom layers` itself).

---

## Maintained

This doc covers: `diff_impact`, `api_diff`, `arch_conformance`, `dep_drift`, `proto_conformance`, `doc_coverage`, `stale_docs`, `review_change` (MCP prompt), `ingest`, `staleness_report`, `fathom ingest-diff`, `fathom verify`, `fathom report`, `fathom claim` (refute/corroborate). Update it when these change.
