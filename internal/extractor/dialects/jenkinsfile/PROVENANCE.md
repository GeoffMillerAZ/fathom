# jenkinsfile — provenance and divergences

LB-Q-jenkinsfile, design `docs/design/language-breadth.md` §3, §13; wave c3 (item jenkins), tier T2.
A tags-engine dialect on the Groovy grammar the Gradle dialect already reads (LB-F93); no incumbent:
a Jenkinsfile fell through to no extractor before it.

- Grammar: `murtaza64/tree-sitter-groovy` at commit `2a6ddd558b6aa39c5b77d8db9fe9baf817486b2c`
  (MIT, top-level `LICENSE` read at the pin; LB-F93 pinned and built it, see
  `internal/extractor/tsengine/grammars/PROVENANCE.md` and `../gradle/PROVENANCE.md`). No second
  Groovy grammar is pinned here.
- `tags.scm`: written from scratch in this repository (design §13 step 11), never generated or
  copied. The grammar's `queries/` hold no `tags.scm`; `highlights.scm` was read for node vocabulary
  only; aider and nvim-treesitter carry no Jenkinsfile query. Node and field shapes came from
  `indexbench lang-probe` over `testdata/*` and the calibration subjects. The file's header states what
  is and is not a definition and a reference.
- `testdata/`: fixtures written in this repository (none copied from a subject); the goldens were
  written by hand from reading each fixture before the queries ran (red first), and the Groovy
  compiler's AST (the oracle below) agrees with all five entry for entry. `trailing.jenkinsfile` pins
  the grammar's recovery below; `comments.jenkinsfile` was added red first for the shapes the subjects
  showed after a comment.

## How a file is claimed

By the manifest alone: the basename `Jenkinsfile`, the extension `.jenkinsfile`, and the glob
`**/vars/*.groovy`, a shared library's global variables (jenkins-infra/pipeline-library's layout). The
glob is the only way a `.groovy` file is claimed: `internal/fusion/register.go` sends the glob's
extension to the classifier (`connector.ClassifyDialect`), which claims a file only when the glob
matches, so `src/org/acme/Util.groovy`, a `Jenkinsfile.groovy` or a `vars/sub/x.groovy` stays
unclaimed and `.groovy` stays disclosed as unsupported (LB-Q-groovy-rest). Not claimed: a pipeline
script stored under another name (`Jenkinsfile_k8s`, `Jenkinsfile.release`, `pipeline.groovy`; the
subjects hold `pipeline-library/Jenkinsfile_updatecli` and pipeline-examples' 49 `*.groovy` scripts).

## The kill decision: declarative pipelines only

The brief's kill line: more than 5% of the corpus files with an ERROR or MISSING node under the pinned
blob, and the dialect claims declarative pipelines only, a scripted pipeline tolerated. Measured
2026-10-10 with the embedded blob itself (`indexbench`'s `filesWithError` over the gradle dialect's
grammar target, the same blob), on the queue's subjects at the pinned SHAs:

| Files | With an error | Rate |
|---|---|---|
| Every claimed file (6 Jenkinsfiles, 11 `vars/*.groovy`) | 8 of 17 | 47.1% |
| Jenkinsfiles | 2 of 6 (jenkinsci/jenkins, pipeline-examples msbuild: both scripted) | 33.3% |
| pipeline-library `vars/*.groovy` | 6 of 11 | 54.5% |
| Declarative pipelines (pipeline-library's Jenkinsfile and pipeline-examples' 22 declarative examples) | 0 of 23 | 0% |
| Scripted pipeline scripts (pipeline-examples' `*.groovy` and Jenkinsfiles, jenkinsci/jenkins) | 6 of 32 | 18.8% |

The kill fires (47.1%): the level the reading claims is for declarative pipelines, the files a
`pipeline { }` block organises (the reading is scoped to them, its `only` globs named). A scripted
pipeline (`node { stage('x') { } }`) and a shared-library variable are read for what the grammar
parses and are **tolerated**: their stages, functions and calls are indexed, measured beside the
claim and never part of it. The errors are Groovy the grammar does not read: multiple assignment
(`def (platform, jdk) = ...`, jenkinsci/jenkins), a command expression with named arguments inside an
expression (`pwd tmp: true`), a `tool` keyword reading, a trailing-comma list, a closure-call chain.

## What it models

- `pipeline` (JenkinsPipeline, job), `agent` (JenkinsAgent, resource), `stages` (JenkinsStages,
  section), `stage` (JenkinsStage, stage), `steps` (JenkinsSteps, step), `post` (JenkinsPost, section),
  `environment` (JenkinsEnvironment, variable_decl), `parameters` (JenkinsParameters, section),
  `options` (JenkinsOptions, section): a section with its closure (`agent` also bare: `agent any`). A
  stage is named by its string (declarative stage names are unique in a pipeline); a section by its
  keyword under the stage it belongs to (`Build.steps`), a pipeline-level one by the keyword alone.
- `function` (JenkinsFunction, function): a top-level method, so a vars file's `call` is the
  definition a step call runs (`internal/fusion/jenkinsrun.go`, LB-F375).
- `library` (JenkinsLibrary, import): each library loaded by `@Library(...)` or the `library` step,
  named as written (`ops@main`).
- Calls (L2): every bare-name call, attached to the smallest definition enclosing it; a call through
  a receiver and a section keyword are not calls.

## Grammar readings the patterns follow

- The grammar has a `pipeline` rule for a top-level `pipeline { }` that must end the file. A
  statement after it (a helper `def`, an `echo`) makes the parser recover one of two ways: the
  keyword and its block become an ERROR whose children are the block (matched: `trailing.jenkinsfile`),
  or the pipeline stays whole and the trailing method becomes an ERROR of bare identifiers, which
  declares nothing (a counted ceiling in the reading). A helper declared before the pipeline reads
  whole. A `pipeline { }` inside a method is an ordinary command call, matched as one.
- A command call (a call without parentheses) that follows a comment, on its own line or
  trailing the statement before, reads as its parts side by side: `options { }` as the identifier
  and the closure, `sh 'x'` as the identifier and the string, and `agent any  // c` followed by
  `steps { }` as the identifier `agent`, a command call `any steps` and the closure. The patterns read
  each shape as the definition or call it is, inside a block only (a map entry or an assignment is
  never one), the closure as the definition's end (`comments.jenkinsfile`). On pipeline-examples'
  declarative examples these shapes held 10 of the 156 definitions and 23 of the 127 references.
- `@Library('x') _` reads as a declaration carrying the annotation; the `_` is dropped.

## Measurement

`docs/bench/readings/lang/jenkinsfile.json` against the Groovy compiler's own AST
(`calibration/oracles/jenkinsfile`, Groovy 4.0.x at the CONVERSION phase plus a small walk that
names the same definitions and calls; tier C, because the walk, not the compiler, says what a stage
or a section is). The oracle is bench-only: never a product dependency.
