### `plans` — computed SDD progress and archival

```sh
fathom plans status  --service <svc> [--plan P]... [--store DB | --url U] [--json]   # per-plan rollup: total/proven/claimed-unproven/measured-stale/open/deferred/other-skipped
fathom plans status  --service <svc> --weighted [--json]   # proven weight / total weight by fleet, project, plan and section (`weight:`, default 1; [-] rows carry none)
fathom plans next    --service <svc> [--eligible-only] [--free] [--root DIR] [--json] # the ordered work frontier, with {id, symbol, package} anchors
fathom plans history --service <svc> --plan P [--url U] [--json]                      # the burn-up: scope and claimed rows per commit of the plan, the ledger's proven trend (absent under 14 days of rows), the cross-check
fathom plans history [--service <svc>] [--url U] [--json]                             # no --plan: the claimed-vs-proven trend of the current plans of one service (or every registered one), from the verify ledger's runs of those plans only
fathom plans context --service <svc> --row <id> [--budget 4000]                       # render a row's ctx: context pack (elided beyond the byte budget)
fathom plans context <plan.md> --service <svc> --row <id> --store DB --preflight [--score] # + the row's task preflight: difficulty profile and advisory tier (WI-16..WI-18)
fathom plans outcome <plan.md> --service <svc> --row <id> --result proven|unproven --model M [--effort E] [--tier T] # record what a preflight met (WI-19)
fathom plans lease   --plan P <row-id> [--holder H] [--ttl 2h] [--release] [--root DIR] # TTL advisory lease on a row (.fathom/leases.jsonl)
fathom plans leases  [--root DIR] [--json]                                            # list live leases
fathom plans archive <plan> --service <svc> [--root DIR]   # retire a fully-proven plan to docs/plans/archive/ (out of gate scope)
fathom plans restore <plan> [--root DIR]                   # bring it back
```

State is always derived from the index by the verify engine, never asserted.
With `--service` and no `--plan`, plans are discovered in the service's
registered root and their lexical oracles and self-skip history are read from
that root too. `status --json` names the measurement rows among `proven` and
`open` as `op_proven` and `op_open` (subsets, not buckets).
`measured-stale` (PC-5, ADR-027): when the daemon that answered is not this
binary's build (or, inside the fathom checkout, not HEAD), a claimed row it did
not prove is a coverage gap in that column — never `claimed-unproven` — and a
footer names why; `fathom intent audit` and doctor's `unproven_claims` apply
the same rule to the verify ledger's newest reading per plan.
`next --eligible-only` hides rows blocked on an unmet `needs:` target (the
derived `eligible` / `blocked_on` state); `--free` hides rows holding a fresh
lease. `archive` refuses unless every row id is lexically absorbed into
`CHANGELOG.md`, `ADR.md`, or `docs/changelog/*.md` (the per-release
narrative) — the refusal names all three. Heads-up: `plans status` currently
verifies every active plan serially over HTTP (minutes on a large plan set —
O29 in `docs/dogfood-observations.md` tracks the fix).

**Task preflight (U6, `docs/design/work-intelligence.md`).** `plans context --preflight`
(direct mode, `--store`) prints a row's difficulty profile — the anchor's transitive
callers, the rules governing its files by level, the certificates a change would
re-prove, its zones' interest rates, its own claimed-unproven readings and weight, and
the effort of its 5 nearest proven rows — and the tier the published rule table
(T1..T13, revision 1, in the design doc) draws from it: `light`, `standard` or `heavy`,
relative to what your harness offers. It is advice: fathom recommends and the harness
decides; nothing here starts, assigns or routes work. An unknown fact never lowers a tier
or certifies a light one (with rules unevaluated the tier reads "at least"). `--score`
also asks the decision port for its Score over the same measured numbers — never the
row's text, symbol or paths — only with `[decide] provider` set and `preflight` in
`[decide] sites`, one call per invocation, logged to `decisions.jsonl` as never applied.
The plan-row console page shows the same profile and tier but never asks the port.
`plans outcome` appends one line to `~/.fathom/preflight-outcomes.jsonl`
(`FATHOM_PREFLIGHT_LEDGER`): the agent identity (harness, model, effort — a person's name
or email is withheld), proven or not, and the tiers recommended; `--first-attempt` is read
from the proof history for a proven row. The calibration under a preflight is read back
from that ledger: per recommended tier, how many rows were proven at the first attempt,
uncalibrated below 10 outcomes.
