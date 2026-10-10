# Jenkins pipelines (LB-Q-jenkinsfile) — reading, 2026-10-10

**Verdict.** Keep. fathom now reads a declarative Jenkinsfile the way the Groovy compiler does: every
pipeline section and stage, and every step each one calls, agree with the compiler's own AST on all
23 declarative pipelines of the calibration subjects (163 definitions, 133 references; level met L2,
grade good, because the oracle is tier C). Before this item a Jenkinsfile was read by nothing. A
pipeline's `sh`/`bat`/`powershell` steps join the Makefile targets, Taskfile tasks, package.json
scripts and scripts they run through the same token rule GitHub Actions run steps use (9 edges from
jenkinsci/docker's Jenkinsfile), and a shared library's step call joins its `vars/<name>.groovy`
(pipeline-library: precision 1, recall .60). It pairs with the GitHub Actions dialect (the same token
rule) and the Makefile, Taskfile and package.json dialects (the targets). It does not replace a Jenkins
controller: nothing evaluates Groovy, so a GString's `${...}` resolves nothing, a scripted pipeline is
tolerated, not claimed, and a library loaded from another repository is not followed.

`jenkinsfile.json` is the claim: `indexbench lang-oracle` at fathom `912051b9`, scoped by `-only` to
the declarative pipelines (the kill decision below), against `calibration/oracles/jenkinsfile`
(Groovy 4.0.33's AST at the CONVERSION phase plus a small walk, tier C).

## The kill decision

The brief's line: more than 5% of the corpus files with an ERROR or MISSING node under the pinned
groovy blob (murtaza64/tree-sitter-groovy 2a6ddd55, LB-F93) means declarative pipelines only, a
scripted pipeline disclosed as tolerated. Measured with the embedded blob itself:

| Files | With an error |
|---|---|
| Every file the dialect claims in the subjects (6 Jenkinsfiles, 11 `vars/*.groovy`) | 8 of 17 (47.1%) |
| pipeline-library `vars/*.groovy` | 6 of 11 (54.5%) |
| Jenkinsfiles (both errors in scripted pipelines: jenkinsci/jenkins, pipeline-examples msbuild) | 2 of 6 (33.3%) |
| Declarative pipelines (pipeline-library's Jenkinsfile, pipeline-examples' 22 examples) | 0 of 23 |
| Scripted pipeline scripts (Jenkinsfiles and pipeline-examples' `*.groovy`) | 6 of 32 (18.8%) |

The kill fires. The claim is declarative pipelines; scripted pipelines and `vars/` files are read and
measured below, never claimed.

## Subjects

| Subject | Commit | Licence | In the claim |
|---|---|---|---|
| jenkinsci/pipeline-examples | `fb9575a8` | MIT | its 22 declarative examples, stored as `declarative-examples/**/*.groovy` and renamed to `*.jenkinsfile` in the clone's working tree (the only change) |
| jenkins-infra/pipeline-library | `96587b48` | MIT (pom.xml) | its declarative `Jenkinsfile` |
| jenkinsci/jenkins | `cd2a077a` | MIT | none: its `Jenkinsfile` is scripted (tolerated) |
| jenkinsci/docker | `dec1d0a5` | MIT | none: counted for the shell-step token rule only |

## The claim (`jenkinsfile.json`)

| Metric | Value | Tier |
|---|---|---|
| sym_recall, sym_precision | 1, 1 (163 of 163) | C |
| kind and range agreement, signatures | 163 of 163 each | C |
| ref_recall, ref_precision | 1, 1 (133 of 133) | C |
| faults, files with an error | 0, 0 | |
| throughput (CPU) | 1.21 MB/s | |

Throughput on 17 KB of small files is load-dominated: five readings at load average 11 to 13 read
0.77 to 1.60 MB/s (median 1.21, the one committed; two of the five fell under 1 MB/s), and five earlier
ones 1.09 to 1.55. A steady-state loop over the same files (each extracted 20 times after a warm-up)
reads 1.40 MB/s against the gradle dialect's 1.44 on the same blob; the tolerated run below, 140 KB,
reads 2.37.

Ten of the 156 pipeline-examples definitions and 23 of its 127 references sat in a statement after a
comment, which the grammar reads as its parts side by side; the patterns read those shapes
(`comments.jenkinsfile`), and without them the reading was sym_recall .9387, ref_recall .8271.

## Tolerated, measured beside the claim

Every claimed file of the three subjects (39 files, 140 KB), same oracle and commit, not committed as a
reading:

| Subject | Definitions | References | Files with an error |
|---|---|---|---|
| jenkinsci/pipeline-examples (all 26: declarative and scripted) | 161 of 161 | 136 of 136 | 1 |
| jenkinsci/jenkins (scripted) | 4 of 4 | 39 of 39 | 1 |
| jenkins-infra/pipeline-library (Jenkinsfile and 11 `vars/`) | 80 of 102 | 287 of 385, precision .9696 | 6 |
| pooled | sym_recall .9176, precision 1 | ref_recall .825, precision .9809 | 8 |

All 22 missed definitions are in three `vars/` files the grammar cannot read (`infra.groovy` 7,
`terraform.groovy` 12, `updatecli.groovy` 3): Groovy constructs such as a command expression with named
arguments inside an expression (`pwd tmp: true`) leave an ERROR that swallows whole methods.

## Shell steps (LB-F374), counted on the corpus

From a scratch ingest of each subject at `912051b9` (the doc-xref pass's `jenkins:` line and the
store's `typed_edges` rows, `pkg_path = 'doc-xref'`):

| Subject | Literal shell steps | Edges | Unresolved |
|---|---|---|---|
| jenkinsci/docker | 16 | 9: 6 to Makefile targets (`hadolint`, `shellcheck`, `prepare-test`, `docker-init` from two stages, `publish`), 3 to `make.ps1`'s first function (`./make.ps1 build`) | 3 make targets written as GStrings (`build-${targetToBuild}`, `test-${...}`, `buildarch-${...}`), refused as plainWord refuses a shell variable |
| jenkinsci/jenkins | 10 | 0: `launchable`, `git` and `bash ath.sh`, a script that defines no function | 1 step outside every stage |
| jenkins-infra/pipeline-library | 1 | 0: `mvn` | |
| jenkinsci/pipeline-examples | 43 | 0: the rule reads `npm test`, `./dockerBuild.sh` and `./dockerPushToRepo.sh`, but an example's targets live in the project it illustrates and the repository holds no package.json or script; the rest run `echo`, `mvn`, `npm install`, `docker` and the like | 3 steps outside every stage |

Every edge was checked by hand against jenkinsci/docker's Makefile (each target declared) and `make.ps1` (`Test-CommandExists`, its first function: the rule's script target is `main`, else the first function). A `vars/` file's own shell steps
join nothing: a library step runs in the calling job's workspace, not the library's repository.

## Shared-library variables (LB-F375), measured on pipeline-library

`lang-oracle` scores no cross-file reference for a tags dialect without an import strategy, so the
oracle's `cross_references` (a reference whose callee names a `vars/<name>.groovy` declaring `call`)
are compared with the scratch ingest's `library_step` edges: **precision 1 (3 of 3), recall .60 (3 of
5)**, file granularity. The two misses are calls from `vars/terraform.groovy`, whose `call` and stages
the grammar loses to an ERROR. Within the in-repository scope only: a Jenkinsfile that loads a library
from another repository (`@Library('pipeline-library') _`, every plugin's `buildPlugin()`) joins nothing
until the library's repository is indexed alongside it (a ceiling).

## Build jobs (LB-F376)

`build job: 'x'` is recorded and counted unresolved by its reason (`job-undeclared`, `job-templated`):
no dialect declares an in-repository job to join it to. JCasC declares jobs only inside Job DSL script
strings, which the doc engine cannot read a name out of, so `jcasc` is deferred (LB-F376-rest). No
claimed file of the subjects holds a `build` step (pipeline-examples' two are in unclaimed `*.groovy`
scripts); the rule is proven on fixtures (`TestResolveJenkins_BuildJobsUnresolved`).

## Ceilings

- Scripted pipelines and `vars/` files: tolerated, never claimed (47.1% of claimed files in error).
- A declarative pipeline followed by another top-level statement: the grammar's `pipeline` rule must
  end the file; one recovery is matched (`trailing.jenkinsfile`), the other loses the trailing method.
- A pipeline stored under another name (`Jenkinsfile_k8s`, `Jenkinsfile.release`, `*.groovy`) is not
  claimed (pipeline-library's `Jenkinsfile_updatecli`, pipeline-examples' 49 scripts).
- Nothing evaluates Groovy: a step whose script is a variable, a GString word, a `dir(variable)`
  resolves nothing; a library from another repository is not followed.
