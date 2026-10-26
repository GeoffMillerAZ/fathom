# WGSL: native L3 oracle ceiling

Assessment date: 2026-10-25. No support level is raised.

The committed `docs/bench/readings/lang/wgsl.json` proves L2 good: 2,573 oracle call pairs found, call recall and precision 1.0, tier B. The target remains L2. L3 import resolution and file-edge metrics are null.

Standard WGSL has no source-file import relation. The [WGSL specification](https://www.w3.org/TR/WGSL/) defines a shader program in one module. At the calibration pin `40f4a34ebaf56f9a046231f54125ad046239d3f3`, [naga's WGSL frontend](https://github.com/gfx-rs/wgpu/blob/40f4a34ebaf56f9a046231f54125ad046239d3f3/naga/src/front/wgsl/mod.rs) accepts one source string, parses it, builds its local index, and lowers it to one module. The API takes no source-file resolver. Validation of that module does not establish cross-file imports.

The existing L3 contract requires independently measured import resolution and file-edge precision/recall. Their native import denominator here is zero. Under [owner ruling D19](https://github.com/GeoffMillerAZ/fathom/issues/7), at least 20 references are needed in each graded metric's own denominator. Zero imports cannot become perfect resolution, and calls inside one file cannot be relabelled cross-file edges. No naga L3 grading run is claimed.

Ceiling: retain L2 good on the existing reading. Retry when a standard WGSL import facility or a separately scoped host module format has an independent resolver and at least 20 references in each graded L3 denominator. Host-side concatenation, Rust module imports and preprocessor extensions require their own scope, corpus and oracle. They do not raise standard WGSL's level.
