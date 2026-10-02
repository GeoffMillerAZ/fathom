; source: written for this dialect (tree-sitter-grammars/tree-sitter-meson v1.3.0 ships queries/ with highlights,
; folds, indents and injections only; neither aider nor nvim-treesitter carries Meson tags). Node shapes read
; with `indexbench lang-probe`, never from grammar.js. A meson.build file is a sequence of statements; a
; definition is one function call, named by what Meson names it.

; project('name', 'c', ...)
((normal_command
   command: (identifier) @_fn
   . (variableunit . (string) @name .)) @definition.project
 (#eq? @_fn "project"))

; executable / library / custom_target ... : Meson names a target by its first string argument, the name
; `meson introspect --targets` lists, not the variable the call may be assigned to
((normal_command
   command: (identifier) @_fn
   . (variableunit . (string) @name .)) @definition.target
 (#any-of? @_fn "executable" "static_library" "shared_library" "library" "both_libraries" "shared_module" "custom_target" "jar" "build_target" "run_target"))

; subproject('name'). cmake.subproject('name') is a method of the cmake module, not this function: a method call is
; the `function:` of an expression_statement, so only the parents a statement can have are matched
((source_file
   (normal_command
     command: (identifier) @_fn
     . (variableunit . (string) @name .)) @definition.subproject)
 (#eq? @_fn "subproject"))
((if_command
   (normal_command
     command: (identifier) @_fn
     . (variableunit . (string) @name .)) @definition.subproject)
 (#eq? @_fn "subproject"))
((elseif_command
   (normal_command
     command: (identifier) @_fn
     . (variableunit . (string) @name .)) @definition.subproject)
 (#eq? @_fn "subproject"))
((else_command
   (normal_command
     command: (identifier) @_fn
     . (variableunit . (string) @name .)) @definition.subproject)
 (#eq? @_fn "subproject"))
((foreach_command
   (normal_command
     command: (identifier) @_fn
     . (variableunit . (string) @name .)) @definition.subproject)
 (#eq? @_fn "subproject"))
((operatorunit
   (normal_command
     command: (identifier) @_fn
     . (variableunit . (string) @name .)) @definition.subproject)
 (#eq? @_fn "subproject"))

; option('name', type : ...) in meson.options and meson_options.txt
((normal_command
   command: (identifier) @_fn
   . (variableunit . (string) @name .)) @definition.option
 (#eq? @_fn "option"))

; zdep = dependency('zlib'), core_dep = declare_dependency(...): a dependency object names nothing of its own,
; so the symbol is the variable that holds it, the name later arguments spell
((operatorunit
   (identifier) @name
   "="
   (normal_command
     command: (identifier) @_fn)) @definition.dependency
 (#any-of? @_fn "dependency" "declare_dependency"))

; libfoo = static_library('foo', ...): later arguments spell the variable (link_with : libfoo), not the target's
; own name 'foo', so the variable that holds a target is a definition too (the handle a reference resolves to,
; across the files a subdir() brings into scope)
((operatorunit
   (identifier) @name
   "="
   (normal_command
     command: (identifier) @_fn)) @definition.variable
 (#any-of? @_fn "executable" "static_library" "shared_library" "library" "both_libraries" "shared_module" "custom_target" "jar" "build_target" "run_target"))

; What a target or a dependency object is built from: link_with, link_whole, dependencies and depends name
; other targets and dependency objects by the variable that holds them. A reference belongs to the smallest
; enclosing definition above (a call that is an argument of another call is not a definition).
((pair
   key: (identifier) @_k
   value: (list variable: (identifier) @name @reference.call))
 (#any-of? @_k "link_with" "link_whole" "dependencies" "depends"))

((pair
   key: (identifier) @_k
   value: (identifier) @name @reference.call)
 (#any-of? @_k "link_with" "link_whole" "dependencies" "depends"))
