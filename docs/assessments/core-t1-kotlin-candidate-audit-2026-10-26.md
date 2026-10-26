# Kotlin producer/reader candidate audit (X-R1)

The committed reading remains L3 good. This wave has not produced a new Kotlin index or
changed its grades. The earlier statement that scip-java necessarily wrote every occurrence
without a range is stronger than the evidence: the bundled reader saw empty legacy fields.

| Candidate or route | Version / pin | Checked result and concrete limit |
|---|---|---|
| Kotlin PSI dump | kotlin-compiler-embeddable 2.3.20 | Existing L1 excellent, L2/L3 good. It joins syntax rather than resolving compiler semantics, so its tier-B truth caps L2/L3. |
| Earlier Kotlin SCIP attempt | scip-java 0.13.1; plugin Kotlin 2.2.0; Exposed 1.3.0 | The plugin rejected the subject's 2.3.20 compiler. An explicitly modified build indexed 166/794 files; the reader saw 58,698 empty legacy ranges. This does not establish the contents of newer typed-range wire fields. The old index is not available in this lane. |
| New Kotlin producer | scip-java 0.13.2; Kotlin plugin/compiler 2.4.21 | Release includes Kotlin PR973 and updated SCIP bindings. Not runtime-indexed here. It is a named open candidate, not a failed oracle. Exact compatibility with the pinned Exposed build remains unmeasured. |
| Fathom SCIP occurrence reader | main 5b84aa5b4; bundled legacy fields1/7 | `internal/langbench/scip.go` uses GetRange; the bundled proto lacks typed occurrence range fields8/9 and enclosing fields10/11 that already exist in SCIP0.9. Raw unknown fields have not been counted on the old/new Kotlin index. A reader blind spot remains a hypothesis, not a demonstrated cause. |
| Android-backed subject build | okhttp parent-5.4.0 | Prior own-build attempt required Android SDK. Replacing it with toy source would not prove the committed corpus. |

Primary evidence: [scip-java0.13.2 release](https://github.com/scip-code/scip-java/releases/tag/v0.13.2),
[Kotlin producer PR973](https://github.com/scip-code/scip-java/pull/973), and
[SCIP0.9 occurrence schema](https://github.com/scip-code/scip/blob/v0.9.0/scip.proto).
The release is evidence that a candidate exists, not proof that its emitted index repairs
this reading. A maintainer's test addition is not a calibration result.

The next bounded experiment must first count raw fields1/8/9 and10/11, then distinguish
producer failure from reader incompatibility. Any reader change is an ENGINE wave with
all affected SCIP readings regraded. No producer absence or exhausted-tool ceiling is
claimed. Owner Q5 is asked to prioritize this audit against finishing Go in this wave.
