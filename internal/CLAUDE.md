# internal/ — engine rules

## Determinism is a contract

- Sort every output slice; map iteration order must never reach a result.
- Inject clocks (`now func() time.Time`) — never call `time.Now()` in
  query/analysis logic.
- Golden fixtures: `internal/connector/testdata/fauxtown/` is the
  deterministic fixture fleet; prefer golden tests against it.
- Always run `go test -race` before claiming green — the gate uses it, and
  several test bugs only surface under it (e.g. reading a server's output
  buffer while a `Serve` goroutine writes it — run `Serve` synchronously
  against finite input).

## Store purity (ADR-020 / ADR-025)

`internal/store` is pure Go (`modernc.org/sqlite`) — no CGO. CGO in this
repo comes only from the tree-sitter extractor bindings (ADR-013). Do not
add CGO dependencies to the store or query layers.

## The domain model is the keystone

Shared types live in `internal/core`; extend it additively.
`pkg/client` re-exports for external consumers.

## Adding things

- **Extractor**: implement `extractor.Extractor`, register by extension in
  `fusion.New()`, add its ext→kind in `connector`, golden-test it.
- **Analyzer**: a pure `internal/analysis/<name>` package over
  `[]core.SymbolNode` / `[]core.Observation`; deterministic; tested.
- **MCP tool**: register in `internal/mcp` with a group; extend the backend
  interface minimally if it needs new data; cover with a fake-backend test.
  Then update the catalog in `docs/mcp-tools.md` and the relevant
  `docs/use-cases/*.md` `## Maintained` line.
