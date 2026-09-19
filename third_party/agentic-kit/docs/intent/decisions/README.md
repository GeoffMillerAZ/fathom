# Decisions

Index of the kit's decisions. Most were recorded where they were made (the uplift plan,
a design doc, `compat.md`); this index points at them and carries the short revisit
condition. A pointed-at decision is promoted to a record here (`NNNN-slug.md`, template in
`.agents/skills/intent-init/templates/decisions/`) only when it is reopened.
New decisions are written here directly with `intent-decide`.

Record a decision when any holds: reversal is expensive; it constrains code outside its
own package; it creates or removes an invariant, non-goal, or boundary; it accepts a
notable tradeoff or dependency; it defers something significant and names what would
un-defer it; it rejects an alternative someone will propose again; without the record a
future reader would rebuild this differently. Otherwise: a comment, a test, a commit.

Status: proposed · accepted · rejected · deferred · superseded by <id> · deprecated.

## Records in this directory

| Id | Status | Type | Title | Revisit when |
|---|---|---|---|---|
| [0001](0001-condition-lines-may-carry-oracle-annotations.md) | proposed | process | Condition lines MAY carry an optional author-written `{oracle: …}` annotation; nothing generates one | 3 hand-written annotations exist across 2 repos (accept), or 30 days pass from 2026-08-27 with fewer (reject) |
| [0002](0002-tool-effects-nil-is-irreversible.md) | accepted | architecture | Tool effects are declared data; undeclared/destructive dispatch under opt-in Strict needs a per-call approver; MCP annotations bridge both ways (no durable token is shipped) | a consumer reports a tool that cannot declare an effect; MCP's annotation vocabulary changes; the trifecta decision is reopened |
| [0003](0003-mcp-revisions-track-never-chase.md) | accepted | dependency | MCP revisions are tracked with a deprecation clock (a test that fails on the removal floor), never chased; the 2026-07-28 stateless move is deferred with its own floor | the floor test fires; a consumer's client rejects 2025-11-25; another revision deprecates something the kit implements |
| [0004](0004-bounded-incubation.md) | accepted | strategy | A named consumer or bounded incubation admits work; at most two implementation incubations, dated reviews, explicit evidence to promote | missed reviews, consumer regressions displaced, or maintenance capacity changes |
| [0005](0005-workflow-assurance.md) | accepted | architecture | Governance claims identify tested workflow wiring and host coverage; provider and harness adapters are distinct | host behavior changes, bypass found, or profile meaning diverges |
| [0006](0006-evidence-and-lessons.md) | accepted | domain | Prose links specific claims to versioned evidence; reviewed lessons travel independently of consumer-owned memory | unsupported source shapes, harmful transfer, or file distribution becomes insufficient |
| [0008](0008-bound-approval-transactions.md) | accepted | architecture | Stored approval binds exact operation, actor and versions inside the HITL transaction; ambiguous delivery requires reconciliation | target/decision atomicity or destination reconciliation is unavailable |
| [0009](0009-bounded-lesson-transfer.md) | accepted | architecture | G6/G8 exit opens G7: bounded lesson files need receiver-owned acceptance, reproduction and freshness checks; hashes grant no authority | unsafe applicability, harmful transfer, or authenticated distribution needed |
| [0010](0010-pinned-experimental-adoption.md) | accepted | compatibility | Throughline retains experimental guardrails with an explicit pin, vendor-tested upgrade and bounded risk review; no tier promotion | next release/production upgrade, 2026-10-12, or a broken consumer contract |
| [0011](0011-workload-selection-evidence.md) | accepted | architecture | Workload solutions use scoped, versioned benchmark evidence; existing evaluator owns the bounded receipt/comparison slice | 2026-10-16/next release, stale or unauthorized recommendation, or shared contract mismatch |
| [0012](0012-composable-engineering-assessments.md) | accepted | architecture | Share domain assessment evidence, not a universal score; configurable model screening holds uncertain execution and never grants authority | 0011 review date, collector mismatch, screening bypass/false refusal, or new evidence trust requirements |
| [0013](0013-operational-selection-evidence.md) | accepted | architecture | Consumer-observed collection and local eligibility/decision policies operationalize 0011/0012; exact reuse keeps original observations | 0011 review, missing dependencies/attempts, stale approval, held-output bypass, or second policy subscriber |

