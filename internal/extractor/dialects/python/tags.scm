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
; the same @definition.variable kind. It keeps the bare name it has always had
; (TestPythonDialectKeepsLegacySymbols); the member patterns below name every
; other class attribute Owner.name.
(class_definition
  body: (block
    (expression_statement
      (assignment
        left: (identifier) @name
        type: (type)
        right: (_))) @definition.variable))

; Attributes and fields (LB-F242, wave lx): every name scip-python 0.6.6
; defines on a class or module that the patterns above miss, named Owner.name
; and kept once per (Owner, name), the first in source order — the member pass
; (internal/extractor/tsextract_members.go) reads the marks. @member: a class
; attribute when the nearest enclosing definition is a class (also inside an
; `if` in the class body), a module-level name when there is none (an
; assignment inside a module-level `if`/`for`/`with`/`try`), dropped inside a
; function (a local). @member.strict: the same, dropped at module scope.
; @member.self: an attribute a method sets on `self`, named after the class
; around the method. A shape universal-ctags 6.2.1 tags as variable stays a
; variable (verified directly: an unannotated class attribute, every target
; of an unparenthesized tuple assignment, the first target of a chained one,
; an assignment nested in a block); the shapes it never tags are fields
; (PythonField, outside oracle_kinds): an annotation without a value, a
; chained assignment's later targets, an attribute set on self.
(expression_statement
  (assignment
    left: (identifier) @name
    right: (_))) @definition.variable @member

(expression_statement
  (assignment
    left: (pattern_list
      (identifier) @name))) @definition.variable @member

(expression_statement
  (assignment
    left: (identifier) @name
    type: (type)
    !right)) @definition.field @member.strict

(assignment
  right: (assignment
    left: (identifier) @name) @definition.field @member.strict)

(expression_statement
  (assignment
    left: (attribute
      object: (identifier) @_self
      attribute: (identifier) @name))
  (#eq? @_self "self")) @definition.field @member.self

(expression_statement
  (assignment
    left: (pattern_list
      (attribute
        object: (identifier) @_self
        attribute: (identifier) @name)))
  (#eq? @_self "self")) @definition.field @member.self

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

; Every assignment statement attaches its calls to the definition at its node
; (LB-F242): a module-level variable (as before), a class attribute, the first
; `self.x = ...` of an attribute. A statement no kept definition sits at (a
; local, a later assignment to an attribute already defined) attaches nothing,
; and its calls stay with the function around it.
(expression_statement
  (assignment)) @calls.to @calls.from

; @body (LB-F3, design §3.3): a callable's Signature is its definition's text up to
; the start of this capture, whitespace collapsed; without it, the first line.
