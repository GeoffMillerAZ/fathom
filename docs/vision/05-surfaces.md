# 05 — MCP surface cleanup for agent ergonomics

**Verdict.** Collapse five named surfaces to **two** — one opinionated
15-tool **agent** surface that the user-scope MCP registration actually
points at, and the full expert registry one endpoint away — and change the
gate from reject-with-error to **execute-and-flag**. The call log already
decided membership: only 16 of 87 tools were ever called; the curated
companion list is almost exactly right but sits unused because the primary
client isn't registered against it; and the top non-hook tool
(`verify_plan`, 30 calls) isn't even on it.

## 1. The three facts that decide everything

1. **87 tools registered; 16 ever called; ~50 non-hook calls in 8 days.**
   Top non-hook tool: `verify_plan` — not on the companion surface.
   Provenance caveat from the critique pass: the verify CLI calls the
   `verify_plan` tool over `/rpc` and the pre-push gate runs that CLI, and
   the log has no origin field — so an unknown share of those 30 is gate
   plumbing, not agent tool-picking. The promotion argument stands on
   should-be-callable grounds either way, but **origin tagging must land
   before any kill clock runs** ([04 Gap 1](04-coding-agents.md)).
2. **The multi-surface design is right in shape, dead in practice.**
   Companion saw 11 calls, all on launch day, including one gated bounce
   (`secret_scan`) — the agent kept using companion that day, then *all*
   agent-chosen traffic went dark, and abandonment vs no-sessions is
   unattributable (Gap 1 again). Since then the only companion call was a
   second gated bounce (`capabilities`, a week later; log dates are UTC).
   What's certain: the primary client is registered against full `/rpc`, so
   companion has no default consumer; the gate has bounced both agents that
   ever reached for an off-surface tool; and the four group surfaces
   (`coding`/`investigation`/`shared`/`platform`) have zero observed
   traffic ever — they model fathom's internal taxonomy, not any consumer's
   workflow.
3. **Argument inconsistency is a measured failure.** `find_symbol` takes
   `name`, `find_callers` takes `symbol`, `references` takes `query`; two
   of three `references` calls failed before one succeeded. And
   `obligations` called with `service:"sextant"` silently returns empty
   (the indexed service is `fathom`) — silent-empty on a wrong service name
   is a trust-killer.

## 2. Target topology

- **`POST /rpc/agent`** — the primary surface, 15 tools (§3). The
  user-scope HTTP MCP registration points here. Renaming companion→agent is
  optional; **re-pointing the registration is not — that one config line is
  the whole ballgame.** Companion failed for want of exactly this.
- **`POST /rpc`** — the full expert registry, unchanged. Hooks keep hitting
  it directly.
- Delete the four group surface paths. Delete `set_profile` too — noting
  honestly that this is a further amendment: ADR-014 kept it valid on stdio
  (advisory list-filtering), so killing it entirely amends the ADR-010/014
  stdio clause, not just dead HTTP weight. No stdio consumer exists today.
- The specced **`/rpc/expert`** surface (expert-modules.md §6) is *not*
  one of the taxonomy surfaces being deleted — it stays a spec-only
  commitment ([03 §5](03-findings-and-experts.md)) until a non-code module
  is served to someone. Two surfaces today; expert is entry-conditioned.

**Gate semantics: execute-and-flag, not reject.** ADR-014's hard gate was
built to keep the surface honest; both agents that ever reached for an
off-surface tool got an error back, and errors produce avoidance — which
looks identical to no-demand in the log. For a solo-operator daemon,
isolation is not the threat model — measurement is. A logged
`off_surface:true` pass-through measures demand. Keep hard gating behind a
flag for any future multi-tenant shape. **This amends ADR-014 — write the
amendment**, and record two adjacent facts in it: (a) surfaces are
ergonomics + measurement, never security — bare `/rpc` already serves all
87 tools unauthenticated, including `ingest` (writes) and `compile_layer`
(LLM spend); (b) therefore change `serve-http`'s **default** addr from
`:7575` (all interfaces) to `127.0.0.1:7575` in the same change — today
only the launchd plist override makes it loopback. `surface_test.go`'s
hard-gate assertions flip to flag assertions.

## 3. Keep / kill / merge

**AGENT SURFACE — 15 tools** (companion 15, minus `batch` and `brief`,
plus `verify_plan` and `capabilities` — the arithmetic is 15−2+2=15):

