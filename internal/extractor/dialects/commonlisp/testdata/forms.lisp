;;;; forms.lisp --- unusual spellings and text that only looks like a definition

(in-package :forms)

(defun forms/namespaced (a)
  "Slash-separated name."
  a)

(defun %forms-private (a) a)

(defun cl-user::forms-qualified (a)
  "A package-qualified name is kept whole."
  a)

(defun forms-doc-with-parens (a)
  "Mentions (defun not-a-definition (x) x) and \"quotes\" in prose."
  a)

(defun forms-body-mentions-def (a)
  (list 'defun 'defvar a "(defvar in-a-string)"))

(defvar *forms-multi-line*
  '(alpha
    beta
    gamma)
  "A value that spans several lines.")

(defun forms-keys (a &key (b 1) (c 2) &allow-other-keys)
  (list a b c))

(defun forms-aux (a &aux (b (* 2 a)))
  b)

(defmacro forms-macro (name &body body)
  "Define NAME around BODY."
  `(defun ,name () ,@body))

(defparameter *forms-list* (list 1 2))
(defconstant +forms-pi+ 3.14159d0)
(defvar *forms-a* 1) (defvar *forms-b* 2)

(defun forms-a () 1) (defun forms-b () 2)

(defun forms-char-literals ()
  (list #\( #\) #\Space #\a))

(defun forms-lambda-list (a &optional (b nil b-p) &rest rest)
  (list a b b-p rest))

(defun (setf forms-place) (value place)
  "Set PLACE to VALUE."
  (setf (car place) value))
