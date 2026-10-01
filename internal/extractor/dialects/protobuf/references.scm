; dialects/protobuf/references.scm — the file resolver's references (LB-F74,
; LB-F77; vocabulary in internal/extractor/tsrefs.go), compiled apart from
; tags.scm and imports.scm (TSExtractor.ExtractFileSpecs).
;
; A type reference as written, its package path included
; (`other_package.Embed.Enumerated`, `.buf.validate.FieldRules`): tags.scm
; names a reference by its last identifier, which cannot tell a nested type
; (Embed.Enumerated) or two messages of one name in two packages apart. The
; manifest's imports.type_paths "package-dir" makes the resolver read the
; leading segments as the package, the rest as the type, among the files the
; imports bring in.
(message_or_enum_type) @ref.name
