# surface prune — the kill clock ran; the registry pays rent or leaves

> **Archived** 2026-10-10T05:37:54Z · HEAD `a21080acab36bc06f74a0919cfab5631d5655f8a` · 3 tasks total; 3 claimed (3 verified, 0 failing); 0 skipped

> Governing source: `docs/vision/05-surfaces.md` §3 (the registered rule:
> "after 30 days of a correctly registered agent surface, delete what's
> still at zero") and §5. The agent surface itself (16 tools) is NOT in
> scope — this prunes the never-called expert registry behind it.
>
> **Covenant (ADR-0070 class):** failing test first, `task validate`
> green before commit, dated CHANGELOG entry, boxes flip only on
> `fathom verify --service fathom --plan docs/plans/surface-prune.md`.
>
> **Frozen evidence (2026-08-23, call log 31,034 calls, all surfaces,
> whole history):** 59 of 88 registered tools have zero calls ever:
> antipatterns, api_diff, api_surface, arch_conformance, batch, brief,
> call_hierarchy, call_path, clones, cognitive, community, complexity,
> cost_report, cross_repo_blast_radius, cross_repo_callers, dep_drift,
> dependencies, deprecation, diff_impact, doc_coverage,
> endpoint_inventory, env_keys, err_audit, estimate_cost, field_usage,
> findings_near, fleet_drift, get_go_symbols, god_object, history,
> hotspots, http_client_calls, import_cycles, license, license_policy,
> markers, migration_targets, naming, package_deps, propagate_concern,
> proto_conformance, proto_contract, query_cross_service, reachable,
> rename_impact, rename_service, review_diff, routes, schema_export,
> security_rollup, shell_out, spec_coverage, spec_status, sql_map,
> stale_docs, struct_tags, traceability, traverse, type_hierarchy.

## The prune

- [x] every frozen-list tool that survives a two-part exemption check is deleted from the registry: exempt only if (a) fathom's own code calls it internally (hook, gate, observatory, CLI plumbing — grep proves it), or (b) a shipped skill, the stanza, or a use-case doc directs an agent to call it by name; each exemption is recorded in this plan's Exemptions section with its evidence, and everything else goes — registration, handler wiring, and schema {id: SP-B1, symbol: registerExtraTools, package: internal/mcp, reachable_from: TestRegistryCarriesNoZeroCallTools}
- [x] the three-place membership contract holds after the cut: surface registration, its registry test, and docs/mcp-tools.md agree on the surviving set, and no use-case doc's Maintained line names a deleted tool — asserted by test so the catalog cannot drift from the registry {id: SP-B2, symbol: TestMcpToolsDocMatchesRegistry, package: internal/mcp, skip_test: true, skip: "doc-sync row — the acceptance evidence IS the registry/doc assertion test itself"}
- [x] `instrument --adoption` gets the IT-1 read-error classification (observation O17): an unreadable call log renders as a read error with the errno and the external-volume wedge hint, never as 'zero-call registered tools (88 of 88)' — universal-zero adoption from a wedged reader becomes impossible {id: SP-B3, symbol: classifyCallLogRead, package: cmd/fathom, reachable_from: TestAdoptionNamesReadErrorNeverUniversalZero}

## Exemptions (filled during SP-B1, with evidence)

25 of the 59 frozen names passed the two-part check and were kept; the
other 34 were deleted (registration, handler, schema, and every helper
reachable only from a deleted handler — `internal/mcp/source_tools.go`,
`nav_tools.go`, `analysis_tools.go`, `platform_tools.go`, `extra_tools.go`).
All 25 exemptions below are check (b) — a shipped skill or a docs/use-cases
worked-example instruction directing an agent to call the tool by name;
none passed check (a) (no internal fathom code — hook, gate, observatory,
CLI plumbing — calls any of the 59 as an MCP tool; `cmd/fathom/report.go`'s
`--analyzer security_rollup` flag value and `fathom purge history`'s
subcommand name are the two near-misses grep surfaced, and both are
same-string CLI concepts that reuse the underlying analyzer/verb directly
rather than calling the deleted MCP tool — not exemptions).

| Tool | Evidence |
|---|---|
| `api_diff` | `docs/use-cases/change-review.md` Appendix worked example: `{"name":"api_diff","arguments":{...}}` |
| `arch_conformance` | `docs/use-cases/change-review.md` Appendix worked example: `{"name":"arch_conformance","arguments":{...}}` |
| `batch` | `docs/use-cases/coding-context.md` worked example step 6: `batch {"calls":[...]}` |
| `brief` | `docs/use-cases/coding-context.md` worked example step 5: `brief {...}` |
| `community` | `docs/use-cases/incident-investigation.md` worked example step 8: `{"name":"community","arguments":{...}}` |
| `complexity` | `docs/use-cases/fleet-health.md` worked example step 3: `{"name":"complexity","arguments":{...}}` |
| `cross_repo_blast_radius` | `docs/use-cases/cross-repo-blast-radius.md` worked example: `{"name":"cross_repo_blast_radius","arguments":{...}}` |
| `cross_repo_callers` | `docs/use-cases/cross-repo-blast-radius.md` worked example step 2: `{"name":"cross_repo_callers","arguments":{...}}` |
| `dep_drift` | `docs/use-cases/change-review.md` Appendix worked example: `{"name":"dep_drift","arguments":{...}}` |
| `diff_impact` | `docs/use-cases/change-review.md` + `cross-repo-blast-radius.md` worked examples: `{"name":"diff_impact","arguments":{...}}` |
| `findings_near` | `docs/use-cases/incident-investigation.md` worked example step 9: `{"name":"findings_near","arguments":{...}}` |
| `fleet_drift` | `docs/use-cases/fleet-health.md` worked example step 5: `{"name":"fleet_drift"}` |
| `god_object` | `docs/use-cases/fleet-health.md` worked example step 4: `{"name":"god_object","arguments":{...}}` |
| `history` | `docs/use-cases/incident-investigation.md` worked example step 6: `{"name":"history","arguments":{...}}` |
| `http_client_calls` | `docs/use-cases/fleet-health.md` worked example step 2: `{"name":"http_client_calls","arguments":{...}}` |
| `proto_conformance` | `docs/use-cases/change-review.md` Appendix worked example: `{"name":"proto_conformance","arguments":{...}}` |
| `reachable` | `docs/use-cases/cross-repo-blast-radius.md` worked example step 6: `{"name":"reachable","arguments":{...}}` |
| `rename_impact` | `docs/use-cases/cross-repo-blast-radius.md` worked example step 5: `{"name":"rename_impact","arguments":{...}}` |
| `rename_service` | `.claude/skills/fathom-rewire/SKILL.md` (+ `dist/agents/skills/fathom-rewire/`): "`rename_service`/`remove_service` fix historical registrations" |
| `review_diff` | `docs/use-cases/change-review.md` worked example step 2: `{"name":"review_diff","arguments":{...}}` — the doc's own headline verb |
| `shell_out` | `docs/use-cases/fleet-health.md` worked example step 2: `{"name":"shell_out","arguments":{...}}` |
| `spec_coverage` | `docs/use-cases/spec-verification.md` worked example step 3 ("the agent calls `spec_coverage`:") |
| `stale_docs` | `docs/use-cases/change-review.md` Appendix worked example: `{"name":"stale_docs","arguments":{...}}` |
| `traceability` | `.claude/skills/fathom-plan/SKILL.md` (+ `dist/agents/skills/fathom-plan/`): "`spec_refs` or `traceability` will confirm the ID is real" |
| `traverse` | `docs/use-cases/incident-investigation.md` Setup & notes: "use `traverse` with `kinds: [\"calls\"]` and `direction: \"in\"`..." |

The remaining 34 (below) had no check (a) or (b) hit anywhere in
`cmd/`, `internal/`, `scripts/`, `.claude/skills/`, `dist/agents/skills/`,
`cmd/fathom/init.go`'s stanza, or `docs/use-cases/*.md`, and were deleted:
`antipatterns`, `api_surface`, `call_hierarchy`, `call_path`, `clones`,
`cognitive`, `cost_report`, `dependencies`, `deprecation`, `doc_coverage`,
`endpoint_inventory`, `env_keys`, `err_audit`, `estimate_cost`,
`field_usage`, `get_go_symbols`, `hotspots`, `import_cycles`, `license`,
`license_policy`, `markers`, `migration_targets`, `naming`,
`package_deps`, `propagate_concern`, `proto_contract`,
`query_cross_service`, `routes`, `schema_export`, `security_rollup`,
`spec_status`, `sql_map`, `struct_tags`, `type_hierarchy`.

Three of fathom's own canned MCP prompts (`investigate_incident`,
`onboard_service`, `plan_requirement` — `internal/mcp/meta.go`) instructed
a caller to invoke `call_hierarchy`, `api_surface`, and `type_hierarchy` by
name; canned prompts are not one of the two exemption tests (they are
neither internal code calling the tool nor a shipped skill/stanza/
use-case doc), so this did not exempt those three tools — but leaving a
prompt's own instructions pointing at a just-deleted tool would be a live
orphan, so the three prompts were rewritten to drop the dead steps
(`call_hierarchy` → `find_callers`; the `api_surface`/`type_hierarchy`
steps removed outright, no equivalent single-call replacement existing on
the surviving registry) and `docs/mcp-tools.md`'s Prompts table updated to
match.

## Explicitly out of scope

- The 16-tool agent surface (all called; membership unchanged).
- Lazy merges (`callgraph`, `complexity {metric}`) from 05 §3 — merging
  is for demanded tools; zero-demand tools are deleted, not consolidated.
- The 29 surviving registered tools keep their 30-day clocks running;
  the next review reads the same instrument.
