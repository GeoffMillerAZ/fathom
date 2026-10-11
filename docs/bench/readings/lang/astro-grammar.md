# Astro's grammar against the 5% ERROR-file gate (wave lf7, item angularastro)

**Verdict: virchau13/tree-sitter-astro passes the gate with room to spare. On the two calibration
subjects at their pins it leaves an ERROR or MISSING node in 0 of 172 `.astro` files (0.0%,
against §14.2's 5% kill line): withastro/docs 0 of 81, withastro/astro's `examples/` 0 of 91. The
rest of withastro/astro, its packages' test fixtures (some written broken on purpose), reads 6 of
1,490 (0.4%). The grammar is pinned for the astro dialect (wave lf7, item angularastro).**

Measured 2026-10-11 at fathom 40aecf63 plus the uncommitted astro blob, before any dialect query or
reading (the item's order: the gate first, §13.1).

## Grammar

virchau13/tree-sitter-astro at `213f6e6973d9b456c6e50e86f19f66877e7ef0ee` (the repository publishes
no release tag; its default branch's head, which tree-sitter-language-pack's catalogue pins too:
`indexbench lang-pin -id astro`). Licence MIT, read at the pin from the top-level `LICENSE`
(`indexbench lang-pin -tree`: `{"licence": "MIT", "licence_basis": "LICENSE", "refused": false}`).
The release ships `src/parser.c` (`LANGUAGE_VERSION 14`) and `src/scanner.c`, so the lock row reads
`generate: false`, `scanner: true` (the catalogue's `scanner: false` is wrong at this pin), ABI 14.
Built with `build.sh astro` (zig 0.16.0, wasm-opt 129): module 150,848 bytes, gzip 61,599 bytes.

The grammar is HTML's with an Astro frontmatter (`frontmatter` / `frontmatter_js_block`), attribute
and child interpolations (`attribute_interpolation` / `attribute_js_expr`, `html_interpolation` /
`permissible_text`) whose JavaScript it leaves as text for an injection, and script and style
elements. Its own `queries/injections.scm` injects TypeScript into all three. That leniency is why
the rate is low: an expression's syntax is never the grammar's to reject, so the dialect reads it
through the TypeScript injection, where TypeScript's grammar meets it.

## Method

`indexbench lang-oracle -dialect astro` on each subject (the harness's own `files_with_error`,
LB-F59: a file counts when the tree on the dialect's pinned grammar holds an ERROR or a MISSING node,
the extractor's `tsHasError` test, counted after timing). The file set is what the manifest claims,
every `*.astro` file, after ingest's gates (generated and minified files skipped). The query run
was a placeholder; `files_with_error` reads the tree, not the query.

| Subject | Pin | Licence (read at the pin) | Scope | `.astro` files | With an error |
|---|---|---|---|---|---|
| withastro/docs | main `22752c0ed9088bae0fabd2f15499f4151fe84cec` (no release tags) | MIT (`LICENSE`) | whole repository | 81 | **0 (0.0%)** |
| withastro/astro | `astro@7.3.8` `3571e2d2e80a66764254fa9b7750704a150ca3f6` | MIT (`LICENSE`) | `examples/**` | 91 | **0 (0.0%)** |
| withastro/astro (beside the gate) | the same | the same | `packages/**` | 1,490 | 6 (0.4%) |
| **the gate's corpus** | | | docs + examples | **172** | **0 (0.0%)** |

The six files in error under `packages/` are test fixtures, four of them named by a per-file probe
(`indexbench lang-probe`): `language-tools/language-server/test/fixture/cachingTest.astro`,
`language-tools/astro-check/test/fixture/fileWithWarnings.astro`,
`language-tools/language-server/test/check/fixture/fileWithWarnings.astro` (fixtures of the
language server's diagnostics) and `integrations/cloudflare/test/fixtures/top-level-return/src/pages/index.astro`
(a top-level `return` in the frontmatter).
