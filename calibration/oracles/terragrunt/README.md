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

## Wave ox: tier-A attestation (`terragrunt_attest.py`, measured, not scored)

```sh
<scratch>/venv/bin/python calibration/oracles/terragrunt/terragrunt_attest.py -subject <checkout> -scratch <dir outside it> \
    -terragrunt <bin> -files <files.txt from terragrunt_oracle.py> -out <json> -summary-out <json>
cd calibration/oracles/terragrunt && TERRAGRUNT=<bin> <scratch>/venv/bin/python -m unittest test_terragrunt_attest
```

It keeps only the entities and references Terragrunt 1.1.6 (Homebrew bottle, binary sha256
`22bdb12d…ead8dd`) evidences, offline in a scratch copy: what `render --json --with-metadata` attributes to
the declaring file (`found_in_file`: dependency, dependencies, terraform, remote_state, generate, inputs keys,
and a unit's own locals, which render does list), the include blocks `find --json --include` names per
unit, and, by the kustomize oracle's ablation rule, the rest: an included or read file's local (its
definition renamed), a feature block (its label renamed), a file no render names (moved away), every
reference (its attribute renamed inside the entity that writes it), each re-rendered on every unit that can
read the file. Names and lines stay python-hcl2's: fathom's dialect and Terragrunt both parse with
`hashicorp/hcl/v2`, so lines come from the independent Lark grammar, never from a shared parser.

What it evidenced: gruntwork-io/terragrunt-infrastructure-live-example, all 74 definitions and 20 references.
gruntwork-io/terragrunt test/fixtures, 3,056 of 3,571 definitions and 246 of 342 references (969 units, 959
render; 3,703 tool runs): 240 files no render names (152 `terragrunt.stack.hcl`, which `render` does not
read; 78 other `.hcl` files, read only by units that do not render or by none, such as the `hcl fmt`
fixtures; 10 units whose render fails), holding 373 definitions; 174 locals of included or read files that
no rendered output depends on; 96 references whose ablation changes nothing a render prints (a dependency
output read with dependency outputs off (`--no-dependency-outputs`), a feature read inside a block render omits). The fixtures therefore cannot
be measured at tier A: the scored definitions and references stay python-hcl2's (tier C), and with them the
L1 and L2 families (said in LB-Q-terragrunt-rest).

## Index row (calibration/oracles/README.md)

| `terragrunt` | C (definitions, references); A (cross_references) | python-hcl2 8.x; Terragrunt 1.1.6 `render --json --with-metadata`, `find --json --include` | definitions (a `package` per file, an `import` per include, a `target` per dependency, a `module` per terraform block, a `resource` per generate and remote_state, a `variable` per local, a `key` per inputs key), `references` (local., dependency., include., feature. reads), `cross_references` (include, dependency, local module source, input to variable; file granularity) | `<venv>/bin/python calibration/oracles/terragrunt/terragrunt_oracle.py -subject <checkout> -scratch <dir> -terragrunt <bin> -out <json> -xref-out <json> -files-out <txt>` |
