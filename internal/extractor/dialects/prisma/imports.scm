; source: written for this dialect. A Prisma schema is the union of the .prisma files of one folder
; (prismaSchemaFolder) with no import statement (LB-F580): a field's type identifier is a spec whose name is its
; module, and the manifest's global-name strategy binds it to the one file of the importer's folder defining a model,
; view, composite type or enum of that name. The nine built-in scalars name no file.
((column_type (identifier) @import.name @import.module)
  (#not-any-of? @import.name "String" "Boolean" "Int" "BigInt" "Float" "Decimal" "DateTime" "Json" "Bytes"))
