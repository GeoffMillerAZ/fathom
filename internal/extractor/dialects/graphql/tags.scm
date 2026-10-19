; source: written for this dialect (bkegley/tree-sitter-graphql ships queries/graphql/{highlights,locals}.scm only,
; no tags.scm; neither aider nor nvim-treesitter carries GraphQL tags). Node shapes read with `indexbench
; lang-probe`, never grammar.js.
; Kinds: the incumbent graphqlextract.go's two keep their SourceKind strings (GraphQLType for every type
; definition, GraphQLField for a field of an object, interface or input type, named Type.field), and the
; dialect adds an enum value (Role.ADMIN), a directive definition, a type extension (named by its target, so its
; fields are Query.me), a named operation and a fragment. An anonymous operation, a schema definition and a
; schema extension have no name and are no definition.
; References: every named type a definition writes (implements, union members, a field's or argument's type,
; an input field's type, a variable's type, a type condition) and every fragment spread, named by the type or
; fragment, owned by the smallest enclosing definition. The five built-in scalars are no reference.

(object_type_definition
  (name) @name
  (fields_definition)? @body) @definition.type

(interface_type_definition
  (name) @name
  (fields_definition)? @body) @definition.type

(input_object_type_definition
  (name) @name
  (input_fields_definition)? @body) @definition.type

(enum_type_definition
  (name) @name
  (enum_values_definition)? @body) @definition.type

(union_type_definition
  (name) @name) @definition.type

(scalar_type_definition
  (name) @name) @definition.type

(object_type_extension
  (name) @name
  (fields_definition)? @body) @definition.extension

(interface_type_extension
  (name) @name
  (fields_definition)? @body) @definition.extension

(input_object_type_extension
  (name) @name
  (input_fields_definition)? @body) @definition.extension

(enum_type_extension
  (name) @name
  (enum_values_definition)? @body) @definition.extension

(union_type_extension
  (name) @name) @definition.extension

(scalar_type_extension
  (name) @name) @definition.extension

(field_definition
  (name) @name) @definition.field

(input_fields_definition
  (input_value_definition
    (name) @name) @definition.field)

(enum_value_definition
  (enum_value
    (name) @name)) @definition.enum_value

(directive_definition
  (name) @name) @definition.directive

(operation_definition
  (name) @name
  (selection_set) @body) @definition.operation

(fragment_definition
  (fragment_name
    (name) @name)
  (selection_set) @body) @definition.fragment

((named_type
   (name) @name) @reference.call
  (#not-any-of? @name "String" "Int" "Float" "Boolean" "ID"))

(fragment_spread
  (fragment_name
    (name) @name)) @reference.call
