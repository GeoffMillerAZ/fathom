# Incident investigation

Triage a production symptom fast using pre-computed call graphs and complexity hotspots — without reading the codebase cold.

---

## When to use this

- A service is returning errors or exhibiting unexpected behavior in production and you need to identify the blast radius and likely cause quickly.
- You have a symptom (error message, failing endpoint, slow query) but no immediate pointer to which code path is involved.
- Documentation or comments appear to contradict what the code actually does, and you want to surface those conflicts before drawing conclusions.

---

## Tools

| Name | Kind | What it gives you |
|---|---|---|
| `entrypoints` | MCP tool | Main functions, HTTP handlers, CLI root commands — the surface most likely to be the entry to the failing path |
| `localize` | MCP tool | "Which file is this in" from a stack trace, a failing test name, an error string, or a bare description — seeds resolve, then expand two hops through the call graph, ranked and cited, before you have a symbol name to hand `find_callers` |
| `find_callers` | MCP tool | All symbols that call a named symbol; traces a function up through its callers |
| `blast_radius` | MCP tool | Given a set of changed files, returns all transitively affected symbols, test functions, and file paths |
| `contradiction_scan` | MCP tool | Scans concern-scoped query results for symbols with conflicting kinds or doc-vs-code contradictions (concern **required**) |
| `evidence_summary` | MCP tool | Per-tier observation counts (`verified`, `pre-scan`, `doc-claimed`, `sme-stated`, `assumed`) — signals how much to trust the index |
| `community` | MCP tool | Related symbols: callers, callees, and co-located symbols in the same package or file |
| `findings_near` | MCP tool | Prior review/audit findings whose scope lies within N call-graph hops of a symbol or file, ranked by proximity then recency — "has anyone seen this before?" (docs/design/findings-as-evidence.md) |
| `memory_search` | MCP tool | Anchored memories touching a file or matching a query, each labeled `tier` and `state` — what an earlier session learned about this code, and whether its anchor has since drifted (**experimental**, unmeasured) |
| `staleness_report` | MCP tool | Whether the index has fallen behind HEAD; `reingest_recommended: true` means results may be stale |
| `fathom findings near\|status` | CLI | Store-direct twin of `findings_near` (`--hops`, `--include-superseded`), and the findings layer's entry-condition progress (`status`) — the layer is **experimental** until ≥15–20 resolved findings exist |
| `fathom memory add\|list\|review\|show` | CLI | File what the investigation established as an anchored memory (symbol/file/commit anchor + evidence kind); `review` triages `suspect`/`orphaned` ones |
| `fathom scope list\|preview` | CLI | See what the index deliberately hides: any tool answer narrowed by a suppression carries a `suppressed` block naming the reason and the reversing command |

---

## How it enhances Claude Code

Without fathom, investigating an incident means cold-reading unfamiliar code, grepping for handler names, and manually chasing call chains — an expensive, error-prone process that burns large amounts of context.

With fathom:

- **Concern scoping** keeps tool results focused. Passing `concern: "checkout"` to `contradiction_scan` means you get contradictions in the checkout concern only — not a fleet-wide data dump. This is the ~77% token-reduction mechanism described in ADR-006.
- **Pre-computed call graphs** let `find_callers` and `blast_radius` answer "what does this touch?" in a single tool call rather than a grep sweep across dozens of files.
- **`contradiction_scan`** surfaces doc-vs-code drift that might explain why the code is not behaving as the runbook says, without requiring you to read both documents manually.
- **`evidence_summary`** tells you up front whether the index data is primarily verified (parsed from source) or assumed (lower-confidence), so you calibrate how much to trust the tool results before acting on them.
- **`localize`** closes the gap before you have a symbol name at all: paste the stack trace, the failing test, or the error text, and it seeds from parsed frames/literals/test names, then walks the typed call graph two hops out from every resolved seed (ranked by evidence, graph distance, resolved-over-heuristic, then how many seeds converge on it) before `find_callers`/`blast_radius` have anything to grab onto. When the symptom is too vague to seed anything (no frame, no quoted literal, no matching function name), it reformulates from identifier splitting and file basenames rather than coming back empty — never a model call, always logged as its own hop.

Together these move the investigation from open-ended code reading to a directed sequence of structured queries, letting Claude Code close in on a root cause faster and with less context drift.

---

## Worked example

Scenario: `checkout-service` is returning HTTP 500 on `POST /checkout` after a recent deploy. You need to identify the failing path and scope the impact.

**1. Check staleness before trusting any results.**

```json
{ "name": "staleness_report", "arguments": { "service": "checkout-service" } }
```

Returns: `{ "reingest_recommended": false, "indexed_commit": "a3f1c9d", "current_commit": "a3f1c9d" }` — index is current, proceed.

**2. Find the HTTP surface.**

```json
{ "name": "entrypoints", "arguments": { "service": "checkout-service" } }
```

Returns: `{ "entrypoints": [{ "name": "HandleCheckout", "file_path": "internal/handler/checkout.go", "line": 42 }, ...] }` — `HandleCheckout` is the likely entry point.

