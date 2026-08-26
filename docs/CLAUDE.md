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
- The agent-surface tool list is `internal/mcp/surface.go` (17 tools);
  `TestMcpToolsDocMatchesRegistry` holds `docs/mcp-tools.md` to the registry.
- Friction with fathom itself goes to ONE place: `fathom observe` files the
  structured intake into `.fathom/observations.md` (the live log, counted by
  `instrument --adoption`). `docs/dogfood-observations.md` is frozen as the
  narrative record (O1–O29; no new entries; its ids stay historical). Close an
  observation via `CHANGELOG.md` when fixed (ADR-026 decision, 2026-08-26).
- Root `CLAUDE.md` is a router (≤120 lines) with resolving pointers —
  `fathom config audit` (`.fathom/concerns.cue`) polices both.
