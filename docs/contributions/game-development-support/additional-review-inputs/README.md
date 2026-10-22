# Additional review inputs

These drafts are separate from both historical Fathom series in the parent manifest. They preserve useful work, not qualified current-main changes. Never execute or apply them as part of consumer bootstrap.

## Python 3.13 producer

Read [the original scoped recipe](python313/README.md) and [provenance](python313/provenance.json). The three patches target Sourcegraph scip-python 0.6.6 at 8b60bbce1f2a4c7a517776cb395bbafb2e731e4f plus Microsoft Pyright 1.1.390 at 2f28cd390b534ad6647d190e05e9c57f20f6a3a1. Sourcegraph and Microsoft licenses are retained beside them. Reconcile with current upstream before choosing whether to upstream, replace or retire this adapter.

Observed local evidence: TypeScript compiler, bundle and one focused Jest assertion passed. Ten native Python import/ownership cases passed. Python 3.13.16/Pillow 13.0.0 emitted 108/108 selected documents with 46,103 occurrences and no missing or extra documents. The raw private SCIP SHA256 was 2499126beea7cf41a23dfc653788d32c16e923c317ebab4a77846f537f38e2eb. Private corpus and raw output are not included. Clean assembly replay, broad Jest, supported dependency install, memory and L4 excellence remain OPEN. The earlier peer-dependency conflict required --legacy-peer-deps in the local trial; do not treat that as a production remedy.

## Native Roslyn declaration oracle

[The generic patch](roslyn-project-declarations.patch) is based on historical Fathom f844af51-era work. It adds actual MSBuildWorkspace project-mode source evidence and native positive/negative fixtures. A private trial read 135 Unity sources across 11 projects. This establishes a bounded historical reading, not current compiler equivalence, a scorer grade, current-main compatibility or a new full-gate pass. Check existing current-main equivalents before integration. Fathom license and in-source notices apply.

## Blender API facts

A separate native Blender 5.2.2 factory-startup probe observed 109 API paths: 53 present, 52 context-unknown and four addon-unavailable. Those are lookup facts only. They do not establish typed calls, operator poll success, installed addons, model quality, animation or production integration. Use them to shape independent native witnesses for GDEV-12 and GDEV-29, then rerun against the selected Blender build.

All payloads here are review artifacts, not executable installations. The maintainer owns source/license review, generic reproductions, independent native oracles, normal gates and release qualification. Parent SHA256SUMS covers this folder.
