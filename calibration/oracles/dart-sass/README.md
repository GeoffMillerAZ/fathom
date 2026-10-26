# sass_oracle.cjs — the SCSS oracle (LB-F466): Dart Sass, tier A for what it attests

Dart Sass is Sass's reference implementation (design 1.3: the format's own evaluation and
reference parser are tier A). The script runs two of its parts over a subject's `.scss` files and
writes the neutral oracle file `indexbench lang-oracle -tool-oracle` reads:

- `definitions`, from **sass-parser** 0.4.57 (published from the sass/dart-sass repository: Dart
  Sass's own parser exposed as a tree): every `@mixin`, `@function`, `$variable` declaration at any
  depth, `@each` and `@for` loop variable, `@keyframes` rule and custom property, and every class,
  id and placeholder a style rule's selector list writes, read from Dart Sass's selector parser
  (`parsedSelector`, pseudo-class arguments included), each with its own inclusive source span.
  The parser's `source.end` is exclusive; an end at column 1 belongs to the preceding line.
  Loop variable spans are derived from the declared `$name` tokens before `eachExpression` or
  `fromExpression`, skipping comments and decoding CSS escapes to match the parser's names; they
  do not cover the loop body. Unicode and escaped names retain their source spelling in output. A member's name is
  read as written at the node's offset: Dart Sass hands it back normalized (`_` read as `-`).
  Parameters are not definitions here (the dialect has no parameter kind). Universal-ctags' SCSS parser is deficient as an L1 oracle on the
  subjects (O851): it tags numbers as classes (`25rem`, `5`), skips a class after `+` and the
  variables of a multi-variable `@each`, and writes no end line, so the parser is the L1 oracle
  (D2's precedent).
- `imports`, from **Dart Sass** 1.105.1's compiler: each file is compiled alone (`sass.compile`); a
  compile that succeeds attests the files it loaded (`loadedUrls`). Each `@use`, `@forward` and Sass
  `@import` sass-parser reads in a loaded file is paired with the one loaded file its URL can name
  under Sass's load rule (the name or its `_` partial with `.scss`, `.sass` or `.css`, or the
  directory's `_index`/`index` file). A package outside the subject (node_modules) is served an
  empty stylesheet by a stub importer so the in-repo loads around it are still evaluated, and is no
  file of the subject (`to_path` empty); a `sass:` built-in names no file.

- `calls` and `call_exclusions` (design 1.8 W5, `w5_refs.cjs`): SCSS's L2 is name references over the
  `@use`/`@forward`/`@import` closure, walked from the same sass-parser tree (tier A for the tree, the binding
  the harness's own walk). A pair runs from the innermost `@mixin` or `@function` (`<module>` at top level) to
  the name a reference writes: `@include m` and `@include ns.m`, a call of a user-defined `@function`, a `$v` or
  `ns.$v` read (the innermost enclosing block first, a parameter binding with no definition, then the module),
  and `@extend %p`. A namespaced member resolves through the `@use ... as ns` that names it and its `@forward`
  chain (`as prefix-*`, `show`, `hide`; private `-x`/`_x` members stay private). Built-ins, packages outside the
  subject and names the closure does not define are counted in `disclose`, and every name no read of a file
  resolves is a `call_exclusion` that leaves both sides of that file's pairs.

Scope (`-files-out`, the reading's `-only`): a file sass-parser cannot parse (0.4.57 rejects a
keyframe selector written as a percentage, `0% { }`, which Dart Sass compiles) has no definition
truth, and a file whose loads no successful compile evaluated (a test file needing sass-true's
mixins) has no import truth; both are named on stderr and left out.

MIT; bench-only, installed into scratch:

    npm install --prefix <scratch>/npm sass-parser@0.4.57 sass@1.105.1
    git -C <subject> ls-files '*.scss' | sort > files.txt
    NODE_PATH=<scratch>/npm/node_modules node calibration/oracles/dart-sass/sass_oracle.cjs <subject> files.txt \
        -files-out parsed.txt > out.json
    indexbench lang-oracle -dialect scss -subject <subject> -tool-oracle out.json -only "$(paste -sd, parsed.txt)" \
        -out docs/bench/readings/lang/scss.json

Regression test: `NODE_PATH=<scratch>/npm/node_modules node --test calibration/oracles/dart-sass/sass_oracle.test.cjs`.
