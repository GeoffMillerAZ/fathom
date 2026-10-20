; source: written for this dialect (duskmoon314/tree-sitter-thrift v0.5.0 ships highlights, folds and locals queries
; only; neither aider nor nvim-treesitter carries Thrift tags). Node shapes read with `indexbench lang-probe`, never
; grammar.js.
; Kinds: a namespace (named by its scope, `namespace java com.acme` is `java`: this grammar splits a dotted namespace
; into sibling nodes), a typedef, a const, an enum and its values (Enum.VALUE), a struct, a union and an exception with
; their fields (Struct.field), a service with its functions (Service.function).
; References: a type name written in a field, a parameter, a throws list, a return type, a const or a typedef, and the
; service a service extends. An unqualified reference is named by its identifier, a qualified one by `file.Type`. A built-in type
; (i32, string, list<...>) is no reference; `uuid` is one but this grammar version (v0.5.0) predates it and reads it as
; a type name, so it is excluded by name.

(namespace_declaration (namespace_scope) @name) @definition.namespace
(typedef_definition (typedef_identifier) @name) @definition.typedef
(const_definition (identifier) @name) @definition.const

(enum_definition type: (identifier) @name) @definition.enum
(enum_definition type: (identifier) (identifier) @name @definition.enum_value)

(struct_definition type: (identifier) @name) @definition.struct
(union_definition type: (identifier) @name) @definition.union
(exception_definition (identifier) @name) @definition.exception
(field (identifier) @name) @definition.field

(service_definition . type: (identifier) @name) @definition.service
(function_definition (identifier) @name [";" ","]? @body) @definition.function

; A type reference: unqualified (`Order`, the file's own type) or qualified by the included file's name (`shared.Money`).
; The qualified one keeps its qualifier as the reference text, so the same-file resolver never binds `shared.Money` to
; a Money of the file's own; the file resolver binds it through the include (references.scm).
((type type: (identifier) @name .) @reference.call
  (#not-any-of? @name "uuid"))
(type type: (identifier) . (identifier)) @reference.call
((definition_type type: (identifier) @name .) @reference.call
  (#not-any-of? @name "uuid"))
(definition_type type: (identifier) . (identifier)) @reference.call

; A service names the service it extends, after `extends`, qualified by the included file's name or not. The
; unqualified name is the second `type:` identifier with nothing named after it but the body.
(service_definition . type: (identifier) . type: (identifier) @name @reference.call . [(function_definition) (comment) (annotation_definition)])
(service_definition . type: (identifier) . type: (identifier) @name @reference.call .)
(service_definition . type: (identifier) . type: (identifier) . (identifier) @name @reference.call)
