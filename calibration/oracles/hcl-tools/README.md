# hcl-tools oracle (LB-Q-hcl-tools)

Bench-only; never a product dependency. Writes `indexbench lang-oracle`'s tool-oracle JSON for the
hcl-tools native dialect (Packer, Nomad, docker-bake), tier C.

| Part | Tier | Tool |
|---|---|---|
| definitions, references (Packer, Nomad, bake) | C | python-hcl2 8.x (MIT), a Lark HCL2 grammar: an independent parser library (design §1.3), shares no code with hashicorp/hcl; `hcl2tree.py` reads its tree |
| bake target and group names (cross-check, never scored) | A | `docker buildx bake --progress=quiet --list=type=targets,format=json -f <file>` (buildx 0.31.x; no daemon, no build) |
| Packer, Nomad | not run | `packer inspect` and `nomad job validate -json` need binaries that are not installed here (a HashiCorp release zip is not fetched without the orchestrator's approval); both stay tier C |

`hcl2tree.py` is shared with `../terragrunt/`.

## Install (scratch, on the external volume)

```sh
python3 -m venv <scratch>/venv && <scratch>/venv/bin/pip install -r calibration/oracles/hcl-tools/requirements.txt
```

## Run

```sh
<scratch>/venv/bin/python calibration/oracles/hcl-tools/hcltools_oracle.py -subject <checkout> \
    -out workspace/tmp/oracle/hcl-tools-<subject>.json -files-out <files.txt> [-summary-out <json>]
go run ./cmd/indexbench lang-oracle -dialect hcl-tools -subject <checkout> -subject-name <owner/repo> \
    -tool-oracle workspace/tmp/oracle/hcl-tools-<subject>.json -only "$(paste -sd, <files.txt>)" \
    -out docs/bench/readings/lang/hcl-tools.json
```

## What the truth is

The module docstring (`hcltools_oracle.py`) states each rule; the naming contract is the dialect's
(`internal/extractor/dialects/hcl-tools/PROVENANCE.md`). The oracle classifies a file as the manifest
does (name, extension, glob, a top-level `job` block). The summary names the files the parser rejected
and counts how many bake files' target and group names `docker buildx bake --list` agrees with
(allowing for a private `_` target, which buildx hides, and a matrix target, which it expands).

## Index row (calibration/oracles/README.md)

| `hcl-tools` | C (definitions, references); A (bake names, cross-check) | python-hcl2 8.x; `docker buildx bake --list` 0.31 | definitions (Packer source/build/variable/locals/data/step and pkrvars keys; Nomad job/section/task/service/variable/locals; bake target/stage/variable/function), `references` (var., local., data., source. reads; build sources; group targets; target inherits and contexts; bare variables and declared function calls) | `<venv>/bin/python calibration/oracles/hcl-tools/hcltools_oracle.py -subject <checkout> -out <json> -files-out <txt>` |
