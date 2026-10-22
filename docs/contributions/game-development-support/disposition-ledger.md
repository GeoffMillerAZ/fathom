# Game development support disposition ledger

HEAD: `76f942a3319cee942ce9bc59a84e20cb8ead909b`. Local `origin/main` matches. Handoff: `237cb3ed8da3dca498e7df552917fd4bdfb4d2a7`.

31 units cover 33 payloads: main-0001..0027, O23-0001..0002, grouped P1..P3, and R1. P1 is Pyright core, P2 producer adapter, P3 manifests/locks. Their inventories and individual application checks are retained in JSON. Raw supplemental diffs have no From line; authorship is not inferred from copyright.

This is a proposed disposition, not adoption or qualification. JSON records paths, touched Go declarations, changed/affected tests, direct application diagnostics, per-unit HEAD evidence, pickaxe searches, licenses and risk explanations. Existing test names alone do not prove the fix. No tests, patches, commits, network calls, daemon writes or verification claims were made.

Flags: T = tenant/lifecycle/memory-pressure handling, B = MCP result budgeting, C = cancellation/deadlines, S = store table/column, N = automatic network fetch, O = consumer-specific reference. `-` means all false. P1..P3 T refers to the external producer memory guard, not the Fathom governor. Public dependency names and required notices are retained as provenance.

