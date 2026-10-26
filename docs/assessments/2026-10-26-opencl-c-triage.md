# OpenCL C triage and implementation proposal

G-R1 authorizes OpenCL C after GLSL, then Slang. This is the requested pre-landing triage and design. No dialect, grammar blob, canonical corpus or support level is added. The [queue proposal](../../calibration/oracles/opencl-c/queue-proposal.json) targets L2 on `.cl` only, conditional on GLSL landing and the implementation gates below.

## Inputs frozen before measurement

[Exact pins and digests](../../calibration/oracles/opencl-c/pins.json) describe two native grammar candidates and two complete external-kernel populations. Every tracked regular lowercase `.cl` is included, with no sampling, trimming or outcome exclusions. Symlink targets and gitlink commits are recorded without traversal. Per-path manifests, raw ASTs and diagnostics stay private. Public receipts contain only counts and hashes.

| Grammar candidate | Pin | License and maintenance |
|---|---|---|
| lefp/tree-sitter-opencl | `8e1d24a57066b3cd1bb9685bbc1ca9de5c1b78fb` | MIT license file; package metadata says ISC. Three-commit repository, 2023 HEAD; README explicitly disclaims full OpenCL C coverage. Helix uses this exact pin. |
| tree-sitter/tree-sitter-c v0.24.2 | `b780e47fc780ddc8da13afa35a3f4ed5c157823d` | MIT; maintained baseline, without an OpenCL layer in this comparison. |

The Khronos SDK pin is `6090378a113db8dab20ae278c8fa57ab10b92300`, ten external kernels, Apache-2.0 through its REUSE declaration. Hashcat is `6a7e3b5a96abcd6373c2e4112bc358abb24dc538`, 1,349 external kernels and include fragments, MIT. Their results remain separate so hashcat cannot swamp the reference examples.

Pocl `bfc71431d534d80432a92788a929f9a06b05d2c6` is a pinned diversity candidate, not measured here: its 1,016 `.cl` files mix builtin implementations, generated and negative tests requiring predeclared roles before a grade. OpenCL-CTS `16b197d191903abad0047e6bcb113167b589ecb4` and OpenCL-Guide `1269ba618ecab9f4feb6f8a083bbb0f4c63eb56d` have zero external `.cl` files in their exact recursive trees. Embedded/Markdown extraction would define separate populations and is outside this initial proposal.

## Native grammar comparison

| Population | Dedicated OpenCL clean | C baseline clean | Dedicated ERROR / MISSING only / both | C ERROR / MISSING only / both | Native faults |
|---|---:|---:|---:|---:|---:|
| SDK, all 10 files / 29,468 bytes | 9/10 (90%) | 0/10 (0%) | 1 / 0 / 0 | 10 / 0 / 0 | 0 for both |
| Hashcat, all 1,349 files / 24,761,953 bytes | 5/1,349 (0.37%) | 5/1,349 (0.37%) | 221 / 0 / 1,123 | 87 / 0 / 1,257 | 0 for both |

The ERROR-or-MISSING union is 1/10 versus 10/10 on the SDK, and 1,344/1,349 for both on hashcat. [SDK](data/opencl-c-triage-2026-10-26/native-sdk.json) and [hashcat](data/opencl-c-triage-2026-10-26/native-hashcat.json) receipts keep the complete denominators. These results make the dedicated grammar the more promising unmodified starting point, while neither candidate is ready to ship on these inputs. They do not establish a ceiling: a maintained C-based OpenCL extension and independently specified macro-context handling remain untried.

The SDK full tracked tree is 213 regular files / 2,263,956 bytes plus three gitlinks; hashcat is 4,437 regular files / 84,195,252 bytes. Neither has symlinks or untracked/ignored files in these runs. Exact manifest digests are in the receipts; submodule contents are explicitly uncovered.

This is an unmodified native-parser comparison, not the product's WASM gate, tenancy, throughput or language grade. Both candidates use their committed parser and own header without regeneration. Exact full tracked manifests and untracked/ignored path identities match before and after each run. Native parsing consumes captured verified bytes, and the compiler consumes a separate private read-only kernel snapshot preserving relative paths. A transient mutation of the original checkout cannot change those measured bytes. Hidden Git assume-unchanged/skip-worktree edits cannot pass: the screen independently verifies each raw file or symlink target against its indexed Git blob identity.

