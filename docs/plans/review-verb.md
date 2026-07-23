# Phase C — The review verb (vision 11 §3.C)

**C1/C2 are blocked on C3, an operator decision** (pre-push hook change):
accept a non-blocking `fathom review --since origin/main` summary in the
pre-push gate + rewrite the review_change MCP prompt to call the composed
verb. Until C3 is accepted, C1/C2 rows stay `[ ]` and are not built —
nothing ships spec-only (vision 11 anti-move discipline). C4 is NOT gated
and proceeds now.

Design (normative):
- **C4 rule packs.** `schema/rules/archconformance/schema.cue` — #Rule
  {from_pkg, to_pkg, allow}, #RuleSet {rules, deny_by_default: bool|*false}
  — with `testdata/valid.cue` + commented `invalid.cue` per repo convention.
  Loader `internal/projectrules` reads `.fathom/rules/archconformance.cue`
  from a service's corpus root; `arch_conformance` uses loaded rules when
  the caller passes none, and reports WHICH source it used (inline args |
  project rules | none). Fathom's own three-layer boundary (connector ↛
  query, extractor ↛ mcp, analysis packages pure) ships as this repo's own
  `.fathom/rules/archconformance.cue` — dogfood from day one.
- **C4 honesty audit.** "Empty config reads as clean" is a confirmed
  never-silent violation in arch_conformance (zero rules → {"violations":
  [], "count": 0}, indistinguishable from a real clean scan). Audit every
  deterministic analyzer tool for the class; each gets either a
  schema-required config param (license_policy's proven pattern) or an
  explicit `checked: false, reason: "no rules configured"` field in its
  response. No tool may return a clean-shaped answer for work it didn't do.
- **C1 review_diff** (blocked): one pure-Go composed verb — diff impact
  (typed edges) + blast radius + obligations + arch conformance + spec
  coverage — one cited verdict, CLI mirror, the program's single new MCP
  tool. **C2 findings bridge** (blocked): review findings become
  content-addressed ClaimIDs via FT-7's pre-armed trigger; refuted findings
  stay refuted across re-reviews.

## Tasks

- [x] archconformance rule-pack CUE schema + valid/invalid testdata, Go-test-asserted negatives {id: C4-1, symbol: Check, package: internal/analysis/archconformance}
- [x] internal/projectrules loader: .fathom/rules/ discovery, CUE validation, source attribution; unit-tested incl. malformed-file refusal {id: C4-2, symbol: Check, package: internal/analysis/archconformance, skip_test: true, skip: "loader is a new package whose consumer anchor is the analyzer it feeds; its own tests are the coverage — anchor shared with C4-1 pending the package landing"}
- [x] arch_conformance consumes project rules when args omit them; response reports rule source; fake-backend test covers all three source states {id: C4-3, symbol: registerArchConformance, package: internal/mcp}
- [x] Fathom's own .fathom/rules/archconformance.cue encoding the three-layer boundary; violation caught by test fixture {id: C4-4, symbol: Check, package: internal/analysis/archconformance, skip_test: true, skip: "config-as-data row; C4-3's tests exercise the load path against this file"}
- [x] Honesty audit across all deterministic analyzer tools: every empty-config case returns checked:false+reason or requires config by schema; table of findings recorded here {id: C4-5, symbol: registerArchConformance, package: internal/mcp, skip_test: true, skip: "audit row — per-tool fixes carry their own tests; the audit table is the deliverable"}
- [ ] (op-gated: C3) Pre-push non-blocking review summary + review_change prompt rewrite {id: C3-1, symbol: registerArchConformance, package: internal/mcp, skip_test: true, skip: "blocked on operator channel acceptance; not built until then"}
- [ ] (blocked on C3) review_diff composed verb + CLI mirror {id: C1-1, symbol: registerArchConformance, package: internal/mcp, skip_test: true, skip: "blocked on C3; anchor updated to the new package when built"}
- [ ] (blocked on C3) findings-as-claims bridge via FT-7 {id: C2-1, symbol: registerArchConformance, package: internal/mcp, skip_test: true, skip: "blocked on C3"}

## C4-5 audit

Enumerated from `internal/mcp` registrations (`extra_tools.go`, `source_tools.go`,
`analysis_tools.go`, `nav_tools.go`) cross-checked against `docs/mcp-tools.md` (which
is itself missing most of `extra_tools.go`'s tools — a pre-existing catalog gap, not
introduced here, and out of this row's scope to backfill). The audited class is
specifically **operator-supplied policy/rules config that can legitimately be
omitted** — not general "empty index returns empty results" (real absence of data is
already honest; the bug class is a *config* absence dressed up as a clean scan).

### Tools with a real config surface

| Tool | Empty-config behavior before | Verdict | Fix applied |
|---|---|---|---|
| `arch_conformance` | `rules` optional; omitted/empty → `archconformance.Check(edges, nil)` → `{"violations":[],"count":0}`, structurally identical to a real clean scan. | **Bug (confirmed instance).** | C4-2/C4-3: falls back to the service's `.fathom/rules/archconformance.cue` project rule pack when no inline rules are passed; when neither inline nor project rules exist, response adds `rule_source:"none"` and `checked:false` + `reason` (never runs `Check` in that state, so `violations`/`count` can't be mistaken for a real scan). `rule_source` is `"inline"` \| `"project"` \| `"none"`. Tests: `internal/mcp/extra_tools_test.go` `TestArchConformance_RuleSource_*`. |
| `license_policy` | `allowed` is schema-`required`; the call fails with `ErrCodeParams` before any scan runs if omitted. | **Already safe** (the model pattern C4-5 cites). | none |
| `security_rollup` | `allowed_licenses` optional; empty → `license.Policy(detected, nil)` **fails closed**: `license_ok:false` with explicit reason `"license %q is not permitted: no licenses are allowed"` (`internal/analysis/license/license.go:109`) — never a false "ok". `secrets`/`shell_outs`/`http_calls` are unconditional real scans with no config surface. | **Already safe** — fails closed with a stated reason instead of a clean-shaped answer. | none |
| `migration_targets` | `deprecated_pkgs` optional; empty → `migrationtargets.DefaultDeprecatedPkgs` (a real, non-empty, hardcoded list) is used — never an unconfigured no-op. | **Safe for the empty-config case.** Separate finding: a *non-empty* caller-supplied list **replaces** rather than merges with the default list, contradicting the tool's own parameter description ("default list is always included") — a different bug class (under-report on custom config, not false-clean on absent config); fixing it requires updating `migrationtargets_test.go`'s existing replace-semantics assertions, out of scope for this pass. | none (logged as `docs/dogfood-observations.md` O11) |
| `dead_code` | `entrypoints` optional; empty → BFS reachable-set seed is empty, so every symbol with zero callers is flagged "dead" (over-report: real entrypoints with no in-repo caller look dead too). | **Different failure mode — not this bug class.** Over-reporting, never a false-clean answer; not in scope for a "clean-shaped answer for work it didn't do" fix. | none |
| `god_object` | `method_threshold`/`field_threshold` optional; `<=0` → `godobject.Config.methodMin`/`fieldMin` default to a real `10` (`internal/analysis/godobject/godobject.go:41-51`). | **Safe** — real non-empty default, a check still runs. | none |
| `traceability` | `prefixes` optional; empty → `sdd.ExtractRefs` uses its own built-in non-empty default pattern (US/EP/FR/NFR etc.), not "match nothing". | **Safe** | none |
| `spec_refs` | `pattern` optional; empty → built-in default regex covering `[US/EP/FR/NFR-N]`, `MN.N`, `doc N`. | **Safe** | none |
| `get_go_symbols` | `kinds` optional; empty → no filter applied (all kinds returned) — "no filter requested" semantics, not a policy check that silently no-ops. | **N/A** — not a policy/rules config. | none |
| `spec_coverage`, `spec_status` | `spec_ids` schema-`required`. | **Already safe** | none |
| `propagate_concern` | `concern` + `seed` schema-`required`. | **Already safe** | none |
| `contradiction_scan`, `pack_context` | `concern` schema-`required`. | **Already safe** | none |
| `import_cycles` | `pkg_imports` schema-`required`. | **Already safe** | none |
| `cross_repo_blast_radius` | `symbol` schema-`required`. | **Already safe** | none |

### Tools with no config surface (data-driven only — N/A for this audit class)

These take only `service` (or `service` + a required target like `symbol`/`file`/
`changed_files`) and run a fixed, hardcoded check against whatever source/symbols are
indexed. There is no operator-supplied policy parameter that can be silently absent, so
"empty config reads as clean" does not apply; an empty result here honestly means "no
data matched," not "no rules were checked." Listed for completeness per the audit's
enumeration requirement:

`naming`, `struct_tags`, `err_audit`, `sql_map`, `field_usage`, `clones`, `doc_coverage`,
`stale_docs`, `api_diff`, `dep_drift`, `package_deps`, `hotspots`, `secret_scan`,
`shell_out`, `http_client_calls`, `antipatterns`, `cognitive`, `cloc`, `deprecation`,
`env_keys`, `markers`, `routes`, `dependencies`, `endpoint_inventory`, `complexity`,
`api_surface`, `community`, `type_hierarchy`, `rename_impact`, `reachable`, `call_path`,
`test_impact`, `proto_contract`, `proto_conformance` (hardcoded naming convention, no
external policy), `schema_export`, `license` (detection only, no policy).

### Summary

One confirmed instance (`arch_conformance`), fixed with the `rule_source`/`checked`
contract this row's design section specifies. One already-safe tool matching the cited
`license_policy` pattern exactly, plus `security_rollup`'s fail-closed license check
(a second, independently-arrived-at safe pattern worth noting for future config-bearing
tools: fail closed with a reason, not fail open with a clean shape). One adjacent but
distinct bug found and logged, not fixed in this pass (`migration_targets` O11). No other
deterministic analyzer tool in the current registry carries an omittable policy/rules
parameter.
