; source: written for this dialect, from the reference patterns of tags.scm (same node shapes, tree-sitter-python MIT).
;
; dialects/conan/imports.scm — a Conan requirement is the one place a recipe names another (LB-Q-conan, L3). The
; reference string `name/version[@user/channel][#revision]` is captured whole as a spec whose name is its module;
; the manifest's global-name strategy binds the name (names.cut_at keeps what precedes the "/") to the one recipe
; of the service that declares it. A recipe at another version, a name two recipes declare and a package no
; recipe of the service declares bind nothing.

; class attributes: requires = "a/1.0"
(class_definition
  superclasses: (argument_list
    [(identifier) @_base
     (attribute attribute: (identifier) @_base)])
  body: (block
    (expression_statement
      (assignment
        left: (identifier) @_attr
        right: (string (string_content) @import.name @import.module))))
  (#eq? @_base "ConanFile")
  (#match? @_attr "^(requires|tool_requires|build_requires|test_requires|python_requires)$"))

; requires = ["a/1.0", "b/2.0"]
(class_definition
  superclasses: (argument_list
    [(identifier) @_base
     (attribute attribute: (identifier) @_base)])
  body: (block
    (expression_statement
      (assignment
        left: (identifier) @_attr
        right: (list (string (string_content) @import.name @import.module)))))
  (#eq? @_base "ConanFile")
  (#match? @_attr "^(requires|tool_requires|build_requires|test_requires|python_requires)$"))

; requires = ("a/1.0", "b/2.0")
(class_definition
  superclasses: (argument_list
    [(identifier) @_base
     (attribute attribute: (identifier) @_base)])
  body: (block
    (expression_statement
      (assignment
        left: (identifier) @_attr
        right: (tuple (string (string_content) @import.name @import.module)))))
  (#eq? @_base "ConanFile")
  (#match? @_attr "^(requires|tool_requires|build_requires|test_requires|python_requires)$"))

; requires = "a/1.0", "b/2.0"
(class_definition
  superclasses: (argument_list
    [(identifier) @_base
     (attribute attribute: (identifier) @_base)])
  body: (block
    (expression_statement
      (assignment
        left: (identifier) @_attr
        right: (expression_list (string (string_content) @import.name @import.module)))))
  (#eq? @_base "ConanFile")
  (#match? @_attr "^(requires|tool_requires|build_requires|test_requires|python_requires)$"))

; self.requires("a/1.0", ...) and its tool/test/build siblings; a first argument that is not a string
; (self.requires(self.tested_reference_str), a variable) names no recipe.
(call
  function: (attribute
    object: (identifier) @_self
    attribute: (identifier) @_method)
  arguments: (argument_list . (string (string_content) @import.name @import.module))
  (#eq? @_self "self")
  (#match? @_method "^(requires|tool_requires|build_requires|test_requires)$"))
