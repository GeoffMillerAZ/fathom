# cue named ceiling routes (X-R1)

The committed reading is unchanged: level L3, grades L1 excellent, L2 excellent, L3 good, L4 not-attempted.

| Candidate or route | Version / rule | Checked evidence and concrete limit |
|---|---|---|
| CUE language server behind vet gate | cue v0.16.1 | Existing tier-A oracle resolves import paths and cross-file definitions. import_resolution1 and file precision1 already prove those; recall .4898 is a shared package-key target problem, not absent oracle resolution. |
| Per-file import targets | main5b84aa5b4 cue-module/file-edge key model | Package files share one entity key and scoring reads one file. A per-file key or owner package-to-files rule is not implemented. This changes shared identity/scoring and needs ENGINE evidence and affected regrades. |
| SCIP writer over LSP definitions | cue v0.16.1 protocol; writer unbuilt | The LSP already has file/line answers, but no adapter writes a SCIP index for this reading. Building that producer is an explicit engineering route, not a claim that CUE lacks semantic information. |

Retain the written per-file-target and LSP-writer triggers. No change to shared package identity or L4 grades is proposed.

Governing row: `LB-Q-cue-rest`. No extractor, reading, corpus, holdout, epoch, or generated board changes.
