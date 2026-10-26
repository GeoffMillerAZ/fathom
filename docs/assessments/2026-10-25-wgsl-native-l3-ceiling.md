# WGSL: native L3 oracle ceiling

Assessment date: 2026-10-25. Source inspection finds no native import relation to grade in standard WGSL. Existing module resolvers target separate language populations. No new grading run is claimed. Decision: `LB-Q-wgsl-rest`.

The [committed reading](../bench/readings/lang/wgsl.json) remains the authority for support levels and measurements.

## Candidate inventory

Source inspection on 2026-10-25 covers standard WGSL as defined by the [dated 2026-10-21 W3C snapshot](https://www.w3.org/TR/2026/CRD-WGSL-20261021/), which has no source-file import relation. The pinned subjects contain no import directives. No new L3 oracle run was made.

| Candidate and pin | Concrete ceiling on the existing corpus |
|---|---|
| [Naga WGSL frontend 40f4a34e](https://github.com/gfx-rs/wgpu/blob/40f4a34ebaf56f9a046231f54125ad046239d3f3/naga/src/front/wgsl/mod.rs) | `parse_str` accepts one source string and returns one module without a source-file resolver. It cannot establish cross-file import truth for standard WGSL. |
| [Dawn Tint 628398941dd5](https://dawn.googlesource.com/dawn/+/628398941dd561970ead2c924015d287b78e3fac/src/tint/lang/wgsl/reader/reader.cc) | `Parse` accepts one `Source::File` and resolves one program; `WgslToIR` lowers that program to one module. This entry point has no source-file resolver, so it supplies no cross-file import truth for standard WGSL. |
| [wgsl-parse 0.6.0](https://docs.rs/crate/wgsl-parse/0.6.0) | The existing L2 oracle parses WGSL. Its sibling WESL compiler resolves imports, but this parser does not add imports to standard WGSL or supply native L3 truth. |
| [wgsl-analyzer 2026-09-30](https://github.com/wgsl-analyzer/wgsl-analyzer/releases/tag/2026-09-30) | Its [README at this release](https://github.com/wgsl-analyzer/wgsl-analyzer/blob/2026-09-30/README.md) identifies WESL support as experimental. The release lists import-resolution and module-graph work, but standard WGSL has no imports and the pinned subjects contain none; WESL needs a separate population for L3 grading. |
| [WESL 0.6.0](https://docs.rs/crate/wesl/0.6.0) | Its resolver handles imports in `.wesl`. That is a separate language population requiring its own queue item, pinned corpus and independent resolution truth. |
| [naga_oil 0.23.0](https://docs.rs/crate/naga_oil/0.23.0) | Composer resolves `#import` and `#define_import_path` shader modules. Those directives are outside standard WGSL and absent from the pinned subjects, so they require a separate corpus and queue item. |
| [wgsl-macro 0.4.0](https://docs.rs/crate/wgsl-macro/0.4.0) | Its `#import` and conditional directives are a host preprocessor language, outside standard WGSL and absent from the pinned subjects. Its resolver could grade a separate preprocessed corpus, not native L3 here. |
| [wgsl-tmpl 0.1.0](https://docs.rs/crate/wgsl-tmpl/0.1.0) | Its `*.tmpl.wgsl` templates and include fragments are a separate source format; the pinned plain WGSL subjects contain no template include relation to grade. |
| [wgsl_preprocessor 1.1.4](https://docs.rs/crate/wgsl_preprocessor/1.1.4) | Its `//!include` directive adds a host-side file relation that standard WGSL does not have and the pinned subjects do not exercise. |

## Route search receipt

All sources and searches below were read 2026-10-25. A missing search hit means no candidate was found by that search, not that none exists.

| Source category | Search and disposition |
|---|---|
| Own compiler/runtime dump and introspection | The [dated WGSL specification](https://www.w3.org/TR/2026/CRD-WGSL-20261021/) defines one shader module with no import statement. [Naga's pinned frontend](https://github.com/gfx-rs/wgpu/blob/40f4a34ebaf56f9a046231f54125ad046239d3f3/naga/src/front/wgsl/mod.rs) parses one string; [Tint's pinned reader](https://dawn.googlesource.com/dawn/+/628398941dd561970ead2c924015d287b78e3fac/src/tint/lang/wgsl/reader/reader.cc) parses one source file. Their compiler output can inspect a module but has no native source-file import edge to dump. |
| SCIP/LSIF indexers | The [Sourcegraph recommended indexer list](https://sourcegraph.com/docs/code-navigation/writing-an-indexer), [precise-navigation list](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), and [SCIP project tooling list](https://github.com/scip-code/scip/blob/main/README.md) list no WGSL indexer. GitHub repository searches for [`scip-wgsl`](https://github.com/search?q=scip-wgsl&type=repositories) and [`lsif-wgsl`](https://github.com/search?q=lsif-wgsl&type=repositories) yielded no identified WGSL source-file resolver. Index serialization would not create an absent language relation. |
| Language server | [wgsl-analyzer 2026-09-30](https://github.com/wgsl-analyzer/wgsl-analyzer/blob/2026-09-30/README.md) supports WESL experimentally, but that changes the source population; see candidate row. |
| Package registries: parser/typechecker/AST dump | Rust crate listings for [Naga at 40f4a34e](https://github.com/gfx-rs/wgpu/blob/40f4a34ebaf56f9a046231f54125ad046239d3f3/naga/src/front/wgsl/mod.rs), [wgsl-parse 0.6.0](https://docs.rs/crate/wgsl-parse/0.6.0), [WESL 0.6.0](https://docs.rs/crate/wesl/0.6.0), [naga_oil 0.23.0](https://docs.rs/crate/naga_oil/0.23.0), [wgsl-macro 0.4.0](https://docs.rs/crate/wgsl-macro/0.4.0), [wgsl-tmpl 0.1.0](https://docs.rs/crate/wgsl-tmpl/0.1.0), and [wgsl_preprocessor 1.1.4](https://docs.rs/crate/wgsl_preprocessor/1.1.4) were checked. Parsers provide syntax; the resolvers target import/include extensions with separate corpus requirements. |
| Runtime call/require tracing | [WebGPU's shader-module descriptor](https://gpuweb.github.io/types/interfaces/GPUShaderModuleDescriptor.html) takes WGSL source code. A host may concatenate or preprocess files before this call, but the [dated WGSL language](https://www.w3.org/TR/2026/CRD-WGSL-20261021/) cannot trace native requires/imports because it has none. The preprocessor candidates above can trace their own directives in a separate population. |

The L3 contract requires independently measured import resolution and file-edge precision/recall. Their native import denominator here is zero. [Owner ruling D19](https://github.com/GeoffMillerAZ/fathom/issues/7), `MinL3Evidence = 20` ([round-1 source](https://github.com/GeoffMillerAZ/fathom/blob/a9c3c367c/internal/langbench/reading.go#L250), landing in issue #7's round 2), requires at least 20 references in each graded metric's own denominator. This cites the pending ruling, not an enforcement claim about main. Zero imports cannot become perfect resolution, and calls inside one file cannot be relabelled cross-file edges. No naga L3 grading run is claimed.

WESL and naga_oil are outside `LB-Q-wgsl`. Grading those supersets under this item would change its proof population.

Retry trigger: an owner-ruled separate queue entry for WESL (`.wesl`, wesl-rs resolver) or naga_oil `#import` shaders, with its own pinned corpus holding at least 20 independent references in each graded L3 denominator (D19). A future standard WGSL import facility would also reopen the native question. Proposed neighbors, after the already approved OpenCL C and Slang work: WESL first (existing oracle family), naga_oil modules second. This is a proposal, not queue authorization or a support claim.
