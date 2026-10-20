### Added

- CUE (`.cue`) reaches L3 (L1 and L2 excellent, L3 good), measured against the cue binary on cue-lang/cue, NetMeta and fathom's own schemas: an import path resolves to the files of the package CUE loads for it (the importing module's packages, then `cue.mod/gen`, `pkg` and `usr`; the standard library and registry dependencies bind nothing), and the stored cross-file references load an import's files by the same rules.
