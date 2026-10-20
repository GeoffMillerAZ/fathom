; source: written for this dialect (tree-sitter-grammars/tree-sitter-starlark v1.3.0 ships queries/ with folds,
; highlights, indents, injections and locals only; no tags). Node shapes read with `indexbench lang-probe`, never
; from grammar.js. One grammar reads every Bazel file: a BUILD file is a flat list of rule calls, a .bzl file defines
; functions, rules and macros, MODULE.bazel and WORKSPACE declare repositories. A statement that is a call with a
; literal `name = "x"` argument instantiates the target x (what `bazel query` lists), at the top level only: a call
; inside a def is a macro's body, whose names are computed.

; cc_library(name = "x", ...) and every macro or rule call written the same way, plain (cc_library) or through a
; struct (selects.config_setting_group)
((module
   (expression_statement
     (call
       function: (identifier) @_fn
       arguments: (argument_list
         (keyword_argument
           name: (identifier) @_k
           value: (string (string_content) @name)))) @definition.target))
 (#eq? @_k "name")
 (#not-any-of? @_fn "load" "package_group" "workspace" "module" "bazel_dep" "http_archive" "http_file" "http_jar"
   "git_repository" "new_git_repository" "local_repository" "new_local_repository" "local_path_override"
   "archive_override" "git_override" "single_version_override" "multiple_version_override" "use_extension"
   "use_repo_rule" "register_toolchains" "register_execution_platforms" "maven_install" "pip_parse"
   "crate_universe"))

((module
   (expression_statement
     (call
       function: (attribute object: (identifier) attribute: (identifier))
       arguments: (argument_list
         (keyword_argument
           name: (identifier) @_k
           value: (string (string_content) @name)))) @definition.target))
 (#eq? @_k "name"))

; package_group(name = "x", packages = [...]): a visibility group, a target no rule backs
((module
   (expression_statement
     (call
       function: (identifier) @_fn
       arguments: (argument_list
         (keyword_argument
           name: (identifier) @_k
           value: (string (string_content) @name)))) @definition.package_group))
 (#eq? @_k "name")
 (#eq? @_fn "package_group"))

; the module's own declaration, its dependencies and its repositories
((module
   (expression_statement
     (call
       function: (identifier) @_fn
       arguments: (argument_list
         (keyword_argument
           name: (identifier) @_k
           value: (string (string_content) @name)))) @definition.dependency))
 (#eq? @_k "name")
 (#any-of? @_fn "workspace" "module" "bazel_dep" "http_archive" "http_file" "http_jar" "git_repository"
   "new_git_repository" "local_repository" "new_local_repository" "local_path_override" "archive_override"
   "git_override" "single_version_override" "multiple_version_override" "maven_install" "pip_parse"))

; def macro(name, ...): the body runs to the end of the definition
(function_definition
  name: (identifier) @name
  body: (block) @body) @definition.function

; my_rule = rule(...), x = repository_rule(...), ext = module_extension(...), a = aspect(...), m = macro(...)
((module
   (expression_statement
     (assignment
       left: (identifier) @name
       right: (call function: (identifier) @_c)) @definition.rule))
 (#any-of? @_c "rule" "repository_rule" "module_extension" "aspect" "macro" "provider" "tag_class"))

; A label written ":x" inside a top-level call's argument: a reference to the target x of the same package
; (a string, a list of strings, a list joined with +, a select() branch). `load` is an import, not a call.
((module
   (expression_statement
     (call
       function: (_) @_fn
       arguments: (argument_list
         (keyword_argument
           value: (string (string_content) @name) @reference.call)))))
 (#not-eq? @_fn "load")
 (#match? @name "^:[A-Za-z0-9_]"))

((module
   (expression_statement
     (call
       function: (_) @_fn
       arguments: (argument_list
         (keyword_argument
           value: (list (string (string_content) @name) @reference.call))))))
 (#not-eq? @_fn "load")
 (#match? @name "^:[A-Za-z0-9_]"))

((module
   (expression_statement
     (call
       function: (_) @_fn
       arguments: (argument_list
         (keyword_argument
           value: (binary_operator (list (string (string_content) @name) @reference.call)))))))
 (#not-eq? @_fn "load")
 (#match? @name "^:[A-Za-z0-9_]"))

((module
   (expression_statement
     (call
       function: (_) @_fn
       arguments: (argument_list
         (keyword_argument
           value: (call
             function: (identifier) @_s
             arguments: (argument_list
               (dictionary (pair value: (list (string (string_content) @name) @reference.call))))))))))
 (#not-eq? @_fn "load")
 (#eq? @_s "select")
 (#match? @name "^:[A-Za-z0-9_]"))

; The rule or macro a statement calls: `cc_library(name = "x", ...)` in a BUILD file, `helper(x)` in a def. These are
; the calls that bind to a load() (what the file resolver follows to the .bzl file defining the callee). Only a
; statement's own call counts: a glob() or select() inside an argument is the call's argument, not what the target
; instantiates.
(expression_statement
  (call function: (identifier) @name) @reference.call)

(expression_statement
  (call function: (attribute attribute: (identifier) @name)) @reference.call)

(return_statement
  (call function: (identifier) @name) @reference.call)

(assignment
  right: (call function: (identifier) @name) @reference.call)
