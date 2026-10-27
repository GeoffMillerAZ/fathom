# cue named ceiling routes (X-R1)

Measured baseline: [cue reading](../bench/readings/lang/cue.json). No new reading is claimed.

| Candidate or route | Version / rule | Checked evidence and concrete limit |
|---|---|---|
| CUE language server behind vet gate | cue v0.16.1 | Existing tier-A oracle resolves import paths and cross-file definitions. import_resolution 1 and file precision 1 already prove those; recall .4898 is a shared package-key target problem, not absent oracle resolution. |
| Per-file import targets | main 5b84aa5b4 cue-module/file-edge key model | Package files share one entity key and scoring reads one file. A per-file key or owner package-to-files rule is not implemented. This changes shared identity/scoring and needs ENGINE evidence and affected regrades. |
| SCIP writer over LSP definitions | cue v0.16.1 protocol; writer unbuilt | The LSP already has file/line answers, but no adapter writes a SCIP index for this reading. Building that producer is an explicit engineering route, not a claim that CUE lacks semantic information. |

Retain the written per-file-target and LSP-writer triggers. No change to shared package identity or the committed reading is proposed.

Governing row: `LB-Q-cue-rest`. No extractor, reading, corpus, holdout, epoch, or generated board changes.

## X-R4 five-source audit

Search date: 2026-10-26 UTC. The [Sourcegraph precise-navigation list](https://sourcegraph.com/docs/code-navigation/precise-code-navigation), [SCIP indexer catalog](https://github.com/scip-code/scip#readme), legacy LSIF repository searches ([TS/JS example](https://github.com/sourcegraph/lsif-node), [Go example](https://github.com/sourcegraph/lsif-go)), and GitHub repository searches for `scip-<language>` were checked. These are bounded catalogs, not evidence that an indexer cannot exist. Runtime traces cover executed paths only and cannot substitute for complete static source truth. No new subject or oracle was run here.

| Source class | Searched source and disposition |
|---|---|
| Own toolchain / compiler | [CUE LSP documentation](https://github.com/cue-lang/cue/wiki/LSP%3A-Getting-started) describes package/import analysis; registry [cuelang.org/go v0.17.1](https://proxy.golang.org/cuelang.org/go/@latest) is newer than the measured v0.16.1. New version alone does not repair package-key identity. |
| SCIP / LSIF | Shared catalogs and [GitHub `scip-cue` search](https://github.com/search?q=scip-cue&type=repositories) yielded no measured writer. The existing LSP-to-SCIP writer route remains engineering work. |
| Language server | [Official CUE LSP](https://github.com/cue-lang/cue/wiki/LSP%3A-Getting-started) exposes navigation, with documented dynamic-field/evaluation limits. **LIVE UNMEASURED** for the shared per-file key repair, not absent semantic data. |
| Registry AST | [CUE `ast.Ident.Node`](https://pkg.go.dev/cuelang.org/go/cue/ast) is a resolved-node hook; no evidence it alone produces independent package-file targets. |
| Runtime tracing | [cue eval/vet](https://cuelang.org/docs/) evaluates configurations, not a complete static cross-file reference inventory. |

The measured reading and governing row above are unchanged; a registry or catalog hit alone changes no grade.
