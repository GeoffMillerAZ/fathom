; source: Aider-AI/aider aider/queries/tree-sitter-language-pack/bash-tags.scm
; @ ff191affc6e1413f33dad914d9930a5993aad853 (Apache-2.0), verbatim below this
; header. tree-sitter/tree-sitter-bash v0.25.1 (a06c2e44) ships no
; queries/tags.scm (only highlights.scm), so this is the nearest
; licence-allowed upstream tags query (design §8.2, §13.2 step 11 order).

(function_definition
  name: (word) @name.definition.function) @definition.function

(variable_assignment
  name: (variable_name) @name.definition.variable) @definition.variable

(command
  name: (command_name) @name.reference.call) @reference.call
