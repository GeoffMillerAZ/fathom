# svelte — provenance

- Grammar: `tree-sitter-grammars/tree-sitter-svelte` v1.0.2
  (`774a65aea563accc35f5d45fafa4d96ec5761f57`, MIT), its shipped ABI 14 `src/parser.c` and `src/scanner.c`,
  built into `internal/extractor/tsengine/grammars/svelte.wasm.gz`; pin and digests in
  `internal/extractor/tsengine/grammars/PROVENANCE.md`. The queue's grammar; the catalogue's
  Himujjal/tree-sitter-svelte (MIT) failed the 5% ERROR-file gate on the same subjects (9.87% against
  1.26%; `docs/bench/readings/lang/svelte-grammar.md`, LB-F533).
- `tags.scm`: written in this repository (LB-F537). The grammar ships highlights, injections, indents,
  folds and locals queries, no tags query, and neither aider nor nvim-treesitter carries a Svelte tags query
  (checked 2026-10-11); never GitNexus. Every shape comes from `indexbench lang-probe -dialect svelte`.
  - An element is a call from the component's module symbol when the Svelte compiler parses it as a
    component (svelte/compiler 5.57.2 `phases/1-parse/state/element.js`): a tag starting with an upper-case
    letter, or a dotted identifier path (`<Card.Root>`); a `svelte:` tag never is. A dotted tag is stored
    as written; its pair names its last segment (owner ruling M2).
- `injections.scm`: as the vue dialect's (both after the grammars' own `queries/injections.scm`, MIT).
- The manifest's script kinds are the TypeScript and JavaScript dialects' (`injection_host`, F2;
  `sharedSourceKindGroups`). The module symbol, `SvelteComponent`, is the component, keyed by
  `+page.svelte`, never the `+page.ts` beside it (F1).
- Imports: relative-path's TS/JS rule, no implicit `.svelte` suffix (SvelteKit's Vite tries none).
- Fixtures `testdata/*.svelte` are written here.