| Tool | Disposition | Rationale |
|---|---|---|
| find_symbol, references, query_symbols | keep | the lookup trio; called; tiered find_symbol is the house style |
| symbol_context, find_callers | keep | def+graph in one call; the hot graph question |
| outline, repo_map, pack_context, entrypoints | keep | orientation loop |
| ingest, staleness_report, list_services | keep | freshness loop; all called |
| obligations | keep | 1,070 calls (hook) + agent-callable |
| **verify_plan** | **promote** | top non-hook tool (30 calls, some share gate plumbing); agents should be able to call the headline tool from the primary surface |
| **capabilities** | **promote** | agents demonstrably reach for it; the discovery escape hatch that makes a small surface safe |
| brief | demote to expert | zero calls; overlaps repo_map+pack_context |
| batch | demote; kill at next 30-day review if still zero | never called once; MCP clients parallelize natively |

**KILL NOW (zero calls across the whole log — the evidence the kills were
gated on):** the 4 alias pairs — `platform_callers`, `test_debt`,
`api_routes`, `deps` (keep canonicals `cross_repo_callers`, `markers`,
`routes`, `dependencies`) — plus `set_profile`. `get_go_symbols`: verify
redundancy with `outline`, then fold or delete.

**MERGE (lazily, on the expert surface, only where naming overlap causes
wrong-tool choice):**

| Family | Into |
|---|---|
| call_hierarchy, call_path, traverse, reachable, find_callees | one `callgraph {symbol, direction, depth, to?}`; find_callers stays standalone on the agent surface |
| complexity, cognitive, hotspots | `complexity {metric}` |
| secret_scan, shell_out, http_client_calls, license, license_policy | `security_rollup` stays the entry point; individuals expert-only |

**DEMOTE, DON'T MERGE — the other ~45 zero-call analysis/spec/proto/fleet
tools:** leave them on `/rpc` untouched and start the 30-day
generalize-or-kill clock *from the registration flip, with origin tagging
already in place* (the current log measures abandonment, not demand, and
cannot tell hooks from agents). Don't spend effort consolidating tools
with zero demonstrated demand. After 30 days of a correctly registered
agent surface, delete what's still at zero; expect the registry to land
near 35–45. Membership stays a three-place contract change (surface.go +
surface_test.go + docs/mcp-tools.md).

## 4. Conventions — what makes 15 tools feel like 8

- **One argument vocabulary:** `service`, `symbol`, `file`, `plan`, plus
  tool-specific extras. `find_symbol.name` → `symbol`; decide whether
  `references` is symbol-lookup (`symbol`) or free-text (`query`) and say
  so in its description. Accept old names as silent aliases for one release.
- **`service` optional with resolution, never silent-empty:** one service
  in the store → use it; unknown service → `{"error":"unknown service
  \"sextant\"","did_you_mean":["fathom"],"services":[...]}`. The edit-hook
  already does alias resolution client-side; move it into the server so
  every caller benefits. **The single highest-leverage trust fix in the
  log.**
- **Uniform response envelope:** every agent-surface response carries
  `service`, the index watermark, and the constant-size staleness flag
  `query_symbols` already has. An agent should never need a second call to
  know whether an answer is fresh.
- **Tiered verbosity as a convention:** standardize `detail:
  "summary"|"full"` (default summary) on every list-returning tool, atop
  the 16KB budget clamp. find_symbol's tier pattern (exact → fold →
  substring → did_you_mean, `matched_via` reported) is the house style.
- **Errors must be machine-followable:** every error names the exact next
  call — the log proves agents follow results, not prose redirects.
- **Discovery is a tool, on the surface:** `capabilities` returns the
  compact full-registry catalog (name, one-liner, surface, arg summary — no
  schemas). Not a resource (nothing reads MCP resources in practice), not
  only docs. Progressive disclosure: 16 schemas up front, 87 names on
  demand, any expert tool one `/rpc` call away.

## 5. Sequencing, and what NOT to do

- **Week 1:** alias + set_profile kills; verify_plan/capabilities
  promotion; registration flip to `/rpc/agent`; execute-and-flag gate
  (ADR-014 amendment).
- **Week 2:** arg-name unification + server-side service resolution +
  response envelope.
- Then let the 30-day clock run and delete at zero.

Do **not** build per-use-case surfaces for investigation/review — those
workflows arrive as expert modules and prompts; taxonomy-shaped surfaces
demonstrably attract no traffic. Do **not** invest in batch semantics.

Caveat, held honestly: 8 days of solo dogfooding with hooks dominating is a
thin sample — which is why demotions are reversible and only the aliases,
whose redundancy is structural, die outright today.
