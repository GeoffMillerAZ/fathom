# Decisions

Index of the kit's decisions. Most were recorded where they were made (the uplift plan,
a design doc, `compat.md`); this index points at them and carries the short revisit
condition. A pointed-at decision is promoted to a record here (`NNNN-slug.md`, template in
`.agents/skills/intent-init/templates/decisions/`) only when it is reopened
(`questions.md` Q4). New decisions are written here directly with `intent-decide`.

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
| [0002](0002-tool-effects-nil-is-irreversible.md) | accepted | architecture | Tool effects are declared data; an undeclared effect is irreversible under Strict; irreversible dispatch needs a single-use approval token; MCP annotations bridge both ways | a consumer reports a tool that cannot declare an effect; MCP's annotation vocabulary changes; the trifecta decision is reopened |
| [0003](0003-mcp-revisions-track-never-chase.md) | accepted | dependency | MCP revisions are tracked with a deprecation clock (a test that fails on the removal floor), never chased; the 2026-07-28 stateless move is deferred with its own floor | the floor test fires; a consumer's client rejects 2025-11-25; another revision deprecates something the kit implements |

## Decisions recorded elsewhere

| Id | Status | Type | Decision (one line) | Where | Revisit when |
|---|---|---|---|---|---|
| D1 | accepted | dependency | Zero deps is constitution, not marketing; `docextract/{pdf,xlsx}` are nested modules; the root `require` stays empty | `docs/plans/2026-07-uplift.md` §D1 | never |
| D2 | accepted | strategy | The product is the kits on-ramp, proven not asserted; a kit with zero consumers after its wave is demoted or retired | uplift §D2 | a kit reaches a wave end with no consumer |
| D3 | accepted | process | Three named subscribers force the agent layer: throughline, sextant/fathom, growing-gardens-bi; all in `downstream-check` | uplift §D3 | a subscriber leaves or a fourth arrives |
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
