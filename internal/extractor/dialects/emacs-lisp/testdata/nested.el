;;; nested.el --- definitions below the top level  -*- lexical-binding: t; -*-

(eval-when-compile
  (require 'cl-lib))

(eval-and-compile
  (defmacro nested-when-compiling (&rest body)
    "Expand to BODY."
    `(progn ,@body))

  (defun nested-helper (x)
    "Return X."
    x))

(with-eval-after-load 'nested-dep
  (defvar nested-dep-map (make-sparse-keymap)
    "Keymap added once the dependency loads.")

  (defun nested-dep-command ()
    "Command defined after load."
    (interactive)
    (message "loaded")))

(if (fboundp 'nested-compat)
    nil
  (defun nested-compat (x)
    "Fallback for old Emacs."
    x))

(let ((counter 0))
  (defun nested-next ()
    "Return the next number."
    (setq counter (1+ counter))))

(defun nested-outer (x)
  "Define a function while running."
  (defun nested-inner (y)
    (+ x y))
  (nested-inner 1))

(when (boundp 'nested-flag)
  (defcustom nested-option t
    "Only defined when the flag exists."
    :type 'boolean))

(defun nested-last () nil)
