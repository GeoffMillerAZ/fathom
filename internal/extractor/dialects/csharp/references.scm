; dialects/csharp/references.scm — the file resolver's references (LB-F74;
; vocabulary in internal/extractor/tsrefs.go), compiled apart from tags.scm
; and imports.scm (TSExtractor.ExtractFileSpecs).
;
; C# writes a type name as an identifier, so only type positions are named:
; any node's `type:` field (a field, a parameter, a local, a return type, a
; cast, typeof, `new X`, a pattern, a catch), a base list, a type argument, a
; qualified name `A.B`, and the type a static member access names
; (`Util.Call()`, `Other.Prop`, `Json.Parse<T>()`). The extractor keeps those
; whose head starts with an upper case letter or a using binds (C# sees its
; namespace's and every enclosing namespace's types unqualified). An
; attribute is not named: `[Test]` means the class TestAttribute first, which
; a name path cannot say.
(_ type: [(identifier) (qualified_name)] @ref.name)
(_ type: (generic_name . (identifier) @ref.name))
(_ returns: [(identifier) (qualified_name)] @ref.name)
(_ returns: (generic_name . (identifier) @ref.name))

(base_list [(identifier) (qualified_name)] @ref.name)
(base_list (generic_name . (identifier) @ref.name))

(type_argument_list [(identifier) (qualified_name)] @ref.name)
(type_argument_list (generic_name . (identifier) @ref.name))

(member_access_expression
  expression: (identifier) @ref.head
  name: (identifier) @ref.member)

(member_access_expression
  expression: (identifier) @ref.head
  name: (generic_name . (identifier) @ref.member))
