### Added

- WGSL (`.wgsl`) extraction at L2 (L1 excellent, L2 good), measured against wgsl-parse 0.6.0 (an independent syntactic parser, tier B) on gfx-rs/wgpu and webgpu-samples: functions, structs, global variables, constants (`const` and `override`) and type aliases, and the calls a function or a global's initializer makes (a builtin or a type constructor is a call, named without its template arguments). Imports do not exist in WGSL, so the level stops at L2.

### Known limits

- WGSL: a template nested thousands deep (wgpu's `deep-template.wgsl`, a negative test its own front end rejects) takes 6 to 8 s to parse, and once exceeded the 10 s per-file budget. It is outside the measured subjects, so the WGSL reading covers every file of the remaining calibration set with no restriction; a template-nesting ingest guard is follow-up work.
