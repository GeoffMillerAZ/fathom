; source: ngalaiko/tree-sitter-go-template queries/highlights.scm @ aa71f63de226c5592dfbfc1f29949522d7c95fac (MIT)
; changes: highlights.scm carries no @definition/@reference captures to borrow, so this file is written from
;   scratch against `indexbench lang-probe` output over testdata/; only the node shapes are the grammar's.
;
; dialects/helm/tags.scm — Helm templates on the go-template grammar (ADR-048, LB-Q-helm).
;
; A named template is `{{ define "chart.name" }} … {{ end }}` (or `block`); it is a callable, so the
; `include "x"` / `template "x"` calls inside it attach to it. A `.Values.a.b` accessor is a HelmValueRef
; symbol named for the values key it reads (`a.b`, the manifest's names.strip_prefix takes off `.Values.`),
; the incumbent extractor's shape and the name a values.yaml key already carries, so `references` joins them;
; an `include`/`template` call likewise leaves a HelmTemplateRef named for the template it uses.
; Manifests are Go templates whose YAML is opaque `text`, so a manifest's own kind/name are not captured here.

; ── definitions ───────────────────────────────────────────────────────────

; The signature is the header, `{{- define "x" -}}` (the engine reads from the definition's start to the
; start of @body), so a header that wraps lines still signs whole. A define with nothing in it has no
; @body and is its own pattern (`!body`), so none matches twice.

(define_action
  name: (interpreted_string_literal) @name
  .
  body: (_) @body) @definition.template

(define_action
  name: (interpreted_string_literal) @name
  !body) @definition.template

(define_action
  name: (raw_string_literal) @name
  .
  body: (_) @body) @definition.template

(define_action
  name: (raw_string_literal) @name
  !body) @definition.template

(block_action
  name: (interpreted_string_literal) @name) @definition.template

; A define whose `{{ end }}` is missing (a file mid-edit) is an ERROR node opening with the `define`
; keyword: still a definition, to the end of the broken region — what the incumbent's regex fallback found.
(ERROR
  "define"
  .
  (interpreted_string_literal) @name) @definition.template

; ── template calls: include "x" . / template "x" . ───────────────────────
; Each is an edge from the enclosing named template (a call) and a HelmTemplateRef symbol named for the
; template it uses. Most calls are not inside a define at all — a manifest body is `include "x.labels" .`
; from its first line — and a call outside a named template has no symbol to attach to; the ref symbol is
; what `references` on the template's name finds them by.

(function_call
  function: (identifier) @_fn
  arguments: (argument_list . (interpreted_string_literal) @name)
  (#eq? @_fn "include")) @reference.call

(template_action
  name: (interpreted_string_literal) @name) @reference.call

(function_call
  function: (identifier) @_fn
  arguments: (argument_list . (interpreted_string_literal) @name)
  (#eq? @_fn "include")) @definition.template_ref

(template_action
  name: (interpreted_string_literal) @name) @definition.template_ref

; ── .Values.a.b accessors ─────────────────────────────────────────────────
; A chain `.Values.a.b` is a selector_expression nested once per segment; only the outermost one names
; the key a template reads, so each pattern anchors on a parent that cannot itself be a selector chain.

(template (selector_expression) @name @definition.value_ref (#match? @name "^\\$?\\.Values\\."))
(define_action (selector_expression) @name @definition.value_ref (#match? @name "^\\$?\\.Values\\."))
(block_action (selector_expression) @name @definition.value_ref (#match? @name "^\\$?\\.Values\\."))
(if_action (selector_expression) @name @definition.value_ref (#match? @name "^\\$?\\.Values\\."))
(with_action (selector_expression) @name @definition.value_ref (#match? @name "^\\$?\\.Values\\."))
(range_action (selector_expression) @name @definition.value_ref (#match? @name "^\\$?\\.Values\\."))
(range_variable_definition (selector_expression) @name @definition.value_ref (#match? @name "^\\$?\\.Values\\."))
(template_action (selector_expression) @name @definition.value_ref (#match? @name "^\\$?\\.Values\\."))
(chained_pipeline (selector_expression) @name @definition.value_ref (#match? @name "^\\$?\\.Values\\."))
(parenthesized_pipeline (selector_expression) @name @definition.value_ref (#match? @name "^\\$?\\.Values\\."))
(argument_list (selector_expression) @name @definition.value_ref (#match? @name "^\\$?\\.Values\\."))
(variable_definition (selector_expression) @name @definition.value_ref (#match? @name "^\\$?\\.Values\\."))
(assignment (selector_expression) @name @definition.value_ref (#match? @name "^\\$?\\.Values\\."))
(method_call method: (selector_expression) @name @definition.value_ref (#match? @name "^\\$?\\.Values\\."))
