# Ansible native candidates and remaining work

The canonical reading remains 694/694 definitions, 9/9 local references and
60/60 cross-file edges, all C. The old absent-native-position/binding premise
is withdrawn. Native instruments exist; constructing the complete L1/L2
reading is work, rather than a ceiling waiting for a tool to appear. L3
remeasurement remains held by D19/T-R4.

## Measured candidate results

`native-candidates-r2.json` records a fresh run of ansible-core 2.21.4,
ansible-lint 26.10.1 and ansible-runner 2.4.3 on all three pinned subjects.
Every YAML file and inventory candidate is considered; non-task native kinds
remain counted. There is no extractor input and no task execution.

| Subject | INI accepted/candidates | Native lint task files/YAML candidates | Positioned tasks | Unnamed | Existing child links |
|---|---|---|---|---|---|
| mac-dev-playbook | 2/2 | 6/13 | 24 | 5 | 30 |
| ansible-role-docker | 0/0 | 9/22 | 57 | 6 | 5 |
| ansible-for-devops | 10/11 | 81/122 | 257 | 17 | 49 |

Native INI inventory interpretation is A. The 12 accepted inputs yield per-file
sums of 34 groups, 27 hosts, 42 child relationships and 28 memberships. These
include implicit groups and duplicates across files; they are not unique
repository totals. The rejected candidate is a dynamic inventory client's
settings file, not host/group inventory. It is retained as a rejection, not
silently removed. The previous count of ten considered extensionless files
named inventory/hosts only; this measure also tests all .ini files. Only the
built-in INI plugin is enabled, so executable inventory scripts do not run.

The lint loader supplies 338 positioned task objects, including 28 without a
name, and 84 existing child links. All 338 core Origin line tags agree with
lint annotations. Core evaluates 335 module actions; three block markers are
synthetic lint actions and carry no native action-authority claim. No dependencies
were installed. Native classifications cover all 157 YAML candidates; the
96 task/playbook/handler files are a bounded loader family, not the entire
694-entity canonical vocabulary. Dynamic includes, templated names and
requirements must be accounted for in a complete semantic reading.

T-R8 admits A **for fields ansible-core's loader evaluates**. Module/action
normalisation uses core ModuleArgsParser; lint's task line annotations use
AnsibleLoader parser marks. Neither supplies an end-range measurement here.
A raw HandleChildren traversal is not silently promoted to core Playbook/Role
binding evaluation. No complete-family tier or canonical grade is changed.

## Named candidates and all five X-R4 sources

| Required source | Named/versioned candidate and concrete result |
|---|---|
| Own toolchain, dumps and introspection | ansible-core 2.21.4 `ansible-inventory --list` measured on all 13 candidates above, including native INI groups, ranges and children. The earlier `ansible-playbook --syntax-check` accepted 27/63 **without installing the subjects' requirements.yml**; this does not mean dependencies are unavailable. Core `Playbook.load`/`Role.load`, `Base.get_path()` and `StrategyBase.search_handlers_by_notification` are real positioned-declaration/notify-binding APIs. Their full-population harness, with pinned subject dependencies, remains concrete work. |
| SCIP/LSIF catalogs and GitHub | Read [Sourcegraph supported indexers](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), [SCIP list](https://github.com/sourcegraph/scip#tools-using-scip), and [LSIF.dev](https://lsif.dev/). No Ansible YAML producer listed. GitHub `scip-ansible in:name` returned five SciPy/ordinary-script substring hits, no semantic indexer. Python SCIP indexes Python implementation files. These are bounded search results. |
| Language servers | Official MIT `@ansible/ansible-language-server` 26.6.0 source/checksum audit: module-code definitions, no symbol/reference provider for the measured play/task/notify family. Its module navigation is not that family. `audit_lsp_candidate.py` reproduces the source check; this is not a runtime failure claim. |
| Registry parsers, type checkers and AST dumpers | Ansible's own ansible-lint 26.10.1 (GPL-3.0-or-later) installed and measured above. Bench subprocess licences are not the grammar/fixture allowlist; GPL is no exclusion here. `Task`, `parse_yaml_linenumbers` and `Runner.find_children`/`HandleChildren` produce the measured fields, subject to T-R8. ansible-compat 26.10.0 (MIT, installed dependency) provisions runtimes; ansible-runner 2.4.3 (Apache-2.0, installed) exposes execution events. Neither is dismissed because a prior harness was absent. Universal-ctags 6.2.1 originally emitted zero AnsiblePlaybook tags on the subjects. |
| Runtime call/require tracing | Read [core callback documentation](https://docs.ansible.com/ansible/latest/plugins/callback.html) and [runner event interface](https://ansible.readthedocs.io/projects/runner/en/stable/intro/). Core `v2_playbook_on_notify(handler, host)` and task-start callbacks expose native objects with `get_path()`. They require executing tasks with roles/collections and environment context. No subject task was executed; Docker/Molecule is prohibited here. Under P-T1, traces attest visited tasks only, never complete recall, so static core loading/handler lookup is the preferred complete-population route. |

## Fired triggers and concrete follow-up

The Conan-shaped INI decoder is not an Ansible inventory strategy. The native
inventory oracle is available now; implementing its host/group grammar is
shared strategy work held T-R2. No golden is weakened. No group_vars/host_vars
instance was found in the pinned subjects, so the old unmeasured joining
limitation is not presented as a measured miss.

`not_has_key` also exists: its old absence claim is withdrawn. The unchanged
dialect still needs a play/task split and dynamic module naming for unnamed
tasks. Native full-family work is: load all original playbook/role entry points
with their requirements pinned; collect `Base.get_path()` declarations and
core handler lookup results; explicitly count unresolved/dynamic cases and
compare the native population before grading. L1/L2 work is open now. This is
a deferral of unbuilt work, not an oracle-availability ceiling.

## Reproduce the candidate measurement

Use private Python 3.13.5 with ansible-core==2.21.4,
ansible-lint==26.10.1, ansible-runner==2.4.3 and PyYAML==6.0.3 installed.
Put clean corpus subjects at the named subdirectories of the subjects path.
All config/cache paths are under the scratch work directory. The driver
selects an available UTF-8 locale and never executes inventory scripts.

```sh
FATHOM_ORACLE_ANSIBLE_INVENTORY=workspace/oracle/bin/ansible-inventory \
  python3 -m unittest discover -s calibration/oracles/ansible -p test_native_candidates.py
python3 calibration/oracles/ansible/measure_native_candidates.py \
  -subjects workspace/calibration/ansible-r2 -work workspace/tmp/ansible-native-r2 \
  -out workspace/tmp/ansible-native-candidates-r2.json
python3 calibration/oracles/ansible/audit_lsp_candidate.py -download \
  -archive workspace/tmp/ansible-language-server-26.6.0.tgz \
  -out workspace/tmp/ansible-lsp-candidate.json
```

Only counts, versions, identities and hashes are committed. Source inventories,
task content and diagnostics remain in scratch. The initial probe used the
reserved ANSIBLE_INVENTORY environment variable for its test CLI path, causing
recursive CLI startup inside lint runtime initialisation. That run was stopped
and discarded. The task-specific variable above and built-in-only inventory
plugin fix that collision; the fresh native control and corpus run pass.
