; source: tree-sitter/tree-sitter-c-sharp queries/tags.scm @ cac6d5fb595f5811a076336682d5d595ac1c9e85 (MIT)
; changes: upstream's own tags.scm captures only class/interface/method/
; namespace DEFINITIONS plus several @reference.class/@reference.send type
; references fathom does not consume (design §3.4: type refs are counted,
; not stored) — this file keeps none of it verbatim (every pattern below was
; rewritten against this repository's own field/kind probe of the grammar,
; docs/design/language-breadth.md §13 step 12) but follows the same node
; vocabulary. Kept from the incumbent CGO port (csharpextract.go, LB-Q-csharp,
; §11 step 3 port parity): CSharpNamespace/CSharpClass/CSharpInterface/
; CSharpStruct/CSharpEnum/CSharpMethod source kinds unchanged, record_declaration
; mapped to CSharpClass (the incumbent's own choice — records ARE classes to
; fathom, reference or value semantics undistinguished), constructor_declaration
; mapped to CSharpMethod, and no recursion into enum bodies (member values are
; not symbols). Added beyond the incumbent (a port may add, never drop, §11
; step 3): CSharpProperty (queue note "properties" — the incumbent never
; extracted properties at all), file-scoped namespace handled identically to
; block-scoped (queue note "namespaces (block and file-scoped)" — the
; incumbent's own walkCSharp already special-cased both node kinds but this
; port additionally uses the block form's own node range as a package scope,
; design §3.5's "C#'s shape" — see tspackage_test.go), and @scope.package/
; @scope so SymbolNode.Package carries the namespace instead of pkgFromPath's
; directory guess. "partial" is a plain modifier token in this grammar (not a
; distinct node), so two partial class_declaration bodies for the same type
; (even across files) simply produce two CSharpClass definitions, matching
; every other partial-type precedent in this codebase (no merge step exists
; anywhere in the tags engine).

; Namespaces: a block namespace's own node range is also the package scope
; for everything declared inside it (nested block namespaces join with "."
; via tsPackageFromCapture's outer-first ordering); a file-scoped namespace
; has no bounded body node in this grammar (its siblings in compilation_unit
; are NOT its children — verified: lang-probe on a file-scoped fixture shows
; class_declaration as compilation_unit's own child, not nested under
; file_scoped_namespace_declaration), so it carries no @scope range and reads
; as tsPackageFromCapture's file-wide default instead (Kotlin/Java shape).
(namespace_declaration
  name: (_) @name @scope.package) @definition.namespace @scope

(file_scoped_namespace_declaration
  name: (_) @name @scope.package) @definition.namespace

; Types. class_declaration also matches "partial class" (partial is a
; modifier token) and is the target of record_declaration's own mapping
; below (never a class_declaration node itself — record_declaration is a
; distinct grammar rule for "record"/"record class"/"record struct", verified:
; lang-probe shows no "class"/"struct" keyword distinguishing them as a named
; field, so this port does not distinguish them either, matching the
; incumbent's TestCSharpExtractor_RecordDeclaration expectation).
(class_declaration
  name: (identifier) @name
  body: (_)? @body) @definition.class

(record_declaration
  name: (identifier) @name
  body: (_)? @body) @definition.class

(interface_declaration
  name: (identifier) @name
  body: (_)? @body) @definition.interface

(struct_declaration
  name: (identifier) @name
  body: (_)? @body) @definition.struct

; No @body: incumbent never recurses into enum bodies (enum_member_declaration
; values are not symbols fathom extracts), so an enum has no Signature either
; way (it is not a callable kind) — the field is simply not captured.
(enum_declaration
  name: (identifier) @name) @definition.enum

; Methods and constructors. body: (_)? covers both a block body and an
; expression-bodied "=> expr;" member (arrow_expression_clause) generically;
; an interface/abstract member has neither and the pattern still matches
; (verified: lang-probe on IWidget.Render and Base.DoIt — no body field at
; all, and both still produced a @definition.method match).
(method_declaration
  name: (identifier) @name
  body: (_)? @body) @definition.method

(constructor_declaration
  name: (identifier) @name
  body: (_)? @body) @definition.method

; Conversion operators ("public static implicit/explicit operator T(...)")
; have no name field in this grammar at all (verified: lang-probe on
; JToken.cs's own conversion operators — modifier/type/parameters/body only)
; — the target type stands in for a name (real value beyond a plain-`+`/
; `==` operator_declaration, which additionally has no type-adjacent
; capturable node either and is not attempted here; both are a disclosed,
; bounded L1 gap, PROVENANCE.md). universal-ctags names these "operator
; bool"/"operator JToken" (its own synthetic "operator "-prefixed form, not
; reproducible from a structural capture alone — the query engine captures
; node text, never a synthesized string), so this does not name-match ctags
; even though the symbol itself is real and useful to fathom's own callers.
(conversion_operator_declaration
  type: (_) @name
  body: (_)? @body) @definition.method

; Properties (new beyond the incumbent). Not a callable kind (no Signature),
; so neither accessors: (get;/set;) nor an expression-bodied value: needs
; capturing — the engine only emits Signature when d.callable (tsextract.go).
(property_declaration
  name: (identifier) @name) @definition.property

; Naive same-file call collection: the callee is the invocation's whole
; "function" field text, matching the incumbent's own
; treesitter.CollectCalls(n, src, map[string]string{"invocation_expression":
; "function"}) exactly (a bare "Helper" from "Helper()", the full
; "Console.WriteLine" from "Console.WriteLine(...)" — never split), and the
; SAME convention every other tags dialect in this tree uses for its own
; call capture (c/tags.scm, dialects/kotlin/tags.scm, dialects/python/
; tags.scm, dialects/ruby/tags.scm — none of them puts a @name inside
; @reference.call; that inner-@name convention is reserved for a reference
; whose enclosing node is NOT itself the callee text, e.g. SQL's FK
; `REFERENCES table(...)`, design §3.4's fallback path). Keeping the
; receiver segment also lets a whole-module import's dotted-prefix match
; (namespace-dir, internal/fusion/resolvestrategy.go) actually see it.
(invocation_expression
  function: (_) @reference.call)
