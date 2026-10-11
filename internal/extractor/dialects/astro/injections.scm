; dialects/astro/injections.scm — the ranges an Astro component hosts (F2, injection_host; design §1.8)
; source: virchau13/tree-sitter-astro queries/injections.scm @ 213f6e6973d9b456c6e50e86f19f66877e7ef0ee (MIT)
; changes: the frontmatter, script and style patterns are kept; a script is injected only when its `type` is
; absent or names JavaScript (a JSON-LD or import-map script is data); a style is CSS, or SCSS when lang="scss"
; (another lang has no dialect and is not injected); the interpolation patterns are dropped: an interpolation's
; text is cut by the elements it holds into fragments no parser reads as one expression (design §1.8:
; template expressions are out of scope, counted).

; The frontmatter is the component's module script: TypeScript (Astro compiles it as such).
((frontmatter
  (frontmatter_js_block) @injection.content)
 (#set! injection.language "typescript"))

; A <script> with no type attribute: JavaScript, read as TypeScript (Astro processes it as TypeScript).
((script_element
  (start_tag) @_start
  (raw_text) @injection.content)
 (#not-match? @_start "[[:space:]]type[[:space:]]*=")
 (#set! injection.language "typescript"))

; A <script> whose type names JavaScript.
((script_element
  (start_tag
    (attribute
      (attribute_name) @_attr
      [
        (quoted_attribute_value (attribute_value) @_type)
        (attribute_value) @_type
      ]))
  (raw_text) @injection.content)
 (#eq? @_attr "type")
 (#match? @_type "^(module|text/javascript|application/javascript)$")
 (#set! injection.language "typescript"))

; A <style> with no lang: CSS.
((style_element
  (start_tag) @_start
  (raw_text) @injection.content)
 (#not-match? @_start "[[:space:]]lang[[:space:]]*=")
 (#set! injection.language "css"))

; A <style lang="scss">: SCSS.
((style_element
  (start_tag
    (attribute
      (attribute_name) @_attr
      [
        (quoted_attribute_value (attribute_value) @_lang)
        (attribute_value) @_lang
      ]))
  (raw_text) @injection.content)
 (#eq? @_attr "lang")
 (#eq? @_lang "scss")
 (#set! injection.language "scss"))
