; source: written for this dialect against `indexbench lang-probe` output over testdata/; only the node
;   shapes are tree-sitter-python's (MIT, the pinned python grammar blob this dialect shares).
;
; dialects/checkov-policies/tags.scm — Checkov Python checks on the python grammar (LB-Q-checkov-policies).
;
; A check is a class that writes its id as a string literal in __init__ and hands it to its base with
; super().__init__(...): a local `id = "CKV_X_1"` (the shape checkov's own checks use, beside `name` and
; `supported_resources` locals) or the keyword `id="CKV_X_1"` of the call; checkov's shared base classes
; for a family of checks (the Abs* port and role checks) call theirs `check_id`. The base is whatever the
; class derives from: a family's base is itself a check class that no name pattern tells from a helper's.
; It is a callable named for that id, so the resource types it is registered for attach to it:
; `supported_resources` (and `supported_resource`), `supported_entities`, `supported_kind`,
; `supported_provider` and `supported_data`, written as a
; list or a tuple of strings, a local in __init__ or a keyword of the super().__init__ call. A class with
; no id literal (an abstract base, an id computed from an argument) is no check, and what it names
; belongs to no one.

; id = "CKV_X_1" in __init__, then super().__init__(...)
(class_definition
  superclasses: (argument_list)
  body: (block
    (function_definition
      name: (identifier) @_init
      body: (block
        (expression_statement
          (assignment
            left: (identifier) @_id
            right: (string (string_content) @name)))
        (expression_statement
          (call
            function: (attribute
              object: (call function: (identifier) @_super)
              attribute: (identifier) @_initcall))))))
  (#eq? @_init "__init__")
  (#match? @_id "^(check_)?id$")
  (#eq? @_super "super")
  (#eq? @_initcall "__init__")) @definition.check

; super().__init__(..., id="CKV_X_1", ...)
(class_definition
  superclasses: (argument_list)
  body: (block
    (function_definition
      name: (identifier) @_init
      body: (block
        (expression_statement
          (call
            function: (attribute
              object: (call function: (identifier) @_super)
              attribute: (identifier) @_initcall)
            arguments: (argument_list
              (keyword_argument
                name: (identifier) @_kw
                value: (string (string_content) @name))))))))
  (#eq? @_init "__init__")
  (#eq? @_super "super")
  (#eq? @_initcall "__init__")
  (#match? @_kw "^(check_)?id$")) @definition.check

; supported_resources = ["aws_x", "aws_y"]
(assignment
  left: (identifier) @_attr
  right: (list (string (string_content) @name) @reference.call)
  (#match? @_attr "^supported_(resources?|entities|kind|provider|data)$"))

; supported_resources = ("aws_x", "aws_y")
(assignment
  left: (identifier) @_attr
  right: (tuple (string (string_content) @name) @reference.call)
  (#match? @_attr "^supported_(resources?|entities|kind|provider|data)$"))

; super().__init__(..., supported_resources=["aws_x"], ...) and the tuple form
(keyword_argument
  name: (identifier) @_attr
  value: (list (string (string_content) @name) @reference.call)
  (#match? @_attr "^supported_(resources?|entities|kind|provider|data)$"))

(keyword_argument
  name: (identifier) @_attr
  value: (tuple (string (string_content) @name) @reference.call)
  (#match? @_attr "^supported_(resources?|entities|kind|provider|data)$"))
