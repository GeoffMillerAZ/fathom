# wgsl — provenance

- Grammar: `include-yy/tree-sitter-wgsl` (MIT), commit `b58422ee34029f81ae8b7dd44fc14d043809b321` (the repository publishes no
  release tags), ABI 14, external scanner (nested block comments), grammar at the repository root (`tree_sitter_wgsl`), built
  into `internal/extractor/tsengine/grammars/wgsl.wasm.gz` (183,727 bytes raw, 66,803 bytes gzip). Pin and digests in
  `internal/extractor/tsengine/grammars/grammars.lock.json`. Licence read at the pinned commit: the top-level `LICENSE` (MIT).
  The other WGSL grammars were not chosen: `szebniok/tree-sitter-wgsl` and
  `tree-sitter-grammars/tree-sitter-wgsl-bevy` read the pre-template WGSL.
- `tags.scm`: written for this repository (the grammar ships only `queries/highlights.scm`), from the node names `lang-probe`
  prints: `function_decl`, `struct_decl`, `global_variable_decl`, `global_const_decl`, `global_override_decl`, `type_alias_decl`
  and `call_expression`. `const` and `override` are both `WgslConstant`. A global declaration's own match carries
  `@calls.from`, so a call in its initializer (`const c = vec3<f32>(1.0)`) is the global's.
- Calls: the callee's name without template arguments (`vec4<f32>(...)` is `vec4`, `array<f32, 4>(...)` is `array`);
  builtins and type constructors are calls, as the oracle counts them. A call belongs to the innermost function, or to the
  global declaration whose initializer holds it.
- Templates are resolved by GLR conflicts in the grammar, with no discovery scanner, so `a < b` can read as a template list.
  Across the calibration subjects 13 of 343 files carry a syntax error (3.8%); the fixtures include a comparison.
- Product limit: a template nested thousands deep (wgpu's `naga/tests/naga/deep-template.wgsl`, a negative test naga's own front end
  rejects) takes tree-sitter's GLR parse 6.1 to 8.1 s on the measuring VM, and once exceeded the 10 s guest budget. The file is
  excluded from the calibration subjects (`calibration/corpus.d/wgsl.yaml`) and disclosed in the reading; a template-nesting
  ingest guard is follow-up work.
- Not extracted, on purpose: parameters, struct members, locals, `enable` / `requires` / `diagnostic` directives and
  `const_assert`. WGSL has no imports, so there is no `imports.scm` and the target is L2.
- Oracle (calibration/oracles/README.md): `wgsl-parse` 0.6.0 (MIT OR Apache-2.0, feature `naga-ext`), an independent Rust
  parser, tier B for definitions and calls (ADR-048 amendment D2: ctags 6.2.1 has no WGSL parser).
