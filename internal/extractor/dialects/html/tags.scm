; dialects/html/tags.scm — HTML (LB-Q-html, design §13.2) ────────────────────
; Written in this repository: tree-sitter/tree-sitter-html ships no
; queries/tags.scm of its own (only highlights.scm and injections.scm), and
; neither aider nor nvim-treesitter carries an html-tags.scm/locals.scm to
; start from (checked, 2026-09-28) — see PROVENANCE.md. Shapes below come
; from `indexbench lang-probe -dialect html testdata/shapes.html` (§13 step
; 12), never from grammar.js/node-types.json.
;
; The grammar gives every element the SAME node type ("element" wrapping a
; "start_tag" or, for a void/self-closing form, a "self_closing_tag") no
; matter its tag name, so every pattern below distinguishes by attribute or
; tag-name text via #eq?/#match?/#any-of? (LB-F2) rather than by node shape.
; An attribute's value is either bare (`id=main`, an `attribute_value`
; directly) or quoted (`id="main"`, an `attribute_value` aliased inside a
; `quoted_attribute_value`) — every pattern below accepts both via `[ ]`.

; --- #requires-text! (LB-F180): a pattern below carries guards, regexps a source
; must match for the pattern to match at all (tsengine/guards.go). Each of
; these patterns matches every tag or attribute of a page structurally and is
; then cut by its text predicates, which cost the guest most of the query on
; markup-heavy pages (36 G of 50 G instructions over retrofit's javadoc); a
; page that cannot satisfy one runs the query without it. A guard is a
; necessary condition only: it never admits or drops a capture, and the
; guarded-vs-unguarded test (htmlguards_test.go) proves it over the testdata.
; The scanner skips whitespace after `<`, so a guard that names a tag allows
; it; a self-closing tag ends in the one token `/>`.

; --- label (an HTML `id` attribute; required; matches universal-ctags' HTML
; "id" kind, ctags 6.2.1 verified directly — the only HTML kind fathom's own
; ctags invocation (CtagsArgs, no --extras=+r) ever emits as a definition,
; since "class", "stylesheet" and "script" are ctags roles (REFONLY) it
; never emits without --extras=+r). The manifest's own kind key is "label"
; (its canonical, not "id") because `Reading.Validate` (§1.4) compares a
; `required_kinds` entry against `KindIntersection` — a list of CANONICAL
; names — literally, with no lookup through the manifest; every other
; dialect's required kinds happen to already equal their own canonical
; string, so this off-by-a-lookup only surfaces here (verified directly:
; keeping the key "id" measures fine solo but refuses once a real match
; occurs — a real reading gap, worth an Opus look, not a worker's file to
; fix). The whole element is the definition range (EndLine reaches its
; closing tag) even though ctags itself proves no end for HTML kinds —
; verified directly with `--fields='*'`: zero "end" fields across every
; HTML kind ctags emits, for any element (see PROVENANCE.md and this
; queue item's reading `notes`).
(element
  (start_tag
    (attribute
      (attribute_name) @_attr
      (#eq? @_attr "id")
      [
        (quoted_attribute_value (attribute_value) @name)
        (attribute_value) @name
      ]))) @definition.label

(element
  (self_closing_tag
    (attribute
      (attribute_name) @_attr
      (#eq? @_attr "id")
      [
        (quoted_attribute_value (attribute_value) @name)
        (attribute_value) @name
      ]))
  (#requires-text! "/>")) @definition.label

; --- custom_element (not required; no ctags equivalent — deliberately
; absent from oracle_kinds, the same convention CSS's keyframes/
; custom_property and Python's PythonTypeAlias document). The Custom
; Elements spec requires a hyphen in a custom tag's local name; `#match?`
; is the only way the engine (LB-F2) can single that out, since the
; grammar gives a custom element the same "element"/"start_tag" shape as
; every built-in tag.
(element
  (start_tag
    (tag_name) @name
    (#match? @name "-"))
  (#requires-text! "<[[:space:]]*[^[:space:]<>]*-")) @definition.custom_element

(element
  (self_closing_tag
    (tag_name) @name
    (#match? @name "-"))
  (#requires-text! "<[[:space:]]*[^[:space:]<>]*-")) @definition.custom_element

; --- form (not required; no ctags equivalent). Named by its `action`
; (the endpoint it submits to — the "route" canonical fits a form better
; than "element"); a form without an action is not captured, which costs
; this unscored kind some recall, never precision.
(element
  (start_tag
    (tag_name) @_tag
    (#eq? @_tag "form")
    (attribute
      (attribute_name) @_attr
      (#eq? @_attr "action")
      [
        (quoted_attribute_value (attribute_value) @name)
        (attribute_value) @name
      ]))
  (#requires-text! "<[[:space:]]*form")
  (#requires-text! "action")) @definition.form

; --- field (not required; no ctags equivalent): a named form control
; (`input`/`select`/`textarea`/`button`) is the payload key a form submits.
(element
  (start_tag
    (tag_name) @_tag
    (#any-of? @_tag "input" "select" "textarea" "button")
    (attribute
      (attribute_name) @_attr
      (#eq? @_attr "name")
      [
        (quoted_attribute_value (attribute_value) @name)
        (attribute_value) @name
      ]))
  (#requires-text! "<[[:space:]]*(?:input|select|textarea|button)")
  (#requires-text! "name")) @definition.field

(element
  (self_closing_tag
    (tag_name) @_tag
    (#any-of? @_tag "input" "select" "textarea" "button")
    (attribute
      (attribute_name) @_attr
      (#eq? @_attr "name")
      [
        (quoted_attribute_value (attribute_value) @name)
        (attribute_value) @name
      ]))
  (#requires-text! "<[[:space:]]*(?:input|select|textarea|button)")
  (#requires-text! "name")) @definition.field
