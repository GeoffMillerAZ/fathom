# circleci -- provenance and divergences

LB-Q-circleci, design `docs/design/language-breadth.md` section 4 and 13. A
doc-engine dialect (no `tags.scm`, no grammar, no incumbent). This file records
what it models, what it leaves out and where the oracle comes from.

## What it models

- **Jobs** (`CircleCIJob`, canonical `job`): each map under `jobs:`. The
  signature is the first of the job's `executor` (name), `executor.name`, first
  `docker` image, `machine.image`, `macos.xcode` or `resource_class`.
- **Workflows** (`CircleCIWorkflow`, canonical `target`): each map under
  `workflows:`. The legacy `version: 2` scalar is not a map and is not one.
- **Commands** (`CircleCICommand`, canonical `mixin`): each map under
  `commands:`, signed by its `description`.
- **Executors** (`CircleCIExecutor`, canonical `resource`): each map under
  `executors:`, signed by its first docker image, machine image, Xcode version
  or resource class.
- **References** (`Calls`, every one only to a name the same file declares, so an
  orb job, an approval job and a built-in step such as `run` are never edges):
  a workflow to each job it runs (a bare name or the key of a one-key map) and
  each job its `requires` names; a job or command to each command its `steps`
  invoke, at any depth (`when` and `unless` nest steps); a job to its executor
  (`executor: name` or `executor: {name: ...}`).

## What it does not model, and why

- **Instance names.** A workflow entry may rename a job (`name:`) or expand it
  (`matrix:`). `requires` names the instance, so a `requires` that names an
  instance whose name is not a job key is no edge. The oracle attests the
  workflow-to-job edge through the compiler's instance names and the dialect
  cannot (the doc engine has no join between a `name:` and its sibling key).
- **Dropped workflows.** The compiler drops a workflow whose `when` evaluates
  false. The dialect has no evaluation and reads it (the one precision miss in
  the reading: cimg-base's `monthly-release`).
- **Orbs and functions.** An orb's jobs, commands and executors live in the
  registry, not the repository: `node/install-packages` names no declaration of
  the file. A config line that invokes a parameterised command with arguments
  reads as the command's name.
- **Cross-file.** `.circleci/config.yml` names no other file of the repository (a
  continuation config is handed to the continuation orb by path at run time), so
  the dialect has no `cross_file` and targets L2. Only `config.yml` and
  `config.yaml` under a `.circleci` directory are claimed; a continuation file
  of another name is not.

## Oracle

`calibration/oracles/circleci`: `circleci config process` (tier A, circleci-cli
1.7.0 from the tool cache, `HOME` outside the subject) for definitions; the
reference file is tier C because the compiler inlines commands and executors.
See `calibration/oracles/README.md`.
