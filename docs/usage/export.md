### `export` — interchange with other code-intel tooling (VB-32)

```sh
fathom export scip --service <id> [--store <path>] [-o index.scip] [--module <dir>]
```

Writes `service`'s symbol and typed-edge tables as a [SCIP](https://sourcegraph.com/blog/announcing-scip)
index, over the generated wire-format types vendored at `third_party/scip` (see that
directory's `README.md` for the exact upstream tag/commit and why only those files are
vendored rather than depending on the upstream module directly) — store-direct only,
mirroring `review`'s structure (no daemon round trip). Each symbol's resolution tier
(`core.EdgeConfidenceOf`: `resolved`/`heuristic`) travels on the CALLER's
`SymbolInformation.Documentation`, one line per typed-edge relationship, formatted
`fathom:resolution <target-symbol> = <tier>` — SCIP's `Relationship` message has no
free-form field, so this is the "Relationship-adjacent field" the tier lives in; a
definition's own occurrence carries no tier (tier is a property of an edge, never of a
definition site). An edge whose target isn't among the service's own indexed nodes is
skipped (SCIP's `ExternalSymbols` table would be the correct home for that; out of scope
here).

After writing, it round-trips: with `--module <dir>` naming a real Go module root and
`scip-go` on `PATH`, it runs `scip-go` independently over that module and compares bare
symbol names against the store's own list — a genuine cross-tool check. Otherwise (the
common case — most machines running this command don't have `scip-go` installed) it
decodes the just-written bytes with the bindings and compares the resulting symbol table
against the store's, printing a `round-trip (bindings-decode): N/N symbols matched — ok`
line and naming that the `scip-go` comparison is pending on a machine that has it.

| Flag | Default | Description |
|---|---|---|
| `--service` | (required) | Service to export |
| `--store` | `<repo>/workspace/db/fathom.sqlite` (see §CLI subcommands) | store path (.sqlite/.db) |
| `-o` | `index.scip` | Output path for the SCIP index |
| `--module` | (none) | A real Go module root on disk, for the `scip-go` round-trip comparison (ignored when `scip-go` is not on `PATH`) |

`fathom ingest --scip` — importing symbols FROM a SCIP index, rather than exporting TO
one — is a later, separate decision; not built here.
