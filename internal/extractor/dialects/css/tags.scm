; dialects/css/tags.scm — CSS (LB-Q-css, design §13.2) ──────────────────────
; Written in this repository: tree-sitter/tree-sitter-css ships no
; queries/tags.scm of its own (only highlights.scm), and neither aider nor
; nvim-treesitter carries a css-tags.scm/locals.scm to start from (checked,
; 2026-09-27) — see PROVENANCE.md. Shapes below come from `indexbench
; lang-probe -dialect css testdata/shapes.css` (§13 step 12), never from
; grammar.js/node-types.json.
;
; A rule's selector list is one CSS construct with N comma-separated
; selectors (`.a, .b { ... }` is ONE rule_set with a `selectors` node of two
; children). universal-ctags' CSS parser tags each such selector
; independently, at the rule's own position — this pattern reproduces that:
; one match per direct child of `selectors`, each carrying the SAME
; enclosing rule_set as its definition range (so EndLine reflects the rule's
; real close-brace line even though ctags itself proves no end for CSS —
; see PROVENANCE.md). `(_)` matches any selector shape: class_selector,
; id_selector, pseudo_class_selector (`:root`, `a:hover`), tag_name,
; universal_selector, attribute_selector, nesting_selector (CSS Nesting's
; `&`).
(rule_set
  (selectors
    (_) @name)
  (block) @body) @definition.selector

; @keyframes <name> { ... } — one definition per animation; the individual
; from/to/N% blocks inside are not captured (ctags has no equivalent and
; the vision's fleet-gap note names "keyframes" as a whole, not its steps).
(keyframes_statement
  (keyframes_name) @name
  (keyframe_block_list) @body) @definition.keyframes

; A custom property (`--name: value;`) is syntactically an ordinary
; declaration; #match? (LB-F2) is the only way to single it out. Declared
; anywhere a declaration can appear (:root, a class, inside @media) — the
; vision's fleet-gap note names "custom properties" as their own kind.
; ctags 6.2.1 has no CSS kind for these (checked directly, PROVENANCE.md);
; deliberately absent from oracle_kinds, same convention as PythonTypeAlias.
(declaration
  (property_name) @name
  (#match? @name "^--")) @definition.custom_property
