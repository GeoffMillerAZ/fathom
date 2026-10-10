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
; Any named child of `selectors` but a comment: a trailing `/* note */` after
; a comma in a multi-line group (real shape, testdata/comments.css) is itself a
; direct child of `selectors`, and matched as a spurious definition named
; "/* note */" (found against calibration/normalize.css, §13 step 21). One
; wildcard, not an alternation of the selector node types (plain and compound
; tag/class/id/universal/attribute/pseudo-class/pseudo-element/namespace/
; nesting selectors and the four combinators, each confirmed with
; `indexbench lang-probe`): the query engine emits an alternation's matches
; grouped by alternative, so `:where(a), [role=link]` listed [role=link]
; first, and the owner of a rule's references is the rule's FIRST selector in
; source order (the attachment below takes the first definition of the
; rule's range; LB-F459, found on pico).
(rule_set
  (selectors
    (_) @name
    (#not-match? @name "^/[*]"))
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

; References (design 1.8 W2, LB-F459). CSS has no calls; its two name
; references are a var() read of a custom property and an animation naming an
; @keyframes rule. Each is a @reference.call whose callee is the referenced
; name as written (`--gap`, `fade-in`), so a pair is (owner, name) and the name
; resolves like any callee. Shapes from `indexbench lang-probe -dialect css`.
;
; var(--x) and var(--x, fallback): the first argument, a dashed ident. A var()
; nested in the fallback is its own call_expression, matched on its own.
(call_expression
  (function_name) @_fn
  (#eq? @_fn "var")
  (arguments
    .
    (plain_value) @name
    (#match? @name "^--"))) @reference.call

; animation-name (and its vendor-prefixed spellings): every name in the list.
; `none` names no @keyframes rule, and the CSS-wide keywords are no names
; (CSS Animations 1, the <keyframes-name> grammar).
(declaration
  (property_name) @_p
  (#match? @_p "^(-[a-z]+-)?animation-name$")
  (plain_value) @name @reference.call
  (#not-any-of? @name "none" "initial" "inherit" "unset" "revert" "revert-layer"))

; The animation shorthand: an identifier that is none of the shorthand's
; keywords is its <keyframes-name> (CSS Animations 1 section 3.11: a keyword
; is read as the keyword, never as a name). Times, numbers and functions
; (steps(), cubic-bezier()) are not plain_value nodes, so never match.
(declaration
  (property_name) @_p
  (#match? @_p "^(-[a-z]+-)?animation$")
  (plain_value) @name @reference.call
  (#not-any-of? @name "none" "initial" "inherit" "unset" "revert" "revert-layer"
    "linear" "ease" "ease-in" "ease-out" "ease-in-out" "step-start" "step-end"
    "infinite" "normal" "reverse" "alternate" "alternate-reverse"
    "forwards" "backwards" "both" "running" "paused" "auto"))

; Owners (LB-F65 attachments): a reference belongs to the narrowest definition
; around it. No CSS kind is callable, so each owner is named here: a rule's
; block goes to the rule's first selector (the first definition of that
; range), a custom property's declaration to the property (`--b: var(--a)` is
; --b reading --a), an @keyframes rule's blocks to the rule. A reference in no
; such range (@font-face, @page) has no owner and is not stored.
(rule_set
  (block) @calls.from) @calls.to

(declaration
  (property_name) @_cp
  (#match? @_cp "^--")) @calls.to @calls.from

(keyframes_statement
  (keyframe_block_list) @calls.from) @calls.to
