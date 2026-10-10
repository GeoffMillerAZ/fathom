# lightningcss_oracle.cjs — the CSS oracle of design 1.8 W2 (tier A for what it attests)

Ruling W2 (design 1.8): lightningcss, built on Mozilla's cssparser (the CSS parser of Firefox's
Servo style engine), is tier A for what a browser parser attests: rule and at-rule definitions,
their ranges, custom property definitions and the `@import` graph, the precedent of design 1.3's
runtime trace, tier A only for what it attests. It attests no cascade, so `var()` and
`animation-name` references grade as name references within the import closure, and the reading
says so.

The script parses every `.css` file of a subject with lightningcss's own `transform` (error
recovery on, as a browser recovers; `drafts.customMedia` on, so a sheet that declares
`@custom-media` keeps its `@media (--name)` rules) and writes the neutral oracle file
`indexbench lang-oracle -tool-oracle` reads:

- `definitions`: one `selector` per selector of each style rule's list (nested rules and the rules
  of `@media`, `@supports`, `@container`, `@layer`, `@scope`, `@starting-style` and
  `@-moz-document` included), one `keyframes` per `@keyframes` rule (vendor spellings included),
  one `custom_property` per `--name:` declaration of a style rule or of a declaration run inside a
  group rule nested in one. `end` is 0: lightningcss's AST carries a rule's start (line, column)
  and no end, so `range_agreement` is not measured and L1 is capped at good (design 1.2).
- `calls`: every `var()` name lightningcss parses in a declaration's value (a fallback's own
  `var()` included) and every `<keyframes-name>` of `animation-name` and of the `animation`
  shorthand, as (owner, name). An animation declaration lightningcss leaves unparsed (its value
  holds a `var()`) names no keyframes and is counted (`animation_unparsed`).
- `imports`: every `@import` rule lightningcss keeps (one after another rule is invalid CSS and is
  dropped, as a browser drops it), each resolved by lightningcss's own bundler: `bundleAsync` of
  the file alone, every other file read as empty, so the bundler's resolver names the file each of
  the file's own imports loads (`to_path` when that file is in the subject; an external URL or a
  missing file is in-repo for nobody).

Positions. lightningcss locates a rule, never a selector of its list or a declaration. A rule's
selectors are the source from that location to the block's `{`, cut at top-level commas (outside
strings, comments, parentheses and brackets), each named by its own text and line, and used only
when the cut gives as many selectors as lightningcss parsed (else counted, `selector_cut_mismatch`,
and left out). A custom property is the k-th top-level `--name:` of its rule's block, the name and
the count being lightningcss's (`custom_unlocated` counts one with no such declaration). What
exists is lightningcss's; the names and lines are the source's.

Owners, the dialect's own rule (`internal/extractor/dialects/css/tags.scm`): a style rule's
declarations to the rule's first selector, a custom property's value to the property, an
`@keyframes` rule's declarations to the rule, a descriptor of any other at-rule (`@font-face`,
`@page`) to the file's module symbol, `<module>`.

The grading rule (W2, `cmd/indexbench/langoracle_css.go`, LB-F460): a pair is graded only when its
name is a custom property or `@keyframes` rule the oracle defines in the pair's file or in a file
its `@import` closure reaches, on both sides; every other pair is out of both, counted
(`css_w2_oracle_outside_closure`, `css_w2_fathom_outside_closure`) and disclosed.

MPL-2.0: run in the measuring harness only, never vendored or modified, installed into scratch:

    npm install --prefix <scratch>/npm lightningcss@1.33.0
    NODE_PATH=<scratch>/npm/node_modules node calibration/oracles/lightningcss/lightningcss_oracle.cjs <subject> [dir...] > <out>.json
    indexbench lang-oracle -dialect css -subject <subject> -subject-name <name> -tool-oracle <out>.json -out docs/bench/readings/lang/css.json

The script prints its counts on stderr: files, warnings (error recovery), unknown at-rules,
`selector_cut_mismatch`, `custom_located`, `custom_unlocated`, `animation_unparsed`, imports
(resolved, unread), bundle errors and parse failures.
