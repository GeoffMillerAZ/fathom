# OpenCL C triage

This package measures native grammar parse diagnostics and probes one pinned compiler's AST capabilities. It registers no dialect, ships no grammar, adds no canonical corpus, and claims no language level. Implementation follows GLSL landing under G-R1. Pins and content digests are in [pins.json](pins.json).

Use an isolated Python environment with `tree-sitter==0.25.2`. All fetched sources, builds, raw ASTs, path manifests and logs stay under ignored `workspace/`. The caller must enforce the shared-machine load limit and nice 10. The examples use an existing `workspace/tools/python/bin/python`.

## Inputs and native grammar builds

Fetch the exact `archive_url` for each grammar in `pins.json`, verify its `archive_sha256` before extracting, and retain the supplied `src/parser.c` and header. Do not regenerate either candidate: this compares their committed upstream parsers. The dedicated grammar's top-level LICENSE is MIT; its package metadata says ISC, so the license conclusion uses the actual license file, whose digest is pinned. No Node package or binding is used.

Clone the two exact subject commits into `workspace/triage/opencl-r1/subjects/sdk` and `workspace/triage/opencl-r1/subjects/hashcat`. The SDK's `.reuse/dep5` applies Apache-2.0 to its files. Hashcat's license file is MIT. Full upstream source and attribution stay in the external checkouts, never in public receipts. The population rule was frozen before parsing: every tracked regular lowercase `.cl`, including include fragments, with no sampling or outcome exclusions. Uppercase variants, symlinks and unpopulated gitlinks are counted separately by the manifest.

For grammar IDs `opencl` and `c`, unpack under `workspace/triage/opencl-r1/grammars/<id>` and build:

```sh
clang -O2 -dynamiclib -fPIC \
  -I workspace/triage/opencl-r1/grammars/opencl/src \
  workspace/triage/opencl-r1/grammars/opencl/src/parser.c \
  -o workspace/triage/opencl-r1/grammars/opencl.dylib
clang -O2 -dynamiclib -fPIC \
  -I workspace/triage/opencl-r1/grammars/c/src \
  workspace/triage/opencl-r1/grammars/c/src/parser.c \
  -o workspace/triage/opencl-r1/grammars/c.dylib
```

These local dylibs are triage artifacts. Native success is not a WASM tenancy, fault or throughput proof.

## Complete-population parse screen

Run one subject at a time, with a fresh output directory:

```sh
workspace/tools/python/bin/python -I -B calibration/oracles/opencl-c/triage.py \
  --subject workspace/triage/opencl-r1/subjects/sdk \
  --commit 6090378a113db8dab20ae278c8fa57ab10b92300 \
  --grammar opencl=tree_sitter_opencl=workspace/triage/opencl-r1/grammars/opencl.dylib \
  --grammar c=tree_sitter_c=workspace/triage/opencl-r1/grammars/c.dylib \
  --out workspace/triage/opencl-r1/native-sdk
```

Repeat with subject `workspace/triage/opencl-r1/subjects/hashcat`, commit `6a7e3b5a96abcd6373c2e4112bc358abb24dc538`, and output `workspace/triage/opencl-r1/native-hashcat`.

The screen verifies HEAD and a clean tracked tree, captures exact `git ls-files --stage -z` manifests before and after, hashes regular bytes and symlink targets without following symlinks, and records gitlink commits without traversal. It also records untracked and ignored path identities before/after. The parser consumes the immutable bytes verified during capture, rather than reopening subject files during measurement. Every node is checked for ERROR and MISSING. Faulting files remain in the denominator. Only aggregate counts, rates, pins and hashes go in `summary.json`. Paths and diagnostic messages remain under `private/`. A native process crash is a failed screen, not zero faults or a publishable summary.

## Compiler capability probe

The installed compiler and two resource headers must match all three pinned digests. The command is reconstructed CL3.0 SPIR64 context, not a subject's original runtime device, build options or feature set. It is tier B. No output is a complete grading oracle yet.

```sh
workspace/tools/python/bin/python -I -B calibration/oracles/opencl-c/compiler_probe.py \
  --clang /Library/Developer/CommandLineTools/usr/bin/clang \
  --resource-dir /Library/Developer/CommandLineTools/usr/lib/clang/21 \
  --sdk workspace/triage/opencl-r1/subjects/sdk \
  --out workspace/triage/opencl-r1/compiler-r3
```

The subprocess uses `--no-default-config -x cl --target=spir64-unknown-unknown -cl-std=CL3.0 -fsyntax-only -w -Xclang -finclude-default-header -Xclang -ast-dump=json`. The authored [probe.cl](probe.cl) requires two function bodies with end ranges, one kernel, two resolved call sites (one source function and one external builtin), and one vector literal outside the call count. Apple Clang 21 emits `DeviceKernelAttr`; the classifier recognizes that exact spelling and `OpenCLKernelAttr`. SDK failures would stay in the ten-file denominator. The ten verified kernel byte strings are copied to a private read-only snapshot preserving relative paths before compiler invocation; there are no subject #include directives at this SDK pin. The authored fixture is likewise copied, and each private compiler input hash is checked before/after execution. External-call counts still need declaration-origin mapping before builtin exclusions or canonical truth are possible. Serialized AST IDs and paths can vary, so raw AST hashes identify these private runs and are not promised reproducible across processes.

Run the focused tests:

```sh
workspace/tools/python/bin/python -I -B calibration/oracles/opencl-c/test_triage.py
workspace/tools/python/bin/python -I -B calibration/oracles/opencl-c/test_compiler_probe.py
```
