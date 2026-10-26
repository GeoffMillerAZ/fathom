# cue named ceiling routes (X-R1)

Measured baseline: [cue reading](../bench/readings/lang/cue.json). No new reading is claimed.

| Candidate or route | Version / rule | Checked evidence and concrete limit |
|---|---|---|
| CUE language server behind vet gate | cue v0.16.1 | Existing tier-A oracle resolves import paths and cross-file definitions. import_resolution 1 and file precision 1 already prove those; recall .4898 is a shared package-key target problem, not absent oracle resolution. |
| Per-file import targets | main 5b84aa5b4 cue-module/file-edge key model | Package files share one entity key and scoring reads one file. A per-file key or owner package-to-files rule is not implemented. This changes shared identity/scoring and needs ENGINE evidence and affected regrades. |
| SCIP writer over LSP definitions | cue v0.16.1 protocol; writer unbuilt | The LSP already has file/line answers, but no adapter writes a SCIP index for this reading. Building that producer is an explicit engineering route, not a claim that CUE lacks semantic information. |

Retain the written per-file-target and LSP-writer triggers. No change to shared package identity or L4 grades is proposed.

Governing row: `LB-Q-cue-rest`. No extractor, reading, corpus, holdout, epoch, or generated board changes.
