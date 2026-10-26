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
