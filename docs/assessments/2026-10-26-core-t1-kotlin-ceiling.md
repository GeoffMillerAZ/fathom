# Kotlin producer/reader candidate audit (X-R1)

Measured baseline: [Kotlin reading](../bench/readings/lang/kotlin.json). No new Kotlin index or reading was produced. The bundled reader saw empty legacy occurrence ranges; raw typed-range fields remain unaudited.

| Candidate or route | Version / pin | Checked result and concrete limit |
|---|---|---|
| Kotlin PSI dump | kotlin-compiler-embeddable 2.3.20 | The committed reading records its observed definitions, calls and imports. It joins syntax rather than resolving compiler semantics, so its tier-B truth caps L2/L3. |
| Earlier Kotlin SCIP attempt | scip-java 0.13.1; plugin Kotlin 2.2.0; Exposed 1.3.0 | The plugin rejected the subject's 2.3.20 compiler. An explicitly modified build indexed 166/794 files; the reader saw 58,698 empty legacy ranges. This does not establish the contents of newer typed-range wire fields. The old index is not available in this lane. |
| New Kotlin producer | scip-java 0.13.2; Kotlin plugin/compiler 2.4.21 | Release includes Kotlin PR #973 and updated SCIP bindings. Not runtime-indexed here. It is a named open candidate, not a failed oracle. Exact compatibility with the pinned Exposed build remains unmeasured. |
| Fathom SCIP occurrence reader | main 5b84aa5b4; bundled legacy fields 1 and 7 | `internal/langbench/scip.go` uses GetRange; the bundled proto lacks typed occurrence range fields 8 and 9 and enclosing fields 10 and 11 that already exist in SCIP 0.9. Raw unknown fields have not been counted on the old/new Kotlin index. A reader blind spot remains a hypothesis, not a demonstrated cause. |
| Android-backed subject build | okhttp parent-5.4.0 | Prior own-build attempt required Android SDK. Replacing it with toy source would not prove the committed corpus. |

Primary evidence: [scip-java 0.13.2 release](https://github.com/scip-code/scip-java/releases/tag/v0.13.2),
[Kotlin producer PR #973](https://github.com/scip-code/scip-java/pull/973), and
[SCIP 0.9 occurrence schema](https://github.com/scip-code/scip/blob/v0.9.0/scip.proto).
The release is evidence that a candidate exists, not proof that its emitted index repairs
this reading. A maintainer's test addition is not a calibration result.

The next bounded experiment must first count raw range fields 1, 8 and 9 plus enclosing fields 7, 10 and 11, then distinguish
producer failure from reader incompatibility. Any reader change is an ENGINE wave with
all affected SCIP readings regraded. No producer absence or exhausted-tool ceiling is
claimed. Ruling C-R5 (#20, 2026-10-26T03:14:37Z) schedules this audit after Go closes.

Governing row: `LB-Q-kotlin-rest`. No extractor, reading, corpus, holdout, epoch, or generated board changes.
