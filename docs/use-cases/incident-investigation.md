# Incident investigation

Triage a production symptom fast using pre-computed call graphs, complexity hotspots, and commit history — without reading the codebase cold.

---

## When to use this

- A service is returning errors or exhibiting unexpected behavior in production and you need to identify the blast radius and likely cause quickly.
- You have a symptom (error message, failing endpoint, slow query) but no immediate pointer to which code path is involved.
- You suspect a recent commit introduced a regression and want to correlate timing with code changes.
- Documentation or comments appear to contradict what the code actually does, and you want to surface those conflicts before drawing conclusions.

---

## Tools

| Name | Kind | What it gives you |
|---|---|---|
| `investigate_incident` | MCP prompt | Structured investigation sequence: entrypoints → complexity → call hierarchy → community → staleness; returns step-by-step tool instructions tailored to `service` + `symptom` |
| `entrypoints` | MCP tool | Main functions, HTTP handlers, CLI root commands — the surface most likely to be the entry to the failing path |
| `find_callers` | MCP tool | All symbols that call a named symbol; traces a function up through its callers |
| `blast_radius` | MCP tool | Given a set of changed files, returns all transitively affected symbols, test functions, and file paths |
| `history` | MCP tool | Git commit history for a service, filterable by file suffix, spec ID, or symbol; results are newest-first |
| `contradiction_scan` | MCP tool | Scans concern-scoped query results for symbols with conflicting kinds or doc-vs-code contradictions (concern **required**) |
| `evidence_summary` | MCP tool | Per-tier observation counts (`verified`, `pre-scan`, `doc-claimed`, `sme-stated`, `assumed`) — signals how much to trust the index |
| `community` | MCP tool | Related symbols: callers, callees, and co-located symbols in the same package or file |
| `traverse` | MCP tool | BFS over typed edges (calls, imports, spec-refs, routes, env-keys) from a starting symbol or route |
| `staleness_report` | MCP tool | Whether the index has fallen behind HEAD; `reingest_recommended: true` means results may be stale |

---

## How it enhances Claude Code

Without fathom, investigating an incident means cold-reading unfamiliar code, grepping for handler names, and manually chasing call chains — an expensive, error-prone process that burns large amounts of context.

With fathom:

- **Concern scoping** keeps tool results focused. Passing `concern: "checkout"` to `contradiction_scan` means you get contradictions in the checkout concern only — not a fleet-wide data dump. This is the ~77% token-reduction mechanism described in ADR-006.
- **Pre-computed call graphs** let `find_callers` and `blast_radius` answer "what does this touch?" in a single tool call rather than a grep sweep across dozens of files.
- **`history` with symbol resolution** collapses "find the commit that changed this function" into one call — it resolves the symbol to its file and filters commit history to that file automatically.
- **`contradiction_scan`** surfaces doc-vs-code drift that might explain why the code is not behaving as the runbook says, without requiring you to read both documents manually.
- **`evidence_summary`** tells you up front whether the index data is primarily verified (parsed from source) or assumed (lower-confidence), so you calibrate how much to trust the tool results before acting on them.

Together these move the investigation from open-ended code reading to a directed sequence of structured queries, letting Claude Code close in on a root cause faster and with less context drift.

---

## Worked example

Scenario: `checkout-service` is returning HTTP 500 on `POST /checkout` after a recent deploy. You need to identify the failing path, scope the impact, and find the regressing commit.

**1. Start the structured investigation prompt.**

```json
{
  "method": "prompts/get",
  "params": {
    "name": "investigate_incident",
    "arguments": {
      "service": "checkout-service",
      "symptom": "POST /checkout returning 500 after deploy"
    }
  }
}
```

The prompt returns an ordered sequence of tool calls to run. Follow it, but the steps below capture the essential pattern.

**2. Check staleness before trusting any results.**

```json
{ "name": "staleness_report", "arguments": { "service": "checkout-service" } }
```

Returns: `{ "reingest_recommended": false, "indexed_commit": "a3f1c9d", "current_commit": "a3f1c9d" }` — index is current, proceed.

**3. Find the HTTP surface.**

```json
{ "name": "entrypoints", "arguments": { "service": "checkout-service" } }
```

Returns: `{ "entrypoints": [{ "name": "HandleCheckout", "file_path": "internal/handler/checkout.go", "line": 42 }, ...] }` — `HandleCheckout` is the likely entry point.

**4. Trace callers of the failing handler to understand how it is reached.**

