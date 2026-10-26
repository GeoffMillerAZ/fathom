# Kotlin producer/reader audit (C-R5, X-R1, X-R4)

The [committed reading](../bench/readings/lang/kotlin.json) remains L3 good. No new index or grade was produced. Two scip-java 0.13.2 routes failed before indexing the unchanged subject. Separately, **source-read evidence establishes a reader incompatibility**: scip-java 0.13.1 and 0.13.2 Kotlin writers use typed occurrence/enclosing ranges; Fathom's bundled proto and readers understand only legacy fields. Empty legacy arrays never established that the producer omitted ranges. This bounded audit claims no exhausted-tool ceiling.

| Candidate or route | Version / pin | Evidence and concrete limit |
|---|---|---|
| Kotlin PSI dump | kotlin-compiler-embeddable 2.3.20 | Existing definitions/calls/import reading; syntax/name joins do not establish compiler reference bindings. |
| Earlier SCIP attempt | scip-java 0.13.1, plugin 2.2.0 | Rejected Exposed's Kotlin 2.3.20. A modified build indexed 166/794 files; its 58,698 empty legacy arrays were reader output. Tagged writer uses typed-only ranges. Old index unavailable here; translated input proves no native grade. |
| Official producer, two routes | scip-java 0.13.2, SHA-256 `5248136592d19ea2c468f64ebd18587d32de7c961536ff19346f8a025af1d13c` | Launcher reports `0.0.0-SNAPSHOT`, nested artifact 0.13.2. Direct route failed at buildSrc; pristine warmup passed, then exposed-core failed. Both lacked ExtensionPointDescriptor; no index. Its `testKotlin240` uses compiler 2.4.21, outside Exposed 2.3.20's version. |
| Fathom reader | bundled proto at main 5b84aa5b4, legacy fields 1/7 | Lacks SCIP 0.9 fields 8/9/10/11. `internal/langbench/scip.go` and other SCIP readers consume GetRange/GetEnclosingRange. Requires ENGINE before a typed-only index is usable. |
| Upstream replacement | scip-java PR #963, open/unmerged at audit | Analysis API route removes compiler-plugin binary coupling. Not in the pinned release; no released artifact measured. Its merge/release is a checkable retry event. |
| Mozilla plugin fork | mozsearch/semanticdb-kotlinc 0.11.0 | Targets Kotlin 2.4.20, so it does not provide a matching 2.3.20 plugin. No run claimed. |
| Official Kotlin language server | Kotlin/kotlin-lsp `kotlin-lsp/v263.7084.0` | Vendor-maintained semantic server, possible tier-A L3 reference-bindings route under P-T1. Export adapter and full-corpus measurement unbuilt; remains live, not rejected. |
| Older Kotlin server | fwcd/kotlin-language-server, deprecated repository | Upstream directs users to the official server; not an independently exhausted route. |
| Official Analysis API / compiler IR dumps | Kotlin 2.3.20 subject; current vendor API/docs | Reference/type interface and compiler dump flags exist. Source-location conversion and whole-project measurement unbuilt. No absence or tier ruling inferred. |
| Native newer subject | preregistered Kotlin 2.4+ project, not selected | Could match a plugin without downgrading source. Requires a separate corpus decision and full native measurement. No substitute grade claimed. |
| Android-backed subject | okhttp parent-5.4.0 | Existing own-build attempt needs Android SDK. Toy input would not prove that corpus. |
| JVM runtime tracing | JaCoCo Gradle plugin / JVM tooling, version not pinned | Executed-code evidence omits unexecuted static references and declarations. Not an L4 whole-corpus replacement; no run claimed. |

## Source-read reader finding (F1)

