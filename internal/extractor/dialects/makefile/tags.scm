; source: tree-sitter-grammars/tree-sitter-make queries/highlights.scm @ 70613f3d812cbabbd7f38d104d60a409c4008b43 (MIT), read for node vocabulary only; no rule below is copied from it
; changes: written from scratch (the grammar ships no tags.scm; aider and nvim-treesitter carry no make tags/locals query), against `indexbench lang-probe` output over testdata/*.mk
;
; dialects/makefile/tags.scm: rule targets (MakeTarget, MakePatternRule), variables
; (MakeVariable), `define` blocks (MakeDefine) and each rule's literal prerequisites as
; same-file references (LB-Q-makefile, design §13).
;
; What a definition is (each kind mirrors universal-ctags' Make parser, the L1 oracle):
;  - a rule's target is a definition: one per target word of `a b: dep` (and `a b &: dep`),
;    spelled as written, so `$(BUILD)/main.o` is the symbol `$(BUILD)/main.o` (a static
;    extractor cannot expand a variable); `.c.o` suffix rules are targets too.
;  - a target containing `%` is a pattern rule (MakePatternRule), not a target `make` can name.
;  - the special built-in targets (.PHONY, .SUFFIXES, .PRECIOUS, .DEFAULT, ...: a dot and
;    capitals) declare attributes; they define nothing and are skipped.
;  - `NAME = / := / ::= / ?= / !=` define a variable; `+=` appends to one defined elsewhere
;    and is not a definition. `override`, `export` and `private` prefixes are transparent.
;    Two forms are lost to the grammar: an assignment with no space before its operator and a
;    `?` or `+` touching the name (`CC?=gcc`), and a tab-indented assignment inside a
;    conditional (`ifeq ..` / TAB `X=1`), which it reads as a recipe line of raw text.
;  - a target-specific assignment (`test: CFLAGS = -g`) names its targets, which are emitted
;    as targets (GNU make creates the target's database entry; ctags does the same), and
;    defines CFLAGS in the target's scope.
;  - `define NAME ... endef` is a multi-line variable (a canned recipe or function body); its
;    body is raw text and is not parsed.
;
; References (L2): every literal word (no `$`) in a rule's normal or order-only prerequisites
; is a call from the rule's target, spelled as written, the ADR-021 §3 naive same-file
; collection: it resolves when a target of that name exists in the file, and stays unresolved
; otherwise. Not captured:
;  - a prerequisite spelled with a variable or a function ($(HDRS), lib/$(NAME).h,
;    $(wildcard ..), $(addprefix ..)): a value, not a name (the reading measures the spelled
;    literal edges make evaluated; calibration/oracles/make);
;  - the prerequisites of a static pattern rule (`$(OBJS): %.o: %.c`): patterns, not names;
;  - `$(MAKE) target` in a recipe and `include` (L3).
;
; tree-sitter-make does not know the `private`/`override` modifiers of a target-specific
; assignment (`$(OBJS): private CFLAGS += -Wall`): it reads them as a rule whose prerequisites
; are `private`, `CFLAGS`, ... (or as a variable named `private`). The targets are still
; emitted; the words after the modifier are not prerequisites and are not references, and
; `private` is not a variable.
;
; Three limits of the grammar/engine that shape the output, disclosed in the reading:
;  - tree-sitter-make's nodes absorb the blank lines that follow them, so a definition's
;    EndLine can run past its last line to the line before the next construct (ctags ends a
;    rule at the line before the next definition, comments included);
;  - a conditional block that directly follows a rule (`ifeq ... endif`) is parsed inside
;    that rule's recipe, so the rule's EndLine runs to the block's `endif`;
;  - the engine attaches a call site to the smallest callable whose range contains it, and the
;    targets of `a b: dep` share one range, so `dep` is a call of `a` (the first) only.

; VAR = / := / ::= / ?=  (also the value of `override VAR = ..`, `export VAR := ..`). The grammar
; reads a `?` or `+` that touches the name into the name (`VAR?=1` is the name `VAR?`, `VAR+=1`
; is `VAR+`); the second is an append and neither has a usable name, so they are skipped.
(variable_assignment
  name: (_) @name
  ["=" ":=" "::=" "?="]
  (#not-any-of? @name "private" "override" "export" "unexport")
  (#not-match? @name "[?+]$")) @definition.variable

; VAR != shell command
(shell_assignment
  name: (_) @name) @definition.variable

; define NAME ... endef
(define_directive
  name: (_) @name) @definition.define

; test: CFLAGS = -g   (the targets a target-specific assignment names)
(variable_assignment
  target_or_pattern: (list (_) @name)
  (#not-match? @name "%")) @definition.target

; %.o: CFLAGS = -g
(variable_assignment
  target_or_pattern: (list (_) @name)
  (#match? @name "%")) @definition.pattern

; %.o: %.c   gen-%.h: gen.sh
(rule
  (targets (_) @name)
  (recipe)? @body
  (#match? @name "%")) @definition.pattern

; all: prog   a b: dep   $(BUILD)/main.o: main.c   lib.a:: a.o
(rule
  (targets (_) @name)
  (recipe)? @body
  (#not-match? @name "%")
  (#not-match? @name "^\\.[A-Z_]+$")) @definition.target

; Prerequisites (normal and order-only): a literal word is a same-file reference by name. A
; leading `private`/`override`/`export` is a target-specific assignment's modifier (see the
; grammar note above), and the words after it are not prerequisites. `endif` and `else` are
; conditional directives the grammar reads as a prerequisite when a backslash-continued list is
; followed by a blank line and one of them.
(prerequisites
  . (word) @reference.call @name
  (#not-any-of? @name "private" "override" "export" "unexport" "endif" "else"))

(prerequisites
  . (_) @_first
  (word) @reference.call @name
  (#not-any-of? @_first "private" "override" "export" "unexport")
  (#not-any-of? @name "endif" "else"))
