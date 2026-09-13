# internal/ — engine rules

## Determinism is a contract

- Sort every output slice; map iteration order must never reach a result.
- Inject clocks (`now func() time.Time`) — never call `time.Now()` in
  query/analysis logic.
- A test that seeds a FIXED fixture date and then passes the REAL clock is a
  time bomb: it passes until the fixture crosses an age horizon, then fails
  on a day nobody changed anything (two of these fired on 2026-08-26 against
  the thread brief's 14-day cutoff). Either inject the same fixed `now`, or
  stamp the fixture with `time.Now()` — never mix the two.
- Golden fixtures: `internal/connector/testdata/fauxtown/` is the
  deterministic fixture fleet; prefer golden tests against it.
- Always run `go test -race` before claiming green — the gate uses it, and
  several test bugs only surface under it (e.g. reading a server's output
  buffer while a `Serve` goroutine writes it — run `Serve` synchronously
  against finite input).

## Store purity (ADR-020 / ADR-025)

`internal/store` is pure Go (`modernc.org/sqlite`) — no CGO. CGO in this
repo comes only from the tree-sitter extractor bindings (ADR-013). Do not
add CGO dependencies to the store or query layers — `task store-purego`
(`CGO_ENABLED=0 go test -race ./internal/store/`, a `task validate` dep)
fails the gate if you do (invariant I2).

## The domain model is the keystone

Shared types live in `internal/core`; extend it additively.
`pkg/client` re-exports for external consumers.

## Adding things

- **Extractor**: implement `extractor.Extractor`, register by extension in
  `fusion.New()`, add its ext→kind in `connector`, golden-test it.
- **Analyzer**: a pure `internal/analysis/<name>` package over
  `[]core.SymbolNode` / `[]core.Observation`; deterministic; tested.
- **MCP tool**: `registerTool` in `internal/mcp` with a `group`; extend the
  backend interface minimally if it needs new data; cover with a
  fake-backend test (`fakeBackend`, `tools_test.go`). If coding agents need
  it, add it to the `companionTools` allowlist in `internal/mcp/surface.go`
  (19 tools) and the pinned `TestCompanionSurfaceToolsList`. Then update
  `docs/mcp-tools.md` — `TestMcpToolsDocMatchesRegistry`
  (`surface_prune_test.go`) fails until the doc matches the registry — and
  the relevant `docs/use-cases/*.md` `## Maintained` line.
- **Custodian check kind**: a `CheckKind` in `internal/custodian`
  (`custodian.go`), loaded from `.fathom/concerns.cue`; only
  `stanza_tools_served` may ask the daemon, and it degrades to
  unable-to-check, never to violated.