Tagged producer source writes single/multi-line occurrence ranges and single/multi-line enclosing ranges at [0.13.1 lines 104-113](https://github.com/scip-code/scip-java/blob/v0.13.1/scip-kotlinc/src/main/kotlin/org/scip_code/scip_java/kotlinc/ScipTextDocumentBuilder.kt#L104-L113) and [0.13.2 lines 112-121](https://github.com/scip-code/scip-java/blob/v0.13.2/scip-kotlinc/src/main/kotlin/org/scip_code/scip_java/kotlinc/ScipTextDocumentBuilder.kt#L112-L121). The [SCIP 0.9 schema](https://github.com/scip-code/scip/blob/v0.9.0/scip.proto) defines those fields. The bundled proto lacks them; reader calls at `internal/langbench/scip.go` use only legacy getters. This establishes the compatibility gap from source, without pretending that a real-index census ran.

A reader ENGINE wave must update the bundled schema, implement typed precedence, retain legacy compatibility, verify source alignment in each document's position encoding, and regrade **every affected SCIP reading**, including Java, Rust and C-family consumers. Run the committed wire census on the first real Kotlin index as confirmation. The census's sign/order checks alone do not establish source alignment.

## X-R4 search sources (2026-10-26 UTC)

Each required category was searched, including non-hits. These searches identify candidates; they do not replace native measurements.

| Required source searched | Query / source | Result |
|---|---|---|
| Own compiler/runtime | [Kotlin compiler options](https://kotlinlang.org/docs/compiler-reference.html), [vendor Analysis API](https://github.com/Kotlin/analysis-api), tagged producer sources above | IR dump flags and semantic API exist; not evaluated as a corpus oracle. Existing PSI dump remains syntax truth. |
| SCIP lists and GitHub search | [SCIP indexers](https://github.com/scip-code/scip#readme), [Sourcegraph recommendations](https://sourcegraph.com/docs/code-navigation/writing-an-indexer#sourcegraph-recommended-indexers); GitHub/web query `scip-kotlin` | scip-java is listed; archived [scip-kotlin](https://github.com/sourcegraph/scip-kotlin) was merged into it. PR #963 and Mozilla fork are distinct candidates above. |
| LSIF list | Search `site:github.com/sourcegraph/lsif-indexers Kotlin`; historical [Sourcegraph indexer list](https://github.com/sourcegraph/sourcegraph/blob/main/doc/code_intelligence/references/indexers.md) | Search exposed the historical lsif-java lineage; current recommendations point to scip-java. No separate maintained Kotlin LSIF artifact identified by this search. Historical URL availability is not a measured producer failure. |
| Language servers | [official releases](https://github.com/Kotlin/kotlin-lsp/releases), [fwcd server](https://github.com/fwcd/kotlin-language-server) | Official v263.7084.0 candidate remains live; fwcd deprecates its server. |
| Package registry | [Maven Central compiler-embeddable 2.3.20](https://central.sonatype.com/artifact/org.jetbrains.kotlin/kotlin-compiler-embeddable/2.3.20), [Mozilla 0.11.0 publication](https://bugzilla.mozilla.org/show_bug.cgi?id=2069987) | Compiler/PSI/Analysis API route and Mozilla plugin found. Neither provides an already measured replacement reading here. |
| Runtime call/require tracing | [Gradle JaCoCo documentation](https://docs.gradle.org/current/userguide/jacoco_plugin.html), JVM runtime-tooling route | Coverage observes executed code. No static whole-corpus oracle established; no dynamic completeness claim. |

The [0.13.2 test configuration](https://github.com/scip-code/scip-java/blob/v0.13.2/scip-kotlinc/build.gradle.kts#L29-L38) pins 2.4.21. [PR #973](https://github.com/scip-code/scip-java/pull/973) points to [PR #963](https://github.com/scip-code/scip-java/pull/963); the latter is the explicit Analysis API replacement, not a generic request for compatibility. Upstream integration-test success does not establish index payload accuracy.

## Measured routes and next triggers

The [sanitized receipt](data/2026-10-26-core-t1-kotlin-producer.json) records counts, hashes, exit codes, caps, command templates and minimal task/error frames. The [protocol](../bench/kotlin-producer-reader-protocol.md) and committed bounded runner reproduce the method. Original private logs remain in scratch; no subject content or file lists are committed.

Retry L4 when **both** the reader ENGINE wave above is integrated and a scip-java release containing #963 (or a compiler-plugin matrix including Kotlin 2.3.x) exits 0 on unchanged Exposed 1.3.0's exposed-core/dao/jdbc modules. Then census and align the real index before grading. Separately, evaluate official Kotlin LSP L3 bindings, or preregister a native Kotlin 2.4+ subject. Do not downgrade source/compiler. Governing row `LB-Q-kotlin-rest` records these triggers. C-R5 (#20, 2026-10-26T03:14:37Z) authorized the bounded audit; F1/F2 correct its reader conclusion and candidate inventory. No engine, epoch, corpus, holdout or grade changes.
