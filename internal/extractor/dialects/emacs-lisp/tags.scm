; source: Wilfred/tree-sitter-elisp queries/tags.scm @ c0417c751a3dcc8724d1f2f6d1437a99dee1e090 (MIT)
; changes: upstream tags `defun`/`defsubst` and `defmacro` as @definition.function and nothing else. This file
; keeps the two grammar nodes and gives each its own kind (defsubst is `subst`, defmacro is `macro`, as in
; universal-ctags), adds @body (the first form after the parameter list: a docstring or the first body form)
; so a Signature stops at the parameters, and tags the definers the grammar does not give a node of their own
; by their head symbol (`defvar` and `defconst` are special forms with an anonymous keyword; `defcustom`,
; `defgroup`, `defface`, `defvar-local`, `define-inline`, `defalias`, `define-obsolete-function-alias`,
; `define-derived-mode` and the minor-mode definers are plain lists told apart by #eq?/#any-of? on the head).
; No call is captured: `(head arg ...)` is a call, a binding pair, a macro clause and a data list in the same
; syntax (`(let ((x 1)) ...)`, `(dolist (e xs) ...)`, `(cond (t 1))`), so calls belong to L2.

; `(defun name (params) [doc] body...)` and `(defsubst ...)`.
(function_definition
  "defun"
  name: (symbol) @name
  parameters: (_)
  .
  (_) @body) @definition.function

(function_definition
  "defun"
  name: (symbol) @name
  parameters: (_) .) @definition.function

(function_definition
  "defsubst"
  name: (symbol) @name
  parameters: (_)
  .
  (_) @body) @definition.subst

(function_definition
  "defsubst"
  name: (symbol) @name
  parameters: (_) .) @definition.subst

; `(defmacro name (params) [doc] body...)`.
(macro_definition
  name: (symbol) @name
  parameters: (_)
  .
  (_) @body) @definition.macro

(macro_definition
  name: (symbol) @name
  parameters: (_) .) @definition.macro

; `(defvar name)`, `(defvar name value [doc])` and `(defconst name value [doc])`: special forms.
(special_form
  "defvar"
  .
  (symbol) @name) @definition.variable

(special_form
  "defconst"
  .
  (symbol) @name) @definition.const

; `(defvar-local name value [doc])`.
((list
  .
  (symbol) @_head
  .
  (symbol) @name) @definition.variable
 (#eq? @_head "defvar-local"))

((list
  .
  (symbol) @_head
  .
  (symbol) @name) @definition.custom
 (#eq? @_head "defcustom"))

((list
  .
  (symbol) @_head
  .
  (symbol) @name) @definition.face
 (#eq? @_head "defface"))

((list
  .
  (symbol) @_head
  .
  (symbol) @name) @definition.group
 (#eq? @_head "defgroup"))

; `(define-inline name (params) [doc] body...)`.
((list
  .
  (symbol) @_head
  .
  (symbol) @name
  .
  (_)
  .
  (_) @body) @definition.inline
 (#eq? @_head "define-inline"))

((list
  .
  (symbol) @_head
  .
  (symbol) @name
  .
  (_) .) @definition.inline
 (#eq? @_head "define-inline"))

; Modes: `(define-derived-mode name parent "Lighter" [doc] body...)` and the minor-mode definers.
((list
  .
  (symbol) @_head
  .
  (symbol) @name) @definition.derivedMode
 (#eq? @_head "define-derived-mode"))

((list
  .
  (symbol) @_head
  .
  (symbol) @name) @definition.minorMode
 (#any-of? @_head "define-minor-mode" "define-globalized-minor-mode" "define-global-minor-mode"))

; Aliases name the new symbol in a quote: `(defalias 'new #'old)`.
((list
  .
  (symbol) @_head
  .
  (quote
    (symbol) @name)) @definition.alias
 (#any-of? @_head "defalias" "define-obsolete-function-alias"))

; cl-lib and eieio (universal-ctags has no kind for these): `cl-defun`, `cl-defmacro` and `cl-defsubst` are
; `defun`, `defmacro` and `defsubst` with a plain-list spelling; a method or generic is named by the first
; symbol after the head (`(cl-defmethod area :around ((s shape)) ...)` is `area`); a struct is named by its
; symbol or by the head of its options list; a class is eieio's `defclass`.
((list
  .
  (symbol) @_head
  .
  (symbol) @name
  .
  (_)
  .
  (_) @body) @definition.function
 (#eq? @_head "cl-defun"))

((list
  .
  (symbol) @_head
  .
  (symbol) @name
  .
  (_) .) @definition.function
 (#eq? @_head "cl-defun"))

((list
  .
  (symbol) @_head
  .
  (symbol) @name
  .
  (_)
  .
  (_) @body) @definition.macro
 (#eq? @_head "cl-defmacro"))

((list
  .
  (symbol) @_head
  .
  (symbol) @name
  .
  (_) .) @definition.macro
 (#eq? @_head "cl-defmacro"))

((list
  .
  (symbol) @_head
  .
  (symbol) @name
  .
  (_)
  .
  (_) @body) @definition.subst
 (#eq? @_head "cl-defsubst"))

((list
  .
  (symbol) @_head
  .
  (symbol) @name
  .
  (_) .) @definition.subst
 (#eq? @_head "cl-defsubst"))

((list
  .
  (symbol) @_head
  .
  (symbol) @name) @definition.method
 (#eq? @_head "cl-defmethod"))

((list
  .
  (symbol) @_head
  .
  (symbol) @name) @definition.generic
 (#eq? @_head "cl-defgeneric"))

((list
  .
  (symbol) @_head
  .
  (symbol) @name) @definition.struct
 (#eq? @_head "cl-defstruct"))

((list
  .
  (symbol) @_head
  .
  (list
    .
    (symbol) @name)) @definition.struct
 (#eq? @_head "cl-defstruct"))

((list
  .
  (symbol) @_head
  .
  (symbol) @name) @definition.class
 (#eq? @_head "defclass"))