| Unit | Subject | Disposition | Reason (20 words maximum) | GDEV | Flags |
| --- | --- | --- | --- | --- | --- |
| main-0001 | feat(fathom): integrate current-source and native evidence | adapted | Generic decomposition required; native columns remain frozen and consumer default must become caller input. | GDEV-02, GDEV-05, GDEV-10, GDEV-11, GDEV-14, GDEV-21, GDEV-25, GDEV-27, GDEV-28, GDEV-30 | TBCSO |
| main-0002 | fix(verify): preserve the source-window result budget | adapted | Missing window protocol; preserve declared budget in final retrieval after generic implementation. | GDEV-05 | B |
| main-0003 | docs(changelog): keep source-window changes in one section | adapted | Absent branch-only fragment; fold single-heading correction into new source-window notes. | GDEV-05 | - |
| main-0004 | test(extractor): bound dialect parser admission | adopted | Parallel parser fanout remains unbounded in the existing test. | none | - |
| main-0005 | fix(mcp): load plan graph evidence only when required | adapted | Graph stage is still acquired before plan parsing; rebase demand selection. | GDEV-02, GDEV-25 | C |
| main-0006 | fix(hooks): report preflight expiry before root lookup | current-main equivalent | 2974c3153 already handles root-discovery deadlines with stronger attribution controls. | none | C |
| main-0007 | fix(store): share native payloads across newest test cases | adapted | Native payload columns are absent; deduplicate selected payloads only after approved native integration. | GDEV-02, GDEV-30 | - |
| main-0008 | fix(mcp): fit source-window task reports to complete budget | adapted | Window server is absent; reimplement complete saved-report accounting with generic windows. | GDEV-05 | B |
| main-0009 | perf(mcp): load only context navigation dependencies | adopted | Context still loads unnecessary graph variants; dependency-scoped patch applies. | GDEV-02 | C |
| main-0010 | perf(evidence): reuse bounded buffers during source validation | adapted | Receipt validators are absent; integrate reusable buffers during generic implementation. | GDEV-02, GDEV-05, GDEV-30 | C |
| main-0011 | perf(scip): release unused receipt index payloads | adapted | Receipt summary is absent; retain full protobuf validation while dropping unused payloads. | GDEV-02, GDEV-05 | C |
| main-0012 | fix(scip): avoid copying protobuf receipt mutexes | adapted | Fold pointer ownership into the adapted summary decoder; avoid introducing copylocks. | GDEV-02, GDEV-05 | - |
| main-0013 | test(fusion): isolate typed cache fixture mutations | adopted | Caching test still mutates its tracked fixture before cleanup. | none | - |
| main-0014 | fix(review): bound source reads and guard finding writes | adopted | Bounded review reads and pre-write identity/deadline fences are absent. | GDEV-03, GDEV-05 | C |
| main-0015 | fix(review): reuse coherent graph snapshot for review_diff | adapted | Coherent review bundle is absent; rebase after bounded reads and preserve publication fencing. | GDEV-02, GDEV-03, GDEV-05 | C |
| main-0016 | fix(review): use governed symbol snapshot loader | adapted | Fold governed snapshot loader into coherent review adaptation and preserve disclosure replay. | GDEV-02, GDEV-03 | - |
| main-0017 | fix(mcp): preserve review dimension status when details are elided | adopted | Dimension statuses are absent; pair with main-0018 before compilation. | GDEV-04 | B |
| main-0018 | test(mcp): remove unused review status import | adapted | New test file is absent; fold unused-import cleanup into main-0017. | GDEV-04 | - |
| main-0019 | fix(taskfile): recognize unnamed version-3 YAML Taskfiles | adopted | Unnamed version-three Taskfile detection is absent; adopt with main-0021. | GDEV-20 | - |
| main-0020 | perf(conserve): stream inventory inputs and honor cancellation | adopted | Conservation retains input bodies and lacks cancellation stops. | GDEV-03 | C |
| main-0021 | test(fusion): expect Taskfile classification for v3 task map | adopted | Legacy classifier expectation must change with main-0019. | GDEV-20 | - |
| main-0022 | fix(conserve): stream generated header classification | adapted | Streaming generated checks require main-0020 context-aware readers. | GDEV-03 | C |
| main-0023 | perf: avoid surface reads without extractors and reduce header buffers | adapted | Eligibility requires streaming accumulator and generated-reader prerequisites. | GDEV-03 | - |
| main-0024 | perf(callgraph): reduce transient ambiguity index allocations | adapted | Three temporary ambiguity count maps remain; adapt to current key loops. | GDEV-02 | - |
| main-0025 | feat: record memo statistics before pressure actions | adopted | Diagnostic is absent and applies cleanly; governor change held until RT-M1 PASS. | GDEV-01 | T |
| main-0026 | perf(store): reserve symbol projection capacity from bounded row count | adapted | Direct projection is absent; bound SQL count work as well as allocation capacity. | GDEV-02 | - |
| main-0027 | docs(plans): reopen unproven CX-18 claim | superseded | HEAD product directly calls traceRequirements; historical test-only wrapper premise is false. | GDEV-25 | - |
| O23-0001 | fix(mcp): bound review diff result envelope | adapted | Redacted reference requires generic whole-ToolResult accounting after outer disclosures. | GDEV-04 | BO |
| O23-0002 | test(mcp): classify raw JSON selector in forwarding scan | adopted | Concrete-selector classification applies; pair with rewritten O23 envelope cap. | GDEV-04 | - |
| P1..P3 | Python 3.13 SCIP producer: Pyright core, producer adapter, dependency manifests and locks | rejected | External fork bypasses memory reclamation and lacks clean dependency assembly; reject under ADR-047. | GDEV-12 | TC |
| R1 | Native Roslyn project declaration evidence (raw diff has no Subject header) | adopted | Native project declarations are absent; reuse bench and omit duplicate omnibus hunks. | GDEV-10, GDEV-11, GDEV-21 | - |

## Adopt first for P0

Selection order below includes rewrites and prerequisites. Apply no complete historical series. `adopted` units still require the normal integration gates later.

