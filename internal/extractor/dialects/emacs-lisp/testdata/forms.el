;;; forms.el --- unusual spellings and text that only looks like a definition  -*- lexical-binding: t; -*-

(defun forms/namespaced (a)
  "Slash-separated prefix."
  a)

(defun forms--private (a) a)

(defun forms-doc-with-parens (a)
  "Mentions (defun not-a-definition (x) x) and \"quotes\" in prose."
  a)

(defun forms-body-mentions-def (a)
  (list 'defun 'defvar a "(defvar in-a-string)"))

(defvar forms-multi-line
  '(alpha
    beta
    gamma)
  "A value that spans several lines.")

(defun forms-keys (&rest args &key _a _b)
  args)

(defmacro forms-macro-doc (name &rest body)
  "Define NAME around BODY."
  (declare (indent defun) (debug (symbolp body)))
  `(defun ,name () ,@body))

(defvar forms-unbound)
(defconst forms-pi 3.14159)
(defalias 'forms-alias-quoted 'forms/namespaced)
(defvar forms-empty-doc nil "")

(defun forms-a () 1) (defun forms-b () 2)

(defun forms-arglist (a &optional b &rest c)
  (list a b c))
