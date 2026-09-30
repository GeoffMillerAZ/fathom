;;;; system.asd --- an ASDF system definition holds no definition the dialect extracts

(asdf:defsystem "demo"
  :description "A demo system."
  :version "1.0"
  :depends-on ("alexandria" "cl-ppcre")
  :serial t
  :components ((:file "package")
               (:file "definitions")
               (:module "src"
                :components ((:file "clos")
                             (:file "nested"))))
  :in-order-to ((test-op (test-op "demo/tests"))))

(asdf:defsystem "demo/tests"
  :depends-on ("demo" "fiveam")
  :components ((:file "tests"))
  :perform (test-op (o c) (uiop:symbol-call :fiveam :run! :demo)))
