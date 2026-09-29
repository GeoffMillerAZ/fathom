;;;; broken.lisp --- a malformed form; the definitions around it are still found

(defun broken-before (a)
  "Complete."
  (list a))

(defun broken-unclosed (a (list a))

(defvar *broken-after* 1
  "Follows the unclosed form.")

(defclass broken-class ()
  ((slot :initarg :slot)))

(defmethod broken-method ((x broken-class))
  x)

(defun broken-truncated (a
