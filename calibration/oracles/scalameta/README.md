# ScalametaDump — tier-B Scala L1/L2 oracle (syntax only)

Neither oracle the queue named for Scala can grade L1. universal-ctags 6.2.1 has no Scala parser at all
(`ctags --list-languages` lists none), and scip-java reads Scala through SemanticDB, which needs each subject
compiled by its own sbt build (sbt is not installed here, and a cross-built typelevel/cats or http4s/http4s build is a
heavy JVM run); a SemanticDB index is the L3/L4 oracle in any case (LB-Q-scala-rest). ScalametaDump parses every
`.scala` and `.sc` file with scalameta, the Scala parser scalafmt and Metals use (BSD-3-Clause, no tree-sitter code,
no resolution), run as a separate process, and writes the definitions and call pairs the scala dialect declares. Tier
B on the D2 precedent (JavaParser for Java, the Kotlin PSI, luaparse): an independent syntactic parser, so L1 can be
excellent and L2 at most good. Bench-only: a JVM program, never a product dependency.

    coursier fetch org.scalameta:scalameta_2.13:4.17.4      # into COURSIER_CACHE; the jars and their sha256:
    #   scalameta_2.13-4.17.4.jar      c496685cdce1778b766d7bd120721995c6362eb21908dcf59fee038df5da3304
    #   parsers_2.13-4.17.4.jar        e9cb0563567563e9736cbde50e6e8887b4d3d131174b5d5c2ab094152b9f14f7
    #   trees_2.13-4.17.4.jar          c573aacf0c77babf3717b47209e9c6a0c37935c4b022609045705698fe62fc44
    #   trees2_2.13-4.17.4.jar         78609de08420be3a7a55708e40095d48a380b4ec074b5ee86f94a6c737f4ff0e
    #   common_2.13-4.17.4.jar         e8acfb37a2b538d083d649a91bd31db9d62c699890204f7d2310effa05ea190c
    #   common2_2.13-4.17.4.jar        07ce41f35fcdb910c9c846ee65313993702138ab9653010bf49f28945bf3fe9a
    #   io_2.13-4.17.4.jar             c0b4bb7b99f11b3b34bee658624fbea839da7f94310d2b2075dbbf4d1283594e
    #   scala-library-2.13.15.jar      8e4dbc3becf70d59c787118f6ad06fab6790136a0699cd6412bc9da3d336944e
    #   sourcecode_2.13-0.4.4.jar      bd4e99aef8267a410b6ed716c487cf5256f801425f158a8c9cbd056eb032d80d
    #   portable-scala-reflect_2.13-1.1.3.jar 920f62979293069cf721865f931e42f9f7b0b2720ee9f6a9ddff76a19ecf8d4e
    CP="$(coursier fetch org.scalameta:scalameta_2.13:4.17.4 | tr '\n' ':')"
    java -Xmx512m -cp "$CP" ScalametaDump.java -calls <clone>/cats   > out/scala-cats.json
    java -Xmx512m -cp "$CP" ScalametaDump.java -calls <clone>/http4s > out/scala-http4s.json
    # typelevel/cats v2.13.0 (32a50dcf): 835 files, 15,027 definitions, 10,094 call pairs
    # http4s/http4s v0.23.38 (24f8d91f): 667 files, 11,192 definitions, 8,320 call pairs; 0 parse failures on both

JDK: Homebrew OpenJDK 27, no install beyond it; peak RSS about 450 MB at `-Xmx512m`. The dump prints `files`, the
dialect each file parsed with, `parse_failures` (each named) and the counts on stderr. scalameta has no error
recovery, so a file no dialect parses contributes nothing; a file under a `scala-3` source directory is tried as
Scala 3 first, any other as Scala 2.13 with `-Xsource:3` first, each falling back to the other and then to 2.12
(cats: 831 + 4, http4s: 660 + 7). Manifest kinds: `dialects/scala/dialect.json` `oracle_kinds.tool`.

What it counts, and does not. The parser is independent; the scope is the dialect author's rule, stated here so a
reader can disagree with it:

