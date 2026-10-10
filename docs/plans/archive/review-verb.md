# Phase C — The review verb (vision 11 §3.C)

> **Archived** 2026-10-10T00:54:04Z · HEAD `8415dfd60d7a59f964220ed53617941694bc69be` · 8 tasks total; 8 claimed (8 verified, 0 failing); 0 skipped

**C3 accepted (owner: "yes", 2026-07-23) — C1/C2/C3 all built.** The
operator gate that previously blocked C1/C2 (pre-push hook change: accept a
non-blocking `fathom review --since origin/main` summary in the pre-push
gate + rewrite the review_change MCP prompt to call the composed verb) is
resolved; nothing here shipped spec-only (vision 11 anti-move discipline
honored — C1/C2 stayed `[ ]` until this acceptance landed, then were built
together with C3 in the same slice). C4 shipped first and is unaffected.

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
- **C1 review_diff.** One pure-Go composed verb (`internal/analysis/review`,
  `Run(Input) Verdict`) — diff impact (typed edges via
  `impact.DiffImpactWithTypedEdges`) + obligations/findings scoped to the
  changed files + arch conformance (`archconformance.EdgesFromNodes` +
  `internal/projectrules.ResolveRules`, shared with the standalone
  `arch_conformance` tool so the two never drift) + spec coverage — one
  cited verdict (`Verdict.Dimensions[].Findings[]`, each carrying a
  `citation`). The program's single new MCP tool: `review_diff`
  (`internal/mcp/review_tool.go`, `GroupInvestigation`). CLI mirror: `fathom
  review --store DB --service S --since <ref>` (`cmd/fathom/review.go`,
  mirrors `cmdReport`'s store-direct structure). **C2 findings bridge —
  soundness contract (post-critic round; five proven write-path defects
  fixed).** Review findings become content-addressed ClaimIDs
  (`review.AttachClaims` → `ApplyClaimStatus` → `StaleReviewClaims`,
  composed as `review.BuildPersist` so the MCP tool and CLI share ONE write
  path). The write is DELIBERATE and disclosed: MCP `review_diff` and plain
  `fathom review` persist; `fathom review --no-persist` is a pure report
  and is what the C3 hook uses — a hook is never an undisclosed writer to
  the shared store. What persists and how:
  - **Oracle dimensions only.** `arch_conformance` + `spec_coverage`
    persist; `obligations` NEVER does (it re-serves already-compiled
    claims — re-minting them as wrappers grows the store unboundedly on
    identical re-runs and displaces real obligations from ObligationsFor's
    MaxObligations budget via exact-path scope ranking); `impact`
    info-noise never does. Idempotent: identical re-run ⇒ identical claim
    set (TestReview_Idempotent_DoubleRun,
    TestReview_ObligationsForUnpolluted).
  - **Substrate-sound anchors.** Every persisted cites link carries the
    REAL cited-line hash resolved at emit time; a bare-path/unhashable
    citation gets NO cites link (never a lying one) — so the daemon's
    post-refresh `SweepClaims` produces zero flips and zero fabricated
    "anchor no longer resolves" events over fresh review claims
    (TestReview_SweepZeroFlips, TestAttachClaims_LinkSoundness). Every
    persisted claim carries a scope-fp drift anchor (store fingerprint,
    FT-6); a finding whose file has no fingerprint is not persisted at all
    (TestAttachClaims_NoFingerprintMeansNoPersist — upholding
    DriftedFindings' zero-links invariant, whose comment now names this
    bridge as the second legitimate writer).
  - **Reconcile, no rot.** A persisting run over changed-file set F
    supersedes review-minted claims on files in F its checked dimensions no
    longer emit — absent = fixed; unchecked dimensions never supersede
    (TestReview_Reconcile_FixedFindingSuperseded,
    TestStaleReviewClaims_UncheckedDimensionNeverSupersedes).
  - **Tier admission.** Persisted claims carry tier `verified`
    (deterministic oracle, Model empty — ADR-018 oracle classes / ADR-016
    M1); the findings layer.cue records this admission beside its
    sme-stated corpus tier, pinned by
    TestClaimsVocabularyMatchesSemanticLayer.
  - **Refutation durability** rides the store's own `PutClaims` upsert (no
    re-derived logic) — TestFindingsBridge_RefutationDurability (pure),
    TestReviewDiff_FindingsBridge_RefutationSticksAcrossReReview (MCP),
    TestReview_FindingsBridge_RefutationDurability (CLI, real store).
  **C3 forced channel.** lefthook.yml pre-push gains a non-blocking
  `review` step — `--no-persist`, always exits 0, fail-open on operational
  errors, ~12s worst-case measured on a pathological 1777-file diff; the
  `review_change` MCP prompt now calls `review_diff` instead of narrating
  an 8-step sequence (kept as `docs/use-cases/change-review.md`'s appendix
  for tool-by-tool users).

