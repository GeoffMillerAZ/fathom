; dialects/python/references.scm — the file resolver's references (LB-F74;
; vocabulary in internal/extractor/tsrefs.go), compiled apart from tags.scm
; and imports.scm (TSExtractor.ExtractFileSpecs).
;
; Every identifier and dotted attribute path outside a call's own text: a
; type hint, a base class, a value passed or assigned, an isinstance target,
; `pkg.mod.Name` through `import pkg.mod`. The extractor keeps only those
; whose head an import binds (and, in a file with `from m import *`, those
; starting with an upper case letter), so a local, a parameter or an
; attribute of self never reaches the store. Python re-exports by binding: a
; module's own imports are its attributes, which the resolver follows
; (internal/fusion/exports.go), so there is no @export capture here.
(identifier) @ref.name
(attribute) @ref.name

; Not references of their own: an attribute's name (`pm.helper`'s helper)
; and a keyword argument's label.
(attribute
  attribute: (identifier) @ref.not)

(keyword_argument
  name: (identifier) @ref.not)
