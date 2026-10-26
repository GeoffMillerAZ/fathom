# Ansible oracle and qualified ceilings

The committed 2026-10-02 reading remains 694/694 definitions, 9/9 same-file
handler pairs and 60/60 cross-file edges, all C. This audit changes no metric,
product extractor, oracle output or L3 grade. T-R4 still holds new L3 work.

## Named candidates checked (X-R1 / P-T1)

| Candidate | Version | Concrete result / limitation |
|---|---|---|
| Ansible's `ansible-playbook --syntax-check` and `--list-tasks` | ansible-core 2.21.4, recorded by the original oracle run | Original canonical cross-check: 27/63 playbooks accepted; 36 need roles/collections absent from those checkouts. List-tasks prints 294 play/task names, 292 non-template, 289 present in the walk. This is partial native name/acceptance evidence, not the complete positioned declaration or handler-reference population. No full-population A claim follows. |
| Ansible YAML language server | `@ansible/ansible-language-server` 26.6.0, MIT, checksum in lsp-candidate-26.6.0.json | Published source inspected, not installed/run. Its advertised semantic provider is definition on module keys to module implementation files. No document-symbol or reference provider is advertised. This does not attest plays/tasks/variables or notify-to-handler pairs. The audit script verifies the archive and the exact compiled/provider source contracts. |
| Ansible native DataLoader / Playbook loader APIs | ansible-core 2.21.4 | Source inspected, not a new API runtime experiment: DataLoader.load delegates to generic YAML/JSON parsing; Playbook.load invokes Play/PlaybookInclude semantic loading and real dependencies (same dependency issue observed by the CLI). Native parser source positions alone do not establish the full Ansible declaration/reference family. Source URLs/hashes are in native-loader-sources-2.21.4.json. An installed-role native API harness remains a possible reopening route; it is not described as an absent API. |
| universal-ctags AnsiblePlaybook subparser | 6.2.1 | Original canonical run produced zero tags; its single play kind does not cover the required declaration family. No tier-B reading resulted. |
| Independent YAML walk | gopkg.in/yaml.v3 v3.0.1 | Existing oracle covers the measured syntax family; independent third-party parsing remains C under P-T1. Native acceptance does not promote its unmatched fields. |

The official server's [documentation](https://github.com/ansible/vscode-ansible/blob/main/docs/README.md#jump-to-module-code)
also describes module-code navigation. The bounded maintained-SCIP catalogue
search ([SCIP](https://github.com/scip-code/scip), 2026-10-26) identified no
Ansible producer; this is a search result, not proof that none exists.

Reproduce the source contract check without executing the server or any AI
feature (standard-library Python only):

```sh
python3 calibration/oracles/ansible/audit_lsp_candidate.py -download \
  -archive workspace/tmp/ansible-language-server-26.6.0.tgz \
  -out workspace/tmp/ansible-lsp-candidate.json
```

The checked archive is SHA512-pinned, version 26.6.0, and declares MIT. Selected
source SHA256s and exact capability counts are in the committed audit receipt.
The script refuses changed identities/contracts; a source-only check is never
reported as a runtime failure.

## Fired triggers, qualified

The generic INI decoder now exists, but `docdecode_ini.go` implements Conan's
physical option lines, not Ansible inventory grammar (host variables, host
ranges, group `:children` / `:vars`, and native inventory interpretation).
Its existence does not satisfy the original inventory trigger. Ten INI
inventories remain outside the measured YAML scope; the source has no YAML
inventory on which the old `ansible-inventory --list` cross-check could run.
Inventory decode/strategy work remains held by T-R2. No golden is weakened.

`not_has_key` also exists now, so its old absence is withdrawn. It can help
separate plays from task items; it does not supply native positioned declaration
or handler-binding truth, dynamic module-key naming, collection lookup, or
inventory semantics. Neither primitive is a new oracle authority. The current
play-as-task / unnamed-task coverage limitations remain disclosed; no new
coverage is claimed. A complete native instrument could reopen L1/L2 now,
without waiting for D19; L3 re-evaluation still waits for its controller gate.

## X-R4 source search, 2026-10-25

| Required source | Search / concrete result |
|---|---|
| Own toolchain / introspection | ansible-core 2.21.4 CLI and native DataLoader/Playbook APIs above inspected. Partial list-tasks acceptance is measured, not called a complete oracle failure. [ansible-inventory](https://docs.ansible.com/ansible/latest/cli/ansible-inventory.html) exposes list/graph/export, but the original claimed scope contains no YAML inventory and INI inventory is outside its decoder. |
| SCIP and LSIF lists plus GitHub | Read [Sourcegraph supported indexers](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), [SCIP list](https://github.com/sourcegraph/scip#tools-using-scip), and Sourcegraph-maintained [LSIF.dev](https://lsif.dev/). No Ansible YAML producer listed. GitHub repository query `scip-ansible in:name` returned five substring hits for SciPy/ordinary script repositories, no Ansible semantic indexer. Python SCIP indexes implementation modules rather than play/task/handler YAML bindings. |
| Language servers | Official MIT `@ansible/ansible-language-server` 26.6.0 source/checksum audit above, plus official vscode-ansible module navigation docs. It supplies module-code definition, with no advertised symbol/reference provider. |
| Registry parsers / type checkers / AST dumpers | PyPI metadata queried for ansible-runner, ansible-compat and ansible-lint, and npm metadata for the pinned server. Runner is an execution/event interface; compat provisions runtime dependencies; lint validates Ansible conventions. None of those published interfaces claims a complete positioned task/handler-binding dump. Exact newly searched metadata versions are recorded below. No candidate was installed or described as a failed runtime experiment. |
| Runtime call / require tracing | Read [official callback plugin documentation](https://docs.ansible.com/ansible/latest/plugins/callback.html) and [runner event interface](https://ansible.readthedocs.io/projects/runner/en/stable/intro/). Native task callbacks/event data are a real candidate, requiring execution context and roles/collections; they report paths actually visited, not automatically the full disabled/static population. Subject tasks were not executed. A bounded native loader/event harness remains a reopening route, not an absent capability. |

The L1/L2 instrument limit is provisional on a full-population native loader or
bounded event experiment with required dependencies. L3 stays behind D19; this
source audit changes no reading. The independent parser is not promoted by the
existence of a native API or linter.

Registry identities queried: ansible-runner 2.4.3 (Apache-2.0), ansible-compat
26.10.0 (MIT), ansible-lint 26.10.1 (GPL-3.0-or-later); only metadata/interfaces
were inspected. The lint package's license also excludes it as the requested
permissively licensed oracle. Runner remains a concrete native-event route,
not a rejected parser merely because it requires an experiment.
