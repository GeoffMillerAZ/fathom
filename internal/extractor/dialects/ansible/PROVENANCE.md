# ansible — provenance and divergences

LB-Q-ansible, design `docs/design/language-breadth.md` §4, §13; the iac wave (EXCELLENCE T3). A doc-engine
dialect (no `tags.scm`, no upstream grammar, no incumbent: an Ansible file was unclassified YAML, never
extracted). The decode is `gopkg.in/yaml.v3` through the doc engine. Fixtures are written for this
dialect; the shapes follow the Ansible documentation's playbook, role and inventory examples (GPL-3.0
text is not copied).

## How a file is claimed: path and content, last

The manifest claims no name. `fusion.ansibleDetects` (LB-F150) is the YAML classifier's LAST signal
(`classifyYAML`, after CloudFormation), so every earlier verdict is unchanged: Kubernetes, Compose, GitLab
CI, OpenAPI and CloudFormation files, and a file under `.github/workflows/`, are never Ansible's. A file is:

- by path: `roles/<r>/{tasks,handlers,defaults,vars,meta}/**.yml`, `group_vars/` and `host_vars/` (a file or
  a directory of files), an `inventory/` or `inventories/` YAML whose top-level `all:` carries `hosts:` or
  `children:`;
- by content: a top-level sequence whose items carry `hosts:`, `import_playbook:` or
  `ansible.builtin.import_playbook:` (a playbook);
- by two signals beyond the brief's roles/ layout, because most Ansible a repository holds is not under a
  directory named roles/: a **standalone role** (its root holds `tasks/main.yml`, as every role repository
  does; a sibling-index fact, which ReingestFile reads from the disk) reads like a role under roles/, and a
  YAML **sequence of maps under a `tasks/` or `handlers/` directory** (the task files a playbook imports).

An INI inventory is not YAML and the doc engine has no INI decoder (wave cx owns the decode enum): it is
read by nothing (LB-Q-ansible-rest). The signal is the ToolKind `ansible`; ingest reaches the dialect
through `toolExts`, ReingestFile through the same decision.

## What it models

- **The file** (`AnsibleFile`, `package`): one entity per file, named by the file.
- **Tasks** (`AnsibleTask`, `task`): every item of a task list — a file's top-level items, a play's
  `tasks`, `pre_tasks`, `post_tasks`, every `block`, `rescue` and `always` list — named by `name`, else (a
  play) by its `hosts`. A play is a task whose signature is its hosts pattern: the engine's `where`
  filters the key an entity was reached by, never the keys it holds (design §4), so no selector tells a
  play from a task file's item, and a second kind would double every named play.
- **Handlers** (`AnsibleHandler`, `rule`): a play's `handlers[]`, named by `name` and, as a second entity,
  by `listen`.
- **Roles** (`AnsibleRole`, `service`): a play's `roles:` entries (a name, or a map's `role`/`name`), an
  `include_role`/`import_role` (short and `ansible.builtin.` forms), a meta `dependencies[]` entry.
- **Includes** (`AnsibleInclude`, `import`): `include_tasks`, `import_tasks`, `include_vars` (short and
  `ansible.builtin.` forms) of any task or handler and a play list's `import_playbook`, named by the path
  (a map's `file`).
- **Variables** (`AnsibleVar`, `variable_decl`): a mapping file's top-level keys (a vars, defaults or meta
  file's, an inventory's `all`) and a play's `vars` keys; a key written with no value counts.
- **Inventory** (`AnsibleGroup` `section`, `AnsibleHost` `element`): the entries of `children:` and
  `hosts:` at any depth, with or without a value.
- **References** (`Calls`, L2, declared only): a task's `notify` (a name or a list) to a handler of the same
  document, by name or `listen`.
- **Cross-file references** (L3): `repo-path` from the referring file's directory for an include, an
  `import_playbook` and a `vars_files` entry; a role to `roles/<name>/tasks/main.yml` beside the referring
  file and, from a file inside a role, beside that role (`../../<name>/tasks/main.yml`, how a role's own
  include_role and its meta dependencies reach a sibling); `service-name` for a `notify` to the one handler
  of that name in another file (a role's handlers file).

## Divergences, each disclosed (and where it is a ceiling, a -rest row)

- **A play is a task.** See above; the oracle writes a play as a `task` the same way.
- **"Else module" is not selectable.** A task without a `name` is no entity: its module key cannot be
  picked out of the item's keys by a selector. A task with only an include or import is its include entity.
- **Collections are not roles on disk.** `geerlingguy.mac.homebrew` names `roles/geerlingguy.mac.homebrew/`
  and resolves to nothing (a collection or a `roles_path` elsewhere): counted on the oracle's stderr.
- **A templated path resolves to nothing.** `include_tasks: "{{ item }}"` names no file.
- **FQCN keys have no selector.** A key with dots (`ansible.builtin.import_tasks`) cannot be written in a
  selector, so every include and role key is read as an entry of `.*` filtered by `where.key_in`.
- **A mapping file's root is read as one list item** (`[*]` over a map is the map), so a vars file with a
  top-level `name:` or `hosts:` key is also read as a task; none of the subjects has one.
- **Tier C for entities and edges.** `ansible-playbook --syntax-check` and `--list-tasks -i localhost,` and
  `ansible-inventory --list` (ansible-core, offline, no play is run) are the tier-A acceptance and name
  cross-check, run by the oracle and never scored: neither prints a line or an include edge.

## Calibration

Subjects in `calibration/corpus.d/ansible.yaml`; the oracle is `calibration/oracles/ansible` (an
independent `gopkg.in/yaml.v3` walk, with the tools behind `-ansible-playbook` and `-ansible-inventory`).
The queue names ctags' Ansible subparser as a tier-B oracle. Universal-ctags 6.2.1 has the
AnsiblePlaybook subparser, but it has one kind (`play`) and, run as `--languages=Yaml,AnsiblePlaybook`
or alone over the subjects' playbooks (ansible-for-devops `security/main.yml`, `first-ansible-playbook`),
it tags nothing: no tier-B reading could be taken, and the tier-C walk stands.
