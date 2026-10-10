# scss — provenance

- Grammar: `P233/tree-sitter-scss v1.0.1` (ee500b9c, MIT: the top-level LICENSE read at the tag with
  `indexbench lang-pin -tree`), built into `internal/extractor/tsengine/grammars/scss.wasm.gz`; pin
  and digests in `internal/extractor/tsengine/grammars/PROVENANCE.md`. Chosen by the parse-error gate
  (design §14.2, O850): of every maintained tree-sitter-scss grammar with an OSI licence read at its
  pin, it is the one that parses the queue's subjects (twbs/bootstrap v5.3.8 and primer/css v22.3.2,
  `calibration/corpus.d/scss.yaml`, 235 files) with no file in error (0 of 235); the queue's
  tree-sitter-grammars/tree-sitter-scss leaves 116. It is an independent grammar, not a fork of the
  official one, with its own node schema.
- `tags.scm`: written in this repository (LB-F465). The grammar ships `queries/highlights.scm` only.
  Every capture shape comes from `indexbench lang-probe -dialect scss` against real source (testdata
  and the calibration subjects), never from grammar.js or node-types.json:
  - `@mixin` and `@function` are callables; the definition's own `{` token is captured as `@body`,
    because the grammar's declaration_block starts after it, so a signature ends at the block.
  - Variables: every `$name:` declaration at any depth, and the loop variables `@each` (each name
    before `in`) and `@for` declare; a loop variable's definition is the variable node, so a loop
    owns nothing.
  - Class, id and placeholder names of a style rule's selector list, as universal-ctags' SCSS parser
    tags them (each `.name`, `#name`, `%name`, also in a compound or complex selector and one level
    inside a pseudo-class's selector arguments; never in `@extend`). Each is a tag: its definition
    is the selector, so a rule's references stay with the definition around the rule (the file's
    module symbol at the top level). The manifest's `names.strip_prefix` drops the sigil, as Sass's
    own AST and ctags name them.
  - `@keyframes` rules and custom properties as the css dialect reads them; `@include` and function
    calls as references (a CSS function such as `rgba()` names no definition and binds nothing).
- `imports.scm`: written in this repository. `@use` (a `sass:` built-in names no file; `as` binds the
  namespace), `@forward`, and a Sass `@import` (a wildcard: its members are global), never a plain
  CSS import (a `.css` URL, an absolute or protocol URL, `url()`). The `relative-path` strategy with
  `partial_prefix: "_"`, the `.scss` and `.css` suffixes and the `_index`/`index` files follows the
  load rule Dart Sass's own loader uses; with `module_symbol` each load is a file edge (K18).
- `testdata/`: fixtures written in this repository; goldens written by hand first, then compared with
  the extractor's output as sets (the order is the extractor's).
