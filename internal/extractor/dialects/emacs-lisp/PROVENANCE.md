# emacs-lisp — provenance

- Grammar: `Wilfred/tree-sitter-elisp 1.7.2` (`c0417c751a3dcc8724d1f2f6d1437a99dee1e090`, MIT, licence checked at the pin: the top-level
  `LICENSE` only, `queries/` and `src/` carry none), ABI 15, no external scanner, built into
  `internal/extractor/tsengine/grammars/elisp.wasm.gz` (176,723 bytes raw, 62,982 gzipped, 377 peak module pages on lsp-mode); pin and digests
  in `internal/extractor/tsengine/grammars/PROVENANCE.md`. The repository commits no `src/parser.c` (it is in `.gitignore`), so the lock says
  `"generate": true` and `build.sh` runs the pinned tree-sitter CLI 0.26.11 over `grammar.js`.
- Parse coverage (every `.el` file of both calibration subjects, 202): 0 have an ERROR or MISSING node.
- `tags.scm`: starts from the grammar's own `queries/tags.scm` (MIT; header on the file), which tags `defun`/`defsubst` and `defmacro` as
  functions. This one keeps those two grammar nodes and gives each its own kind (as universal-ctags does), and tells the definers the grammar
  has no node for apart by the head symbol (`#eq?` / `#any-of?`, the same shape as ruby's `attr_accessor`): `defvar` and `defconst` are special
  forms whose keyword is an anonymous token, matched by the string itself; `defvar-local`, `defcustom`, `defface`, `defgroup`, `define-inline`,
  `defalias`, `define-obsolete-function-alias`, `define-derived-mode`, `define-minor-mode`, `define-globalized-minor-mode` and the cl-lib and
  eieio definers are plain lists.
  - A callable's `@body` is the first form after its parameter list (a docstring, else the first body form) so a Signature stops at the
    parameters; a definition with nothing after the parameters is a second pattern anchored to the last child, and its Signature is its first line.
    Modes, `cl-defmethod` and `cl-defgeneric` have no `@body`: their Signature is the first line.
  - A bare `(defvar x)` (a forward declaration) is a variable definition, as universal-ctags reports it.
  - A definition below the top level (`(with-eval-after-load ...)`, `(eval-and-compile ...)`, `(if ...)`, `(let ...)`, another `defun`) is
    extracted; there are no containers, so a name is never qualified. universal-ctags reads only a `(` in column 1: on the calibration subjects
    it misses 43 such definitions that are really there (lsp-protocol.el defines lsp-get and its siblings in both arms of an `if`), which is
    most of the 76 dialect-only definitions and why `sym_precision` is .9861 and not 1; the rest are the 33 `cl-defun`, `cl-defmacro` and
    `cl-defsubst` forms ctags has no kind for.
  - Kinds outside ctags' EmacsLisp vocabulary (`cl-defmethod` and `cl-defgeneric` are canonical `method`, `cl-defstruct` `struct`, eieio's
    `defclass` `class`) are extracted but sit outside the kind intersection, so they change no metric: 130 of them on the subjects, listed as
    `fathom:EmacsLisp...` under `excluded_by_kind`.
  - Not extracted: `transient-define-prefix`, `-suffix` and `-argument`, `defvar-keymap`, `define-error`, `defvaralias`,
    `define-obsolete-variable-alias`, `deftheme` and `ert-deftest` (ctags reports the last five as `unknown`, `error`, `varalias` or `theme`,
    kinds the manifest leaves unmapped, and has none for transient; the tags engine could name them, but nothing would measure them).
  - No call is captured: `(head arg ...)` is a call, a `let` binding pair, a `dolist` spec, a `cond` clause and a data list in the same
    syntax, and the tags engine has no way to know which. Calls belong to L2, which needs an oracle for them (universal-ctags has none).
  - A definer written inside quoted data (`'((defun x ...))`) or a backquoted template with a literal name is still tagged: the grammar is not
    quote-aware. A template that names its definition with `,name` defines nothing.
- `name_match: identity`: Emacs Lisp names carry `/`, `:` and `.` (`magit/foo`, `foo::bar`), which the harness's last-segment rule would
  strip on both sides.
- `testdata/`: six fixtures written in this repository (none copied from a calibration subject, which is GPL-3.0-or-later); goldens written by
  hand from each fixture before any query existed (`TestDialectGoldens/emacs-lisp` red), then the queries were written against
  `indexbench lang-probe`. `cl.el` and its kinds were added after the first measurement, golden first.

## Measured (docs/bench/readings/lang/emacs-lisp.json)

Against universal-ctags 6.2.1 (tier B) on magit v4.7.1 and lsp-mode 10.0.0 (`clients/` and the root), 202 files, 3.2 MB: `level_met` L1,
grade good. `sym_recall` 1, `sym_precision` .9861, `kind_agreement` 1, `signature_coverage` 1 over 2,606 callables, 4.83 MB/CPU-s, 0 faults,
377 peak module pages. Good and not excellent because ctags writes no end line for EmacsLisp, so `range_agreement` is not measured (design
section 1.2); the tracker shows L1 good and no level is claimed above it.
