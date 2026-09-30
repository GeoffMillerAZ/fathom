; source: none upstream. tree-sitter-grammars/tree-sitter-tcl (850a72ab6436e06645b33b11cfa60cbdb04b1f01, MIT) ships no
; queries/tags.scm (only folds, highlights and indents), and neither aider's nor nvim-treesitter's collections carry a Tcl
; tags file, so these patterns are written for this repository against `indexbench lang-probe` trees.
; changes: not applicable (an original file). It tags `namespace eval NAME {BODY}` as a namespace, `proc NAME ARGS BODY` as a
; procedure (anywhere the grammar parses one: a namespace body, an `if` block, another procedure), and, inside
; `oo::class create NAME {BODY}`, the class and each `method NAME ARGS BODY`; every command with a plain name is a call.

; `namespace eval NAME { ... }`: the name is a word, the script a braced body. `namespace export`, `namespace import`,
; `namespace ensemble` and the rest are the same node with a different first word, so the first word is checked.
(namespace
  (word_list
    .
    (simple_word) @_sub
    .
    (simple_word) @name
    .
    (braced_word)
    .)
  (#eq? @_sub "eval")) @definition.namespace

; `proc NAME ARGS BODY`. A name built at run time (`proc $x`) or quoted has no simple word to record and is skipped.
(procedure
  name: (simple_word) @name
  body: (_) @body) @definition.procedure

; TclOO: `oo::class create NAME { ... }`.
(command
  name: (simple_word) @_cmd
  arguments: (word_list
    .
    (simple_word) @_create
    .
    (simple_word) @name
    .
    (braced_word)
    .)
  (#match? @_cmd "^(::)?oo::class$")
  (#eq? @_create "create")) @definition.class

; `method NAME ARGS BODY` as a direct child of a class body (a class named at run time has none).
(command
  name: (simple_word) @_cmd
  arguments: (word_list
    .
    (simple_word) @_create
    .
    (simple_word)
    .
    (braced_word
      (command
        name: (simple_word) @_method
        arguments: (word_list
          .
          (simple_word) @name
          .
          (_)
          .
          (_) @body
          .)) @definition.method))
  (#match? @_cmd "^(::)?oo::class$")
  (#eq? @_create "create")
  (#eq? @_method "method"))

; Calls: every command with a plain word for its name. `variable`, `global` and `upvar` declare names and `method`
; defines one, so none of them is a call. A callee spelled with a variable or a substitution (`$obj method`,
; `[lindex $l 0] x`) names nothing and is left out.
(command
  name: (simple_word) @name
  (#not-any-of? @name "variable" "global" "upvar" "method")) @reference.call
