# astro-compiler — Astro's tier-A definitions, calls and imports

`astro_oracle.mjs` reads each `.astro` file of a subject with
[@astrojs/compiler](https://github.com/withastro/compiler) 4.0.0 (MIT), Astro's own parser,
and the frontmatter and JavaScript scripts it delimits with TypeScript's own parser
([typescript](https://github.com/microsoft/TypeScript) 5.9.3, Apache-2.0, `createSourceFile`),
and writes `indexbench lang-oracle -tool-oracle`'s neutral JSON (design §1.8's Astro paragraph,
owner ruling W4; wave lf7, item angularastro):

- `definitions`: what the frontmatter and each JavaScript `<script>` (no `type`, or `module`,
  `text/javascript`, `application/javascript`) declare, on the TypeScript dialect's kinds: a
  function declaration (an overload signature included, at any depth), a top-level `const`,
  `let` or `var` bound to an arrow or function expression (function), a class (any depth), a
  class's method, constructor or accessor and an interface's method signature (method), an
  interface, a type alias, an enum, and each name a top-level `const` binds otherwise
  (constant: an identifier, every name of a destructuring pattern). `line` and `end` are host
  lines.
- `calls`: (path, caller, callee). A component tag, the compiler's own `component` node, is a
  call of the component's module symbol `<module>`, named as written (the harness reads a member
  tag `<UI.Card>` by its last segment, M2). A range's call, `new`, tagged template, `super(...)`
  and `import(...)` is owned by the innermost enclosing definition above (a destructuring
  `const`'s initializer, and anything outside every definition, by the module symbol: K18, F2),
  named by the callee expression's last name; a subscript or a called result names none and is
  not written. One pair per distinct (file, caller, callee).
- `imports`: every import declaration, `import x = require(...)` and `export ... from` a range
  writes with a relative specifier, resolved as Vite resolves a relative path (the file named,
  else the spec plus `.ts`, `.tsx`, `.js`, `.jsx`, `.mjs`, `.mts` or `.json`, else its `index`
  file); `to_path` is subject-relative, `""` when it names no file of the subject. An alias
  (`~/`, `@/`, a tsconfig `paths` name) and a package are counted, never resolved.
- `disclose`: files, component tags and fragments; template expressions (not injected: out of
  both sides); styles (CSS: their pairs leave fathom's side); scripts read and left; imports by
  kind; files either parser reported a syntax error in (still read).

Tier A for what each parser attests: the compiler decides every node and its type, the
frontmatter's text and lines; TypeScript decides every declaration, call site and import
statement in a range. The ownership of a call (which definition makes it) is K18's module-symbol
rule as the TypeScript dialect applies it, stated in design §1.8.

The reading scores the astro dialect's own extractor under `astro.scoring.json`: the dialect's
kinds plus the TypeScript kinds its hosted ranges keep (F2), with the oracle's kinds mapped onto
the same canonical kinds. `cmd/indexbench/langoracle_astro.go` leaves a style's CSS pairs out of
Astro's L2 (counted, `astro_injected_css_pairs`) and scores the dialect's file edges against the
oracle's imports (LB-F57).

```sh
npm --prefix <tools> install @astrojs/compiler@4.0.0 typescript@5.9.3
node calibration/oracles/astro-compiler/astro_oracle.mjs -tools <tools> -subject <dir> [-under <subpath>] -out <subject>.astro.json
go run ./cmd/indexbench lang-oracle -dialect astro -manifest calibration/oracles/astro-compiler/astro.scoring.json \
  -subject <dir> [-only <glob>] -tool-oracle <subject>.astro.json -out docs/bench/readings/lang/astro.json
ASTRO_TOOLS=<tools> node --test calibration/oracles/astro-compiler/
```

Run in the measuring harness only, never by fathom.