## Tasks

- [x] archconformance rule-pack CUE schema + valid/invalid testdata, Go-test-asserted negatives {id: C4-1, symbol: Check, package: internal/analysis/archconformance}
- [x] internal/projectrules loader: .fathom/rules/ discovery, CUE validation, source attribution, absolute-root guard; unit-tested incl. malformed-file and relative-root refusal {id: C4-2, symbol: Load, package: internal/projectrules}
- [x] arch_conformance consumes project rules when args omit them; response reports rule source; fake-backend test covers all three source states {id: C4-3, symbol: runArch, package: internal/analysis/review, reachable_from: TestRun_Arch_ReportsViolationsWithLineCitation}
- [x] Fathom's own .fathom/rules/archconformance.cue encoding the three-layer boundary; violation caught by test fixture — guards function-origin edges only; method-origin edges carry receiver-typed packages (goextract funcNode) and are a disclosed gap until both-sides edge-package normalization lands {id: C4-4, symbol: Load, package: internal/projectrules, skip_test: true, skip: "config-as-data row; the pack file is the deliverable — TestLoad_RepoOwnRuleFile_CatchesFixtureViolation exercises it through Load"}
- [x] Honesty audit across all deterministic analyzer tools: every empty-config case returns checked:false+reason or requires config by schema; table of findings recorded here {id: C4-5, symbol: runArch, package: internal/analysis/review, skip_test: true, skip: "audit row — per-tool fixes carry their own tests; the audit table is the deliverable"}
- [x] Pre-push non-blocking `fathom review --since origin/main --no-persist` summary step (a SUMMARY truthfully: never writes the shared store; always exit 0, fail-open on operational errors, ~12s worst-case on a 1777-file diff) + review_change MCP prompt rewritten to call review_diff instead of narrating the 8-step sequence (kept as change-review.md's appendix) {id: C3-1, symbol: registerPromptsHandlers, package: internal/mcp, reachable_from: TestAdoptionServiceScoped}
- [x] review_diff composed verb (diff impact + obligations/findings + arch conformance + spec coverage, one cited verdict) as the program's one new MCP tool, plus the `fathom review --since <ref>` CLI mirror with --no-persist {id: C1-1, symbol: registerReviewDiff, package: internal/mcp, reachable_from: TestAdoptionServiceScoped}
- [x] Findings-as-claims bridge via FT-7's pre-armed trigger, sound against the live claims substrate (see the C2 design block): oracle-dimensions-only persistence, hashed cites + scope-fp anchors (zero sweep flips), idempotent re-runs, fixed-findings reconcile (superseded), verified-tier manifest admission, refutation-durable via the store upsert {id: C2-1, symbol: AttachClaims, package: internal/analysis/review}

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
| `license_policy` (retired, SP-B2; its `internal/analysis/license` package retired CV-M6) | `allowed` is `required` in InputSchema, but that is NOT what protects it: dispatch never enforces InputSchema `required` (see footnote), so an omitted `allowed` reaches the handler and the scan runs. What actually protects (historical — package deleted): `license.Policy(detected, nil)` **fails closed** — `ok:false` with reason `"no licenses are allowed"` — never a false permit. | **Already safe — but via the analyzer's fail-closed empty-list behavior, not schema enforcement.** (This table's earlier draft claimed `ErrCodeParams` before any scan; the critic disproved that live.) | none |
| `security_rollup` (its `license` sub-check retired with `internal/analysis/license`, SP-B2/CV-M6; `secrets`/`shell_outs`/`http_calls` remain) | `allowed_licenses` optional; empty → `license.Policy(detected, nil)` **fails closed** (historical — package deleted): `license_ok:false` with explicit reason `"license %q is not permitted: no licenses are allowed"` — never a false "ok". `secrets`/`shell_outs`/`http_calls` are unconditional real scans with no config surface. | **Already safe** — fails closed with a stated reason instead of a clean-shaped answer. | none |
| `migration_targets` | `deprecated_pkgs` optional; empty → `migrationtargets.DefaultDeprecatedPkgs` (a real, non-empty, hardcoded list) is used — never an unconfigured no-op. | **Safe for the empty-config case.** Separate finding: a *non-empty* caller-supplied list **replaces** rather than merges with the default list, contradicting the tool's own parameter description ("default list is always included") — a different bug class (under-report on custom config, not false-clean on absent config); fixing it requires updating `migrationtargets_test.go`'s existing replace-semantics assertions, out of scope for this pass. | none (logged as `docs/dogfood-observations.md` O11) |
| `dead_code` | `entrypoints` optional; empty → BFS reachable-set seed is empty, so every symbol with zero callers is flagged "dead" (over-report: real entrypoints with no in-repo caller look dead too). | **Different failure mode — not this bug class.** Over-reporting, never a false-clean answer; not in scope for a "clean-shaped answer for work it didn't do" fix. | none |
| `god_object` | `method_threshold`/`field_threshold` optional; `<=0` → `godobject.Config.methodMin`/`fieldMin` default to a real `10` (`internal/analysis/godobject/godobject.go:41-51`). | **Safe** — real non-empty default, a check still runs. | none |
| `traceability` | `prefixes` optional; empty → `sdd.ExtractRefs` uses its own built-in non-empty default pattern (US/EP/FR/NFR etc.), not "match nothing". | **Safe** | none |
| `spec_refs` | `pattern` optional; empty → built-in default regex covering `[US/EP/FR/NFR-N]`, `MN.N`, `doc N`. | **Safe** | none |
| `get_go_symbols` | `kinds` optional; empty → no filter applied (all kinds returned) — "no filter requested" semantics, not a policy check that silently no-ops. | **N/A** — not a policy/rules config. | none |
| `spec_coverage`, `spec_status` | `spec_ids` is `required` in InputSchema but NOT Go-checked in the handler (only `service` is; see footnote) — an omitted `spec_ids` runs the scan. Result is degenerate-but-visible, not false-clean: `spec_status` returns `statuses: {}` (an empty map, no verdict claimed); `spec_coverage` returns empty `coverage` with every found code ref listed under `orphans`. | **Safe in effect** — empty input yields an empty/degenerate answer, not a clean-shaped verdict. | none |
| `propagate_concern` | `concern` + `seed` Go-enforced in the handler (`ErrCodeParams` before any work). | **Already safe** | none |
| `contradiction_scan`, `pack_context` | `concern` Go-enforced in the handler. | **Already safe** | none |
| `import_cycles` | `pkg_imports` Go-enforced in the handler. | **Already safe** | none |
| `cross_repo_blast_radius` | `symbol` Go-enforced in the handler. | **Already safe** | none |

