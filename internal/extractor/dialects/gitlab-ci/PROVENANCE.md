# gitlab-ci — provenance and divergences

LB-Q-gitlab-ci, design `docs/design/language-breadth.md` §4, §13. A doc-engine dialect
(no `tags.scm`, no upstream grammar, no incumbent): this file records what it models,
what it deliberately does not and why, and where the calibration oracles come from.

## What it models

- **Jobs** (`GitLabCIJob`, canonical `job`): a top-level key whose value is a map and that
  is neither hidden (a leading `.`) nor one of GitLab's reserved keywords (`default`,
  `include`, `stages`, `variables`, `workflow`, `spec`, and the deprecated global `image`,
  `services`, `cache`, `before_script`, `after_script`, `types`). That is GitLab's own rule:
  every other top-level key is a job. The signature is the job's `stage`, else a trigger's
  project or included file, else its first `extends`.
- **Hidden templates** (`GitLabCITemplate`, canonical `mixin`): a top-level `.name` map.
  `.pre` and `.post` are stage names, not keys a file declares, and a hidden key that is a
  list or scalar (a `!reference` target) is not a template. A template's calls and edges
  are read exactly as a job's.
- **References** (`Calls`, both kinds): `needs` (a name, or a `{job: name}` map),
  `dependencies`, and `extends` (one name or a list). Every one is a name, resolved by the
  store's ordinary call resolution wherever the entity lives.
- **Cross-file references**, `service-name` strategy: the same three edges, entity to
  entity, when the name is declared in exactly one other file of the repository
  (`fusion.ResolveDocXrefs`). A name declared in two other files, in none (a remote
  template or component job), or only in the referring file resolves to nothing.
  `include:` (a path or a `local:` map) and `trigger:include` are recorded as `repo-path`
  references (`DocResult.CrossFile`, goldens `pipeline.yml`) and do **not** resolve: see
  the deferral below.
- Multi-document files are walked document by document (a CI/CD component's `spec:` header
  is its first document); an unparsable file yields nothing, as every doc dialect does.

## What it deliberately does not model

- **Stages are not entities.** `SymbolKey` carries no kind, so a stage `test` and a job
  `test` would share one symbol key and one of the two would vanish from the store (the
  reason docker-compose emits no volumes).
- **Merge keys (`<<: *anchor`) are not applied** by the doc decoder (its PROVENANCE has the
  same note): a job that inherits `needs`, `extends` or `stage` through a merge loses them.
  The three subjects carry 73 merge keys. A key written twice in one map reads as its last
  value (as GitLab's loader does), so `extends` written twice keeps the second.
- **`needs` inherited through `extends`** is not modelled: the doc engine does not merge a
  template into the job that extends it. The tool evaluates 22 needs pairs over the three
  subjects (gitaly 2, the development kit 12, gitlab-pages 8) that the job never writes; the
  tier-C truth is what each job writes.
- **`!reference [.tmpl, script]`** is a real dependency on a template and is not read (the
  engine selects by key, not by YAML tag). A `needs:` whose item is `{project:…}` or
  `{pipeline:…}` reads as a local name.
- **Interpolation is not evaluated.** A job named `lint-$[[ inputs.x ]]`, a stage written
  `$[[ inputs.stage ]]` and a `needs:` written with a `parallel:matrix` expansion's name
  (`"helper images: [alpine alpine-pwsh]"`) are the text as written. The tool names a
  matrix job `base: [values]` and a `parallel: N` job `base 1/N`; the file declares `base`.
- **A job that ends in a multi-line block scalar** (`script: - |`, the common case) has an
  `end_line` at the scalar's first line: yaml.v3 gives a scalar no end line, and the doc
  decoder (`docdecode.go`, shared by every doc dialect) does not add the block's own. Ranges
  are not scored for tool formats (§1.2), so no reading shows it; it is the decoder's, not
  this spec's.
- **Ownership is by convention.** The manifest claims `.gitlab-ci.yml`/`.gitlab-ci.yaml` by
  basename and `.gitlab/ci/**/*.y*ml` and `**/*.gitlab-ci.y*ml` by glob. A pipeline file an
  `include:` reaches under any other name (`ci/build.yml`, `templates/go.yml`) is not
  claimed and stays with the kubernetes dialect's extension claim; the doc engine has no
  content-based ownership for an arbitrarily named fragment (compose has the same gap).

## Deferred: include:local and trigger:include edges (LB-Q-gitlab-ci-rest)

The queue entry expects `include:local` edges. They are recorded and unresolved, for two
reasons that are the doc engine's, not the spec's, and that a worker does not extend
(design §13.2 step 13, `needs-strategy`):

1. `fusion.ResolveDocXrefs` joins a `repo-path` reference to the target file's *root entity*
   (an entity selected at `$`). A pipeline file has no scalar that is always present to name
   one (`workflow:name` is optional) and the engine has no file-name value expression, the
   same gap that leaves docker-compose at L2 and GitHub Actions' unnamed workflows out.
2. GitLab resolves `local:` from the repository root whatever directory the including file
   is in. `repo-path` resolves a bare path from the including file's directory (a leading
   `/` is the root) and `repo-root` only reads a `./` prefix, so a bare
   `local: .gitlab/ci/x.yml` in a nested include names the wrong file. From the root
   `.gitlab-ci.yml` (where every subject declares its includes) the two agree.

