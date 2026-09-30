# commonlisp — provenance

- Grammar: `tree-sitter-grammars/tree-sitter-commonlisp v0.4.1` (`9db594efb43574c5fe4a1f880568b1f0d0ee6057`, MIT, licence checked at the pin:
  the top-level `LICENSE.md` only, `queries/` and `src/` carry none), ABI 14, no external scanner, built into
  `internal/extractor/tsengine/grammars/commonlisp.wasm.gz` (839,432 bytes raw, 158,920 gzipped, 374 peak module pages on lem); pin and
  digests in `internal/extractor/tsengine/grammars/PROVENANCE.md`. `src/parser.c` (5.9 MB) ships at the tag, so the lock says
  `"generate": false`.
- Parse coverage (every `.lisp`, `.lsp` and `.asd` file of both calibration subjects, 232): 4 have an ERROR node (1.7%, far under design
  section 14.2's 5% line): lem's `src/keymap.lisp` and `src/ext/prompt-window.lisp` and clack's `src/core/middleware/logger/file.lisp` and
  `src/util/doc.lisp`. The grammar reads a `loop :with` list of four or more variables joined by `:and` (`keymap.lisp`'s `parse-keyspec`) and
  some `format` control strings (`~/` in `file.lisp` and `prompt-window.lisp`, `~:*` in `doc.lisp`) as errors, and the recovery drops the
  definitions that follow. A symbol spelled with escaped parentheses (`insert-\(\)-or-wrap` in lem's `language-mode.lisp`) unbalances the rest
  of its file without an ERROR node at all. Together they are the recall the dialect does not reach: 90 of the 2,340 definitions ctags finds in
  lem (`prompt-window.lisp` 42, `language-mode.lisp` 34, `keymap.lisp` 12, `defcommand.lisp` 2).
- `tags.scm`: starts from the grammar's own `queries/tags.scm` (MIT; header on the file), which tags every `defun_header` as a function, a
  `defclass` as a class and every list with a symbol head as a call. This one keeps the grammar's `defun` node (the grammar gives `defun`,
  `defmacro`, `defgeneric` and `defmethod` one node, told apart by its `defun_keyword` with `#eq?`) and takes the enclosing list as the definition
  so an EndLine reaches the closing paren; `defvar`, `defparameter`, `defconstant`, `deftype`, `defstruct` and `defclass` are plain lists told apart
  by the head symbol (the shape ruby's `attr_accessor` uses).
  - A callable's `@body` is the first form after the header (a docstring, else the first body form, else a `defgeneric` option) so a Signature stops
    at the lambda list; a definition with nothing after the header is a second pattern anchored to the last child, and its Signature is its first
    line. A method's qualifier (`:around`) is part of the Signature.
  - A `(setf name)` function or method is named `name`, as universal-ctags names it; its Signature says `(defun (setf name) ...`.
  - A definition below the top level (`eval-when`, `progn`, `let`, behind `#+sbcl`, a `labels` body) is extracted; there are no containers, so a
    name is never qualified. universal-ctags reads only a `(` in column 1 and writes a `#+sbcl (defun ...)` on one line off, so it misses 59
    definitions the dialect finds, which is why `sym_precision` is .9771 and not 1: annotated forms (`@export (defconstant ...)` in clack's
    logger), definitions nested in a `let` or behind a reader conditional in lem, and the definitions of the `.asd` files ctags does not read.
  - `(defstruct (name options...) slot...)` is named by the head of its options list, as well as `(defstruct name slot...)`.
  - A package-qualified name (`cl-user::name`) is kept whole, as universal-ctags reports it.
  - Not extracted: `define-condition`, `defsetf`, `define-compiler-macro`, `define-symbol-macro` and `define-modify-macro` (ctags reports
    them as `unknown`, a kind the manifest leaves unmapped: 892 definitions in the two subjects, mostly Lem's `define-command`), `defpackage` and
    `in-package`, and a `(:method ...)` option inside a `defgeneric`.
  - No call is captured: `(head arg ...)` is a call, a `let` binding pair, a `cond` clause and a data list in the same syntax, and upstream's
    `@ignore` exclusions are not a capture this engine reads. Calls belong to L2, which needs an oracle for them (universal-ctags has none).
- `.asd` (an ASDF system definition) is claimed with `.lisp` and `.lsp` and defines nothing the dialect extracts (`system.asd` keeps that
  in a golden); `.cl` and `.l` are left alone (OpenCL C and Lex share them). universal-ctags does not read `.asd` either.
- `name_match: identity`: Common Lisp symbols carry `/`, `:` and `.`, which the harness's last-segment rule would strip on both sides.
- `testdata/`: six fixtures written in this repository; goldens written by hand from each fixture before any query existed
  (`TestDialectGoldens/commonlisp` red), then the queries were written against `indexbench lang-probe`. The `(setf name)` cases were
  added to `clos.lisp` and `forms.lisp` after the first measurement, golden first. `broken.lisp` holds an unclosed form: the grammar
  recovers from it, while a stray `)` swallows every form after it, so the fixture cannot use that error.

## Calibration subjects: woo swapped for clack

The queue names lem-project/lem and fukamachi/woo. woo publishes no release tag (`git ls-remote --tags` is empty) and the corpus schema pins
a release tag (design section 13.3), so `calibration/corpus.d/commonlisp.yaml` measures lem v2.3.0 (`subpath: src`) and its neighbour
fukamachi/clack 12.06.0 (MIT, same author and idioms) instead.

## Measured (docs/bench/readings/lang/commonlisp.json)

Against universal-ctags 6.2.1 (tier B) on lem v2.3.0 (`src/`) and clack 12.06.0, 232 files, 1.2 MB: `level_met` L1, grade good. `sym_recall`
.9654, `sym_precision` .9771, `kind_agreement` 1, `signature_coverage` 1 over 2,082 callables, 1.54 MB/CPU-s, 0 faults, 374 peak module pages.
Good and not excellent because ctags writes no end line for Lisp, so `range_agreement` is not measured (design section 1.2); the tracker
shows L1 good and no level is claimed above it.
