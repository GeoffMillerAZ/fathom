;;;; nested.lisp --- definitions below the top level and behind reader conditionals

(in-package :nested)

(eval-when (:compile-toplevel :load-toplevel :execute)
  (defmacro nested-when-compiling (&body body)
    "Expand to BODY."
    `(progn ,@body))

  (defun nested-helper (x)
    "Return X."
    x))

(progn
  (defvar *nested-progn* 1)
  (defun nested-in-progn ()
    *nested-progn*))

(let ((counter 0))
  (defun nested-next ()
    "Return the next number."
    (incf counter)))

#+sbcl
(defun nested-sbcl-only ()
  "Only on SBCL."
  :sbcl)

#-sbcl (defun nested-not-sbcl () :other)

#+(or ccl ecl)
(progn
  (defun nested-ccl-or-ecl ()
    :either))

(defun nested-outer (x)
  "Define a function while running."
  (labels ((local-helper (y)
             (+ x y)))
    (defun nested-inner (z)
      (local-helper z))
    (local-helper 1)))

#|
(defun in-block-comment ()
  "A block comment holds no definition."
  nil)
|#

;; (defun in-line-comment () nil)

(defun nested-after-comments ()
  "The definitions around the comments are found."
  t)
