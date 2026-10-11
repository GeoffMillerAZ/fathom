; dialects/svelte/tags.scm — Svelte components (LB-Q-svelte, wave lf7) ───────
; Written in this repository: tree-sitter-grammars/tree-sitter-svelte ships
; highlights, injections, indents, folds and locals queries, no tags.scm,
; and neither aider nor nvim-treesitter carries a svelte tags query to start
; from (see PROVENANCE.md). Shapes from `indexbench lang-probe -dialect
; svelte`.
;
; A component file's own definitions are its instance and module <script>
; blocks' (the injected TypeScript or JavaScript, F2: dialect.json's
; injection_host) and its module symbol, the component (module_symbol). This
; query names what the markup renders: an element the Svelte compiler parses
; as a component (a Component node) is a call from the component's module
; symbol to it (owner ruling M2; design §1.8, K19). No definition of this
; query encloses markup, so the module symbol owns every one.
;
; svelte/compiler 5.57.2 (phases/1-parse/state/element.js) parses a tag as a
; component when it starts with an upper-case letter or is a dotted
; identifier path (<Card.Root>, <foo.bar>); a svelte: tag (svelte:component,
; svelte:self, svelte:head, ...) is never one. A dotted tag is stored as
; written; its pair names its last segment (M2, bare-name matching).
(element
  [(start_tag (tag_name) @name) (self_closing_tag (tag_name) @name)] @reference.call
  (#match? @name "^(\\p{Lu}|[\\p{L}_$][\\p{L}\\p{N}_$]*\\.)"))
