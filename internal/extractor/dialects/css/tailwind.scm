; dialects/css/tailwind.scm — Tailwind CSS v4 (LB-Q-tailwind) ───────────────
; The css dialect's Tailwind variant: patterns compiled BEFORE tags.scm's for a
; .css file that carries a v4 at-rule (match: the variant's sniff in
; dialect.json), so a node both name keeps this file's definition
; (oneDefPerNode). A plain stylesheet never runs this file. Written in this
; repository from `indexbench lang-probe -dialect css` trees of the shapes in
; testdata/tailwind.css (§13 step 12); tree-sitter/tree-sitter-css is the
; grammar and ships no Tailwind queries. See PROVENANCE.md.
;
; A file's v4 spellings the grammar does not read (the shorthand
; `@custom-variant name (selector);`, `@utility name-*`, `--ns-*: initial;`,
; the arguments of @source, @plugin, @config, the options of @import) are
; blanked by the variant's preprocess key before this query runs
; (tsextract_tailwind.go), so every pattern below reads a clean tree.

; @theme { --name: value; }, also `@theme inline`, `@theme static`,
; `@theme default`. Each custom property of the block is a theme variable
; (the compiler's design-system key); a namespace reset (`--color-*: initial`)
; names no variable and arrives blanked.
(at_rule
  (at_keyword) @_k
  (#eq? @_k "@theme")
  (block
    (declaration
      (property_name) @name
      (#match? @name "^--[A-Za-z0-9_-]*[A-Za-z0-9_]$")) @definition.tw_theme))

; @utility name { ... } and @utility name-* { ... } (functional: the name reads
; `name-` once the star is blanked).
(at_rule
  (at_keyword) @_k
  (#eq? @_k "@utility")
  (keyword_query) @name
  (block) @body) @definition.tw_utility

; @custom-variant name { ... } and @custom-variant name (selector);
(at_rule
  (at_keyword) @_k
  (#eq? @_k "@custom-variant")
  (keyword_query) @name) @definition.tw_variant

; @variant name (selector);  (the v4.0 definition spelling, which the compiler still
; reads; with a block it is a use of the variant, and `{` in the rule's text says so)
((at_rule
  (at_keyword) @_k
  (#eq? @_k "@variant")
  (keyword_query) @name) @definition.tw_variant @_rule
  (#not-match? @_rule "[{]"))

; @apply a b c; — every candidate is a name read. A utility the file's @import
; closure defines resolves to its @utility; any other candidate (a core
; utility, a variant-prefixed one) resolves to nothing and is dropped.
; Two shapes: a lone name is an at_rule's keyword_query, several are a
; postcss_statement's plain_values.
(postcss_statement
  (at_keyword) @_k
  (#eq? @_k "@apply")
  (plain_value) @name @reference.call)

(at_rule
  (at_keyword) @_k
  (#eq? @_k "@apply")
  (keyword_query) @name @reference.call)

; Owners: an @apply or var() inside @utility's body belongs to the utility.
(at_rule
  (at_keyword) @_k
  (#eq? @_k "@utility")
  (block) @calls.from) @calls.to
