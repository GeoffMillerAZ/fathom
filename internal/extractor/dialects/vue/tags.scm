; dialects/vue/tags.scm — Vue single-file components (LB-Q-vue, wave lf7) ────
; Written in this repository: tree-sitter-grammars/tree-sitter-vue ships
; highlights.scm and injections.scm, no tags.scm, and neither aider nor
; nvim-treesitter carries a vue tags or locals query to start from (see
; PROVENANCE.md). Shapes from `indexbench lang-probe -dialect vue`.
;
; A component file's own definitions are its <script> blocks' (the injected
; TypeScript or JavaScript, F2: dialect.json's injection_host) and its module
; symbol, the component (module_symbol). This query names what the template
; renders: a component element is a call from the component's module symbol
; (calls.owner "module") to the component it names (owner ruling M2: an
; element the compiler types a component is a call; design §1.8, K19).
;
; The Vue compiler types an element a component (compiler-core isComponent,
; @vue/compiler-sfc 3.5.43's parse with compiler-dom's options) when its tag
; is not one of the platform's native tags (@vue/shared's HTML_TAGS,
; SVG_TAGS and MATH_TAGS, copied below as written at @vue/shared 3.5.43, MIT)
; and is not <slot> (a slot outlet) or <component> (a dynamic component, whose
; callee is its :is expression, no name of its own): a capitalized tag, a
; hyphenated one, Vue's own Transition, KeepAlive, Teleport and Suspense, and
; any other tag no platform defines. The callee is the name resolveComponent
; resolves it by (names.pascal_case: el-input calls ElInput). A <template>
; is its own node (template_element), never a component.
;
; A top-level element other than <template>, <script> and <style> is an SFC
; custom block (<i18n>, <docs>, <route>: the compiler's
; descriptor.customBlocks), never rendered: it is a definition of its own (a
; block, named by its tag, pascal-cased as every name here is), so the markup
; its content may hold is no call of the component's (the engine attaches no
; call to a definition that is not callable, and the module symbol owns only
; the calls no definition encloses).
(document
  (element
    [(start_tag (tag_name) @name) (self_closing_tag (tag_name) @name)]) @definition.block)

(element
  [(start_tag (tag_name) @name) (self_closing_tag (tag_name) @name)] @reference.call
  (#not-match? @reference.call "^<[A-Za-z][A-Za-z0-9_-]*\\.[A-Za-z_$]")
  (#not-any-of? @name
   "html" "body" "base" "head" "link" "meta" "style" "title" "address" "article" "aside" "footer"
   "header" "hgroup" "h1" "h2" "h3" "h4" "h5" "h6" "nav" "section" "div" "dd" "dl" "dt"
   "figcaption" "figure" "picture" "hr" "img" "li" "main" "ol" "p" "pre" "ul" "a" "b" "abbr" "bdi"
   "bdo" "br" "cite" "code" "data" "dfn" "em" "i" "kbd" "mark" "q" "rp" "rt" "ruby" "s" "samp"
   "small" "span" "strong" "sub" "sup" "time" "u" "var" "wbr" "area" "audio" "map" "track" "video"
   "embed" "object" "param" "source" "canvas" "script" "noscript" "del" "ins" "caption" "col"
   "colgroup" "table" "thead" "tbody" "td" "th" "tr" "button" "datalist" "fieldset" "form" "input"
   "label" "legend" "meter" "optgroup" "option" "output" "progress" "select" "textarea" "details"
   "dialog" "menu" "summary" "template" "blockquote" "iframe" "tfoot" "svg" "animate"
   "animateMotion" "animateTransform" "circle" "clipPath" "color-profile" "defs" "desc" "discard"
   "ellipse" "feBlend" "feColorMatrix" "feComponentTransfer" "feComposite" "feConvolveMatrix"
   "feDiffuseLighting" "feDisplacementMap" "feDistantLight" "feDropShadow" "feFlood" "feFuncA"
   "feFuncB" "feFuncG" "feFuncR" "feGaussianBlur" "feImage" "feMerge" "feMergeNode" "feMorphology"
   "feOffset" "fePointLight" "feSpecularLighting" "feSpotLight" "feTile" "feTurbulence" "filter"
   "foreignObject" "g" "hatch" "hatchpath" "image" "line" "linearGradient" "marker" "mask" "mesh"
   "meshgradient" "meshpatch" "meshrow" "metadata" "mpath" "path" "pattern" "polygon" "polyline"
   "radialGradient" "rect" "set" "solidcolor" "stop" "switch" "symbol" "text" "textPath" "tspan"
   "unknown" "use" "view" "annotation" "annotation-xml" "maction" "maligngroup" "malignmark" "math"
   "menclose" "merror" "mfenced" "mfrac" "mfraction" "mglyph" "mi" "mlabeledtr" "mlongdiv"
   "mmultiscripts" "mn" "mo" "mover" "mpadded" "mphantom" "mprescripts" "mroot" "mrow" "ms"
   "mscarries" "mscarry" "msgroup" "msline" "mspace" "msqrt" "msrow" "mstack" "mstyle" "msub"
   "msubsup" "msup" "mtable" "mtd" "mtext" "mtr" "munder" "munderover" "none" "semantics"
   "slot" "component"))

; A member tag, <Foo.Bar> (a namespaced component of <script setup>): the
; grammar reads the tag as Foo and .Bar as a v-bind .prop shorthand written
; against it; the compiler reads the tag whole. The call is to its last
; segment (owner ruling M2), the name after the dot.
(element
  [(start_tag (tag_name) . (directive_attribute (directive_value) @name))
   (self_closing_tag (tag_name) . (directive_attribute (directive_value) @name))] @reference.call
  (#match? @reference.call "^<[A-Za-z][A-Za-z0-9_-]*\\.[A-Za-z_$]"))
