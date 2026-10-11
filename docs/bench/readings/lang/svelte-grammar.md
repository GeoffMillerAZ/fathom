# The Svelte grammars against the 5% ERROR-file gate (LB-F533; wave lf7, item vuesvelte)

**Verdict: tree-sitter-grammars/tree-sitter-svelte v1.0.2 passes the gate and is pinned: 34 of the 2,696
`.svelte` files of the two Svelte calibration subjects at their pins leave an ERROR or MISSING node, 1.26%,
against §14.2's 5% kill line. The catalogue's Himujjal/tree-sitter-svelte at 60ea1d67, which `indexbench
lang-pin -id svelte` drafts and flags `grammar-differs:tree-sitter-grammars/tree-sitter-svelte`, fails it:
266 of 2,696, 9.87% (14.77% of shadcn-svelte, which writes Svelte 5 throughout). Both are MIT at their pins,
so the gate decides, and the queue's grammar stands.**

Measured 2026-10-11 at fathom 2b8829a9 (wave lf7), before any dialect or reading.

## Grammars

| | tree-sitter-grammars/tree-sitter-svelte (the queue's) | Himujjal/tree-sitter-svelte (the catalogue's) |
|---|---|---|
| Pin | v1.0.2, `774a65aea563accc35f5d45fafa4d96ec5761f57` (latest tag) | `60ea1d673a1a3eeeb597e098d9ada9ed0c79ef4b` (`master`, no tag; package 0.11.0) |
| Tarball sha256 | `5fbca1c9857ea9af125ab43ea417b33e81b8268ef8003286bbd4fa9519bd4f23` | `10113fba418e5fd374d24d1a33c7981b221c80ee22bf765f5b27a50afefcf351` |
| Licence (`lang-pin -tree`) | MIT (`LICENSE`) | MIT (`LICENSE`) |
| Shipped parser | `src/parser.c` ABI 14, `src/scanner.c` | `src/parser.c` ABI 14, `src/scanner.c` with its own allocator and vector helpers |
| Pinned | yes: `svelte.wasm.gz` 171,044 bytes raw, 67,457 gzip (`build.sh svelte`) | no |

## Subjects and method

The gate's file set is every `*.svelte` file of the calibration subjects at their pins (LB-F534's
`calibration/corpus.d/svelte.yaml`): sveltejs/kit @sveltejs/kit@2.70.3 (`39e8e1fb`, MIT, 943 files) and
huntabyte/shadcn-svelte shadcn-svelte@1.7.0 (`33cd03e9`, MIT, 1,753 files). Method as `vue-grammar.md`:
tree-sitter CLI 0.27.0 `parse -p <checkout> --paths <list> -q -j --timeout 5000000`, each grammar compiled
from its own `src/` at the pin; a file counts when its summary says `successful: false`. No parse ran past
the timeout.

| Subject | Files | tree-sitter-grammars v1.0.2 | Himujjal 60ea1d67 |
|---|---|---|---|
| kit 2.70.3 | 943 | 4 (0.42%) | 7 (0.74%) |
| shadcn-svelte 1.7.0 | 1,753 | 30 (1.71%) | 259 (14.77%) |
| **both (the gate)** | **2,696** | **34 (1.26%)** | **266 (9.87%)** |

v1.0.2's 34: Svelte 5.29's `{@attach ...}` attachments, which postdate the grammar (shadcn's
`block-viewer-code.svelte`, `data-table-drag-handle.svelte`), a bare `&` in text (`site-footer.svelte`,
`separator-menu.svelte`), and kit's `test/build-errors/apps/syntax-error/src/routes/+page.svelte`, a file
written to fail. The Svelte 5 runes and snippets (`$state`, `$props`, `{#snippet}`, `{@render}`) the item
was warned of parse: runes are script code, and v1.0.2 reads snippet blocks and render tags.
