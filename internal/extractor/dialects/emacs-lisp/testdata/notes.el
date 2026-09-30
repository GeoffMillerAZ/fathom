;;; notes.el --- a small package as it is written in the wild  -*- lexical-binding: t; -*-

;; Copyright (C) 2026 Example Author
;; Author: Example Author <author@example.invalid>
;; Package-Requires: ((emacs "27.1"))

;;; Commentary:

;; A tiny mode with a keymap, a hook and a few commands.

;;; Code:

(require 'subr-x)

(defgroup notes nil
  "Take notes."
  :group 'applications)

(defcustom notes-directory (expand-file-name "~/notes/")
  "Directory holding the notes."
  :type 'directory
  :group 'notes)

(defcustom notes-hook nil
  "Hook run after a note is saved."
  :type 'hook
  :group 'notes)

(defface notes-title
  '((t :inherit font-lock-function-name-face :height 1.2))
  "Face for note titles."
  :group 'notes)

(defvar notes-mode-map
  (let ((map (make-sparse-keymap)))
    (define-key map (kbd "C-c C-c") #'notes-save)
    (define-key map (kbd "C-c C-n") #'notes-new)
    map)
  "Keymap for `notes-mode'.")

(defvar-local notes--dirty nil
  "Non-nil when the buffer has unsaved changes.")

(defconst notes--extension ".txt"
  "File extension of a note.")

(defun notes--file (title)
  "Return the file that stores the note called TITLE."
  (expand-file-name (concat title notes--extension) notes-directory))

(defun notes-new (title)
  "Create a note called TITLE."
  (interactive "sTitle: ")
  (find-file (notes--file title))
  (notes-mode))

(defun notes-save ()
  "Save the current note."
  (interactive)
  (save-buffer)
  (setq notes--dirty nil)
  (run-hooks 'notes-hook))

(defmacro notes-with-dir (&rest body)
  "Run BODY with `default-directory' set to `notes-directory'."
  (declare (indent 0))
  `(let ((default-directory notes-directory))
     ,@body))

(define-derived-mode notes-mode text-mode "Notes"
  "Major mode for notes."
  (setq-local notes--dirty nil)
  (add-hook 'after-change-functions #'notes--mark-dirty nil t))

(defun notes--mark-dirty (&rest _)
  "Remember that the buffer changed."
  (setq notes--dirty t))

(provide 'notes)
;;; notes.el ends here