## Decisions recorded elsewhere

| Id | Status | Type | Decision (one line) | Where | Revisit when |
|---|---|---|---|---|---|
| D1 | accepted | dependency | Zero deps is constitution, not marketing; `docextract/{pdf,xlsx}` are nested modules; the root `require` stays empty | `docs/plans/2026-07-uplift.md` §D1 | never |
| D2 | superseded by 0004 (retirement criterion) | strategy | The on-ramp remains the product; zero consumers now prompts a bounded value review | uplift §D2; 0004 | 0004's review conditions |
| D3 | accepted; fourth assessed | process | Throughline, fathom, growing-gardens-bi and resume_as_code have pinned delivery outcomes; the fourth permanent gate remains an uncommitted owner change | uplift §D3; [delivery](../../assessments/2026-09-13-governed-workflows.md) | a subscriber leaves or another arrives |
| D4 | deferred | strategy | Public release and 1.0 candidacy: not yet; owner's call | uplift §D4 | Geoff ratifies (`scope.md` G1) |
| D5 | accepted | strategy | The kill list: retire packages with zero consumers (16 deleted, verified) | uplift §D5, §6 | a retired package is re-requested by a consumer |
| D6 | accepted | data | Eval corpus comes from throughline prompts and agent-cli cassettes | uplift §D6 | corpus provenance ruling (`questions.md` Q5) |
| D7 | deferred | architecture | `coordination/*` gets an executor only as `kits/worker-pool` when a real fan-out workload subscribes; plan-only until then | uplift §D7; `docs/designs/eval-waves-d7.md` | serial eval runs exceed interactive time, or multi-wave agreement gains a subscriber |
| D8 | accepted | process | Mine, don't port: ~8 net-new packages with a named subscriber each; a single-consumer package nests under its consumer until a second names the need | uplift §D8; `compat.md` §D8 | a second consumer names a nested package |
| D9 | accepted | process | The eval-runner's first subscriber is the kit's own agent-driven dev process; independence check mandatory, which made the OpenAI-compatible client required | uplift §D9 | a decisive verdict is issued without an independence check |
| D10 | accepted | domain | `ABSENT_PROVEN` claims absence within a declared scope at a named snapshot, never global absence; every verdict carries a coverage manifest | uplift §D10 | a consumer needs a global-absence claim (refuse; it does not exist) |
| D13 | accepted | process | Two consumers make it kit business: build in-kit ahead of full wiring when two or three consumers are named | `docs/designs/catalog-kit.md` §1, §10 | a D13-scoped package reaches its next release with fewer than two live consumers |
| D35-1..5 | accepted | dependency | Five throughline donor functions migrated into the kit (intake-triage FSM, query planner, token lifecycle, authenticity ceiling, source-sync checkpoint retirement) | `docs/plans/handoff-h2-pull-queue.md` | a donor is not deleted after its kit side lands (`TRACKING.md` pull queue) |
| MCP | accepted | architecture | MCP Streamable HTTP built; SSE, sessions, prompts and resources deferred; the deprecated HTTP+SSE transport never | `docs/designs/mcp-streamable-http.md` | `futures.md` F10 |
| V1 | accepted | platform | No `/v1` module-path suffix at 1.0; nested modules tag independently; `go.work` is for local dev only | `docs/compat.md` §v1 decision record | a v2 major becomes necessary |
| CAT | accepted | architecture | `foundations/knowledge/catalog`: catalogs that cannot lie; refuse rather than guess on duplicate keys | `docs/designs/catalog-kit.md` | a consumer needs lenient duplicate handling (refuse; that is the product) |