```json
{ "name": "find_callers", "arguments": { "service": "checkout-service", "symbol": "HandleCheckout" } }
```

Returns: `{ "callers": [{ "name": "RegisterRoutes", "file_path": "internal/server/routes.go" }] }` — one router registration, no unexpected callers.

**5. Scope the blast radius of the files changed in the deploy.**

```json
{
  "name": "blast_radius",
  "arguments": {
    "service": "checkout-service",
    "changed_files": [
      "internal/payment/processor.go",
      "internal/handler/checkout.go"
    ]
  }
}
```

Returns: `{ "symbols": ["ProcessPayment", "HandleCheckout", "ValidateCart", ...], "tests": ["TestProcessPayment", "TestHandleCheckout_Error"], "files": ["internal/payment/processor.go", "internal/handler/checkout.go", "internal/cart/validate.go"] }` — `ValidateCart` is also in the blast radius, which is unexpected and worth investigating.

**6. Find the regressing commit.**

```json
{
  "name": "history",
  "arguments": {
    "service": "checkout-service",
    "symbol": "ValidateCart",
    "limit": 10
  }
}
```

Returns the 10 most recent commits that touched `ValidateCart`'s file, newest-first, with dates, authors, and messages. The most recent commit message reads "refactor: simplify cart validation error handling" — a plausible regression site.

**7. Check for doc-vs-code contradictions in the checkout concern.**

```json
{
  "name": "contradiction_scan",
  "arguments": { "service": "checkout-service", "concern": "checkout" }
}
```

Returns: `{ "contradictions": [{ "symbol": "ValidateCart", "kind_a": "GoFunc", "kind_b": "GoMethod", "type": "kind_conflict" }] }` — `ValidateCart` appears as both a function and a method across sources. This signals the refactor changed its signature in a way the index has partially captured.

**8. Map the community around `ValidateCart` to see what else is entangled.**

```json
{ "name": "community", "arguments": { "service": "checkout-service", "symbol": "ValidateCart" } }
```

Returns: `{ "members": ["HandleCheckout", "CartService", "ErrInvalidCart", "TestValidateCart", ...] }` — confirms `HandleCheckout` is a direct caller, giving you a focused read list.

At this point you have: the failing entry point, the blast radius of the deploy, the regressing commit, a contradiction that explains the runtime confusion, and a tight symbol community to read. That is enough to open the right files with full context.

---

## Setup & notes

**Profile.** Set `investigation` before running these tools. It activates the investigation group (which includes `find_callers`, `blast_radius`, `history`, `contradiction_scan`, `evidence_summary`, `community`, `traverse`) plus the shared group. Some tools (`entrypoints`, `staleness_report`) are available in any profile. (`set_profile` is available on stdio `serve-mcp` only; over the shared HTTP daemon, register against `POST /rpc/investigation` instead.)

```json
{ "name": "set_profile", "arguments": { "profile": "investigation" } }
```

**`contradiction_scan` requires `concern`.** This is not optional. Pass the narrow concern that matches the symptom (e.g. `"checkout"`, `"payment"`, `"auth"`). A missing concern is a hard error.

**Staleness.** A long-running `serve-mcp` process can outlive commits. Always call `staleness_report` first. If `reingest_recommended` is `true`, refresh by calling the `ingest` MCP tool in-session — do not use the CLI `ingest` while the session holds the DuckDB write lock.

```json
{ "name": "ingest", "arguments": { "service": "checkout-service" } }
```

**`history` resolves symbols to files.** Pass `symbol` to filter history to commits touching that symbol's source file. You can also filter by `file` (suffix match) or `spec` (spec ID). Results are limited to 500 commits ingested at ingest time; commits since the last ingest are not visible.

**`traverse` for deeper edge walks.** If `find_callers` does not surface the call path you expect, use `traverse` with `kinds: ["calls"]` and `direction: "in"` from the suspect symbol. It returns up to 200 BFS paths and sets `truncated: true` when capped.

**`evidence_summary` before acting.** If the summary shows a high proportion of `assumed` or `doc-claimed` tiers, treat tool results as orientation rather than ground truth, and verify with direct file reads before concluding.

---

## Maintained

This doc covers: `investigate_incident`, `entrypoints`, `find_callers`, `blast_radius`, `history`, `contradiction_scan`, `evidence_summary`, `community`, `traverse`, `staleness_report`, `set_profile`, `ingest`. Update it when these change.
