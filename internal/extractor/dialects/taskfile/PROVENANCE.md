# taskfile — provenance and divergences

LB-Q-taskfile, design `docs/design/language-breadth.md` §4, §13. A doc-engine dialect (no
`tags.scm`, no upstream grammar, no incumbent: a `Taskfile.yml` was unclassified YAML, and
was skipped as such): this file records what it models, what it deliberately does not and why,
and where the calibration oracle comes from. The decode is `gopkg.in/yaml.v3` through the doc
engine. The manifest claims a file by basename, at any depth: `Taskfile.yml`, `Taskfile.yaml`,
`taskfile.yml`, `taskfile.yaml`, `Taskfile.dist.yml`, `Taskfile.dist.yaml`, `taskfile.dist.yml`
and `taskfile.dist.yaml` — the six the queue entry named and the two lowercase `.dist` spellings
go-task's own search order also reads. An included file under another name (`Taskfile.docker.yml`,
`tasks/build.yml`, `Taskfile`) is not claimed: the name is what says it is a Taskfile, and no glob
can tell the arbitrary ones from other YAML.

## What it models

- **The Taskfile** (`Taskfile`, canonical `package`): one entity per file, named by the file
  (`$file`, so `Taskfile.yml` and `Taskfile.dist.yaml` read as written), at the file's first key,
  signed by its `version`. It is the entity an `includes:` reference resolves to (a Taskfile that
  declares no tasks, only includes, is still one entity).
- **Tasks** (`TaskfileTask`, canonical `task`): every key of `tasks:`, named exactly as written,
  at its key's line. A colon in the key is part of the name (`generate:mocks` is a task, not a
  namespace), a wildcard task keeps its `*` (`release:*`), a quoted key loses its quotes. The
  three shapes go-task accepts are all read: a mapping, a string (`clean: rm -rf bin`) and a list
  of commands (`ci: [lint, test]`); a task written with no value (`todo:`) is kept
  (`include_null`). Signed by `desc`, else `summary`. An `internal: true` task is a task (a
  `task:` call may name it) and is emitted.
- **References** (`Calls` on the task, L2, intra-file): the task names in `deps:` (a string, or a
  mapping with `task:` and `vars:`, or a `for:` loop with `task:`), in `cmds:` entries with
  `task:`, in a `defer:` that calls a task, and the same in a task written as a list of commands.
  A name is recorded as written (`docs:build`, `{{.X}}`): the dialect does not resolve a
  namespaced call into the included Taskfile's task.
- **Includes** (`fusion.ResolveDocXrefs`, file-tier edges Taskfile to Taskfile): `includes:` entries,
  written as a string or under `taskfile:`, resolve by `repo-path` from the including file's
  directory to the included file's Taskfile entity; a directory resolves to the one doc file in it
  (design §4). A remote include (`https://...`), one written with a template
  (`./{{.ENV}}/Taskfile.yml`), one that names nothing the repository holds, one that names its
  own file and one that is `optional: true` and absent resolve to nothing.

## What is not modelled, and why

- **A task's `label:`, aliases and namespace calls.** go-task lists a task with a `label:` under
  the label, and a task can be called by an alias; the dialect names a task by its key, which is
  what a `deps` or `task:` entry writes and what `task <name>` runs (11 of runtime-radar's tasks
  carry a label that is not their key, and the oracle names them by key too). A call through a
  namespace (`task: docs:build`) or an alias is a name that matches no local entity.
- **`vars`, `env`, `dotenv`, `for` loops' values, `preconditions`, `requires`, `platforms`,
  `sources`, `generates`, `status`, `run` and the `silent`/`interval`/`method` options** are not
  entities or references: none names another task or file the dialect draws an edge to.
- **An include of the repository root** (`taskfile: ../`, `.`) names the directory `.`, which
  `repo-path` does not resolve to a file; write the file (`../Taskfile.yml`) and it resolves.
- **YAML merge keys** (`<<: *anchor`) are not applied by the doc decoder; aliases are followed.
- **A directory resolves to the one doc file in it (design §4), and this is the one shape the
  reading scores that an ingest does not always store.** The harness resolves over the files the
  dialect owns; an ingest resolves over every file a doc-engine extractor reads, so
  `taskfile: ./website` in go-task/task, a directory that holds a Taskfile beside a
  `package.json`, resolves in the bench and is dropped at ingest. A file path always resolves. On
  the subjects an ingest stores 13 of the 14 edges the reading scores (runtime-radar's 13 file
  includes, none of go-task/task's one directory include): `xref_recall` .9286 ingest-side,
  still over the L3 excellent line (.80), and precision 1.
  `TestTaskfileDirectoryNeedsTheOneFileRule` pins the rule and fails when it changes; the trigger
  is `LB-Q-taskfile-rest`, and `internal/fusion/docxref.go` was not this item's to edit.

## The calibration oracle

Tier A for definitions and cross-file references, tier C for references: `task --list-all --json
--taskfile <file>` (go-task 3.52.0), run by `calibration/oracles/taskfile/` on every Taskfile of a
scratch copy of the subject as the root of its own run.

- **Definitions** are the tool's: a `package` per Taskfile it accepts and a `task` per task it
  lists whose location is that file, at the tool's line, named by the key at the tool's line and
  column (the tool lists a task under its label, and an included task under its namespace:
  `website:build` is `build` in `website/Taskfile.yml`). `--list-all` leaves an `internal: true`
  task out, so the run is repeated on a copy with that line blanked (no other line moves) and
  the tool lists it; an internal task is a task the tool attests.
- **Cross-file references** are the tool's evidence for `includes:` entries: an include is in
  the truth when the file it names (or the Taskfile its directory holds, by go-task's own search
  order) contributes a task to the tool's listing. A file with no tasks and a templated
  include are counted on stderr and left out; a Taskfile with a remote include is rejected by
  the tool as a whole (remote Taskfiles are an experiment, off by default) and excluded, and none
  of the subjects has one.
- **References** (tier C): go-task lists nothing of what a task depends on, and `--summary`
  renders (and so evaluates) a task, dynamic variables included, so no run of the tool reads
  them. They are a decode with `yq` (an independent YAML reader, anchors and merge keys expanded)
  of the same `deps`, `cmds` and `defer` entries, checked against the tool's names (the name is a
  task the tool lists, an alias, a namespaced task or a wildcard match): 1 of 91 is unknown to it
  (kube-manager's `test-docker` defers `test-docker-cleanup`, which the file never declares).

A Taskfile the tool rejects is excluded and counted; none of the subjects' 18 is. The walk skips
what the connector never reads (`testdata`, `vendor`, `node_modules`, `generated`, `.git` and the
editor directories), so the oracle does not evaluate what fathom would not see.

## Subjects

`go-task/task` v3.52.0 and `go-task/examples` (both MIT), the two the queue names, are thin: the
connector never reads `testdata`, and go-task/task keeps 180 of its 183 Taskfiles there, so fathom
sees its root Taskfile and two under `website/`; go-task/examples holds one. Together 4 Taskfiles,
43 definitions and one include, which is a directory. A third subject, `Runtime-Radar/runtime-radar`
v0.3.0 (Apache-2.0, sparse: its Taskfiles only), is added to the queue's two: a real monorepo whose
root Taskfile includes twelve per-service Taskfiles by file path and one of whose services includes
another. Between the three: 18 Taskfiles, 264 definitions, 91 references, 14 include edges.
