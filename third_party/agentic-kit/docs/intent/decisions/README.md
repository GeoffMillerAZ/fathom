# Decisions

Index of the kit's decisions. Most were recorded where they were made (the uplift plan,
a design doc, `compat.md`); this index points at them and carries the short revisit
condition. A pointed-at decision is promoted to a record here (`NNNN-slug.md`, template in
`.claude/skills/intent-init/templates/decisions/`) only when it is reopened.
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
| [0007](0007-scoped-intent-and-pinned-distribution.md) | accepted | process | Intent follows ownership and affected paths; practice/skills are a reviewed project-local snapshot | missed routing, independent authority, update friction or host drift |
| [0008](0008-bound-approval-transactions.md) | accepted | architecture | Stored approval binds exact operation, actor and versions inside the HITL transaction; ambiguous delivery requires reconciliation | target/decision atomicity or destination reconciliation is unavailable |
| [0009](0009-bounded-lesson-transfer.md) | accepted | architecture | G6/G8 exit opens G7: bounded lesson files need receiver-owned acceptance, reproduction and freshness checks; hashes grant no authority | unsafe applicability, harmful transfer, or authenticated distribution needed |
| [0010](0010-pinned-experimental-adoption.md) | accepted | compatibility | Throughline retains experimental guardrails with an explicit pin, vendor-tested upgrade and bounded risk review; no tier promotion | next release/production upgrade, 2026-10-12, or a broken consumer contract |
| [0011](0011-workload-selection-evidence.md) | accepted | architecture | Workload solutions use scoped, versioned benchmark evidence; existing evaluator owns the bounded receipt/comparison slice | 2026-10-16/next release, stale or unauthorized recommendation, or shared contract mismatch |
| [0012](0012-composable-engineering-assessments.md) | accepted | architecture | Share domain assessment evidence, not a universal score; configurable model screening holds uncertain execution and never grants authority | 0011 review date, collector mismatch, screening bypass/false refusal, or new evidence trust requirements |
| [0013](0013-operational-selection-evidence.md) | accepted | architecture | Consumer-observed collection and local eligibility/decision policies operationalize 0011/0012; exact reuse keeps original observations | 0011 review, missing dependencies/attempts, stale approval, held-output bypass, or second policy subscriber |
| [0014](0014-offline-delivery-live-study-followup.md) | accepted | process | Owner-directed offline completion retains historical evidence and explicitly defers further live study; no benchmark-readiness claim | renewed live authority, production/changed-solution quality claim, or 0011 review |
| [0015](0015-projections-in-go-recorded-divergence.md) | accepted | architecture | CUE owns shape, projections included; a consumer's test-pinned Go projection (racli ADR-016 §4) is a recorded DIVERGENCE — the rule stands, nothing absorbed, no exception | a second consumer pins projection policy in Go; a kit package needs a projection of its own; a Go test over a `cue`-exported fixture lands in the kit |
| [0016](0016-2026-09-13-16-unmerged-branches-disposition.md) | accepted | process | Twenty GitHub branches dated 2026-09-13..16 all check empty on `diff --diff-filter=D` against `docs/intent-excellence-assessment` — none merged separately, all carried by it, none deleted | landing trigger fired 2026-09-25, re-check empty for all twenty; remaining: any branch's re-diff against `main` stops being empty |
| [0017](0017-fast-judgment-is-optional-and-calibrated.md) | accepted | architecture | Fast judgment is an optional, calibrated tier; the kit ships guidance and a provider-neutral port, not thresholds; judgment never replaces proof | a second consumer runs the calibration template; a consumer shows 0.9 is unsafe or too cautious for a site; provider terms change; a study passes a site's bar; DP-12 to DP-24 (the hexagonal hardening rows) are proven |
| [0018](0018-one-decision-port-optional-provider-adapters.md) | accepted | architecture | One decision port; provider adapters (llm, typesafe incl. its OpenRouter route, openai) are hexagonal and optional, replaceable at the contract but not interchangeable without a per-provider calibration reading; refusals are first-class (`Declined`); confidence is derived uniformly; keys come from a file reference or env var name, never inline | a live reading shows two providers agree for a site; OpenAI documents calibration, pinning or limits or leaves beta; OpenRouter's router leaves alpha or changes; a refusal carries a reason worth surfacing; a consumer needs cached image judgments |

`0004`–`0014` came from `docs/intent-excellence-assessment`, which landed after
`0015`–`0016` were recorded; decision 0015 was renumbered on its own merge
(originally authored as `0004`, which collided). The next new record starts at `0019`.
0016's landing trigger fired with that landing: `git diff --diff-filter=D` from each
of its twenty branch tips (unchanged) to the merged tree is empty, so its disposition
stands.

## Decisions recorded elsewhere

| Id | Status | Type | Decision (one line) | Where | Revisit when |
|---|---|---|---|---|---|
| D1 | accepted | dependency | Zero deps is constitution, not marketing; `docextract/{pdf,xlsx}` are nested modules; the root `require` stays empty | `docs/plans/2026-07-uplift.md` §D1 | never |
| D2 | superseded by 0004 (retirement criterion) | strategy | The on-ramp remains the product; zero consumers now prompts a bounded value review | uplift §D2; 0004 | 0004's review conditions |
| D3 | accepted; trigger fired 2026-09-09, fourth assessed | process | Throughline, fathom, growing-gardens-bi and racli (`resume_as_code`) have pinned delivery outcomes; `downstream-check` runs all four (`task racli-check`, RC-1) | uplift §D3; [delivery](../../assessments/2026-09-13-governed-workflows.md) | a subscriber leaves or a fifth arrives |
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
