; dialects/scss/tags.scm — SCSS (LB-Q-scss, LB-F465, wave lf5) ──────────────
; Written in this repository for P233/tree-sitter-scss v1.0.1, the one SCSS
; grammar that parses the queue's subjects under section 14.2's 5% line (0 of
; 235 files in error, O850). It ships highlights.scm only, no tags.scm. Every
; shape below comes from `indexbench lang-probe -dialect scss`, never from
; grammar.js or node-types.json. Names drop their sigil (manifest
; names.strip_prefix: `$`, `.`, `#`, `%`), as Sass's own AST names a variable
; and as universal-ctags' SCSS parser, the L1 oracle, writes every tag.

; @mixin name(...) { ... } and @function name(...) { ... }: callables. The
; block's `{` is the definition's own token (the declaration_block starts
; after it), so the signature ends at that token.
(mixin_definition
  name: (function_name) @name
  "{" @body) @definition.mixin

(function_definition
  name: (function_name) @name
  "{" @body) @definition.function

; $name: value — at the top level and inside any block (ctags tags both).
(variable_declaration
  name: (variable_name) @name) @definition.variable

; Loop variables: `@each $key, $value in $map` declares every variable before
; `in`; `@for $i from a through b` declares $i. The definition is the
; variable itself: a loop is no definition, so the references inside it stay
; with the definition around the loop.
(each_statement
  (variable_name) @name @definition.variable
  "in")

(for_statement
  .
  (variable_name) @name @definition.variable)

; Class, id and placeholder names a style rule's selectors write, as ctags'
; SCSS parser tags them: each `.name`, `#name` and `%name` of the rule's
; selector list, also inside a compound or complex selector and inside a
; pseudo-class's selector arguments (`.a:not(.b)`, one level deep), never in
; @extend (a reference). Each is a tag, as ctags writes it: the definition is
; the selector itself, not its rule, so the references a rule's block makes
; (an @include, a function call) belong to the definition around the rule,
; the file's module symbol at the top level. An interpolated name keeps its
; interpolation (`btn-#{$color}`), where ctags cuts it at `#{` (`btn-`).
(rule_set
  selectors: (selectors
    [
      (class_selector) @name @definition.class
      (compound_selector (class_selector) @name @definition.class)
      (complex_selector (class_selector) @name @definition.class)
      (complex_selector (compound_selector (class_selector) @name @definition.class))
      (pseudo_selector (selector_arguments (class_selector) @name @definition.class))
      (compound_selector (pseudo_selector (selector_arguments (class_selector) @name @definition.class)))
      (complex_selector (pseudo_selector (selector_arguments (class_selector) @name @definition.class)))
      (complex_selector (compound_selector (pseudo_selector (selector_arguments (class_selector) @name @definition.class))))
      (pseudo_selector (selector_arguments (compound_selector (class_selector) @name @definition.class)))
      (compound_selector (pseudo_selector (selector_arguments (compound_selector (class_selector) @name @definition.class))))
      (complex_selector (pseudo_selector (selector_arguments (compound_selector (class_selector) @name @definition.class))))
      (complex_selector (compound_selector (pseudo_selector (selector_arguments (compound_selector (class_selector) @name @definition.class)))))
      (pseudo_selector (selector_arguments (complex_selector (class_selector) @name @definition.class)))
      (compound_selector (pseudo_selector (selector_arguments (complex_selector (class_selector) @name @definition.class))))
      (complex_selector (pseudo_selector (selector_arguments (complex_selector (class_selector) @name @definition.class))))
      (complex_selector (compound_selector (pseudo_selector (selector_arguments (complex_selector (class_selector) @name @definition.class)))))
    ]))

(rule_set
  selectors: (selectors
    [
      (id_selector) @name @definition.id
      (compound_selector (id_selector) @name @definition.id)
      (complex_selector (id_selector) @name @definition.id)
      (complex_selector (compound_selector (id_selector) @name @definition.id))
      (pseudo_selector (selector_arguments (id_selector) @name @definition.id))
      (compound_selector (pseudo_selector (selector_arguments (id_selector) @name @definition.id)))
      (complex_selector (pseudo_selector (selector_arguments (id_selector) @name @definition.id)))
      (complex_selector (compound_selector (pseudo_selector (selector_arguments (id_selector) @name @definition.id))))
      (pseudo_selector (selector_arguments (compound_selector (id_selector) @name @definition.id)))
      (compound_selector (pseudo_selector (selector_arguments (compound_selector (id_selector) @name @definition.id))))
      (complex_selector (pseudo_selector (selector_arguments (compound_selector (id_selector) @name @definition.id))))
      (complex_selector (compound_selector (pseudo_selector (selector_arguments (compound_selector (id_selector) @name @definition.id)))))
      (pseudo_selector (selector_arguments (complex_selector (id_selector) @name @definition.id)))
      (compound_selector (pseudo_selector (selector_arguments (complex_selector (id_selector) @name @definition.id))))
      (complex_selector (pseudo_selector (selector_arguments (complex_selector (id_selector) @name @definition.id))))
      (complex_selector (compound_selector (pseudo_selector (selector_arguments (complex_selector (id_selector) @name @definition.id)))))
    ]))

(rule_set
  selectors: (selectors
    [
      (placeholder_selector) @name @definition.placeholder
      (compound_selector (placeholder_selector) @name @definition.placeholder)
      (complex_selector (placeholder_selector) @name @definition.placeholder)
      (complex_selector (compound_selector (placeholder_selector) @name @definition.placeholder))
      (pseudo_selector (selector_arguments (placeholder_selector) @name @definition.placeholder))
      (compound_selector (pseudo_selector (selector_arguments (placeholder_selector) @name @definition.placeholder)))
      (complex_selector (pseudo_selector (selector_arguments (placeholder_selector) @name @definition.placeholder)))
      (complex_selector (compound_selector (pseudo_selector (selector_arguments (placeholder_selector) @name @definition.placeholder))))
      (pseudo_selector (selector_arguments (compound_selector (placeholder_selector) @name @definition.placeholder)))
      (compound_selector (pseudo_selector (selector_arguments (compound_selector (placeholder_selector) @name @definition.placeholder))))
      (complex_selector (pseudo_selector (selector_arguments (compound_selector (placeholder_selector) @name @definition.placeholder))))
      (complex_selector (compound_selector (pseudo_selector (selector_arguments (compound_selector (placeholder_selector) @name @definition.placeholder)))))
      (pseudo_selector (selector_arguments (complex_selector (placeholder_selector) @name @definition.placeholder)))
      (compound_selector (pseudo_selector (selector_arguments (complex_selector (placeholder_selector) @name @definition.placeholder))))
      (complex_selector (pseudo_selector (selector_arguments (complex_selector (placeholder_selector) @name @definition.placeholder))))
      (complex_selector (compound_selector (pseudo_selector (selector_arguments (complex_selector (placeholder_selector) @name @definition.placeholder)))))
    ]))

; @keyframes name { ... }: one definition per animation (ctags has no SCSS kind).
(keyframes_statement
  (keyframes_name) @name
  body: (declaration_block) @body) @definition.keyframes

; A custom property (`--name: value;`), as the css dialect reads it.
(property_declaration
  name: (property_name) @name
  (#match? @name "^--")) @definition.custom_property

; References: an @include names a mixin, a call names a function (a CSS
; function such as rgba() too, which names no definition and binds nothing).
(include_statement
  name: (function_name) @name) @reference.call

(call_expression
  name: (function_name) @name) @reference.call