**3. Trace callers of the failing handler to understand how it is reached.**

```json
{ "name": "find_callers", "arguments": { "service": "checkout-service", "symbol": "HandleCheckout" } }
```

Returns: `{ "callers": [{ "name": "RegisterRoutes", "file_path": "internal/server/routes.go" }] }` — one router registration, no unexpected callers.

**4. Scope the blast radius of the files changed in the deploy.**

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

**5. Check for doc-vs-code contradictions in the checkout concern.**

```json
{
  "name": "contradiction_scan",
  "arguments": { "service": "checkout-service", "concern": "checkout" }
}
```

Returns: `{ "contradictions": [{ "symbol": "ValidateCart", "kind_a": "GoFunc", "kind_b": "GoMethod", "type": "kind_conflict" }] }` — `ValidateCart` appears as both a function and a method across sources. This signals the refactor changed its signature in a way the index has partially captured.

**6. Map the community around `ValidateCart` to see what else is entangled.**

```json
{ "name": "community", "arguments": { "service": "checkout-service", "symbol": "ValidateCart" } }
```

Returns: `{ "members": ["HandleCheckout", "CartService", "ErrInvalidCart", "TestValidateCart", ...] }` — confirms `HandleCheckout` is a direct caller, giving you a focused read list.

At this point you have: the failing entry point, the blast radius of the deploy, a contradiction that explains the runtime confusion, and a tight symbol community to read. That is enough to open the right files with full context.

**7. Check whether this failure class has shown up here before.**

```json
{ "name": "findings_near", "arguments": { "service": "checkout-service", "subject": "ValidateCart", "hops": 2 } }
```

Returns: `{ "found": true, "findings": [{ "claim_id": "...", "class": "error-handling", "rule": "Propagate the cart-validation error instead of swallowing it.", "scope": ["internal/cart/validate.go"], "status": "fresh", "hops": 0, "superseded": false }] }` — a **live** `error-handling` finding scoped directly to the file the refactor just touched: this failure class was already flagged here, unresolved. Pass `include_superseded: true` to also see PRECEDENT — findings already fixed nearby, each labeled `superseded: true` so it never reads as still live. This is the query only fathom can answer: it already has the call graph and the review history in the same store.

---

## Setup & notes

**Surface.** There is no profile switch (`set_profile` was removed, SW-2). stdio `serve-mcp` serves the full registry; over the shared HTTP daemon the path is the scope — the investigation tools (`find_callers`, `blast_radius`, `contradiction_scan`, `evidence_summary`, `community`, `findings_near`) are served by bare `POST /rpc` since HF-5 deleted the group endpoints; `POST /rpc/agent` lists the agent allowlist (`entrypoints`, `find_callers`, `staleness_report`, `memory_search`, `ingest` are on it). An off-surface call still executes, logged `off_surface:true` — never refused. The `investigate_incident` canned prompt this section used to reference is gone (SR-20, surface reckoning F-78, zero lifetime calls); `prompts/list` now answers empty everywhere. `history` and `traverse` were unregistered (SR-12, surface reckoning F-22): 0 lifetime calls each.

**"Did we already know this?"** Before reading cold, ask `memory_search { "service": "checkout-service", "file": "internal/cart/validate.go" }` — a `fresh` memory is an earlier session's proven claim about this code; a `suspect` one names the anchor that moved since it was confirmed. When the investigation settles a fact worth keeping, `fathom memory add … --anchor file:internal/cart/validate.go --evidence machine-check:<test>` files it; an unanchored note is refused.

**What the index hides on purpose.** A `suppressed` block in any answer means a `fathom scope suppress` narrowed it — `fathom scope list` shows the id and reason, `fathom scope unsuppress <id>` reverses it. Nothing is hidden silently (law 3).

**`contradiction_scan` requires `concern`.** This is not optional. Pass the narrow concern that matches the symptom (e.g. `"checkout"`, `"payment"`, `"auth"`). A missing concern is a hard error.

**Staleness.** A long-running `serve-mcp` process can outlive commits. Always call `staleness_report` first. If `reingest_recommended` is `true`, refresh by calling the `ingest` MCP tool in-session — a concurrent CLI `ingest` can contend with the session's own write (SQLite serializes writers, ADR-020) and fail past `busy_timeout`.

```json
{ "name": "ingest", "arguments": { "service": "checkout-service" } }
```

**`evidence_summary` before acting.** If the summary shows a high proportion of `assumed` or `doc-claimed` tiers, treat tool results as orientation rather than ground truth, and verify with direct file reads before concluding.

---

## Maintained

This doc covers: `entrypoints`, `localize`, `find_callers`, `blast_radius`, `contradiction_scan`, `evidence_summary`, `community`, `findings_near`, `memory_search`, `staleness_report`, `ingest`, `fathom findings` (`near`/`status`), `fathom memory` (`add`/`list`/`review`/`show`), `fathom scope` (`list`/`preview`/`suppress`/`unsuppress`). Update it when these change.
