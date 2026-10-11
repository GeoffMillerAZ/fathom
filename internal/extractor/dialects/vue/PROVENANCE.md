# vue — provenance

- Grammar: `tree-sitter-grammars/tree-sitter-vue` at `ce8011a414fdf8091f4e4071752efc376f4afb08` (MIT; the
  repository publishes no release tags), its shipped ABI 15 `src/parser.c` and `src/scanner.c`, built into
  `internal/extractor/tsengine/grammars/vue.wasm.gz`; pin and digests in
  `internal/extractor/tsengine/grammars/PROVENANCE.md`; the 5% ERROR-file gate in
  `docs/bench/readings/lang/vue-grammar.md` (LB-F533: 1 of 1,105 files).
- `tags.scm`: written in this repository (LB-F536). The grammar ships `highlights.scm` and
  `injections.scm`, no tags query, and neither aider nor nvim-treesitter carries a Vue tags or locals query
  (checked 2026-10-11); never GitNexus. Every shape comes from `indexbench lang-probe -dialect vue`.
  - A template element is a call from the component's module symbol when the Vue compiler types it a
    component (compiler-core `isComponent` under compiler-dom's options, `@vue/compiler-sfc` 3.5.43): its tag
    is none of the platform's native tags and not `<slot>` or `<component>`. The native list is
    `@vue/shared` 3.5.43's `HTML_TAGS`, `SVG_TAGS` and `MATH_TAGS` (MIT), copied as written into the
    pattern's `#not-any-of?`. `names.pascal_case` names the callee as `resolveComponent` resolves it
    (`capitalize(camelize(tag))`: `el-input` is `ElInput`).
  - The grammar reads a member tag `<Foo.Bar>` as the tag `Foo` with a `.Bar` v-bind shorthand: the third
    pattern names the call by the member, the generic pattern skips the tag.
  - A top-level element other than `<template>`, `<script>` and `<style>` is an SFC custom block
    (`<i18n>`, `<docs>`): a `block` definition (not callable), so the markup inside it is no call.
- `injections.scm`: written here after the grammar's own `queries/injections.scm` (MIT), which injects a
  `<script>` by its `lang` and a `<style>` as CSS: `lang="ts"` (or `typescript`, `tsx`) is TypeScript, any
  other script JavaScript, `lang="scss"` SCSS, a style with no `lang` CSS.
- The manifest's script kinds (`JSFunc`, `JSConst`, ...) are the TypeScript and JavaScript dialects': a
  component's definitions are its `<script>` blocks', which the file hosts (`injection_host`, F2), and the
  vue and svelte dialects share those source kinds with them (`sharedSourceKindGroups`). The module
  symbol, `VueComponent`, is the component; `package.strategy` `file` keys it by `Button.vue`, never the
  `button.ts` beside it (F1).
- Imports: relative-path's TS/JS rule (no `suffixes`): the path as written, then TypeScript's extension
  mapping and `.ts`, `.tsx`, `.d.ts`, `.js`, `.jsx`, then a directory's index. No implicit `.vue` suffix is
  tried, as Vite (create-vue's bundler) tries none (design §1.8, W4 for Vue and Svelte).
- Fixtures `testdata/*.vue` are written here.
