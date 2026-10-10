# composer — provenance and divergences

LB-Q-composer, design `docs/design/language-breadth.md` §4, §13; ADR-062 (the Composer manifest as a tool
format). A doc-engine dialect (`decode: json`): no grammar, no queries, nothing pulled from upstream. This file
records the shape decisions and the oracle.

## What is read

- `ComposerPackage`: the manifest itself, named by `name` (a manifest with no name has no package entity;
  Composer calls it `__root__`), signed by `description`. Its `require` and `require-dev` keys are the
  `depends_on` and `dev_depends_on` references (the package-json and cargo dialects' shape): platform packages
  (`php`, `ext-*`, `lib-*`) are references too, as Composer lists them in `getRequires()`.
- `ComposerScript`: one task per key of `scripts`, signed by its first command. A command that begins `@name`
  is a `runs_script` reference to the script `name` the document declares (`declared_only`), Composer's own
  rule for a script reference (`EventDispatcher::isComposerScript`).
- Cross-file: a require names a manifest of the repository by package name (`module-source`, a monorepo's
  components); the autoload entries (`psr-4`, `psr-0`, `classmap`, `files`, in `autoload` and `autoload-dev`)
  are `repo-path` references from the manifest's directory. `[*]` reads a string and a list alike (a scalar is its
  own one item), so one selector serves a psr-4 prefix mapped to a directory or to several.

## Divergences, disclosed

- A script named `php`, `composer` or `putenv` is mistaken for the target of `@php ...`, `@composer ...` or
  `@putenv ...`, which Composer runs as commands: the pattern is `^@(\S+)` (RE2 has no lookahead) and
  `declared_only` is the only filter. Rare; `testdata/script-refs.json` pins the other cases.
- A script value written as an object (Symfony's `auto-scripts`, command to type) is one task with no
  signature; the strings inside it are scanned for references like any others.
- Replace, provide, conflict and suggest are not read.
- The autoload references are extracted and unmeasured (LB-Q-composer-rest).

## The oracle

`calibration/oracles/composer`: Composer 2.10.3 itself (tier A), with `composer validate`, `show --tree
--locked` and `dump-autoload` as cross-checks; see its README. A line is placed by a JSON scanner of the
oracle's own (Composer prints none).
