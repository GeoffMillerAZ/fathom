# GLSL against the 5% ERROR-file gate (lane glsl, item LB-Q-glsl)

**Verdict: no licensed, maintained GLSL grammar passes the gate, so no GLSL dialect is built or dispatched.
tree-sitter-grammars/tree-sitter-glsl `v0.2.0` leaves an ERROR or MISSING node in 272 of the 968 shader files
of the three pinned calibration subjects (28.1%, five and a half times §14.2's 5% kill line): SaschaWillems/Vulkan
32 of 353 (9.1%), KhronosGroup/SPIRV-Cross 153 of 306 (50.0%), KhronosGroup/Vulkan-Samples 87 of 309 (28.2%). LB-Q-glsl
stays open with its contract (retargeted L1 to L2, owner rulings G1 and G2) and LB-Q-glsl-rest is written `[-]`
refused with this measurement. No dialect, blob or lock row is committed; the extensions stay disclosed as
unsupported and no level is claimed.**

Measured 2026-10-25 at fathom 787c146 plus an uncommitted glsl blob and dialect (kept out of the tree on purpose).

## Grammar

tree-sitter-grammars/tree-sitter-glsl `v0.2.0`, commit `e47b8b62b59d0e3529f1c31b03e025d6bd475044` (GitHub's tag
archive, sha256 `65d098362f9ffd47e9bf5916133ed24ccf934b01027debdaeeb46ff3ec41caf0`). Licence MIT, read at the pin from
the top-level `LICENSE`. It ships `src/parser.c` (`LANGUAGE_VERSION 14`) and no scanner, so no generate step. Built with
`internal/extractor/tsengine/grammars/build.sh` (zig 0.16.0, wasm-opt 129, from a scratch lock row): module 686,720
bytes (sha256 `9a6a64fb704c408df33213249282c30d272ac06bf4c280ad740ff827e749ec45`), gzip 155,018 bytes. The default
branch's head, `24a6c8ef698e4480fecf8340d771fbcb5de8fbb4`, is one commit later and changes no grammar file (a Swift
`Package.resolved`), so it parses the same. The catalogue pre-fill (`indexbench lang-pin -id glsl`) names another
repository, `theHamsta/tree-sitter-glsl` at that same head; the queue's repository is the one measured. No other
GLSL grammar was found with a licence on the allowlist.

Reproducing the build in a network that blocks the GitHub archive endpoint and ziglang.org (this lane's VM): the
tarball is `git archive --format=tar --prefix=tree-sitter-glsl-0.2.0/ v0.2.0 | gzip -n -6`, byte-identical to
GitHub's (the same recipe reproduces the lock's core and luau tarball digests), and zig 0.16.0 comes from the PyPI
`ziglang` wheel; the luau blob rebuilt that way matched the lock's `blob_sha256`.

## Method

`indexbench lang-oracle` with a draft `glsl` dialect (tags engine, the pinned blob, kinds function, struct and macro
required and uniform and in/out variables and interface blocks emitted, four fixtures with hand-written goldens that
pass) reads the harness's own `files_with_error` (LB-F59: an ERROR or MISSING node anywhere in the tree). A scratch
`[(ERROR) (MISSING)] @error` query named the first line of each error node (272 files, the harness's count exactly).
The file set is every file whose extension the draft manifest claims (`.glsl .vert .frag .comp .geom .tesc .tese
.rgen .rchit .rmiss .rahit .rint .rcall .mesh .task .vsh .fsh`).

A refused item ships nothing, so the draft dialect and blob are not committed. To reproduce the count: draft the lock
row with `go run ./cmd/indexbench lang-pin -id glsl` at the pin below, build the blob with
`internal/extractor/tsengine/grammars/build.sh glsl`, write a dialect manifest claiming the extensions above with
`dialects/c/tags.scm` as the query's starting point, clone the subjects with `task calibrate-clone -- glsl` (the
committed `calibration/corpus.d/glsl.yaml`), and run `go run ./cmd/indexbench lang-oracle -dialect glsl -subject
workspace/calibration/glsl/<subject>` once per subject; each reading's `tenancy.files_with_error` is the count in the
table.

