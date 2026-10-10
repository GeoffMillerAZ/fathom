# docs/ — plans are gated; docs carry maintenance contracts

## docs/plans/ — the traps (the grammar, every annotation key and the gate: `docs/plans/README.md`)

Every top-level `docs/plans/*.md` except `README.md` is gated at pre-push (`lefthook.yml`: `fathom verify --strict`
against a scratch ingest of HEAD), so a stray `.md` there is a gated plan; `docs/plans/archive/` is out of scope.

- The `{…}` annotation sits **on the checkbox line itself**: the parser is line-based, and an annotation on a
  wrapped continuation line is silently dropped (the row verifies as `no-contract`).
- `symbol:` names FUNCTIONS/METHODS only, never a struct or type, and never a method name two receivers in the
  same package share (`grep -rn "^func.*<Name>("` first; disambiguate with `package:`). A non-code row carries an
  `oracle:` (`lexical`, `ledger`) instead.
- Proof before `[x]`: a box flips only when `fathom verify --service fathom --plan docs/plans/<plan>.md` proves
  it (exit 0 verified; 1 claimed-unproven, fix the code or uncheck the box; 2 operational, fail-open). Done means
  proven AND wired (ADR-018 amendment 2026-09-28): anchor the function a product entry point runs, never a
  test-only wrapper; `library: true` and `skip_test` are never a way to pass (§Self-skip in the README).
- Give every gated row an author-owned `id:`; a `[-]` skip carries `disposition:` and `reason:`.
- Computed progress: `fathom plans status --service fathom`, `fathom plans next --eligible-only`.

## Docs upkeep contracts

- `docs/use-cases/`: exactly one doc per use case; when an MCP tool or CLI command is added, renamed or removed,
  update that doc's `## Maintained` line and the catalog in `docs/mcp-tools.md` (the registry,
  `internal/mcp/surface.go`, owns the tool list; `TestMcpToolsDocMatchesRegistry` holds the catalog to it).
- Every design, vision and ADR carries a "Jev, weighed" section: which typed questions the decision could answer,
  the calibrated-band policy, how it behaves with provider none, and the measurement that would prove or kill it
  (P24). A judgment never asserts a code fact (I62).
- A research or assessment pass is finished only when its summary is in `docs/assessments/` and every decision it
  drove is in the records (an ADR or dated amendment, futures, scope, plan rows). An artifact (a page, a
  scratch file) is never the record.
- Friction with fathom itself: the root `AGENTS.md` Friction-log row.
