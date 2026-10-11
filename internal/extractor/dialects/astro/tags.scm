; dialects/astro/tags.scm — Astro components (LB-Q-astro, wave lf7, item angularastro; design §1.8's Astro paragraph)
; Written in this repository: virchau13/tree-sitter-astro ships highlights.scm and injections.scm only, and
; neither aider nor nvim-treesitter carries an Astro tags.scm (checked 2026-10-11). Shapes come from
; `indexbench lang-probe -dialect astro` over testdata/, never from grammar.js or node-types.json.
;
; An Astro file is one component. Its definitions are its frontmatter's and its scripts' (TypeScript, injected
; and hosted: injections.scm, F2) and its styles' (CSS); the component itself is the file's module symbol
; (`module_symbol`, K18). This query names only the template's calls.
;
; A component tag is a call (M2's principle, design §1.8): the component renders it, so the call is the
; module symbol's wherever the template writes it (`calls.owner "module"`), an element inside an expression
; included (the grammar parses `{items.map((i) => <Item />)}`'s element as an element). @astrojs/compiler's own
; `component` node type decides what is a component: a tag whose name starts A to Z or holds a `.`, except
; `Fragment`, the compiler's `fragment`. A custom element (a `-`) and an HTML element are no calls. The name is
; recorded as written; a member tag (`<UI.Card>`) is read by the harness's bare rule as its last segment (M2) and
; bound by the file resolver through the namespace import that names `UI`.
(start_tag
  (tag_name) @name
  (#match? @name "^[A-Z]|[.]")
  (#not-eq? @name "Fragment")) @reference.call

(self_closing_tag
  (tag_name) @name
  (#match? @name "^[A-Z]|[.]")
  (#not-eq? @name "Fragment")) @reference.call