| Subject | Pin | Licence | Subpath or sparse | Files | With an error |
|---|---|---|---|---|---|
| SaschaWillems/Vulkan | `master` `1cbc8d87b731b438f47dea00a2e083ba7d59051d` | MIT | `shaders/glsl` | 353 | 32 (9.1%) |
| KhronosGroup/SPIRV-Cross | `vulkan-sdk-1.4.363.0` `f11ba9f0b21ba8fc15153d50a2a1ae31ab1cf8f7` | Apache-2.0 | `/shaders/`, minus `*.asm.*` and `*.invalid.*` | 306 | 153 (50.0%) |
| KhronosGroup/Vulkan-Samples | `main` `69622af06f7652c7fc9b9a9180e7142046315cbc` | Apache-2.0 | `shaders` | 309 | 87 (28.2%) |
| **the gate's corpus** | | | | **968** | **272 (28.1%)** |

SPIRV-Cross also holds 97 SPIR-V assembly files whose names use stage extensions (`.asm.vert` and the like); the
sparse pattern leaves them out, and no GLSL dialect reads them (the SPIR-V assembly product gap, disclosed here: a
`.spvasm`-style format is a separate item).

## What the grammar fails on

Counted by the first error node of each file (272 files, each in exactly one row), classified by the source line it
starts on:

| Construct | Example | Files |
|---|---|---|
| layout-only declaration | `layout(local_size_x = 8) in;`, `layout(early_fragment_tests) in;` | 157 (122 of them with no other error) |
| precision qualifier on a parameter or variable | `float jacobian(mediump vec2 dDdx, mediump vec2 dDdy)` | 86 (70 with no other error) |
| array-typed return or constructor | `vec4[2] test()`, `vec2[](...)` | 7 |
| a preprocessor line | one file | 1 |
| other | `vec4[] data;`, `float(consts.numSamples) * pdf`, `pervertexEXT in`, `tensorARM<float, 4>`, a `10s` literal | 21 |

Fixing the two big classes would not be enough: 77 files (8.0%) still carry an error outside them. The brief's note
that the grammar lacks `taskPayloadSharedEXT` is true as well (no corpus file reaches it first).

## Beside the gate

- The draft dialect read L1 `good` against `calibration/oracles/ctags-c-glsl` (universal-ctags 6.2.1's C parser run
  over GLSL, a foreign parser: tier C per owner ruling G1): sym_recall .912 (Vulkan), .920 (SPIRV-Cross), .878
  (Vulkan-Samples); sym_precision 1.0, .987 and 1.0; kind and range agreement 1.0. That reading is not committed:
  `level_met` over the three subjects is `none`, because of the gate above.
- Throughput is under the tenancy floor (1 MB/s per core) on two subjects: 0.66 MB/s (Vulkan), 0.43 MB/s (SPIRV-Cross,
  files average 577 bytes), 1.11 MB/s (Vulkan-Samples); the maximum per-file time is 6 to 17 ms. Small files, so the
  per-file cost dominates; not investigated
  further: the gate refuses first.
- L2 was not attempted: the glslang 16.6.0 call oracle (`glslang-ast`, ruling G2) was built from source (the release
  assets are not reachable from the lane's VM) but its adapter is not written, because a dialect that fails the
  gate is never dispatched.

## Retry

Re-plan when a tree-sitter-glsl commit or release, or another licensed GLSL grammar, parses the three subjects at the
pins of `calibration/corpus.d/glsl.yaml` with at most 5% of their files in error. Then: re-measure with this method,
pin the grammar (`lang-pin`, `build.sh`), restore the dialect (`tags.scm` from `dialects/c/tags.scm`: functions,
structs, `#define` macros, uniform/in/out variables and interface blocks, calls by callee text), write the
`glslang-ast` adapter (calls only, `Function Call:` nested in the tree before `Linked <stage> stage:`, the GLSL 4.60
built-in function index plus the type constructors as `call_exclusions`) and read the three subjects once.