- Definitions: a class or case class (`class`), `trait`, an object or package object (`object`), `enum`, an enum
  case (`enumCase`, each name of `case A, B`), every def anywhere (`def`: a member, an abstract def, a macro def, an
  extension method, a top-level def, a local def in a body), a secondary constructor (`constructor`, named `this`), a
  named given (`given`), a type alias, abstract type member or opaque type (`type`), and a `val` / `var` that is a
  member: its parent is a template (a class, object, trait, anonymous class or `new X { }`), an enum body, a package
  or the file, never a block or a body; one per `Pat.Var` (`val a, b = 1` is two, `val (a, b) = ...` none). A class
  parameter is a field (`val`/`var`) when written so, and every parameter of a case class or of an enum case with
  parameters is a `val`, in every parameter list.
- `line` is the name's row, `end` the definition's last row (a class parameter's own). A name is spelt as the source
  writes it, without backticks (scalameta's own `value` unescapes a backticked `` `\n` `` to a newline), cut to its
  last segment as the harness's BareName cuts (after `.`, `:`, `#`, `/`, `\`, space or `)`), so an operator such as
  `::` or `+:` is `""` on both sides.
- Calls (`-calls`): every `Term.Apply` whose function is a name, a selection or either with type arguments, the
  callee its last name, paired with the innermost enclosing `Defn.Def`, `Defn.Macro` or `Ctor.Secondary`. A call in a
  val initialiser, a class or object body, a given alias or a script's top level belongs to no callable and is no
  pair (fathom's `SymbolNode.Calls` follows the same model). An infix (`xs map f`), `new X(..)`, a pattern, an
  annotation's or a supertype's arguments and a parameterless selection are no `Term.Apply`; a call whose function
  is itself a call (`f(a)(b)`) names nothing, its inner call does.

Imports (`-imports`; LB-F439, K19: the Scala L3 truth, committed before any L3 reading): the dump also writes
`imports`, kotlin-psi's shape (`{path, line, to_path, module}`, tier B), joined by declaration, never by resolution:

- every `Import` anywhere in a file, one entry per importee: a named one (`p.C`, `p.C.member`, `{C => D}`, Scala 3's
  `C as D`) takes the file(s) of package `p` that declare a top-level `C`, the longest package prefix first; a wildcard
  (`p._`, `p.*`, `p.given`, `{given T}`) every file of package `p`, or, when `p` is no package of the subject, the
  file(s) declaring its last segment in its parent (an object's members); an unimport (`{C => _}`) is no entry;
- a path is relative as Scala reads it: its first segment names a member (a package or a top-level declaration) of the
  innermost package around the directive that has one, else the root's (`package cats; package syntax; import
  data.NonEmptyList` is cats.data.NonEmptyList);
- a file's package is the full name of the innermost package clause holding a declaration (`package cats` then
  `package data` is cats.data); a package object's members are top-level declarations of the package it names, and
  its file a file of that package; a directive naming no subject file (a library) is one entry with `to_path` "".

Same-package references, marked `same_package: true` as kotlin-psi marks them (LB-F433's pairing: file-edge truth
only, never an import statement): a name written with no qualifier (a term, a type, a callee, an `extends` type)
joins the other files declaring it at top level in the innermost package around it that declares it (chained clauses
make every enclosing package visible, `package a.b` a.b alone); a selection or an infix operator joins only a
top-level Scala 3 extension method. Scala's precedence where the syntax shows it: a name the file declares at its own
top level, imports by name or renames to, finds in a wildcard import of a subject package, or declares anywhere inside
a definition (a member, a local, a parameter, a pattern variable) joins nothing; names in an import or a package
clause, a named argument's name and a definition's own name are no references.

    java -Xmx1g -cp "$CP" ScalametaDump.java -calls -imports <clone>/cats   > out/scala-cats.json
    # cats: 10,077 import entries (9,003 to a subject file), 2,547 same-package pairs;
    # http4s: 19,572 (16,691), 1,908; definitions and calls byte-identical to the run without -imports

`indexbench lang-oracle -dialect scala -subject <clone> -tool-oracle out/scala-<subject>.json` reads, 2026-10-10:
sym_recall 1, sym_precision 1 (26,218 of 26,219 definitions matched), kind_agreement 1, signature_coverage 1,
range_agreement .8042, call_recall .9999, call_precision .9999 (`docs/bench/readings/lang/scala.json`). The range
gap is the engine's, not the parser's (O761). Cross-check, not scored: the grammar's own upstream `tags.scm`
(`calibration/oracles/ts-tags/scala-tags.scm`, tier C) reads call_recall .9887 but call_precision .3896, because its
only call pattern is a plain `f(x)`: a member call (`xs.map(f)`) is in no truth it writes.
