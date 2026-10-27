# Terragrunt: cheap oracle-first triage

2026-10-26. Main `01ea58c6b`.

This separate note **feeds IAC-20 and GAP1-37 but does not satisfy them**. The canonical combined assessment `docs/assessments/2026-10-26-iac-oracle-first-triage.md` remains absent. Under the current NP-Q5 deferred scope, each new native route is **needs-owner-approval (untried)**. No measured ceiling or new grade follows.

The existing oracle pins Terragrunt 1.1.6 `render --json --with-metadata` and `find --json --include` for cross-file edges, with `python-hcl2` 8.1.4 for definitions and references. Source pins and protocol hashes are in the [receipt](data/terragrunt-triage-2026-10-26/receipt.json). The product dialect has no dedicated `unit`/`stack` semantics; generic `.hcl` content sniffing may admit some stack files.

| Candidate | Potential output | Disposition |
|---|---|---|
| Existing render/find oracle | Rendered configuration and discovered inclusions | Tried by the existing instrument; no new attempt |
| Owner [`stack generate`](https://docs.terragrunt.com/reference/cli/commands/stack/generate/) | Expanded units in `.terragrunt-stack`, not source-line entities/references | needs-owner-approval; untried |
| Owner [`find --json --dependencies --dag`](https://docs.terragrunt.com/reference/cli/commands/find/) or [`list --dag --tree`](https://docs.terragrunt.com/reference/cli/commands/list/) | Unit/stack discovery and dependency graph, subject to pinned-version option support | needs-owner-approval; untried in this triage |
| [`terragrunt-ls` v0.0.6](https://github.com/gruntwork-io/terragrunt-ls/releases/tag/v0.0.6) | Possible position/reference information, pending capability inspection | needs-owner-approval; untried |

The read-only inventory includes hidden and ignored directories in scoped cache tools, direct subject roots, and workspace. It emits role names and aggregate counts, raises on unreadable directories, and records missing roots. At the measured roots: zero Terragrunt executables on PATH or in tools, zero `terragrunt.hcl` and `terragrunt.stack.hcl` files in subject roots, and zero generated stack directories. These counts do not establish machine-wide absence. Set `IAC_CACHE` to the private lane cache root before reproducing:

```sh
python3 -I calibration/oracles/terragrunt-triage/inventory.py --cache-root "$IAC_CACHE"
```

**Non-executable future command shape:** `TG_TF_PATH=/usr/bin/false <pinned-terragrunt> stack generate --parallelism 1` in `<guarded-scratch>`. A runnable attempt requires a pinned binary and local-source fixture, audited stack sources, verified OS-level network denial, an empty scrubbed HOME and environment without cloud credentials, read-only source/cache, and nice/load/time/output limits. `TG_TF_PATH` alone does not ensure offline execution. The IaC lane owns the remaining HCL-tool-config/Packer triage under issue #55 and IAC-20/GAP1-37. It remains open outside this three-item pass under I-R5; Packer required-plugin execution needs separate controller approval, and no plugin is installed here.

Owner documentation and terragrunt-ls v0.0.6 release metadata checked 2026-10-26. These are candidate capabilities, not measured source-position or binding coverage. No complete oracle search or ceiling is claimed.