The dialect's L3 is therefore the job-to-job and job-to-template edges that cross a file,
not the file-to-file `include` edges. Both would start resolving, with no dialect change,
once a pipeline file has a root entity and a repository-root local-path strategy exists.

## The calibration oracles

Two oracle files per subject, written by `calibration/oracles/gitlab-ci` (its own Go
module, never a product dependency; `calibration/oracles/README.md` has the invocation).

**Tier A (definitions):** `gitlab-ci-local --list-json` (4.75.1, npm), run once per pipeline
root. The tool loads the root and every include it can reach — local files from the
checkout, and the remote templates, components and projects a pipeline names, which it
downloads from gitlab.com and caches in a state directory outside the subject — merges them
and lists the jobs, matrix and parallel jobs expanded. Its JSON has no file and no line, so
a yaml.v3 walk supplies them: a pipeline's files are the root plus the local includes a
walk of `include:` reaches, a job's file is the file that declares it, its line the key's
line, and a tool name is folded back to the declared one. A job a remote include adds is
dropped (no file declares it); a declared job the tool does not list stays out of the truth
and counts against `sym_precision`. Hidden templates are not jobs, so the tool never lists
them: the dialect emits them under `mixin`, outside the oracle's kind intersection, and the
reading counts them under `excluded_by_kind` (61). **The tool is not offline**, as the queue
entry says: it fetches remote includes (and a 429 from gitlab.com rejects a pipeline until
the wrapper's retry succeeds); what it downloads only adds jobs no local file declares.

**Tier C (references and cross-file references):** the tool merges a pipeline into one
job map and erases `extends`, so the references the dialect models come from a yaml.v3 walk
of the raw files, kept for the pipelines the tool accepted: every `needs`, `dependencies`
and `extends` a job or template writes. A cross-file reference is the subset whose target is
declared in exactly one other file of the same pipeline, the rule the resolver applies to a
`service-name` reference, so this oracle attests the implementation, not the rule (as the
helm oracle's join does). Every `needs` a job writes appears in the tool's evaluated needs
(`needs a job writes that the tool did not list: 0` on all three subjects), which is the
tier-A cross-check of that edge; `extends` and `dependencies` have no tool-evaluated form.

## Subjects

`gitlab-org/gitaly` v19.4.1 (MIT; one 955-line root pipeline, anchors and merge keys,
matrix jobs, trigger jobs), `gitlab-org/gitlab-development-kit` main @ e769382 (MIT, `.gitlab/`
and the root files only; a root `.gitlab-ci.yml` and fifteen local includes, the
multi-file shape) and `gitlab-org/gitlab-pages` v19.4.1 (MIT, `.gitlab/` and the root files;
a root and two includes). The queue's `gitlab-org/gitlab-runner` is substituted out: the
tool rejects its pipeline (a `needs:` naming a `parallel:matrix` expansion it cannot find),
so it would be a subject with no oracle. `omnibus-gitlab` (Apache-2.0) was tried as its
replacement and the tool rejects it too, on a `rules:` expression it cannot evaluate
(`null && "…" =~ null`). Both rejections are the tool's, disclosed here; fathom extracts both
pipelines.

`gitlab-org/charts/gitlab` was not used: its `LICENSE.md` is GitLab's mixed CE/EE text
("With regard to the GitLab Software"), not a plain allowlisted licence (§8.2).
