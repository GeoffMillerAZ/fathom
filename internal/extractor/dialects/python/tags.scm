; dialects/python/tags.scm — symbol + call queries for the wazero tree-sitter engine
; (internal/extractor.NewTSExtractor("python"), ADR-021 §4).
;
; Predicate-free: every pattern here relies on structural matching alone.
; (The engine evaluates #eq?/#match?/#any-of? and their negations host-side
; since LB-F2 — tsengine/predicates.go — but these patterns predate it and
; need none.) Adapted from
; tree-sitter-python's own queries/tags.scm (upstream, same repo as the
; grammar) with the doc-comment association predicates dropped, since fathom
; extracts symbols, not documentation.

(class_definition
  name: (identifier) @name) @definition.class

(function_definition
  name: (identifier) @name
  body: (_) @body) @definition.function

; Only a DIRECT top-level assignment counts as a module-level variable — the
; nesting in this pattern (module > expression_statement > assignment) is a
; structural constraint the query engine enforces, so assignments inside a
; function or class body never match this pattern at all. (Also matches an
; annotated module-level alias with a value, e.g. `Vector: TypeAlias =
; list[float]` — universal-ctags tags that shape as kind "variable" too, so
; no separate pattern is needed for it.)
(module
  (expression_statement
    (assignment
      left: (identifier) @name)) @definition.variable)

; A class-level attribute declared with a type annotation AND a default
; value (the dataclass/Pydantic idiom: `count: int = 0`) — universal-ctags
; tags this shape as kind "variable" too (verified directly), so it joins
; the same @definition.variable kind. Two related shapes are deliberately
; NOT matched here: a bare, unannotated class attribute (`attr = 1`, no
; `type:` field) — TestTagsDialectParity's frozen shapes.py fixture already
; contains exactly that shape uncaptured, and widening this pattern to admit
; it regresses that frozen legacy-parity output (LB-F1), the same
; not-a-worker's-call-to-make precedent javascript's tags.scm documents for
; its own withheld assignment shapes; and an annotation-only field with no
; value (`name: str`, Pydantic's own most common shape) — universal-ctags
; itself never tags a Python annotation without a value (verified directly:
; zero tags emitted), so admitting it would only cost sym_precision with no
; recall to show for it.
(class_definition
  body: (block
    (expression_statement
      (assignment
        left: (identifier) @name
        type: (type)
        right: (_))) @definition.variable))

; PEP 695 `type X = ...` (tree-sitter-python's type_alias_statement, in the
; grammar since it added 3.12 support; pinned v0.25.0 has it). universal-
; ctags 6.2.1 does not recognize this statement at all (verified directly:
; a real `type` statement gets zero tags), so it is deliberately left out of
; oracle_kinds — excluded_by_kind, never scored — while still landing in
; fathom's own symbol table, which is real value a bench metric doesn't
; capture. Mirrors typescript's "type" kind/canonical "type_alias" naming.
(type_alias_statement
  left: (type
    (identifier) @name)) @definition.type

; Same-file call collection: the expression being called, as source text,
; which dialect.json's calls.qualify (LB-F65) stores compacted — its receiver
; chain plus the called name ("self.make", "db.query().filter"), a subscript
; or a call's result called (`handlers[k](x)`, `f()()`) naming no callee and
; stored as none. Not a typed/resolved edge (ADR-021 §3 typed edges are a
; Go-only enrichment pass).
(call
  function: (_) @reference.call)

; A call outside every function still belongs to a definition (LB-F65, the
; attribution scip-python's enclosing ranges give): a decorator's calls to the
; definition it decorates (`@app.get("/")` is read_item's), a module-level
; assignment's to the variable it defines (`app = FastAPI()` is app's), and an
; annotated class attribute's to that attribute (`count: int = Field(0)`).
(decorated_definition
  (decorator) @calls.from
  definition: (_) @calls.to)

(module
  (expression_statement
    (assignment
      left: (identifier))) @calls.to @calls.from)

(class_definition
  body: (block
    (expression_statement
      (assignment
        left: (identifier)
        type: (type)
        right: (_))) @calls.to @calls.from))

; @body (LB-F3, design §3.3): a callable's Signature is its definition's text up to
; the start of this capture, whitespace collapsed; without it, the first line.
