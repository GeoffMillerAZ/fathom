; dialects/html/imports.scm — src/href file edges (LB-Q-html, design §3.7/§6).
; Generic vocabulary only (@import.stmt/@import.module — LB-F7): an HTML
; `src`/`href` has no local binding to track, only a target file, exactly
; like CSS's bare `@import "x.css"` — same `relative-path` strategy (design
; §6 table: "HTML `src`/`href`").
;
; Excluded (#not-match?, LB-F2) so only a real in-repo path reaches the
; resolver: a same-page fragment (`href="#top"`) and any URI with a scheme
; (`https:`, `mailto:`, `tel:`, `javascript:`, `data:`) — none of these is a
; file this repository holds. `indexbench lang-probe -dialect html
; testdata/shapes.html` confirms exactly the six in-repo specs match
; (link/script/img src|href) and the two excluded ones (`#top`, none of the
; fixture's hrefs happen to be scheme-qualified, so a scheme case is only
; covered by the pattern's own regex, not a fixture assertion).
(attribute
  (attribute_name) @_attr
  (#any-of? @_attr "src" "href")
  [
    (quoted_attribute_value (attribute_value) @import.module)
    (attribute_value) @import.module
  ]
  (#not-match? @import.module "^(#|[a-zA-Z][a-zA-Z0-9+.-]*:)")) @import.stmt
