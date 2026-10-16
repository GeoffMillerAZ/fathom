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
never a reference. `cross_references` (the L3 unit, design §1.8 "Jenkins pipelines' L3"; wave lf8):
each reference whose callee names a `vars/<name>.groovy` of the subject that declares a script-level
`call`, to that file's `call` (the shared-library join LB-F375 makes), and each `load` step whose path
is a string constant (`load 'p'`, `load("p")`, `load(path: 'p')`), joined to the working directory
its enclosing `dir('x')` closures give (the workspace root outside them), to that file when the
subject holds it and the dialect claims it. A load joins nothing, counted by reason in the file's
`disclose` note, when its path is no constant or lies under a `dir()` whose path is none
(`jenkins_load_computed`), is absolute, leaves the subject or names no file (`jenkins_load_missing`),
names a file the dialect does not claim (`jenkins_load_unread`: a `.groovy` outside `vars/`), stands
in a `vars/` file (`jenkins_load_in_library`: it reads the calling job's workspace), or stands outside
every definition (`jenkins_load_outside_definition`). `lang-oracle` scores them at (file, file) against
the doc-xref pass's Jenkins edges (fusion's `JenkinsXrefsForBench`, langoracle_xref.go).

    # L3-only subject: the cross_references alone, tier C
    java -Xmx512m -cp <groovy>/groovy-4.0.33.jar JenkinsfileAstDump.java <subject> -xref-out out/<subject>-xref.json > /dev/null

The Jenkins declarative linter (ruling W4, tier A) attests a pipeline's syntax, not the files its
steps load or the library steps they call, so no tier-A oracle covers the L3 family and tier C caps L3
at good.

The oracle agrees with the dialect's five hand-written goldens entry for entry
(`internal/extractor/dialects/jenkinsfile/testdata`).
