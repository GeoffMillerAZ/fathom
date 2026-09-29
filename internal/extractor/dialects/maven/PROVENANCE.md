# maven — provenance and divergences

LB-Q-maven, design `docs/design/language-breadth.md` §4, §13. A doc-engine dialect
(no `tags.scm`, no upstream grammar, no incumbent: `pom.xml` had no extractor): this file
records what it models, what it deliberately does not and why, and where the calibration
oracles come from. The decode is `encoding/xml` (the doc engine's `xml` decoder: an element
with neither attributes nor children is a scalar, a repeated sibling becomes a list, a
single one reads as a one-item list).

## What it models

- **The artifact** (`MavenArtifact`, canonical `package`): the `<project>` root, named by its
  `artifactId` alone (a file without one is no project: `require_keys`), at the start tag's
  line, signed by its `groupId`, else its parent's. The name is the artifactId, not
  `groupId:artifactId`, because the value expressions the engine has cannot join two fields
  and a dependency, a parent and a module reference are each one element (see below).
- **Build plugins** (`MavenPlugin`, canonical `resource`): each `<build><plugins><plugin>`,
  named by its `artifactId`, signed by its `version` when written. Only `build/plugins`:
  `pluginManagement` and a profile's plugins are left out, since a plugin managed in one place
  and activated in the other is one artifactId twice in a file, and `SymbolKey` carries no
  kind (the same collision that keeps GitLab CI's stages and Compose's volumes out). The
  plugins are **not scored**: the L1 oracle file carries one tier and the tool lists
  evaluated (inherited and lifecycle-default) plugins, not a file's own. What is checked is
  the agreement of the dialect with an independent decode: 77 `build/plugins` entries over
  the two subjects (28 and 49), every one also listed by the effective POM, none repeating
  an artifactId within its file (`maven-oracle` prints the counts; the reading counts the
  entities under `excluded_by_kind`, 77).
- **References** (`Calls` on the artifact): the parent's `artifactId` and the `artifactId`
  of every direct `<dependencies><dependency>`. Every one is a name, resolved by the store's
  ordinary call resolution wherever the entity lives; a dependency on a third party resolves
  to nothing, as an unresolvable call does.
