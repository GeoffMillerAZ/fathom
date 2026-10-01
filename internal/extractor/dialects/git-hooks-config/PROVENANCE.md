# git-hooks-config — provenance and divergences

LB-Q-git-hooks-config, design `docs/design/language-breadth.md` §4, §13. A doc-engine dialect
(no `tags.scm`, no upstream grammar, no incumbent: every one of these files was unclassified YAML
and skipped as such). The decode is `gopkg.in/yaml.v3` through the doc engine. The manifest claims
a file by basename, at any depth, and the doc spec reads each format by its basename (`files`):

- **lefthook**: `lefthook.yml`, `lefthook.yaml`, `.lefthook.yml`, `.lefthook.yaml` and the
  developer overrides `lefthook-local.yml`, `lefthook-local.yaml`, `.lefthook-local.yml`,
  `.lefthook-local.yaml` (the YAML names lefthook 1.13 searches a repository root for).
- **pre-commit**: `.pre-commit-config.yaml` (the one name pre-commit reads without `-c`) and
  `.pre-commit-hooks.yaml`, a hook repository's manifest.

## What it models

- **Hooks** (`LefthookHook`, canonical `stage`): every top-level key of a lefthook config whose
  value is a mapping and which is not one of lefthook's own options (`min_version`, `colors`,
  `templates`, `remotes`, `output`, `skip_output`, `extends`, `source_dir`, `source_dir_local`,
  `rc`, `no_tty`, `assert_lefthook_installed`, `skip_lfs`, `lefthook`, `$schema`, and the
  deprecated `remote` and 2.x `glob_matcher`), named as written, at its key's line. lefthook's
  loader treats any other key as a hook, a git hook's name or a custom one run with `lefthook run
  <name>`, and `lefthook dump` lists it; a hook written with no value (`post-checkout:`) is not.
- **Commands and scripts** (`LefthookCommand`, `LefthookScript`, canonical `job`): the keys of a
  hook's `commands:` and `scripts:`, named as written (`"fmt:go"` keeps its colon, a script is its
  file name), at the key's line, kept when written with no value (lefthook lists them); a command
  is signed by its `run`, a script by its `runner`.
- **Jobs** (`LefthookJob`, canonical `job`): the named entries of a hook's `jobs:` list and of the
  `group.jobs:` lists nested in them, three levels deep, at the entry's first line, signed by its
  `run` or `script`. A job without a `name:` has no name, in lefthook's output or anywhere else,
  and is not an entity.
- **pre-commit hooks** (`PreCommitHook`, canonical `job`): every `repos[*].hooks[*]` entry of a
  `.pre-commit-config.yaml` that carries an `id`, named by it, at the entry's first line, signed
  by its `name`, else its `entry` (remote, `local` and `meta` repositories alike).
- **Hook definitions** (`PreCommitHookDef`, canonical `job`): every entry of a
  `.pre-commit-hooks.yaml` root list that carries an `id`, named by it and signed by its `name`.

## Why `.pre-commit-hooks.yaml` is read

It is pre-commit's second file format: a hook repository's manifest (what `pre-commit
validate-manifest` checks), whose entries are the hooks a config's `repos[*].hooks[*].id` names.
Reading it puts the definition beside its uses in the same index, it is as unambiguous by name as
the config, and the queue's own pre-commit subject (pre-commit/pre-commit-hooks) is mostly this
file: without it the subject is one config of 14 hooks.

## What is not modelled, and why

- **lefthook.json, .lefthook.json, lefthook.toml, .lefthook.toml**: the dialect decodes YAML; a
  JSON config would read, a TOML one would not, and neither appears in the subjects.
- **`.config/lefthook.yml`** (lefthook 2.x): not in the 1.13 search order the oracle runs.
- **References and cross-file edges**: the target is L1. `extends:` and `remotes:` (lefthook) and
  `repo:`/`rev:` (pre-commit) are cross-file and cross-repository references (L3), and nothing in
  either format names another entity of the same file.
- **A job nested more than three group levels deep**, and a command reached only through a YAML
  merge key (`<<: *anchor` under a hook): the doc engine does not expand merge keys.

## Calibration oracle

`calibration/oracles/lefthook-dump/` (bench-only, never a product dependency). lefthook is tier A:
`lefthook dump --format json` (lefthook 1.13.6) on each config alone, joined to written lines by an
independent yq decode, with the set of option keys read from the JSON schema the lefthook binary
embeds. pre-commit is tier C: the tool is not installed on the measuring machine (w6 triage
2026-09-29, no installs), so a Ruby Psych (libyaml) reference parse stands in, which caps that
subject at good (the queue entry's `retargeted` field). Subjects: `calibration/corpus.d/git-hooks-config.yaml`.
