# hcl-tools oracle (LB-Q-hcl-tools)

Bench-only; never a product dependency. Writes `indexbench lang-oracle`'s tool-oracle JSON for the
hcl-tools native dialect (Packer, Nomad, docker-bake), tier C.

| Part | Tier | Tool |
|---|---|---|
| definitions, references (Packer, Nomad, bake) | C | python-hcl2 8.x (MIT), a Lark HCL2 grammar: an independent parser library (design §1.3), shares no code with hashicorp/hcl; `hcl2tree.py` reads its tree |
| bake target and group names (cross-check, never scored) | A | `docker buildx bake --progress=quiet --list=type=targets,format=json -f <file>` (buildx 0.31.x; no daemon, no build) |
| Packer, Nomad | examined in wave ox, not scored | release binaries fetched as local measuring tools only (owner-approved; BUSL permits non-production use, nothing of theirs ships): see "Wave ox" below; both stay tier C |

`hcl2tree.py` is shared with `../terragrunt/`.

## Wave ox: the HashiCorp tools (LB-Q-hcl-tools-rest)

Fetched into `/Volumes/m2ext/tmp/lang-oracles/tools/bin` from releases.hashicorp.com, each zip checked
against the release's SHA256SUMS: packer 1.16.1 darwin_arm64 (zip `c45cb1f8…df424`, binary
`a58935f4…bda34`), nomad 2.0.7 darwin_arm64 (zip `4ada34db…83e58`, binary `d7d01c35…0ed`). What they say
offline, with no plugin fetched:

- `packer inspect` needs plugins for everything but variables: on chef/bento's `packer_templates` it lists
  the 139 input variables and the 4 locals it can evaluate and an empty `builds` section (the template's
  `required_plugins` name nine plugins, none installed); on packer-plugin-docker's fixtures it lists an
  unnamed build with no sources. Sources, builds, provisioners, post-processors and data sources are not
  attested without the plugins, which are never fetched without asking, so Packer stays tier C and with it
  the dialect's families (a family's tier is the weakest of its subjects).
- `nomad job run -output <jobspec>` (no agent: `NOMAD_ADDR` pointed at a closed port) prints the parsed job
  for 21 of nomad-guides' 22 jobspecs (job, groups, tasks, services; `haproxy.nomad` is refused: "Invalid
  single-argument block definition"); variables and locals are evaluated away and would need ablation.
- `docker buildx bake --print` resolves the targets a file's default group (or the named targets) builds,
  and `--list` names every target and group (the cross-check this oracle already runs).

Nomad and bake could reach tier A (the tool's output for blocks; ablation for variables, locals and
references; python-hcl2 for lines), but with Packer at C the reading's families stay C, so the wave did not
build them.

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
