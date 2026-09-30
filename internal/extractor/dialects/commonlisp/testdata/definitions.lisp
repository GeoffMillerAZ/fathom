;;;; definitions.lisp --- every definition kind

(defpackage #:demo
  (:use #:cl)
  (:export #:greet #:point #:area))

(in-package #:demo)

(defvar *name* "world"
  "Whom `greet' greets.")

(defvar *declared*)

(defparameter *count* 3
  "How many times.")

(defconstant +version+ "1.0"
  "Library version.")

(deftype small-integer ()
  "An integer below ten."
  '(integer 0 9))

(defstruct point
  "A point in the plane."
  (x 0)
  (y 0))

(defstruct (point3 (:include point) (:constructor make-point3 (x y z)))
  (z 0))

(defclass shape ()
  ((name :initarg :name :accessor shape-name))
  (:documentation "Base class of shapes."))

(defclass circle (shape)
  ((radius :initarg :radius :accessor circle-radius)))

(defun greet (&optional (name *name*))
  "Greet NAME."
  (format t "Hello, ~a~%" name))

(defun total (numbers
              &key (initial 0)
                   (key #'identity))
  "Sum NUMBERS, starting from INITIAL."
  (reduce #'+ numbers :key key :initial-value initial))

(defun no-arguments ()
  t)

(defun no-body (x))

(defmacro with-name ((var) &body body)
  "Bind VAR to the current name around BODY."
  `(let ((,var *name*))
     ,@body))

(defgeneric area (shape)
  (:documentation "Return the area of SHAPE."))

(defgeneric perimeter (shape))

(defmethod area ((shape circle))
  (* pi (expt (circle-radius shape) 2)))

(defmethod perimeter ((shape circle))
  (* 2 pi (circle-radius shape)))
