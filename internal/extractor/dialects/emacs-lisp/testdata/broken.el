;;; broken.el --- malformed forms; the definitions around them are still found

(defun broken-before (a)
  "Complete."
  (list a))

(defun broken-stray (a)
  a))

(defvar broken-after-stray 1
  "Follows a stray close paren.")

(defun broken-holes (a)
  (let ((x a)
        (y))
    (list x y)))

(defcustom broken-custom 2 "Fine." :type 'integer)

(defun broken-truncated (a