- **Cross-file references** (`fusion.ResolveDocXrefs`, file-tier edges entity to entity):
  - `module` (`repo-path`): each `<module>` (the project's own `<modules>` and every
    profile's) to the POM in the named directory, or the POM file the module names,
    relative to the referring POM, resolved to that file's artifact. A module whose path
    climbs out of the repository, or that is written with an unresolved property
    (`${x}`), resolves to nothing.
  - `parent` (`module-source`): the parent's artifactId to the one other POM of the
    repository declaring that artifactId.
  - `depends_on` (`module-source`): each direct dependency's artifactId to the one other POM
    declaring it (a dependency on another module of the reactor).

## What it deliberately does not model

- **Coordinates.** Names are artifactIds. Two POMs of one repository that declare the same
  artifactId make every reference to it ambiguous, and the resolver drops an ambiguous name
  rather than guess; a dependency that shares an artifactId with a reactor module but not its
  group would resolve to the wrong POM. The oracle joins by groupId:artifactId, so both would
  show as xref misses; neither happens on the subjects (`sharing a reactor artifactId with
  another group: 0`). A doc-engine value expression that joins fields would lift it.
- **Interpolation and inheritance.** `${project.version}`, `${revision}` and every property
  are text as written; a dependency, plugin or property a child inherits from its parent is
  in the parent's file only; `<dependencyManagement>` and BOM imports (`scope` import) are
  policy, not references, and are not read; `<pluginManagement>`, a profile's dependencies
  and plugins, `<extensions>`, `<reporting>` and `<properties>` are not entities. `<relativePath>`
  (and its `../pom.xml` default) is not consulted: a parent resolves by artifactId.
- **`<subprojects>`** (model version 4.1.0's spelling of `<modules>`) is not read.
- **Which profiles are active** is the tool's knowledge, not the file's: the dialect reads
  every profile's `<modules>` (a module edge is declared whether or not the profile runs),
  and the harness scores the edges to the POMs of the reactor the tool built (dubbo's
  distribution modules sit in an inactive `release` profile, so the tool run activates it,
  `-P release`).
- **Ownership is the basename `pom.xml`.** `pom-shaded.xml`, `dependency-reduced-pom.xml`,
  `.flattened-pom.xml`, `*.pom` and polyglot POMs (`pom.yaml`, `pom.groovy`) are not claimed.
- **Fixture POMs are indexed like any POM, and the reading does not measure them.** A
  repository keeps POMs that are no part of its build (test projects under `src/test`,
  invoker integration tests under `src/it`, archetype resources). `apache/maven` has 381
  `pom.xml`, 15 in its reactor and 366 fixtures; the connector reads all of them
  (`testdata` is the only fixture directory it skips). The tool builds a reactor, so its
  truth is the 15, and the reading measures `apache/maven` with `-only 'pom.xml,*/pom.xml'`
  (the root and its module directories). With every `pom.xml` in scope the same run reads
  `sym_precision` 0.0397 (378 dialect artifacts against 15 oracle ones), `xref_precision`
  0.4343 and `xref_recall` 0.6515, `level_met` none: the fixtures are real POMs the oracle
  cannot attest, and a fixture that shares an artifactId with a module makes its references
  ambiguous. `apache/dubbo` has no fixtures (all 116 `pom.xml` are in the reactor once its
  `release` profile is on) and is measured whole. The restriction is disclosed in the
  reading's notes and in the deferred row `LB-Q-maven-rest`.
- **A module directory that also holds another doc-engine file** (a `package.json` beside the
  `pom.xml` of a frontend module) is dropped by the resolver, which joins a directory to its
  one doc file (`fileTargets`, `len(fs) == 1`); the harness resolves over the dialect's own
  files only and so does not see it.
- **Ranges are not scored for tool formats** (§1.2): the artifact's end line is its
  `</project>` line, a plugin's its `</plugin>` line.

## The calibration oracles

Two oracle files per subject, written by `calibration/oracles/maven` (its own Go module,
never a product dependency; `calibration/oracles/README.md` has the invocation).

**Tier A (definitions):** `mvn help:effective-pom` (Apache Maven 3.9.11, maven-help-plugin
3.5.2, Temurin JDK 21), run once at each subject's root POM, offline (`-o`, against a local
repository the first online run warmed) with a local repository outside the subject. The
tool evaluates the whole reactor (module discovery, parent chain, property interpolation,
profile activation) and writes each project's coordinates. It writes no file and no line, so
an `encoding/xml` walk supplies them: the reactor's files are the root POM and the module
directories (or POM files, or profile modules) a walk of `<modules>` reaches, a project's
file is the one declaring the tool's artifactId, its line the `<project>` start tag's. The two
sets must be one: walk-only and tool-only project counts are printed and read 0 and 0 (dubbo
with `-P release`; without it the tool builds 113 of the 116 POMs, and a walk of the top-level
`<modules>` alone reaches only 106 of those 113, which is why the dialect and the walk read
profile modules too).

**Tier C (references and cross-file references):** the effective POM merges inheritance, so
what a file itself declares comes from an `encoding/xml` struct decode of the raw POM (an
independent implementation of the walk the dialect makes over the doc engine's token tree):
the parent's artifactId and each direct dependency's. Every declared name is cross-checked
against the effective POM (`declared-not-effective 0`, `parent-mismatch 0` on both subjects).
A cross-file reference is an edge to another project of the reactor: a `<module>` to the POM
Maven's module rule names, a `<parent>` to the project whose effective coordinates equal the
effective parent's, a declared dependency to the project whose effective groupId:artifactId
equals the effective dependency's (both coordinates, tool-evaluated, so a same-named artifact
of another group is not an edge). The join rule differs from the dialect's (artifactId), so the
xref metrics do measure the name-only shortcut; the declared-name half is the resolver's own
reading of the file, so this oracle attests the implementation there. Tier C caps L2 and L3
at good.

## Subjects

`apache/maven` maven-3.9.16 (Apache-2.0; 15 reactor POMs, 174 references, 66 cross-file
edges: 14 modules, 14 parents, 38 dependencies) and `apache/dubbo` dubbo-3.3.6 (Apache-2.0;
116 POMs, 1,061 references, 776 cross-file edges: 115 modules, 114 parents, 547
dependencies; a three-level aggregator tree, BOM modules, `${revision}` versions).
