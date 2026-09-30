;;; definitions.el --- every definition kind  -*- lexical-binding: t; -*-

;; A library header comment; nothing in it defines a symbol.

(defgroup demo nil
  "Customization group of the demo library."
  :group 'tools
  :prefix "demo-")

(defcustom demo-name "world"
  "Whom `demo-greet' greets."
  :type 'string
  :group 'demo)

(defcustom demo-count 3 "How many times." :type 'integer)

(defvar demo-cache nil
  "Memoized results.")

(defvar demo-declared)

(defvar-local demo-buffer-state 'idle
  "Per-buffer state.")

(defconst demo-version "1.0"
  "Library version.")

(defface demo-highlight
  '((t :weight bold :foreground "red"))
  "Face for emphasised text."
  :group 'demo)

(defun demo-greet (&optional name)
  "Greet NAME, or `demo-name' when NAME is nil."
  (interactive)
  (message "Hello, %s" (or name demo-name)))

(defun demo-sum (numbers
                 &optional
                 initial)
  "Sum NUMBERS, starting from INITIAL."
  (apply #'+ (or initial 0) numbers))

(defun demo-nil-args nil
  "No parameters, spelled nil."
  t)

(defun demo-empty-args ()
  t)

(defun demo-no-body (x))

(defmacro demo-with-cache (&rest body)
  "Run BODY with the cache bound."
  (declare (indent 0))
  `(let ((demo-cache demo-cache)) ,@body))

(defsubst demo-double (x)
  "Twice X."
  (* 2 x))

(define-inline demo-inc (x)
  "One more than X."
  (inline-quote (1+ ,x)))

(defalias 'demo-say #'demo-greet
  "Another name for `demo-greet'.")

(define-obsolete-function-alias 'demo-hello #'demo-greet "1.0")

(define-derived-mode demo-mode text-mode "Demo"
  "Major mode for demo files."
  (setq-local demo-buffer-state 'active))

(define-minor-mode demo-minor-mode
  "Toggle demo mode."
  :lighter " Demo"
  :global nil)

(define-globalized-minor-mode global-demo-minor-mode demo-minor-mode
  demo-minor-mode-on)

(provide 'definitions)
;;; definitions.el ends here
