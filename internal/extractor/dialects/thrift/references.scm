; dialects/thrift/references.scm -- the file resolver's references (LB-F74; vocabulary in internal/extractor/tsrefs.go),
; compiled apart from tags.scm and imports.scm.
;
; A type written through an included file, `shared.Money`: the last identifier is the type, and the file resolver binds
; it among the names the file's includes bring into scope. An unqualified name (`Money`) is always the file's own type
; in Thrift, so it is no cross-file reference and is not read here.
(type type: (identifier) . (identifier) @ref.name)
(definition_type type: (identifier) . (identifier) @ref.name)
(service_definition . type: (identifier) . type: (identifier) . (identifier) @ref.name)