**Footnote — InputSchema `required` is client-side-only across the MCP layer
(real finding from the critic pass, recorded here for a future row, deliberately
NOT fixed in this slice):** `internal/mcp`'s dispatch does not validate incoming
`arguments` against a tool's declared `InputSchema` — `required` entries are
advisory metadata for well-behaved MCP clients, nothing more. Every enforcement
that actually exists is a hand-written Go check inside the individual handler
(`p.Service == ""`, `p.Concern == ""`, …). Consequence for this audit: "the
schema says required" is never, by itself, evidence of safety — each row above
now states what the HANDLER enforces and what the analyzer does with the empty
value instead. A future row should either add dispatch-level InputSchema
validation or systematically align handler checks with declared `required`
lists (`license_policy.allowed`, `spec_coverage.spec_ids`, `spec_status.spec_ids`
are the currently-declared-but-unenforced cases found).

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
contract this row's design section specifies. `license_policy` and `security_rollup`
are both safe via the SAME mechanism — `license.Policy`'s fail-closed empty-list
behavior (`ok:false` + explicit reason), NOT schema enforcement; the design section's
premise that `license_policy` demonstrates a "schema-required param" pattern turned
out to be false on inspection (see the footnote: InputSchema `required` is
client-side-only, dispatch never enforces it). The safe pattern worth copying for
future config-bearing tools is therefore fail-closed-with-a-reason, plus a
hand-written handler check where an empty value should be a parameter error. Two
adjacent findings logged, not fixed in this pass: `migration_targets` replace-vs-merge
(O11) and the layer-wide unenforced-`required` gap (footnote above). No other
deterministic analyzer tool in the current registry carries an omittable policy/rules
parameter.
