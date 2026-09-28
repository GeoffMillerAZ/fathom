; dialects/ruby/tags.scm — symbol + call queries for the wazero tree-sitter engine
; (internal/extractor.NewTSExtractor("ruby"), ADR-021 §4).
;
; Predicate-free by design — see python.scm's header comment for why.

(module
  name: (_) @name) @definition.module

(class
  name: (_) @name) @definition.class

(method
  name: (_) @name) @definition.method

(singleton_method
  name: (_) @name) @definition.method

; Naive same-file call collection — mirrors the incumbent CGO extractor's
; CollectCalls ("method" field text of a `call` node), not a typed/resolved
; edge.
(call
  method: (_) @reference.call)
