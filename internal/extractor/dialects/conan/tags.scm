; source: written for this dialect against `indexbench lang-probe` output over testdata/; only the node
;   shapes are tree-sitter-python's (MIT, the pinned python grammar blob this dialect shares).
;
; dialects/conan/tags.scm — Conan recipes (conanfile.py) on the python grammar (LB-Q-conan).
;
; A recipe is a class that derives from ConanFile and writes `name = "<literal>"`; it is a callable
; named for that string, so everything it requires attaches to it: the class attributes `requires`,
; `tool_requires`, `build_requires`, `test_requires` and `python_requires` (a string, a list, a tuple
; or a bare tuple of strings) and `self.requires("x/1.0")`, `self.tool_requires`, `self.test_requires`
; and `self.build_requires` anywhere in the recipe's methods. The string is a Conan reference,
; `name/version[@user/channel][#revision]`; the manifest's names.cut_at keeps the name. A class without
; a literal name (a test_package recipe, a helper) is no recipe, and what it requires belongs to no one.

(class_definition
  superclasses: (argument_list
    [(identifier) @_base
     (attribute attribute: (identifier) @_base)])
  body: (block
    (expression_statement
      (assignment
        left: (identifier) @_attr
        right: (string (string_content) @name))))
  (#eq? @_base "ConanFile")
  (#eq? @_attr "name")) @definition.recipe

; class attributes: requires = "a/1.0"
(class_definition
  superclasses: (argument_list
    [(identifier) @_base
     (attribute attribute: (identifier) @_base)])
  body: (block
    (expression_statement
      (assignment
        left: (identifier) @_attr
        right: (string (string_content) @name) @reference.call)))
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
        right: (list (string (string_content) @name) @reference.call))))
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
        right: (tuple (string (string_content) @name) @reference.call))))
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
        right: (expression_list (string (string_content) @name) @reference.call))))
  (#eq? @_base "ConanFile")
  (#match? @_attr "^(requires|tool_requires|build_requires|test_requires|python_requires)$"))

; self.requires("a/1.0", ...) and its tool/test/build siblings; a first argument that is not a string
; (self.requires(self.tested_reference_str), a variable) names no recipe.
(call
  function: (attribute
    object: (identifier) @_self
    attribute: (identifier) @_method)
  arguments: (argument_list . (string (string_content) @name))
  (#eq? @_self "self")
  (#match? @_method "^(requires|tool_requires|build_requires|test_requires)$")) @reference.call
