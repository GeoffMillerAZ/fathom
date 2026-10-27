# Independent GLSL syntax-call oracle

This adapter reads the pre-linked AST from the pinned glslang 16.6.0 compiler's `-i` output. It produces a calls-only neutral JSON file for L2 comparison. Ctags-C supplies separate L1 definition truth. The adapter records every corpus file, native-derived invocation, compiler diagnostic, and raw AST in ignored `workspace/`. An oracle JSON is emitted only when that subject parses at least 95% of its unchanged population. These are tier B compiler syntax dumps, including Vulkan-Samples inputs translated from its native glslc flags.

The [reading driver](reading.sh) builds indexbench once, pairs the ctags-C definition oracle with this calls-only AST oracle, and measures all three unchanged subjects three times. It writes the median CPU-throughput run, with generated parse-error, oracle-coverage, catalog-mode, and catalog-fingerprint notes, to its required output path. The run's private `<scratch>/evidence.json` binds the output hash and source commit to the catalog identity, each pinned subject and shader-tree hash, the selected run's unmatched file, and normalized oracle call/scope truth. Paths remain in this private workspace evidence rather than the public Reading schema. Every invocation generates fresh oracle bundles under `workspace/oracles/glslang-ast/catalog-modes/<mode>/`; saved bundles are never reused. Run it under the host's CPU lease.

`GLSL_CATALOG_MODE` selects one of three paired modes: `baseline-core` uses the 197 core functions and the original 44 constructor exclusions; `core-plus-types` uses the 197 core functions and all 126 core type identifiers; `full-registry` adds the independently reviewed extension function and type-identifier catalogs. Type-identifier exclusions include opaque and reserved types and are not all claims of constructibility. The default is `full-registry`. Non-final mode readings must stay under `workspace/`; only the full-registry mode may write the canonical reading path.

The registry's `functions_by_extension` and `types_by_extension` maps attribute names to every name string declared by the source specification. These are overlapping declaring-spec families, not a claim that each child directive independently enables every name. Full-registry exclusions use the global union only.

Fetch these exact archive URLs. API tarball endpoints can return different bytes for the same commit.

| Source | Exact archive URL | SHA-256 |
|---|---|---|
| Khronos GLSL | `https://github.com/KhronosGroup/GLSL/archive/0099bf83b0288469917fc2e2972cb0a2b74d6711.tar.gz` | `4ffb1bb817c970670644ea2d19168dc1127a1d5502eb6bbfeb6600859090a23d` |
| Khronos OpenGL Registry | `https://github.com/KhronosGroup/OpenGL-Registry/archive/6af574a14089ccfee87efe230ebcdd8742859813.tar.gz` | `fa80b86e5c2d91c3088b67a128d4070f2520dac6bb823f0538f4870ec9e50942` |

The committed registry is reproducible from the two pinned Khronos archives plus the committed source-review receipts. The emitter verifies archive hashes, all 96 + 1,025 source texts, exact reviewed row identities, the accepted 3,767-row function digest, the independently reviewed type audit, and zero challenger gaps before writing. Regenerate into ignored workspace, then compare exact bytes:

```sh
python3 -I -B calibration/oracles/glslang-ast/extract_extensions.py \
  workspace/oracles/glslang-ast/extension-source/GLSL-0099bf83.tar.gz \
  workspace/oracles/glslang-ast/extension-source/OpenGL-Registry-6af574.tar.gz \
  workspace/oracles/glslang-ast/extension-source/regenerated-audit.json \
  --with-pinned-types \
  --registry-review calibration/oracles/glslang-ast/registry-ambiguous-source-review.json \
  --outside-shader-review calibration/oracles/glslang-ast/registry-outside-shader-source-review.json \
  --function-review calibration/oracles/glslang-ast/registered-extensions-review.json \
  --catalog-out workspace/oracles/glslang-ast/extension-source/regenerated-extensions.json
cmp calibration/oracles/glslang-ast/registered-extensions.json \
  workspace/oracles/glslang-ast/extension-source/regenerated-extensions.json
```

`registered-functions-source-review.json` is the compact committed positive-source receipt bound by `registered-extensions-review.json`; it carries 1–2 source-hash/line citations per accepted function without copying specification bodies. The two row-level adjudication receipts likewise retain only source identities, locations, candidate identities, statuses, and dispositions. Raw specification lines and contact text stay out of the committed oracle package.

```sh
GLSL_CATALOG_MODE=baseline-core calibration/oracles/glslang-ast/reading.sh workspace/oracles/glslang-ast/catalog-modes/baseline-core-reading.json
GLSL_CATALOG_MODE=core-plus-types calibration/oracles/glslang-ast/reading.sh workspace/oracles/glslang-ast/catalog-modes/core-plus-types-reading.json
GLSL_CATALOG_MODE=full-registry calibration/oracles/glslang-ast/reading.sh docs/bench/readings/lang/glsl.json
```

