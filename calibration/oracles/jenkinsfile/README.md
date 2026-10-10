# JenkinsfileAstDump — the jenkinsfile dialect's tier-C oracle

LB-Q-jenkinsfile (wave c3, item jenkins). Universal-ctags has no Groovy parser and no tool evaluates
a Jenkinsfile offline (a Jenkins controller would, with every plugin its steps name), so the oracle is
the Groovy compiler itself: `org.apache.groovy:groovy` 4.0.33 (Apache-2.0, the POM's licence and the
jar's `META-INF/LICENSE`; sha256 `5240a9151289115e4d447d6b6d4cb9ac8a9fc6374e079714dfeeffda42323be8`,
Maven Central), stopping at the CONVERSION phase: the AST as written, nothing resolved, so a Jenkins
step, a shared library or a plugin class need not exist. A small walk (`JenkinsfileAstDump.java`)
then names the dialect's definitions and calls. **Tier C**: the parser is Groovy's own, but the walk,
not the compiler, says what a stage or a section is, so a reading it grades is capped at good.

Bench-only: a JVM program run as a separate process, never a product dependency.

    # groovy-4.0.33.jar from Maven Central in <groovy>; JDK: Homebrew OpenJDK 27, no install
    java -Xmx512m -cp <groovy>/groovy-4.0.33.jar JenkinsfileAstDump.java <subject> > out/<subject>.json

It reads the files the dialect's manifest claims (the basename `Jenkinsfile`, the extension
`.jenkinsfile`, `vars/<name>.groovy`) and prints the neutral oracle JSON
`indexbench lang-oracle -tool-oracle` reads, plus the counts on stderr (`files`, `refused`,
`definitions`, `references`, `cross_references`). A file the compiler refuses contributes nothing and
is counted; none of the calibration files was refused.

What the walk names (kinds are the dialect's canonical kinds, `oracle_kinds.tool`):

| Kind | What |
|---|---|
| job | a call `pipeline { }` with one closure argument, named `pipeline` |
| stage | `stage('name') { }` (a string and a closure), named by the string as written, one pair of quotes off |
| section, step, variable_decl, resource | `stages`, `post`, `parameters`, `options` (section), `steps` (step), `environment` (variable_decl) with one closure argument; `agent` (resource) with a closure, a name or a string (`agent any`); named by the keyword, qualified by the stage that immediately encloses it (`Build.steps`) |
| function | a script-level method (`def name(...) { }`) |
| import | each library named by `@Library(...)` on a declaration or an import, by `library 'a'` or `library identifier: 'a'` |

`references`: every implicit-this method call (a bare name: `sh '...'`, `echo(...)`, `helper()`),
paired with the innermost definition enclosing it and skipped when none does; a section keyword is
never a reference. `cross_references`: each reference whose callee names a `vars/<name>.groovy` of the
subject that declares a script-level `call`, to that file's `call` (the shared-library join LB-F375
makes); `lang-oracle` does not score it for a tags dialect without an import strategy, so the
reading's markdown scores it against a scratch ingest's doc-xref edges.

The oracle agrees with the dialect's five hand-written goldens entry for entry
(`internal/extractor/dialects/jenkinsfile/testdata`).
