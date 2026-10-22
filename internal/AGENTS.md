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

`internal/store` is pure Go (`modernc.org/sqlite`) — no CGO, and since
ADR-066 so is every Linux build: the only cgo left is cmd/fathom's two
darwin proc readers. New CGO fails the gate: anywhere in cmd/fathom's
dependencies, `cmd/fathom/cgofree_test.go` (CS-9); in the store, `task
store-purego` (`CGO_ENABLED=0 go test -race ./internal/store/`, invariant I2).

## The domain model is the keystone

Put shared types in `internal/core` and extend them additively.
`pkg/client` re-exports for external consumers.

## Adding things

- **Extractor**: a new language or tool format is a dialect directory, not Go —
  follow the worker recipe (`docs/design/language-breadth.md` §13, ADR-048). Only the
  engines, L3 strategies and native typed passes are Go: a native extractor implements
  `extractor.Extractor`, registers by extension in `fusion.New()`, adds its ext→kind in
  `connector`, and is golden-tested.
- **Analyzer**: a pure `internal/analysis/<name>` package over
  `[]core.SymbolNode` / `[]core.Observation`; deterministic; tested.
- **MCP tool**: `registerTool` in `internal/mcp` with a `group`; extend the
  backend interface minimally if it needs new data; cover with a
  fake-backend test (`fakeBackend`, `internal/mcp/tools_test.go`). Surface
  membership is a contract change: the allowlists in `internal/mcp/surface.go`
  (`defaultSurfaceTools`, `companionTools`, `extendedTools`) and the pinned
  `TestDefaultSurfaceToolsList` / `TestExtendedSurfaceToolsList`
  (`internal/mcp/surface_test.go`) move together. The registry owns every
  count: update `docs/mcp-tools.md` — `TestMcpToolsDocMatchesRegistry`
  (`internal/mcp/surface_prune_test.go`) fails until the doc matches it — and
  the relevant `docs/use-cases/*.md` `## Maintained` line.
- **Custodian check kind**: a `CheckKind` in `internal/custodian`
  (`internal/custodian/custodian.go`), loaded from `.fathom/concerns.cue`; only
  `stanza_tools_served` may ask the daemon, and it degrades to
  unable-to-check, never to violated.
