; source: written for this dialect. A GraphQL schema is the union of its files with no import statement (LB-F580):
; every named type a definition writes (an implements list, a union member, a field's, argument's, input field's,
; directive argument's or variable's type, a type condition) is a spec whose name is its module, and the manifest's
; global-name strategy binds it to the one file of the service defining a type of that name. The five built-in
; scalars name no file. A fragment spread names a fragment, not a type, and is no spec.
((named_type (name) @import.name @import.module)
  (#not-any-of? @import.name "String" "Int" "Float" "Boolean" "ID"))
