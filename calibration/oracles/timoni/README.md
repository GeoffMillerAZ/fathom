# timoni oracle (LB-Q-timoni)

Bench-only; never a product dependency. Its own Go module (`fathom.bench/oracles/timoni`, cuelang.org/go v0.16.1 for the
syntax tree). Writes `indexbench lang-oracle`'s tool-oracle JSON for the timoni native dialect, in two files, one per
tier: tier C (definitions, references) and tier A (cross_references).

| Part | Tier | Tool |
|---|---|---|
| definitions, references | C | a walk of the CUE syntax tree (`cuelang.org/go/cue/parser`, the parser the dialect also reads with: the contract is re-implemented here, the library is shared), gated by the tool: a module enters only if `timoni mod vet` accepts it, an object template only if its kind is one `timoni mod vet` renders with the default or the `--debug` values, a bundle only if `timoni bundle vet --print-value` accepts it |
| cross_references: #Instance to the object templates it names | A | `timoni mod vet` on a scratch copy, once per object template with its label renamed in the one file that declares it. CUE reports every use the rename breaks (`reference "#D" not found:` or `undefined field: #D:`, each with `./file:line:col`), so the pair is (the using file, the top-level definition that holds the use, the template's file): the tool names who uses the template |
| cross_references: a bundle instance to its local module | A | `timoni bundle vet --print-value` (the tool's own evaluation, a `<stem>_*.cue` companion passed beside the bundle) names each instance's `module.url`; each `file://` instance is rebuilt alone with `timoni bundle build` in the bundle's directory, the instance named in the output resolved to `<bundle dir>/<path>/timoni.cue` |

Offline throughout: no cluster, no registry pull (an `oci://` instance has no truth and is counted external in the
summary), `TIMONI_CACHING=false`, a cache directory inside the scratch directory.

## Install (scratch, on the external volume)

```sh
# the pinned release binary (darwin_arm64; record the version and the sha256 from the release's checksums file)
V=0.35.0; curl -sL https://github.com/stefanprodan/timoni/releases/download/v$V/timoni_${V}_darwin_arm64.tar.gz | tar xz -C <scratch>/bin
shasum -a 256 timoni_${V}_darwin_arm64.tar.gz   # 68bd5ae946773cdc5687d2c024776d5f1be9012de9c6e21ed821ea97800b235c
cd calibration/oracles/timoni && GOWORK=off go build -o <scratch>/bin/timoni-oracle .
```

The subject is `stefanprodan/timoni` at the corpus shard's sha (`calibration/corpus.d/timoni.yaml`: `examples/`,
`blueprints/`, `schemas/timoni.sh/`), plus the overlay: `overlay/examples/bundles/bundle.cue`, copied beside the
subject's `podinfo.cue` as an untracked file, a bundle of `file://` instances (the subject's own instances all name
a registry). Each module's `cue.mod/pkg/timoni.sh` is a link into `schemas/timoni.sh`, which the CLI needs to build a
module, so the oracle copies `examples`, `blueprints` and `schemas` (`-include`) into its private working copy;
nothing in the subject is written.

## Run

```sh
cp calibration/oracles/timoni/overlay/examples/bundles/bundle.cue <checkout>/examples/bundles/bundle.cue
<scratch>/bin/timoni-oracle -subject <checkout> -include examples,blueprints,schemas -scratch <dir outside it> \
    -timoni <scratch>/bin/timoni -out <defs.json> -xref-out <xref.json> -files-out <files.txt> -summary-out <summary.json>
go run ./cmd/indexbench lang-oracle -dialect timoni -subject <checkout> -subject-name stefanprodan/timoni \
    -tool-oracle <defs.json> -tool-oracle <xref.json> -only "$(paste -sd, <files.txt>)" \
    -out docs/bench/readings/lang/timoni.json
```

`-files-out` lists the measured files (every `.cue` of a vetted module outside `cue.mod/`, and every vetted bundle
or runtime); `lang-oracle -only` restricts the measurement to them. The summary counts the modules the tool
rejected, the ablations and how many broke the build, the bundle instances (local, built by the tool, `oci://`
external) and the pairs of each kind.

## What the truth is

The naming contract is the dialect's (`internal/extractor/dialects/timoni/PROVENANCE.md`). Definitions are written as
the segment the harness keeps (`name_match` bare). References are every `#Def` a field's value names (unification,
the value itself, a list's element type, an import selector), a struct-literal `values:` key to `#Config.<k>`, and a
template's `#config: #Config`. An object template is a definition with a literal `kind:`; one the tool does not
render is not an entity. An object without a literal kind is never ablated and has no pair.

## Index row (calibration/oracles/README.md)

| `timoni` | C (definitions, references); A (cross_references) | a CUE syntax walk gated by `timoni mod vet` and `timoni bundle vet`; timoni 0.35.0 `mod vet` with a renamed template, `bundle vet --print-value` and `bundle build` | definitions (a `package` per module and bundle, a `variable` per values, a `resource` per object template, a `target` per bundle instance and runtime cluster), `references` (#Def uses, values keys to #Config, `#config: #Config`), `cross_references` (#Instance to its templates, bundle instance to module; file granularity) | `timoni-oracle -subject <checkout> -include examples,blueprints,schemas -scratch <dir> -timoni <bin> -out <json> -xref-out <json> -files-out <txt>` |
