; source: tree-sitter/tree-sitter-html queries/injections.scm @ 5a5ca8551a179998360b4a4ca2c0f366a35acc03 (MIT)
; changes: none — the two patterns are copied verbatim (LB-F6, design §3.6:
; the engine reads exactly this `#set! injection.language` convention).
; "javascript" and "css" must be registered dialect ids for the injection to
; run; css is not registered yet (LB-Q-css unmerged at this writing), so the
; style pattern is a silent no-op until it lands (extractInjections skips an
; unknown injection.language).

((script_element
  (raw_text) @injection.content)
 (#set! injection.language "javascript"))

((style_element
  (raw_text) @injection.content)
 (#set! injection.language "css"))
