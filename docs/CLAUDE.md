# docs/ — plans are gated; docs carry maintenance contracts

## docs/plans/ — plan grammar v2 (full spec: `docs/plans/README.md`)

Every top-level `docs/plans/*.md` except `README.md` is run through
`fathom verify` at pre-push (`--quiet`, non-strict — `lefthook.yml`) and by
the manual `scripts/ci.sh` (`--strict`: claimed+unknown rows fail; no CI
runner exists yet) — never add a stray .md file there. `docs/plans/archive/`
is out of gate scope. Essentials:

- The `{…}` annotation sits **on the checkbox line itself** — the parser is
  line-based; an annotation on a wrapped continuation line is silently
  dropped and the row verifies as `no-contract`.
- `symbol:` anchors name FUNCTIONS/METHODS only — never struct/type names,
  and never a method name shared by two receivers in the same package
  (check `grep -rn "^func.*<Name>("` across `internal/ cmd/fathom/ pkg/`
  before anchoring; disambiguate with `package:`).
- Reachability: function values are edges (VS-6) and closure calls count
  for the enclosing test (VS-7) — the "call it at the test's top level"
  rule is retired; `skip_test` is only for rows with no test at all.
- Non-code rows carry an `oracle:` instead of a `symbol:`. `lexical`
  (ADR-030) proves a file exists and matches a pattern. `ledger` (ADR-034
  decision 2, RC-34) proves a *published bench reading*:
  `{oracle: ledger, path: <jsonl>, run_id: R, task: T, arm: A,
  min_trials: N}` verifies when the JSONL row keyed by (run_id, task, arm)
  exists, is not flagged `surface_unused: true`, carries no
  `instrument_defect`, its own `evaluation.verdict` is not `fail`/
  `indeterminate` (add `accept_verdict: any` to certify despite it — name
  the reason it still counts), and ran at least `min_trials` trials
  (default 1) — a scenario row's own `trials`, or a home row's own
  per-cell `min_trials` (never its run-total `treatment_trials`, which
  inflates with task count and proves nothing about replication). A row
  keyed by `run_id` alone matches the home bench's one-row-per-run ledger
  (`docs/bench/ledger.jsonl`) instead; when that run_id has more than one
  such row (the same run scored under two registered criteria), the claim
  is refused as ambiguous unless `criterion: <id>` names the one row's
  `evaluation.criterion_id` to pick. Full key reference:
  `docs/plans/README.md`'s Annotation keys table.
- Give every gated row an author-owned `id:` — positional `md-NNN`
  renumbers when lines are inserted.
- `[-]` skips carry `disposition: refused|deferred` + `reason` (double-quote
  values containing commas). Doc-only rows: `skip_test: true` with a `skip:`
  reason naming the real acceptance evidence — never a fabricated `Test*`
  claim.
- Boxes flip to `[x]` only on proof:
  `fathom verify --service fathom --plan docs/plans/<plan>.md` —
  exit 0 verified; 1 claimed-unproven (fix the code or uncheck the box);
  2 operational (daemon down / index stale — fail-open).
- Computed progress: `fathom plans status` / `next --eligible-only`
  (hides rows blocked on `needs:`); `plans context --row <id>` renders a
  row's `ctx:` pack; `plans archive <plan>` retires a proven plan once its
  row ids are absorbed into `CHANGELOG.md`, `ADR.md`, or `docs/changelog/`.

## Docs upkeep contracts

- `docs/use-cases/`: exactly one doc per use case; when an MCP tool or CLI
  command is added/renamed/removed, update that doc's `## Maintained` line
  and the catalog in `docs/mcp-tools.md`.
- The agent-surface tool list is `internal/mcp/surface.go` (18 tools);
  `TestMcpToolsDocMatchesRegistry` holds `docs/mcp-tools.md` to the registry.
- Friction with fathom itself goes to ONE place: `fathom observe` files the
  structured intake into `.fathom/observations.md` (the live log, counted by
  `instrument --adoption`). `docs/dogfood-observations.md` is frozen as the
  narrative record (O1–O29; no new entries; its ids stay historical). Close an
  observation via `CHANGELOG.md` when fixed (ADR-026 decision, 2026-08-26).
- Root `CLAUDE.md` is a router (≤120 lines) with resolving pointers —
  `fathom config audit` (`.fathom/concerns.cue`) polices both.
