; dialects/ruby/references.scm — the file resolver's references (LB-F74,
; LB-F76; vocabulary in internal/extractor/tsrefs.go), compiled apart from
; tags.scm and imports.scm (TSExtractor.ExtractFileSpecs).
;
; Every constant and every scoped constant path (`Sinatra::Helpers::X`,
; `::Top::Y`). tags.scm already records a constant written in a method as a
; call, by its own name; one written in a class or module body (a superclass,
; `include Helpers`, a constant's value) belongs to no method, so only this
; query reaches it, from the class or module around it. The resolver looks a
; constant up lexically (manifest imports.constants "lexical": the
; reference's enclosing modules and classes, innermost first, then the top
; level).
(constant) @ref.name
(scope_resolution) @ref.name

; Not a reference of its own: a scoped path's last constant
; (`Sinatra::Helpers::Stream`'s Stream, read with its scope).
(scope_resolution
  name: (constant) @ref.not)
