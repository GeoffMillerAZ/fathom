# docs/ — plans are gated; docs carry maintenance contracts

## docs/plans/ — plan grammar v2 (full spec: `docs/plans/README.md`)

Every top-level `docs/plans/*.md` except `README.md` is run through
`fathom verify` at pre-push and CI (`--strict`) — never add a stray .md
file there. `docs/plans/archive/` is out of gate scope. Essentials:

- The `{…}` annotation sits **on the checkbox line itself** — the parser is
  line-based; an annotation on a wrapped continuation line is silently
  dropped and the row verifies as `no-contract`.
- `symbol:` anchors name FUNCTIONS/METHODS only — never struct/type names,
  and never a method name shared by two receivers in the same package
  (check `grep -rn "^func.*<Name>("` across `internal/ cmd/fathom/ pkg/`
  before anchoring; disambiguate with `package:`).
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
- Computed progress: `fathom plans status` / `next`; retire fully-proven
  plans with `fathom plans archive <plan>`.

## Docs upkeep contracts

- `docs/use-cases/`: exactly one doc per use case; when an MCP tool or CLI
  command is added/renamed/removed, update that doc's `## Maintained` line
  and the catalog in `docs/mcp-tools.md`.
- Friction with fathom itself → `docs/dogfood-observations.md` (with a
  reproduction); close via `CHANGELOG.md` when fixed.
