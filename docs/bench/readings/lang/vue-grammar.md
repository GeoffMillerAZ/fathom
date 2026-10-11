# The Vue grammar against the 5% ERROR-file gate (LB-F533; wave lf7, item vuesvelte)

**Verdict: tree-sitter-grammars/tree-sitter-vue at ce8011a4 passes the gate: 1 of the 1,105 `.vue` files of
the two Vue calibration subjects at their pins leaves an ERROR or MISSING node, 0.09%, against §14.2's 5%
kill line. The one file is VitePress's `template/.vitepress/theme/Layout.vue`, an EJS template the
`create-vitepress` scaffolder renders (`<script setup<%= useTs ? ' lang="ts"' : '' %>>`), no Vue source.
The grammar is pinned (LB-F533) and LB-Q-vue proceeds.**

Measured 2026-10-11 at fathom 2b8829a9 (wave lf7), before any dialect or reading.

## Grammar

| Field | Value |
|---|---|
| Repository | tree-sitter-grammars/tree-sitter-vue (the queue's `grammar_source`; the catalogue names the same repository) |
| Pin | `ce8011a414fdf8091f4e4071752efc376f4afb08` (`main`; the repository publishes no release tags) |
| Tarball sha256 | `e62913c5709ed1e68616bb85aedb2641a0c35c6b6709986ab71487d54b16ad0d` (`git get-tar-commit-id`: the pin) |
| Licence | MIT, read at the pin by `indexbench lang-pin -tree` (`licence_basis: LICENSE`, not refused) |
| ABI | 15: the pin ships its generated `src/parser.c` (`LANGUAGE_VERSION 15`) and `src/scanner.c`, so the lock row is `generate: false`, `scanner: true`, ABI 15. The catalogue's draft (`indexbench lang-pin -id vue`) says `generate: true`, ABI 14; the shipped parser is preferred (§13.1 step 6) and no CLI generates it |
| Blob | `vue.wasm.gz`, 155,696 bytes raw, 63,858 gzip (`build.sh vue`, zig 0.16.0, wasm-opt 129) |

## Subjects and method

The gate's file set is every `*.vue` file of the calibration subjects at their pins (the subjects LB-F534
pins in `calibration/corpus.d/vue.yaml`): element-plus/element-plus 2.14.7 (`77d5c32c`, MIT, 1,008 files)
and vuejs/vitepress v1.6.4 (`1fc537b7`, MIT, 97 files). vuejs/docs, the queue's second subject, is not
measured: its LICENSE puts the repository's contents under CC BY 4.0, which is no OSI licence (§13.3), so
LB-F534 substitutes VitePress, the Vue team's own Vue 3 application.

tree-sitter CLI 0.27.0, `tree-sitter parse -p <grammar checkout at the pin> --paths <list> -q -j --timeout
5000000`, the grammar compiled natively from its own `src/parser.c` and `src/scanner.c` (the zsh
measurement's method, `zsh-grammars.md`). A file counts when its summary says `successful: false`, the
CLI's `ts_node_has_error` test (an ERROR or a MISSING node), the extractor's `tsHasError` and the harness's
`files_with_error`. No parse ran past the 5 s timeout.

| Subject | Files | ERROR or MISSING | Rate |
|---|---|---|---|
| element-plus 2.14.7 | 1,008 | 0 | 0.00% |
| vitepress v1.6.4 | 97 | 1 (`template/.vitepress/theme/Layout.vue`, EJS) | 1.03% |
| **both (the gate)** | **1,105** | **1** | **0.09%** |

The grammar parses an SFC's blocks and its template's markup; a `<script>` and `<style>` are raw text the
dialect injects into TypeScript, JavaScript, CSS or SCSS (F2), so the gate reads the SFC grammar alone.