## Compiler capability and oracle design

Apple Clang 21.0.0 (`clang-2100.3.34.2`), CLT 27.0.0.0.1788430756, binary and OpenCL resource-header digests are pinned. [The committed probe](../../calibration/oracles/opencl-c/compiler_probe.py) invokes reconstructed CL3.0 SPIR64 JSON AST mode with the default OpenCL header. All ten SDK kernels compile: 23 function bodies, 19 kernel definitions, six source-function call sites and 149 external call sites, with zero unresolved call sites. These are AST capability counts, not measured extraction accuracy.

The authored fixture proves two function bodies with end ranges, one kernel, two resolved calls and one vector literal outside the call count. Apple Clang uses `DeviceKernelAttr`; the reader accepts that exact kind and `OpenCLKernelAttr`. A focused test first failed on the actual spelling, then passed after normalization.

This route is tier B. A source file does not preserve its runtime device, feature set, concatenated source inputs or `clBuildProgram` options. SPIR enables broad functionality, so a successful reconstructed parse is not proof of the subject's own build. A grading adapter must map expansion/spelling locations and declaration origins, distinguish builtin/external declarations from source-written calls, count errored files, prove explicit zero-call scope, and pass extractor-perturbation plus known-input-change probes before any L2 claim. Hashcat requires a separately evidenced include/macro build-context reconstruction; it was not silently compiled with guessed flags.

Khronos supplies the specification and conformance tests, rather than one canonical OpenCL C frontend. Native runtime compilation through `clBuildProgram` remains a candidate if its device/options/context can be pinned. The [compiler receipt](data/opencl-c-triage-2026-10-26/compiler.json) records these observed capabilities. The [search receipt](../../calibration/oracles/opencl-c/SEARCH.md) records toolchain, SCIP/LSIF, language-server, registry and runtime routes, including misses and untried candidates. No ceiling is asserted.

## Implementation boundary

ADR-048's data-only dialect and independent-reading rules apply. I19 requires public evidence privacy, I23 governs function references, I24 requires disclosed dropped targets and I25 requires readable-file accounting. No applicable revisit trigger is shown to have fired. Extraction remains deterministic, with no decision model or new runtime service.

After GLSL lands, select a grammar using WASM and oracle-backed results, then implement a manifest/query plus authored fixtures. `.cl` is currently unclaimed: Common Lisp deliberately omits it, and Cool also uses it. Require positive OpenCL/C evidence, explicit Lisp/Cool negative fixtures and ambiguous-content refusal. Do not add `.clh`, C++ for OpenCL or embedded source yet. Preserve header/include-fragment tenancy even when they have no standalone translation-unit truth. Predeclare any generated/negative roles before measurement. Report per-subject grammar and oracle coverage, three-run median throughput, faults and per-file latency before READY. Main queue activation and plan proof stay with the controller.

## Reproduction and tests

The [triage README](../../calibration/oracles/opencl-c/README.md) gives runnable native builds, complete-population screens, pinned compiler invocation and focused tests. Public receipts contain no subject authors, email addresses, usernames, copyright holders or file inventories. Tests cover exact ERROR/MISSING unions and fault accounting, private-path separation, raw Git blob identity under assume-unchanged/skip-worktree flags, and compiler-pin enforcement even under optimized Python. Three hidden-tampering cases failed before the identity guard, then passed after restoration. Replacing the explicit compiler-pin exception with an `assert` makes the optimized-entrypoint test fail; restoring it passes the focused compiler suite. A transient-mutation test proves native parsing still consumes captured bytes, and the compiler snapshot test proves later source changes cannot alter copied inputs. All twelve triage tests and four compiler tests pass. These are harness integrity probes, not the independent-oracle mutation proof still required for a future grading adapter. Required X-R2/X-R3 checks run before review readiness; generated aggregate files remain untouched.
