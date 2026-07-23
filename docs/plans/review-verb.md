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

- [ ] archconformance rule-pack CUE schema + valid/invalid testdata, Go-test-asserted negatives {id: C4-1, symbol: Check, package: internal/analysis/archconformance}
- [ ] internal/projectrules loader: .fathom/rules/ discovery, CUE validation, source attribution; unit-tested incl. malformed-file refusal {id: C4-2, symbol: Check, package: internal/analysis/archconformance, skip_test: true, skip: "loader is a new package whose consumer anchor is the analyzer it feeds; its own tests are the coverage — anchor shared with C4-1 pending the package landing"}
- [ ] arch_conformance consumes project rules when args omit them; response reports rule source; fake-backend test covers all three source states {id: C4-3, symbol: registerArchConformance, package: internal/mcp}
- [ ] Fathom's own .fathom/rules/archconformance.cue encoding the three-layer boundary; violation caught by test fixture {id: C4-4, symbol: Check, package: internal/analysis/archconformance, skip_test: true, skip: "config-as-data row; C4-3's tests exercise the load path against this file"}
- [ ] Honesty audit across all deterministic analyzer tools: every empty-config case returns checked:false+reason or requires config by schema; table of findings recorded here {id: C4-5, symbol: registerArchConformance, package: internal/mcp, skip_test: true, skip: "audit row — per-tool fixes carry their own tests; the audit table is the deliverable"}
- [ ] (op-gated: C3) Pre-push non-blocking review summary + review_change prompt rewrite {id: C3-1, symbol: registerArchConformance, package: internal/mcp, skip_test: true, skip: "blocked on operator channel acceptance; not built until then"}
- [ ] (blocked on C3) review_diff composed verb + CLI mirror {id: C1-1, symbol: registerArchConformance, package: internal/mcp, skip_test: true, skip: "blocked on C3; anchor updated to the new package when built"}
- [ ] (blocked on C3) findings-as-claims bridge via FT-7 {id: C2-1, symbol: registerArchConformance, package: internal/mcp, skip_test: true, skip: "blocked on C3"}
