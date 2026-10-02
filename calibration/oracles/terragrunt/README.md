# terragrunt oracle (LB-Q-terragrunt)

Bench-only; never a product dependency. Writes `indexbench lang-oracle`'s tool-oracle JSON for the
terragrunt native dialect, in two files: tier C (definitions, references) and tier A (cross_references).

| Part | Tier | Tool |
|---|---|---|
| definitions, references | C | python-hcl2 8.x (MIT), a Lark HCL2 grammar: an independent parser library (design §1.3), shares no code with hashicorp/hcl; `../hcl-tools/hcl2tree.py` reads its tree |
| cross_references: include, dependency, dependencies, terraform.source, inputs | A | Terragrunt 1.1.6 (MIT): `render --json --with-metadata` and `find --json --include --filter .`, run in place on every `terragrunt.hcl` of a scratch copy, `TG_TF_PATH=/usr/bin/false` (no tofu), `--experiment optional-dependency-outputs --no-dependency-outputs` (no dependency output is fetched, no backend, no network) |
| the .tf files of a local module directory | C | the same HCL2 reader: the files that declare a definition, and the variables each declares (a file list, not a Terragrunt opinion) |

## Install (scratch, on the external volume)

```sh
python3 -m venv <scratch>/venv && <scratch>/venv/bin/pip install -r calibration/oracles/terragrunt/requirements.txt
# terragrunt: the release binary (https://github.com/gruntwork-io/terragrunt/releases, darwin_arm64,
# sha256 from the release's SHA256SUMS), into <scratch>/tools; no tofu or terraform is needed
```

## Run

```sh
<scratch>/venv/bin/python calibration/oracles/terragrunt/terragrunt_oracle.py -subject <checkout> -scratch <dir outside it> \
    -terragrunt <scratch>/tools/terragrunt -out workspace/tmp/oracle/terragrunt-<subject>.json \
    -xref-out workspace/tmp/oracle/terragrunt-<subject>-xref.json -files-out <files.txt> [-summary-out <json>]
go run ./cmd/indexbench lang-oracle -dialect terragrunt -subject <checkout> \
    -tool-oracle workspace/tmp/oracle/terragrunt-<subject>.json -tool-oracle workspace/tmp/oracle/terragrunt-<subject>-xref.json \
    -only "$(paste -sd, <files.txt>)" -out docs/bench/readings/lang/terragrunt.json
```

`-files-out` lists the measured files (every parseable .hcl, minus the units Terragrunt could not
evaluate: an unset variable, a plugin, an experiment, a syntax error); `lang-oracle -only` restricts
the measurement to them.

## What the truth is

The module docstring (`terragrunt_oracle.py`) states each rule; the naming contract is the dialect's
(`internal/extractor/dialects/terragrunt/PROVENANCE.md`). Only a `terragrunt.hcl` has a
cross_reference: Terragrunt reads an included file's relative paths from the including unit, which one
file cannot say. The summary (`-summary-out`) counts the units the tool could not evaluate, the files
the parser rejected, and the agreement of `render --with-metadata`'s found_in_file attribution (inputs
keys, generate, dependency, remote_state) with the parse's entities (never scored).

## Index row (calibration/oracles/README.md)

| `terragrunt` | C (definitions, references); A (cross_references) | python-hcl2 8.x; Terragrunt 1.1.6 `render --json --with-metadata`, `find --json --include` | definitions (a `package` per file, an `import` per include, a `target` per dependency, a `module` per terraform block, a `resource` per generate and remote_state, a `variable` per local, a `key` per inputs key), `references` (local., dependency., include., feature. reads), `cross_references` (include, dependency, local module source, input to variable; file granularity) | `<venv>/bin/python calibration/oracles/terragrunt/terragrunt_oracle.py -subject <checkout> -scratch <dir> -terragrunt <bin> -out <json> -xref-out <json> -files-out <txt>` |