The reading names are `SaschaWillems/Vulkan`, `KhronosGroup/SPIRV-Cross`, and `KhronosGroup/Vulkan-Samples`; internal oracle paths retain short subject keys. The driver requires a clean source worktree, including nonignored untracked files, and checks that HEAD remains stable after building and measuring. `FATHOM_COMMIT`, if set, must equal the full current HEAD. The driver preserves a caller-provided `TMPDIR` (including the origin host's cache location), defaulting to `workspace/tmp`. Set `GLSL_BENCH_CONTEXT` only to record measured host/runtime conditions through indexbench's `-note`; the driver leaves those conditions unspecified otherwise.

From the repository root, run light tests:

```sh
python3 -m unittest discover -s calibration/oracles/glslang-ast -v
python3 -m unittest discover -s calibration/oracles/glslang-ast/subject-recipes -v
```

On macOS, `bash calibration/oracles/glslang-ast/setup-glslang.sh` installs the [official glslang 16.6.0 universal release asset](https://github.com/KhronosGroup/glslang/releases/download/16.6.0/glslang-16.6.0-macos-universal-release.tar.gz) only under ignored `workspace/`. The setup verifies archive SHA-256 `4f8c05f2888a73ed0b7b274bc524af9130e09ae94b43c0f3f9ed83cfae852834` and extracted `bin/glslang` SHA-256 `18c23a68d6f4a21e822a8dcee212a2a448a66494b8f58aaf1655ef6328543d11`; the adapter checks the latter at runtime and records both digests and asset URL in each receipt. This pin is macOS-specific. [glslang's release-tagged license file](https://github.com/KhronosGroup/glslang/blob/16.6.0/LICENSE.txt) describes its component licenses.

Regenerate the ignored recipe manifest, then reproduce one subject's compiler replay, substituting the same subject name in all three places for `spirv-cross` and `vulkan-samples`. On the origin host, the measured run used a separate nice-10, load-one-at-most-9 gate before this command:

```sh
python3 -I -B calibration/oracles/glslang-ast/subject-recipes/generate.py
TMPDIR="$PWD/workspace/tmp" python3 -I -B calibration/oracles/glslang-ast/glslang_ast.py \
  --subject-kind vulkan --compiler workspace/tools/glslang-16.6.0/bin/glslang \
  --builtin-index calibration/oracles/glslang-ast/glsl460-functions.json \
  --catalog-mode full-registry \
  --extension-index calibration/oracles/glslang-ast/registered-extensions.json \
  --extension-review calibration/oracles/glslang-ast/registered-extensions-review.json \
  workspace/calibration/glsl/vulkan \
  workspace/oracles/glslang-ast/runs/vulkan/oracle.json \
  workspace/oracles/glslang-ast/runs/vulkan/receipt.json
```

`glsl460-functions.json` contains 197 factual function identifiers from all 19 function-table subsections and 126 basic type identifiers (including 33 image types) from [the official GLSL 4.60 specification](https://registry.khronos.org/OpenGL/specs/gl/GLSLangSpec.4.60.html). The source is pinned to Khronos registry revision `747576a437935b79d72cc07b6cbed7136b613e71`, HTML SHA-256 `ce264edf9b2417f7f7f6ab42eedf2b4671a81557e0b5ed5e89fd9205bf557540`. The adapter checks the exact catalog digest, pinned revision, HTML digest metadata, all 19 function section keys, and type coverage counts. It accepts `--builtin-source` to recheck a local HTML copy. The full specification stays in ignored workspace, never in this oracle package. After placing that exact HTML at `workspace/oracles/glslang-ast/GLSLangSpec.4.60.html`, regenerate and compare:

`glsl460-baseline-functions.json` preserves the pre-type catalog byte for byte (SHA-256 `b0da4128d106468636a49f08889ef0412cae3fc7bbb450efda92dde0792a0bb5`). Its 197 function identifiers exactly match the current core catalog, allowing paired core-only and core-plus-types readings without unpublished branch ancestry.

```sh
python3 -I calibration/oracles/glslang-ast/extract_builtins.py \
  workspace/oracles/glslang-ast/GLSLangSpec.4.60.html \
  workspace/oracles/glslang-ast/regenerated-glsl460-functions.json
cmp calibration/oracles/glslang-ast/glsl460-functions.json workspace/oracles/glslang-ast/regenerated-glsl460-functions.json
```

The [subject recipes](subject-recipes/README.md) preserve pinned native flags and account for all 968 files. The full per-file recipe stays in ignored workspace; the committed summary records pins, counts and its exact digest. Recipe generation checks each cached CMake file's bytes against its Git blob at the pinned subject commit before parsing those same bytes. For glslc-native includes, the adapter translates [shaderc's pinned implicit include extension](https://github.com/google/shaderc/blob/2737994b552137a85545f956e303d620e6ff232c/libshaderc_util/src/compiler.cc#L223-L227) into glslang `-P`, leaving shader sources unchanged. It checks every preprocessed `#line` path against tracked inputs inside the pinned `shaders/` tree before accepting calls, and rejects ambiguous function origins and builtin-name shadowing. Authored AST fixtures have trailing whitespace normalized, and the original `include_main.frag.ast` diagnostic header was normalized to `testdata/include_main.frag` for publication; the unmodified compiler output remains in ignored workspace. No corpus source, cached CMake text, compiler binary, raw receipt, or full specification is part of this package.

For a fresh grammar remeasurement that preserves earlier oracle bundles, set `GLSL_RUNS_ROOT` to a new directory below `workspace/` when running `reading.sh`. The driver rejects empty paths, traversal and paths that resolve outside that workspace through a symlink. The default remains `workspace/oracles/glslang-ast/catalog-modes/<mode>`.
