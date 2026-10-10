; dialects/typescript/tags.tsx.scm — the tsx variant's own tags patterns (O732,
; LB-F385): compiled before tags.scm for .tsx files alone (dialect.json
; variants[].queries.tags), so a node both name keeps this file's definition
; (oneDefPerNode). They are the React patterns only JSX can satisfy; a .ts,
; .mts or .cts file, which cannot write JSX, never runs them, and every other
; pattern (hooks, props, a component a wrapper call or an alias makes, a class
; component) is tags.scm's, read by both.

; React (LB-F377): a function declaration (at any depth, as before), or a
; function or arrow function bound at module scope or exported, is a
; component when its name is PascalCase (capitalized, with a lower-case
; letter: GET or MAX_SIZE is a constant's or an HTTP verb's) and its body
; writes JSX (a closing `</` or a self-closing `/>`: every JSX expression has
; one). Any other is the function or hook tags.scm names.
(function_declaration
  name: (identifier) @name
  body: (_) @body
  (#match? @name "^[A-Z].*[a-z]")
  (#match? @body "</|/>")) @definition.component

(program
  [(lexical_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.component)
   (variable_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.component)]
  (#match? @name "^[A-Z].*[a-z]")
  (#match? @body "</|/>"))

(export_statement
  [(lexical_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.component)
   (variable_declaration
     (variable_declarator
       name: (identifier) @name
       value: [(arrow_function body: (_) @body) (function_expression body: (_) @body)]) @definition.component)]
  (#match? @name "^[A-Z].*[a-z]")
  (#match? @body "</|/>"))

; A JSX element is a call to its tag (LB-F377): React calls the component the
; tag names, so a capitalized tag or a member tag (<Card/>, <Icon.Glyph/>) is
; a call from the enclosing definition, same-file or imported alike; an
; intrinsic lower-case tag (<div>) is none. The tsx grammar names its JSX
; elements, so the patterns name them as the javascript dialect's do.
([(jsx_opening_element
    name: (identifier) @reference.call)
  (jsx_self_closing_element
    name: (identifier) @reference.call)]
  (#match? @reference.call "^[A-Z]"))

([(jsx_opening_element
    name: (member_expression) @reference.call)
  (jsx_self_closing_element
    name: (member_expression) @reference.call)])
