# WGSL: native L3 oracle ceiling

Assessment date: 2026-10-25. Source inspection finds no native import relation to grade in standard WGSL. Existing module resolvers target separate language populations. No new grading run is claimed. Decision: `LB-Q-wgsl-rest`.

The [committed reading](../bench/readings/lang/wgsl.json) remains the authority for support levels and measurements.

Standard WGSL has no source-file import relation. The [WGSL specification snapshot of 2026-10-21](https://www.w3.org/TR/2026/CRD-WGSL-20261021/) defines a shader program in one module. At the calibration pin `40f4a34ebaf56f9a046231f54125ad046239d3f3`, [naga's WGSL frontend](https://github.com/gfx-rs/wgpu/blob/40f4a34ebaf56f9a046231f54125ad046239d3f3/naga/src/front/wgsl/mod.rs) accepts one source string, parses it, builds its local index, and lowers it to one module. The API takes no source-file resolver. Validation of that module does not establish cross-file imports. The pinned subjects contain no import directives.

The L3 contract requires independently measured import resolution and file-edge precision/recall. Their native import denominator here is zero. [Owner ruling D19](https://github.com/GeoffMillerAZ/fathom/issues/7), `MinL3Evidence = 20` ([round-1 source](https://github.com/GeoffMillerAZ/fathom/blob/a9c3c367c/internal/langbench/reading.go#L250), landing in issue #7's round 2), requires at least 20 references in each graded metric's own denominator. This cites the pending ruling, not an enforcement claim about main. Zero imports cannot become perfect resolution, and calls inside one file cannot be relabelled cross-file edges. No naga L3 grading run is claimed.

Existing free resolvers already cover non-standard module formats:

- [WESL 0.6.0](https://docs.rs/crate/wesl/0.6.0), from the same project and version as the existing `wgsl-parse` oracle, supports import statements and a compiler resolver for `.wesl` sources.
- [naga_oil 0.23.0](https://docs.rs/crate/naga_oil/0.23.0) provides Composer with `#import` and `#define_import_path` shader modules.

Both were checked on 2026-10-25. They are outside `LB-Q-wgsl`: standard `.wgsl` has no imports, and the pinned subjects contain none. Grading those supersets under this item would change its proof population.

Retry trigger: an owner-ruled separate queue entry for WESL (`.wesl`, wesl-rs resolver) or naga_oil `#import` shaders, with its own pinned corpus holding at least 20 independent references in each graded L3 denominator (D19). A future standard WGSL import facility would also reopen the native question. Proposed neighbors, after the already approved OpenCL C and Slang work: WESL first (existing oracle family), naga_oil modules second. This is a proposal, not queue authorization or a support claim.
