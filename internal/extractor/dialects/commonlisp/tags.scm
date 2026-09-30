; source: tree-sitter-grammars/tree-sitter-commonlisp queries/tags.scm @ 9db594efb43574c5fe4a1f880568b1f0d0ee6057 (MIT)
; changes: upstream tags every `defun_header` (defun, defmacro, defgeneric and defmethod alike) as
; @definition.function, `defclass` as @definition.class, and every list with a symbol head as a call. This file
; keeps the grammar's `defun` node but tells its four keywords apart (universal-ctags reports function, macro,
; generic and method), takes the whole list as the definition (so an EndLine reaches the closing paren), adds @body
; (the first form after the header, so a Signature stops at the lambda list) and tags the definers the grammar
; has no node for by their head symbol (`defvar`, `defparameter`, `defconstant`, `deftype`, `defstruct`,
; `defclass`), with #eq? on the head, the shape ruby's attr_accessor uses.
; No call is captured: `(head arg ...)` is a call, a `let` binding pair, a `cond` clause and a data list in the
; same syntax, so calls belong to L2 (upstream's `@ignore` exclusions are not a capture this engine reads).
; Not extracted: `define-condition`, `defsetf`, `define-compiler-macro`, `define-symbol-macro`,
; `define-modify-macro` and `defpackage` (ctags reports the first five as `unknown`, a kind the manifest leaves
; unmapped); a `(:method ...)` option inside a `defgeneric`.

; `(defun name lambda-list [body...])`; a `(setf name)` function is named by `name`, as universal-ctags does.
((list_lit
  (defun
    (defun_header
      keyword: (defun_keyword) @_keyword
      function_name: [
        (sym_lit) @name
        (package_lit) @name
        (list_lit
          .
          (sym_lit) @_setf
          .
          (sym_lit) @name)
      ])
    .
    (_) @body)) @definition.function
 (#eq? @_keyword "defun")
 (#eq? @_setf "setf"))

((list_lit
  (defun
    (defun_header
      keyword: (defun_keyword) @_keyword
      function_name: [
        (sym_lit) @name
        (package_lit) @name
        (list_lit
          .
          (sym_lit) @_setf
          .
          (sym_lit) @name)
      ])
    .)) @definition.function
 (#eq? @_keyword "defun")
 (#eq? @_setf "setf"))

; `(defmacro name lambda-list [body...])`.
((list_lit
  (defun
    (defun_header
      keyword: (defun_keyword) @_keyword
      function_name: [
        (sym_lit) @name
        (package_lit) @name
        (list_lit
          .
          (sym_lit) @_setf
          .
          (sym_lit) @name)
      ])
    .
    (_) @body)) @definition.macro
 (#eq? @_keyword "defmacro")
 (#eq? @_setf "setf"))

((list_lit
  (defun
    (defun_header
      keyword: (defun_keyword) @_keyword
      function_name: [
        (sym_lit) @name
        (package_lit) @name
        (list_lit
          .
          (sym_lit) @_setf
          .
          (sym_lit) @name)
      ])
    .)) @definition.macro
 (#eq? @_keyword "defmacro")
 (#eq? @_setf "setf"))

; `(defgeneric name lambda-list [body...])`.
((list_lit
  (defun
    (defun_header
      keyword: (defun_keyword) @_keyword
      function_name: [
        (sym_lit) @name
        (package_lit) @name
        (list_lit
          .
          (sym_lit) @_setf
          .
          (sym_lit) @name)
      ])
    .
    (_) @body)) @definition.generic
 (#eq? @_keyword "defgeneric")
 (#eq? @_setf "setf"))

((list_lit
  (defun
    (defun_header
      keyword: (defun_keyword) @_keyword
      function_name: [
        (sym_lit) @name
        (package_lit) @name
        (list_lit
          .
          (sym_lit) @_setf
          .
          (sym_lit) @name)
      ])
    .)) @definition.generic
 (#eq? @_keyword "defgeneric")
 (#eq? @_setf "setf"))

; `(defmethod name lambda-list [body...])`.
((list_lit
  (defun
    (defun_header
      keyword: (defun_keyword) @_keyword
      function_name: [
        (sym_lit) @name
        (package_lit) @name
        (list_lit
          .
          (sym_lit) @_setf
          .
          (sym_lit) @name)
      ])
    .
    (_) @body)) @definition.method
 (#eq? @_keyword "defmethod")
 (#eq? @_setf "setf"))

((list_lit
  (defun
    (defun_header
      keyword: (defun_keyword) @_keyword
      function_name: [
        (sym_lit) @name
        (package_lit) @name
        (list_lit
          .
          (sym_lit) @_setf
          .
          (sym_lit) @name)
      ])
    .)) @definition.method
 (#eq? @_keyword "defmethod")
 (#eq? @_setf "setf"))

; `(defvar name ...)` and the other definers whose name is the second element.

((list_lit
  .
  (sym_lit) @_head
  .
  [(sym_lit) (package_lit)] @name) @definition.variable
 (#eq? @_head "defvar"))

((list_lit
  .
  (sym_lit) @_head
  .
  [(sym_lit) (package_lit)] @name) @definition.parameter
 (#eq? @_head "defparameter"))

((list_lit
  .
  (sym_lit) @_head
  .
  [(sym_lit) (package_lit)] @name) @definition.const
 (#eq? @_head "defconstant"))

((list_lit
  .
  (sym_lit) @_head
  .
  [(sym_lit) (package_lit)] @name) @definition.type
 (#eq? @_head "deftype"))

((list_lit
  .
  (sym_lit) @_head
  .
  [(sym_lit) (package_lit)] @name) @definition.class
 (#eq? @_head "defclass"))

((list_lit
  .
  (sym_lit) @_head
  .
  [(sym_lit) (package_lit)] @name) @definition.struct
 (#eq? @_head "defstruct"))

; `(defstruct (name options...) slots...)`: the name is the head of the options list.
((list_lit
  .
  (sym_lit) @_head
  .
  (list_lit
    .
    [(sym_lit) (package_lit)] @name)) @definition.struct
 (#eq? @_head "defstruct"))
