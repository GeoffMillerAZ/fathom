# Kotlin producer/reader candidate audit (X-R1)

Measured baseline: [Kotlin reading](../bench/readings/lang/kotlin.json). No new Kotlin index or reading was produced. The bundled reader saw empty legacy occurrence ranges; raw typed-range fields remain unaudited.

| Candidate or route | Version / pin | Checked result and concrete limit |
|---|---|---|
| Kotlin PSI dump | kotlin-compiler-embeddable 2.3.20 | The committed reading records its observed definitions, calls and imports. It joins syntax rather than resolving compiler semantics, so its tier-B truth caps L2/L3. |
| Earlier Kotlin SCIP attempt | scip-java 0.13.1; plugin Kotlin 2.2.0; Exposed 1.3.0 | The plugin rejected the subject's 2.3.20 compiler. An explicitly modified build indexed 166/794 files; the reader saw 58,698 empty legacy ranges. This does not establish the contents of newer typed-range wire fields. The old index is not available in this lane. |
| New Kotlin producer | official scip-java 0.13.2, SHA-256 `5248136592d19ea2c468f64ebd18587d32de7c961536ff19346f8a025af1d13c` | The official launcher reports `0.0.0-SNAPSHOT`; its nested artifact is 0.13.2 with Kotlin plugin 2.4.21. Release includes Kotlin PR #973 and updated SCIP bindings. On pinned Exposed, the direct route failed at `buildSrc:compileKotlin`; after a successful pristine buildSrc warmup, the bounded indexing route failed at `exposed-core:compileKotlin`. Both report missing `org.jetbrains.kotlin.extensions.ExtensionPointDescriptor`. No SCIP index was emitted, so this is a producer/compiler compatibility failure, not a range result. |
| Fathom SCIP occurrence reader | main 5b84aa5b4; bundled legacy fields 1 and 7 | `internal/langbench/scip.go` uses GetRange; the bundled proto lacks typed occurrence range fields 8 and 9 and enclosing fields 10 and 11 that already exist in SCIP 0.9. Raw unknown fields have not been counted on the old/new Kotlin index. A reader blind spot remains a hypothesis, not a demonstrated cause. |
| Android-backed subject build | okhttp parent-5.4.0 | Prior own-build attempt required Android SDK. Replacing it with toy source would not prove the committed corpus. |

Primary evidence: [scip-java 0.13.2 release](https://github.com/scip-code/scip-java/releases/tag/v0.13.2),
[Kotlin producer PR #973](https://github.com/scip-code/scip-java/pull/973), and
[SCIP 0.9 occurrence schema](https://github.com/scip-code/scip/blob/v0.9.0/scip.proto).
The release is evidence that a candidate exists, not proof that its emitted index repairs
this reading. A maintainer's test addition is not a calibration result.

A [sanitized protocol result](data/2026-10-26-core-t1-kotlin-producer.json) records the two failed routes, the successful pristine warmup, source identity and log digests. The next trigger is a producer artifact compatible with pinned Exposed or separately scoped compiler-plugin compatibility work; source/compiler downgrade is not an authorized shortcut. Only after an index exists can a raw wire census of range fields 1, 8 and 9 and enclosing fields 7, 10 and 11 test the reader hypothesis. Any reader change is an ENGINE wave with all affected SCIP readings regraded. No producer absence or exhausted-tool ceiling is
claimed. Ruling C-R5 (#20, 2026-10-26T03:14:37Z) scheduled this bounded audit after Go closed. Both prescribed producer routes are now measured.

Governing row: `LB-Q-kotlin-rest`. No extractor, reading, corpus, holdout, epoch, or generated board changes.
