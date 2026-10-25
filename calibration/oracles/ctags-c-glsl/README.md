# ctags-c-glsl — tier-C GLSL L1 oracle (LB-Q-glsl, lane glsl)

`ctags_c_glsl.py` runs universal-ctags 6.2.1's **C parser** over every GLSL file under a subject (the extensions of
the draft dialect: `.glsl .vert .frag .comp .geom .tesc .tese .rgen .rchit .rmiss .rahit .rint .rcall .mesh .task
.vsh .fsh`, each mapped with `--map-C=+.ext`) and writes the neutral oracle file `indexbench lang-oracle -tool-oracle`
reads (`definitions`, tier `C`). ctags 6.2.1 has no GLSL parser, so this is a foreign parser over a C-like language,
not GLSL's own tool: tier C, never B, and L1 caps at `good` (owner ruling G1, 2026-10-24). Only function, struct and
macro are written (the kinds a GLSL dialect requires); members, variables and prototypes are counted in `disclose`.
A function's, struct's and macro's `end` is ctags' own `end:` field. Bench-only, never a product dependency.

    # universal-ctags v6.2.1 (GPL-2.0, run as a separate process): ./autogen.sh && ./configure --prefix=<tools>/ctags-inst
    #   --disable-xml --disable-json --disable-yaml && make && make install
    python3 -I ctags_c_glsl.py --ctags <tools>/ctags-inst/bin/ctags <subject> out/glsl-<subject>-defs.json
    # SaschaWillems/Vulkan shaders/glsl: 353 files, 580 definitions; SPIRV-Cross shaders: 306 files, 564;
    # Vulkan-Samples shaders: 309 files, 672

The L2 half of the item (`glslang-ast`, glslang 16.6.0's AST dump, calls only) is not written: LB-Q-glsl stopped at
the grammar gate (docs/bench/readings/lang/glsl-grammar.md).
