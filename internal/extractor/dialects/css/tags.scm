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
; see PROVENANCE.md; tsextract.go takes each match's OWN @name row for
; Line, so a multi-line group's later members keep their real line even
; though @definition.selector spans the whole rule_set).
;
; The alternation below, not `(_)`, is deliberate: `comment` is a "selectors"
; sibling too — a trailing `/* note */` after a comma in a multi-line group
; (real shape, testdata/comments.css) is itself a direct child of
; `selectors`, and `(_)` matched it as a spurious definition named
; "/* note */" (found against calibration/normalize.css, §13 step 21).
; Every shape below was confirmed present with `indexbench lang-probe`
; (never grammar.js/node-types.json): plain and compound (class_selector
; nests for `.a.b`) tag/class/id/universal/attribute/pseudo-class/
; pseudo-element/namespace/nesting selectors, and the four combinators.
(rule_set
  (selectors
    [
      (tag_name)
      (universal_selector)
      (class_selector)
      (id_selector)
      (attribute_selector)
      (pseudo_class_selector)
      (pseudo_element_selector)
      (namespace_selector)
      (nesting_selector)
      (descendant_selector)
      (child_selector)
      (sibling_selector)
      (adjacent_sibling_selector)
    ] @name)
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
