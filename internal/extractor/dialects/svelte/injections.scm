; dialects/svelte/injections.scm — the component's blocks (LB-F6, wave lf7)
; Written in this repository after the grammar's own queries/injections.scm
; (tree-sitter-grammars, MIT), which injects a <script> by its lang
; attribute and a <style> as CSS. Only dialects fathom registers run
; (extractInjections skips any other injection.language): a <script lang="ts">
; (or "typescript", "tsx") is TypeScript, any other <script> JavaScript; a
; <style lang="scss"> is SCSS, a <style> with no lang CSS, and a style in
; another language (less, stylus, postcss) is not read. The host owns each
; range's imports and top-level calls (dialect.json's injection_host, F2).

((script_element
  (start_tag
    (attribute
      (attribute_name) @_lang
      [(quoted_attribute_value (attribute_value) @_ts) (attribute_value) @_ts]))
  (raw_text) @injection.content)
 (#eq? @_lang "lang")
 (#any-of? @_ts "ts" "typescript" "tsx")
 (#set! injection.language "typescript"))

((script_element
  (start_tag) @_tag
  (raw_text) @injection.content)
 (#not-match? @_tag "lang[[:space:]]*=[[:space:]]*[\"']?(ts|typescript|tsx)[\"'[:space:]/>]")
 (#set! injection.language "javascript"))

((style_element
  (start_tag
    (attribute
      (attribute_name) @_lang
      [(quoted_attribute_value (attribute_value) @_scss) (attribute_value) @_scss]))
  (raw_text) @injection.content)
 (#eq? @_lang "lang")
 (#eq? @_scss "scss")
 (#set! injection.language "scss"))

((style_element
  (start_tag) @_tag
  (raw_text) @injection.content)
 (#not-match? @_tag "lang[[:space:]]*=")
 (#set! injection.language "css"))
