;;;; clos.lisp --- generic functions, methods and their qualifiers

(in-package :clos-demo)

(defgeneric describe-it (thing stream)
  (:documentation "Print THING on STREAM.")
  (:method ((thing t) stream)
    (format stream "~a" thing)))

(defmethod describe-it ((thing integer) stream)
  (format stream "integer ~d" thing))

(defmethod describe-it :before ((thing number) stream)
  (write-string "number: " stream))

(defmethod describe-it :after ((thing number) stream)
  (terpri stream))

(defmethod describe-it :around ((thing number) stream)
  (call-next-method))

(defmethod describe-it ((thing (eql :none)) stream)
  (write-string "none" stream))

(defmethod describe-it ((thing string) (stream stream) &key (quote t) &allow-other-keys)
  (when quote
    (write-char #\" stream))
  (write-string thing stream))

(defmethod initialize-instance :after ((object base-thing) &key)
  (setf (slot-value object 'ready) t))

(defmethod print-object ((object base-thing) stream)
  (print-unreadable-object (object stream :type t)
    (format stream "~a" (slot-value object 'name))))

(defgeneric size (container))

(defmethod size ((container list))
  (length container))

(defmethod size ((container hash-table))
  (hash-table-count container))

(defgeneric (setf size) (value container))

(defmethod (setf size) (value (container list))
  (setf (car container) value))
