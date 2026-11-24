### Added

- `fathom-lang extract -rows` (provisional) prints JSON Lines in place of the nested document: a `header` row, then per file a `file` row and its `symbol`, `reference`, `unresolved`, `fact` and `note` rows in array order, each with `path` and `i`. The same reads, flags and exit codes apply, and without `-rows` the output is unchanged. The nested document can be rebuilt from the rows exactly.
- `fathom-lang schema [extract|rows|at|outline]` (provisional) prints the JSON Schema (draft 2020-12) of each output shape, embedded in the binary and published under `docs/schema/fathom-lang/`.
- `view.Rows`, `view.Header`, `view.WriteJSONLines` and the `view.Row` type (provisional) produce the same rows in Go.