1. **GDEV-03:** main-0014 -> main-0015 + main-0016 -> main-0020 -> main-0022 -> main-0023. Establish bounded reads, coherent identity and cancellation before broad review runs.
2. **GDEV-04:** main-0017 + main-0018 -> generic O23-0001 + O23-0002. Remove the unused import before testing; cap both payload copies plus every outer disclosure at 16,000 bytes.
3. **GDEV-02:** main-0024 -> main-0005 -> main-0009 -> schema-free symbol-projection portion of main-0001 -> main-0026. Preserve governor policy and full corpus; bound COUNT work and measure actual cold footprint.
4. **GDEV-05:** generic source-window/SCIP receipt portions of main-0001 -> main-0002 + main-0003 + main-0008 -> main-0010 -> main-0011 + main-0012. Start only after separating schema/lifecycle changes; native receipt rollout remains dependent on the frozen native design.

main-0004 and main-0013 are low-scope validation prerequisites, not direct implementations of GDEV-02..08. R1 and main-0019/main-0021 serve native/tool qualification, not P0. No unit supplies the doctor fix (GDEV-06), current raw control receipts (GDEV-07), or review-specific end-to-end admission proof (GDEV-08). Existing HEAD admission/disconnect tests do not close those qualification gaps.

## Held until RT-M1

- **main-0001:** new `test_runs.native_run_id`, `test_runs.native_json`, `test_cases.native_symbol_key` in `internal/store/sqlite_schema.go` and `sqlite_testevidence.go`; also isolate and hold daemon lifecycle/retained-window memory changes. ADR-049 decision 2 freezes columns as well as tables.
- **main-0025:** `cmd/fathom/governor.go` changes work before upward shed/refuse actions. No threshold increase is proposed, but this remains a governor edit.
- **main-0007** and native-dependent parts of **main-0010:** indirect holds through main-0001 native evidence. main-0007 itself adds no table/column.
- **P1..P3:** rejected external producer memory-pressure bypass. This is not a Fathom governor edit, and RT-M1 PASS would not reverse rejection; require bounded retention, reproducible assembly and upstream notice review.

`docs/plans/responsible-tenant.md:279` remains open and `:294` records the 2026-10-19 FAIL. No live verdict was requested. Do not infer PASS from checked implementation rows.

## Base checks and handoff corrections

Both `2d30f9eb7da43cb921f2f22239c4d423beddf493` and `a22d36856a0f165a75c7a6810ec64cbcfc7a90e0` exist and are ancestors of HEAD (`cat-file` and `merge-base --is-ancestor` exit 0). The handoff commit matches the supplied pin. The checkout is detached at local origin/main; no remote was contacted.

Direct checks are per unit on unchanged HEAD. Hunk conflicts and absent prerequisites are distinguished in JSON. Many `--3way --check` attempts need Git-object writes denied by the read-only object store; those are operational limits, not conflict verdicts. O23-0002 succeeds by direct fallback. Neither redacted reference was submitted to git apply.

- **CX-18:** `internal/review/trace.go:243` invokes `traceRequirements` directly. The reopening patch describes a regression on the historical branch, not this HEAD. No verifier PASS is claimed.
- **Capacity count:** main-0026 uses `SELECT COUNT(*)` without LIMIT. The 131,072 cap bounds its eager allocation, not the number of rows counted.
- **Partial infrastructure:** HEAD already runs installed scip-python (`cmd/fathom/init_scip.go:71`) and consumes SCIP; Roslyn calls already use `MSBuildWorkspace` (`calibration/oracles/roslyn/SemanticCalls.cs:93`). Neither is the missing source-bound receipt/project-declaration implementation. No embedded Pyright/Roslyn/MSBuildWorkspace producer exists under `internal/` or `cmd/`.
- **Python notices:** both supplied license files carry identical MIT text with the Microsoft copyright notice. Keep them, verify the full replaced core/typeshed/producer notices locally before redistribution, and retain P3 dependency notices. Vendoring the recipe means importing full third-party source trees, not just four edited files.

Counts: adopted 11, current-main equivalent 1, adapted 17, superseded 1, rejected 1.

The dossier itself leaves cold arms, corrected full gate, O23 full gate and clean Python assembly open. No historical count or allocation result is promoted to current-main proof.
