; dialects/bash/tags.scm — symbol + call queries for the wazero tree-sitter
; engine (internal/extractor.NewTSExtractor("bash"), ADR-021 §4, LB-Q-bash
; port of the incumbent bashextract.go CGO walker).
;
; source: Aider-AI/aider aider/queries/tree-sitter-language-pack/bash-tags.scm
; @ ff191affc6e1413f33dad914d9930a5993aad853 (Apache-2.0) — tree-sitter-bash
; ships no queries/tags.scm of its own (upstream v0.25.1 has only
; highlights.scm).
; changes: kept the function and call node shapes, but rewritten onto this
; engine's own capture convention — bare @name/@body inside
; @definition.<kind>/@reference.call (internal/extractor/tsextract.go's
; splitTSMatches/tsCalleeName; every existing dialect's tags.scm uses this,
; not aider's own @name.definition.<kind> naming). Added a @body capture
; per LB-F3 so Signature truncates at the function's opening brace.
; Replaced aider's bare `(variable_assignment name: ...)` — which fires on
; every assignment anywhere, including a function's own locals
; (`local x=$1`) — with a pattern anchored on the literal `export` keyword
; inside a `declaration_command`: only `export NAME=value` counts as a
; BashVariable definition (this queue item's notes ask for "exported
; vars", not every local). `declare -x`, `readonly`, `typeset` and a bare
; top-level `NAME=value` are deliberately left uncaptured — the `-x`/`-r`
; flags to `declare`/`typeset` are themselves anonymous `word` siblings,
; indistinguishable from any other flag without reading the flag text, and
; a bare assignment is not an export. `export NAME` with no `=value` (an
; already-set variable marked for export, no assignment node under it) is
; also left uncaptured — it has no natural Line/Signature home as a
; "definition". BashVariable is declared but not in `required_kinds`
; (§13.2 step 9, precedent: `dialects/python/dialect.json`'s "type" kind):
; universal-ctags' `Sh` kind list (`alias`, `function`, `heredoc`, `script`)
; has no variable kind, so it cannot corroborate `variable` via oracle B —
; declaring it required would refuse every measured reading (§1.4,
; internal/langbench/reading.go Validate).
;
; Predicate-free: every pattern relies on structural matching alone.

; --- functions ------------------------------------------------------------
; Both syntaxes tree-sitter-bash folds into the same function_definition/
; word shape: "function name() { }" (the `function` keyword is an anonymous
; sibling, elided here) and the bare POSIX "name() { }".
(function_definition
  name: (word) @name
  body: (compound_statement) @body) @definition.function

; --- exported variables ----------------------------------------------------
; export NAME=value — the literal "export" token distinguishes this from
; declare/local/readonly/typeset, which parse to the same
; declaration_command > variable_assignment shape (see header).
(declaration_command
  "export"
  (variable_assignment
    name: (variable_name) @name)) @definition.variable

; --- calls -------------------------------------------------------------
; Naive same-file call collection (the invoked command's name as source
; text): mirrors every other tags dialect's naive call collection — @name
; is the command's own word, so a dynamic command ("$cmd" arg) falls back
; to the whole `command_name` expression's text (LB-F4). Fires on every
; command word, including shell builtins (echo, cd, local) — "whatever is
; called", same as javascript.scm/python.scm's call_expression capture.
(command
  name: (command_name) @name) @reference.call
