;;; cl.el --- cl-lib and eieio definers  -*- lexical-binding: t; -*-

(require 'cl-lib)
(require 'eieio)

(cl-defstruct cl-point
  "A point."
  x y)

(cl-defstruct (cl-point3 (:include cl-point) (:constructor cl-point3-new))
  z)

(defclass cl-shape ()
  ((name :initarg :name :initform "shape"))
  "Base class of shapes.")

(defclass cl-circle (cl-shape)
  ((radius :initarg :radius))
  "A circle.")

(cl-defgeneric cl-area (shape)
  "Return the area of SHAPE.")

(cl-defmethod cl-area ((shape cl-circle))
  "Area of a circle."
  (* float-pi (expt (oref shape radius) 2)))

(cl-defmethod cl-area :around ((shape cl-shape))
  (or (cl-call-next-method) 0))

(cl-defun cl-make-point (x y &key (z 0))
  "Build a point from X, Y and Z."
  (list x y z))

(cl-defun cl-origin ())

(cl-defmacro cl-with-point ((x y) point &rest body)
  "Bind X and Y from POINT around BODY."
  (declare (indent 2))
  `(let ((,x (car ,point)) (,y (cadr ,point))) ,@body))

(cl-defsubst cl-twice (x)
  "Twice X."
  (* 2 x))
