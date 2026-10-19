# F82 line-overlap replay, 2026-09-19 to 2026-10-19

## Verdict

F82's Promote-when fires on its replay half, by a wide margin. Of 523 merges on `origin/main` in the 30-day window, 263 conflict when recomputed, and 63 of those (51 outside test files) have two lines changing the same function or type. The threshold is 4 a month. F88's threshold also fires: 247 conflicted merges hit append points against a threshold of 4. Neither Drop-if is near (< 1).

Read it with three caveats. (1) The count is a lower bound: git sees a shared symbol, not "line A changes `Open`, line B adds a caller of `Open`". (2) Some of the 63 are the same patch landing from two sides, which inflates it. (3) The brief's premise was partly stale: no 2026-10-16 tool-bytes reading exists in the repository, and F88 is not defined in `docs/intent` (O1333). The reading lives in `docs/bench/readings/`, where this repository keeps readings.

Signal for the owner: append points outnumber semantic overlaps about 4 to 1, but a semantic overlap (same function edited on both sides) is the expensive one, and 51 non-test merges in 30 days is a real cost. Both futures qualify. Neither is built by this reading.

## Counts

Window: merge commits reachable from `origin/main` (6c9c8ff6e) with committer date 2026-09-19 through 2026-10-19, which is exactly 30 days, so the window is also "a month" for the thresholds. Calendar months below are partial (September 12 days, October 19 days).

Unit: a merge counts once per class if at least one of its conflicted files is in that class.

| month | conflicted merges | with SEMANTIC (any Go file) | with SEMANTIC (non-test Go) | with APPEND-POINT | with ADJACENT | with OTHER |
|---|---|---|---|---|---|---|
| 2026-09 | 121 | 37 | 29 | 109 | 7 | 30 |
| 2026-10 | 142 | 26 | 22 | 138 | 5 | 25 |
| window | 263 | 63 | 51 | 247 | 12 | 55 |

| class | conflicted files |
|---|---|
| ADJACENT | 15 |
| APPEND-POINT | 396 |
| OTHER | 98 |
| SEMANTIC | 141 |

Merges in window: 523. Conflict-free on recompute: 260. Conflicted: 263. Octopus merges: 0. Errors: 0.

## Thresholds

| future | line | measured (30 days) | result |
|---|---|---|---|
| F82 | >= 4 true SEMANTIC overlaps a month promotes, < 1 drops, else neither | 63 merges (51 non-test), 141 conflicted files | promotes |
| F88 | >= 4 APPEND-POINT conflicts a month promotes, < 1 drops, else neither | 247 merges, 396 conflicted files | promotes |

The per-calendar-month rates are higher than the 30-day rate would suggest only because the months are partial; both months clear 4 on their own (SEMANTIC 37 and 26 merges, APPEND-POINT 109 and 138).

## Red integrations and live same-file edits

Source: `workspace/durable-2026-09-30/DECISIONS-2026-10-11-PAUSE.md` (read-only) and the 174 logs in `/Volumes/m2ext/tmp/fathom-build/gate-ext/` (2026-10-09 to 2026-10-19; 22 contain a FAIL).

- Pin-shift reds, caused by one line moving text another line's fixture cites by line number: 4 documented (DECISIONS lines for the record pass wf_3d2e4f67, iv2r, rc1, rel16), and 3 have a gate log whose failing test names the shifted line (`iv2r-1`, `rc1-2`, `rel16-1`: `TestHomeCorpusTruthAtHead`, `TestGuardsCorpusDetectorsExist`). This is APPEND-POINT-like (position coupling), not semantic.
- Semantic combination reds with evidence: 0. Two logs are undetermined, not claimed: `war2-1` (`TestDefaultSurfaceSchemaBytesBudget`, 20,834 bytes against a 20,800 budget) and `lang2-2` (`TestCompileErrorsGroupByRootAndEditSet/rich`). The rest of the 22 are single-line lint or test failures (`TestTenancyRules` x6), timing or load flakes (`guardsm1` x3, `hintflake`, `lf5-2`, `lf7-1`, `szr`), or missing isolation wiring (`fd2-1`), none attributed to combining two lines.
- Live same-file concurrent edits in one checkout: 0 expected, not measured. Why 0: every wave worker edits its own clone under `/Volumes/m2ext/tmp/fathom-build/waves/<wave>/<label>` (`docs/guides/agent-waves.md`) and never an owner checkout. No log exists that could measure it. The documented exception is a sibling project that commits to the owner tree (`docs/intent/autonomy.md`), which is outside this window's merge data.

## Method

1. List merges: `git log --merges --since=2026-09-19 --until=2026-10-20 origin/main` (committer dates). Git 2.54.0 (needs >= 2.38).
2. For each merge with exactly two parents p1, p2: `git merge-tree --write-tree --name-only p1 p2`. Exit 0 is clean, exit 1 lists conflicted files, and the printed tree holds the conflicted blobs.
3. Class each conflicted file:
   - APPEND-POINT: path matches CHANGELOG.md, docs/languages.md and other generated boards, docs/plans/*.md, observation or ADR or F or I files, docs/changelog, generated Go (`Code generated` header), or a Go `var`/`const` block (registry, dispatch table, ExtractorEpoch) changed on both sides.
   - SEMANTIC: a Go `func` or `type` that exists at the merge base and whose lines are changed (or, for an insertion, whose body receives one) by BOTH `git diff -U0 base p1` and `git diff -U0 base p2`. The script TSV (its default output) carries the shared function names per file.
   - ADJACENT: Go file conflicted but no shared symbol (both sides add next to each other, or add/add of one new file). Not counted toward either threshold.
   - OTHER: non-Go text in the same region (cue, json, README, tmpl, css). Not counted toward either threshold. These may include undetected append points, so APPEND-POINT is also a lower bound.
4. Count per merge and per month from the TSV.

Reproduce: `python3 -I docs/bench/readings/f82-overlap-replay.py > out.tsv` (about 40 s), then the same script with `summary` or `table` reading `out.tsv` on stdin. Zero model spend, no daemon, git only. Output is sorted and deterministic.

Limits. A merge of `origin/main` into a branch and the later merge of that branch into main are separate events; the second is normally clean, so double counting is rare but not excluded. A symbol changed identically on both sides (a patch landed twice) is counted as SEMANTIC. Callers are not examined, so "line B adds a caller of what line A changed" is invisible here, which is exactly the part only the index could see.

## Every conflict

| merge | date | class | files |
|---|---|---|---|
| 0e18ed8a1 | 2026-09-19 | APPEND-POINT | `.fathom/observations.md`, `CHANGELOG.md`, `docs/plans/substrate.md` |
| 12c7e9cab | 2026-09-19 | APPEND-POINT | `.fathom/observations.md`, `CHANGELOG.md`, `docs/languages.md`, `docs/plans/substrate.md` |
| 12c7e9cab | 2026-09-19 | SEMANTIC | `cmd/fathom/ingest.go`, `cmd/fathom/ingest_json.go` |
| 60bb4847b | 2026-09-19 | SEMANTIC | `cmd/fathom/verify.go` |
| 656489462 | 2026-09-19 | APPEND-POINT | `CHANGELOG.md` |
| 6af03a3cd | 2026-09-19 | APPEND-POINT | `.fathom/observations.md`, `CHANGELOG.md` |
| 7f3de8df4 | 2026-09-19 | APPEND-POINT | `CHANGELOG.md`, `docs/languages.md`, `docs/plans/substrate.md` |
| d2a4cf730 | 2026-09-19 | APPEND-POINT | `CHANGELOG.md` |
| 3f36decde | 2026-09-22 | APPEND-POINT | `docs/plans/decision-port.md` |
| 66de6ea6f | 2026-09-22 | APPEND-POINT | `docs/mcp-tools.md` |
| 53ec55e51 | 2026-09-23 | APPEND-POINT | `docs/plans/test-certifications.md` |
| 96d735530 | 2026-09-23 | SEMANTIC | `internal/fusion/typedcalls.go` |
| 9b40a07aa | 2026-09-23 | APPEND-POINT | `docs/plans/test-certifications.md` |
| b41ab04ea | 2026-09-23 | APPEND-POINT | `.fathom/observations.md`, `CHANGELOG.md` |
| b41ab04ea | 2026-09-23 | SEMANTIC | `internal/store/sqlite.go` |
| bf4eb1cd1 | 2026-09-23 | APPEND-POINT | `ADR.md`, `docs/intent/decisions/README.md` |
| c9556a69b | 2026-09-23 | APPEND-POINT | `CHANGELOG.md`, `docs/plans/test-certifications.md` |
| 2a18ebb4c | 2026-09-24 | APPEND-POINT | `CHANGELOG.md` |
| 3b2906c74 | 2026-09-24 | OTHER | `.fathom/config.toml`, `docs/usage.md` |
| 3b2906c74 | 2026-09-24 | APPEND-POINT | `CHANGELOG.md`, `docs/plans/refactor-conservation.md` |
| 3b2906c74 | 2026-09-24 | SEMANTIC | `cmd/fathom/review_test.go`, `internal/analysis/review/engine.go`, `internal/mcp/review_tool_test.go` |
| 3b2906c74 | 2026-09-24 | ADJACENT | `internal/config/schema_test.go` |
| 3cee27ace | 2026-09-24 | APPEND-POINT | `CHANGELOG.md` |
| 4a205f62d | 2026-09-24 | APPEND-POINT | `CHANGELOG.md` |
| 59319aa61 | 2026-09-24 | APPEND-POINT | `.fathom/observations.md`, `CHANGELOG.md` |
| 64b226344 | 2026-09-24 | APPEND-POINT | `CHANGELOG.md` |
| 668e7cbec | 2026-09-24 | APPEND-POINT | `CHANGELOG.md`, `docs/plans/refactor-conservation.md` |
| 7f41dd281 | 2026-09-24 | APPEND-POINT | `ADR.md` |
| 7f41dd281 | 2026-09-24 | OTHER | `docs/design/refactor-conservation.md` |
| 834c3a6f5 | 2026-09-24 | APPEND-POINT | `docs/plans/refactor-conservation.md` |
| 8f0495f67 | 2026-09-24 | APPEND-POINT | `CHANGELOG.md` |
| c2d48ab31 | 2026-09-24 | APPEND-POINT | `CHANGELOG.md` |
| c8830bf96 | 2026-09-24 | APPEND-POINT | `CHANGELOG.md` |
| e6c215b84 | 2026-09-24 | APPEND-POINT | `CHANGELOG.md` |
| b0e357b77 | 2026-09-25 | APPEND-POINT | `CHANGELOG.md` |
| cf1ca7af7 | 2026-09-25 | OTHER | `docs/intent/scope.md` |
| 14f7a4a3d | 2026-09-26 | SEMANTIC | `cmd/fathom/console_fixture_test.go`, `cmd/fathom/console_registry.go`, `cmd/fathom/console_router.go`, `cmd/fathom/console_test.go`, `cmd/fathom/servehttp.go` |
| 14f7a4a3d | 2026-09-26 | OTHER | `cmd/fathom/ui/app.css`, `docs/app-kit.md`, `docs/usage.md` |
| 43a7c4fb7 | 2026-09-26 | APPEND-POINT | `docs/plans/responsible-tenant.md` |
| 4a50256b2 | 2026-09-26 | APPEND-POINT | `CHANGELOG.md` |
| 4a50256b2 | 2026-09-26 | SEMANTIC | `cmd/fathom/console_calllog.go`, `cmd/fathom/console_fixture_test.go`, `cmd/fathom/console_project.go`, `cmd/fathom/console_registry.go`, `cmd/fathom/console_router.go`, `cmd/fathom/console_test.go`, `internal/console/metrics.go` |
| 4a50256b2 | 2026-09-26 | OTHER | `cmd/fathom/ui/app.css`, `cmd/fathom/ui/console.js`, `docs/app-kit.md`, `docs/usage.md` |
| 648186a3a | 2026-09-26 | SEMANTIC | `cmd/fathom/selfrestart.go`, `cmd/fathom/selfrestart_test.go`, `cmd/fathom/servehttp.go` |
| 648186a3a | 2026-09-26 | APPEND-POINT | `docs/plans/responsible-tenant.md` |
| 7288e4e31 | 2026-09-26 | APPEND-POINT | `CHANGELOG.md` |
| 7288e4e31 | 2026-09-26 | SEMANTIC | `cmd/fathom/certify_status.go`, `cmd/fathom/console_panel.go`, `cmd/fathom/console_test.go`, `internal/certify/gaps.go` |
| 7288e4e31 | 2026-09-26 | OTHER | `cmd/fathom/ui/app.css`, `docs/app-kit.md`, `docs/usage.md` |
| 87cf1f6b0 | 2026-09-26 | APPEND-POINT | `CHANGELOG.md`, `docs/plans/responsible-tenant.md` |
| 87cf1f6b0 | 2026-09-26 | SEMANTIC | `cmd/fathom/daemon.go`, `cmd/fathom/servehttp.go` |
| 87cf1f6b0 | 2026-09-26 | ADJACENT | `cmd/fathom/origin.go` |
| 87cf1f6b0 | 2026-09-26 | OTHER | `docs/usage.md` |
| 92b13169f | 2026-09-26 | APPEND-POINT | `CHANGELOG.md` |
| 92b13169f | 2026-09-26 | OTHER | `appkit.adoption.json`, `cmd/fathom/ui/app.css`, `docs/app-kit.md`, `docs/usage.md` |
| 92b13169f | 2026-09-26 | SEMANTIC | `cmd/fathom/console_fixture_test.go`, `cmd/fathom/console_test.go`, `cmd/fathom/harnesscmd.go` |
| ca4518043 | 2026-09-26 | APPEND-POINT | `CHANGELOG.md`, `cmd/fathom/verbs.go` |
| ca4518043 | 2026-09-26 | SEMANTIC | `cmd/fathom/console_registry.go`, `cmd/fathom/console_router.go`, `cmd/fathom/console_test.go`, `cmd/fathom/main.go`, `cmd/fathom/metrics_traffic.go`, `cmd/fathom/observatory.go` |
| ca4518043 | 2026-09-26 | OTHER | `cmd/fathom/ui/app.css`, `docs/app-kit.md`, `docs/usage.md` |
| d1d924412 | 2026-09-26 | SEMANTIC | `cmd/fathom/servehttp.go` |
| d1d924412 | 2026-09-26 | OTHER | `docs/usage.md` |
| e3732b145 | 2026-09-26 | APPEND-POINT | `docs/plans/responsible-tenant.md` |
| f1d6241a2 | 2026-09-26 | SEMANTIC | `cmd/fathom/edithook.go`, `internal/semantic/serve.go`, `pkg/client/client.go`, `pkg/client/semantic.go` |
| f1d6241a2 | 2026-09-26 | APPEND-POINT | `docs/mcp-tools.md`, `docs/plans/responsible-tenant.md` |
| 10387948f | 2026-09-27 | APPEND-POINT | `CHANGELOG.md`, `docs/mcp-tools.md` |
| 10387948f | 2026-09-27 | SEMANTIC | `cmd/fathom/doctor.go`, `cmd/fathom/governor_rpc.go`, `cmd/fathom/servehttp.go`, `internal/mcp/dispatch.go` |
| 10387948f | 2026-09-27 | OTHER | `docs/usage.md` |
| 216a11a0f | 2026-09-27 | APPEND-POINT | `docs/languages.md` |
| 6654280b9 | 2026-09-27 | SEMANTIC | `cmd/fathom/console_calltail.go`, `cmd/fathom/governor.go`, `cmd/fathom/governor_wiring.go`, `cmd/fathom/servehttp.go`, `internal/store/sqlite.go` |
| 6654280b9 | 2026-09-27 | APPEND-POINT | `docs/plans/responsible-tenant.md` |
| 95ea867ed | 2026-09-27 | SEMANTIC | `cmd/fathom/console_plansdata.go`, `cmd/fathom/console_plansdata_test.go` |
| 95ea867ed | 2026-09-27 | ADJACENT | `internal/gitro/head.go`, `internal/gitro/head_test.go` |
| ae67b7a48 | 2026-09-27 | APPEND-POINT | `CHANGELOG.md`, `docs/plans/responsible-tenant.md` |
| ae67b7a48 | 2026-09-27 | SEMANTIC | `cmd/fathom/doctor.go`, `cmd/fathom/governor.go` |
| ae67b7a48 | 2026-09-27 | ADJACENT | `cmd/fathom/doctor_footprint.go` |
| b2742f84f | 2026-09-27 | APPEND-POINT | `CHANGELOG.md`, `docs/plans/bench-pyramid.md`, `docs/plans/expert-suite-r.md`, `docs/plans/findings-tracer.md`, `docs/plans/layer-catalog.md`, `docs/plans/m1-manifest.md`, `docs/plans/o8-precision-and-speclines.md`, `docs/plans/responsible-tenant.md` |
| b2742f84f | 2026-09-27 | OTHER | `docs/intent/invariants.md`, `docs/usage.md` |
| b2742f84f | 2026-09-27 | SEMANTIC | `internal/fusion/pipeline.go`, `internal/fusion/reingest_file.go`, `internal/mcp/autorefresh.go`, `internal/mcp/autorefresh_test.go`, `internal/mcp/extra_tools.go`, `internal/mcp/server.go`, `internal/store/memory.go`, `internal/store/purge.go`, `internal/store/sqlite.go` |
| b2742f84f | 2026-09-27 | ADJACENT | `internal/mcp/export_test.go` |
| c004f0ffc | 2026-09-27 | SEMANTIC | `cmd/fathom/main.go` |
| c2b6965af | 2026-09-27 | SEMANTIC | `internal/extractor/tsengine/tsengine.go` |
| c77f2c6b9 | 2026-09-27 | SEMANTIC | `cmd/fathom/daemon_lifecycle.go`, `cmd/fathom/daemon_lifecycle_test.go` |
| c77f2c6b9 | 2026-09-27 | ADJACENT | `cmd/fathom/killswitch.go` |
| c77f2c6b9 | 2026-09-27 | APPEND-POINT | `docs/plans/responsible-tenant.md` |
| c77f2c6b9 | 2026-09-27 | OTHER | `docs/usage.md` |
| 10ff509ab | 2026-09-28 | APPEND-POINT | `.fathom/observations.md`, `docs/plans/efficiency-wave.md` |
| 12f61706a | 2026-09-28 | APPEND-POINT | `docs/languages.md` |
| 1a90db2e1 | 2026-09-28 | APPEND-POINT | `CHANGELOG.md` |
| 1e87fffa2 | 2026-09-28 | APPEND-POINT | `CHANGELOG.md` |
| 259090b17 | 2026-09-28 | APPEND-POINT | `CHANGELOG.md` |
| 303b2bdb7 | 2026-09-28 | APPEND-POINT | `docs/languages.md`, `docs/plans/language-breadth.md` |
| 303b2bdb7 | 2026-09-28 | SEMANTIC | `internal/fusion/pipeline.go` |
| 3420672da | 2026-09-28 | APPEND-POINT | `docs/languages.md`, `docs/plans/language-breadth.md` |
| 3420672da | 2026-09-28 | SEMANTIC | `internal/fusion/pipeline.go` |
| 531abf980 | 2026-09-28 | APPEND-POINT | `docs/intent/decisions/README.md` |
| 540c93eed | 2026-09-28 | APPEND-POINT | `docs/plans/governance-learning.md` |
| 585f31f46 | 2026-09-28 | OTHER | `internal/extractor/tsengine/grammars/PROVENANCE.md` |
| 585f31f46 | 2026-09-28 | SEMANTIC | `internal/extractor/tsengine/tsengine_test.go`, `internal/fusion/resolvestrategy_test.go` |
| 5d73eceac | 2026-09-28 | APPEND-POINT | `docs/languages.md` |
| 5d73eceac | 2026-09-28 | OTHER | `internal/extractor/tsengine/grammars/PROVENANCE.md` |
| 5d73eceac | 2026-09-28 | SEMANTIC | `internal/extractor/tsengine/tsengine_test.go` |
| 65f22b05f | 2026-09-28 | APPEND-POINT | `.fathom/observations.md` |
| 6674b551a | 2026-09-28 | APPEND-POINT | `docs/languages.md` |
| 6674b551a | 2026-09-28 | OTHER | `internal/extractor/tsengine/grammars/PROVENANCE.md` |
| 6674b551a | 2026-09-28 | SEMANTIC | `internal/extractor/tsengine/tsengine_test.go` |
| 67b537808 | 2026-09-28 | APPEND-POINT | `CHANGELOG.md`, `docs/plans/governance-learning.md`, `docs/plans/progress-and-economics.md`, `docs/plans/work-intelligence.md` |
| 67b537808 | 2026-09-28 | OTHER | `testdata/scenariobench/findings-loop.json` |
| 686240333 | 2026-09-28 | APPEND-POINT | `docs/intent/decisions/README.md` |
| 765a6502b | 2026-09-28 | APPEND-POINT | `CHANGELOG.md`, `docs/languages.md`, `docs/plans/language-breadth.md`, `internal/fusion/pipeline.go` |
| 7bd02273e | 2026-09-28 | APPEND-POINT | `docs/mcp-tools.md` |
| 88c961ed5 | 2026-09-28 | APPEND-POINT | `ADR.md` |
| 88c961ed5 | 2026-09-28 | OTHER | `calibration/oracles/README.md` |
| 88c961ed5 | 2026-09-28 | SEMANTIC | `cmd/indexbench/langoracle.go` |
| a58dee45c | 2026-09-28 | OTHER | `internal/extractor/tsengine/grammars/PROVENANCE.md` |
| b89f24fb8 | 2026-09-28 | OTHER | `calibration/corpus.d/css.yaml`, `docs/bench/readings/lang/css.json`, `internal/extractor/dialects/css/dialect.json`, `internal/extractor/dialects/css/tags.scm`, `internal/extractor/tsengine/grammars/PROVENANCE.md`, `internal/extractor/tsengine/grammars/grammars.lock.json`, `internal/langbench/languages.tmpl.md` |
| b89f24fb8 | 2026-09-28 | APPEND-POINT | `docs/languages.md`, `docs/plans/language-breadth.md`, `internal/connector/languages_test.go`, `internal/connector/typededges.go` |
| b89f24fb8 | 2026-09-28 | SEMANTIC | `internal/extractor/dialects.go`, `internal/extractor/tsengine/tsengine_test.go`, `internal/fusion/resolvestrategy_test.go`, `internal/langbench/spec.go` |
| bdbf38173 | 2026-09-28 | OTHER | `internal/extractor/tsengine/grammars/PROVENANCE.md` |
| bdbf38173 | 2026-09-28 | SEMANTIC | `internal/extractor/tsengine/tsengine_test.go` |
| bf1f0fea3 | 2026-09-28 | APPEND-POINT | `docs/languages.md`, `docs/plans/language-breadth.md`, `internal/fusion/pipeline.go` |
| d24f1f5d3 | 2026-09-28 | APPEND-POINT | `cmd/fathom/pluginrender.go` |
| d9a3947e0 | 2026-09-28 | APPEND-POINT | `.fathom/observations.md` |
| db17e3f65 | 2026-09-28 | APPEND-POINT | `.fathom/observations.md`, `ADR.md`, `docs/intent/decisions/README.md`, `docs/intent/futures.md`, `docs/plans/exploration-maps.md` |
| e1569a285 | 2026-09-28 | APPEND-POINT | `docs/mcp-tools.md` |
| e505d8f40 | 2026-09-28 | APPEND-POINT | `docs/mcp-tools.md` |
| ee27a380e | 2026-09-28 | APPEND-POINT | `CHANGELOG.md`, `docs/plans/efficiency-wave.md` |
| f4bed50ae | 2026-09-28 | APPEND-POINT | `docs/languages.md`, `internal/fusion/pipeline.go` |
| f4bed50ae | 2026-09-28 | OTHER | `internal/extractor/tsengine/grammars/PROVENANCE.md` |
| fbcdaabbd | 2026-09-28 | ADJACENT | `cmd/fathom/thread.go` |
| fbcdaabbd | 2026-09-28 | APPEND-POINT | `docs/plans/efficiency-wave.md` |
| fdf3537e6 | 2026-09-28 | APPEND-POINT | `.fathom/observations.md`, `CHANGELOG.md`, `docs/plans/efficiency-wave.md` |
| 0e3929786 | 2026-09-29 | APPEND-POINT | `docs/plans/progress-and-economics.md` |
| 12c6b6f2a | 2026-09-29 | APPEND-POINT | `.fathom/observations.md`, `CHANGELOG.md`, `internal/connector/connector.go`, `internal/connector/languages_test.go`, `internal/connector/typededges.go`, `internal/connector/unsupported_language_gen.go`, `internal/core/model.go`, `internal/fusion/pipeline.go` |
| 12c6b6f2a | 2026-09-29 | OTHER | `internal/extractor/tsengine/grammars/PROVENANCE.md`, `internal/extractor/tsengine/grammars/grammars.lock.json` |
| 12c6b6f2a | 2026-09-29 | SEMANTIC | `internal/extractor/tsengine/tsengine_test.go` |
| 145c89934 | 2026-09-29 | APPEND-POINT | `.fathom/observations.md` |
| 230557ca9 | 2026-09-29 | APPEND-POINT | `.fathom/observations.md`, `docs/plans/served-not-kept.md` |
| 3430e30ac | 2026-09-29 | APPEND-POINT | `.fathom/observations.md` |
| 3430e30ac | 2026-09-29 | OTHER | `internal/extractor/tsengine/grammars/PROVENANCE.md` |
| 3430e30ac | 2026-09-29 | SEMANTIC | `internal/extractor/tsengine/tsengine_test.go` |
| 42fd41920 | 2026-09-29 | APPEND-POINT | `.fathom/observations.md`, `CHANGELOG.md` |
| 462a95c29 | 2026-09-29 | APPEND-POINT | `.fathom/observations.md`, `internal/fusion/pipeline.go` |
| 462a95c29 | 2026-09-29 | OTHER | `internal/extractor/tsengine/grammars/PROVENANCE.md` |
| 462a95c29 | 2026-09-29 | SEMANTIC | `internal/extractor/tsengine/tsengine_test.go` |
| 46f901e97 | 2026-09-29 | APPEND-POINT | `.fathom/observations.md`, `CHANGELOG.md` |
| 4ff1c8abf | 2026-09-29 | APPEND-POINT | `.fathom/observations.md`, `CHANGELOG.md` |
| 4ff91283c | 2026-09-29 | APPEND-POINT | `docs/languages.md` |
| 4ff91283c | 2026-09-29 | SEMANTIC | `internal/fusion/pipeline.go` |
| 613016eeb | 2026-09-29 | APPEND-POINT | `.fathom/observations.md`, `CHANGELOG.md` |
| 64ada2bab | 2026-09-29 | APPEND-POINT | `.fathom/observations.md`, `docs/plans/served-not-kept.md` |
| 6aabf6dbd | 2026-09-29 | APPEND-POINT | `.fathom/observations.md`, `CHANGELOG.md` |
| 6dbbfa0c0 | 2026-09-29 | APPEND-POINT | `CHANGELOG.md` |
| 6ec072d05 | 2026-09-29 | APPEND-POINT | `.fathom/observations.md` |
| 719482a47 | 2026-09-29 | SEMANTIC | `cmd/fathom/observatory.go` |
| 72b03c940 | 2026-09-29 | APPEND-POINT | `.fathom/observations.md` |
| 780cfe08c | 2026-09-29 | APPEND-POINT | `.fathom/observations.md` |
| 7dc430bf6 | 2026-09-29 | APPEND-POINT | `.fathom/observations.md` |
| 9a7dfc95c | 2026-09-29 | APPEND-POINT | `.fathom/observations.md`, `CHANGELOG.md`, `internal/connector/languages_test.go`, `internal/connector/typededges.go` |
| 9a7dfc95c | 2026-09-29 | OTHER | `calibration/oracles/README.md` |
| 9a7dfc95c | 2026-09-29 | SEMANTIC | `internal/fusion/pipeline.go` |
| 9ea6da5fe | 2026-09-29 | APPEND-POINT | `.fathom/observations.md` |
| a84a37b90 | 2026-09-29 | APPEND-POINT | `.fathom/observations.md` |
| c1ca2657c | 2026-09-29 | APPEND-POINT | `.fathom/observations.md`, `CHANGELOG.md`, `docs/plans/exploration-maps.md` |
| c4ef6f543 | 2026-09-29 | APPEND-POINT | `CHANGELOG.md` |
| c91741ace | 2026-09-29 | APPEND-POINT | `CHANGELOG.md`, `docs/mcp-tools.md`, `internal/fusion/pipeline.go` |
| c91741ace | 2026-09-29 | OTHER | `docs/bench/readings/lang/milestones.md` |
| d595ea02a | 2026-09-29 | APPEND-POINT | `CHANGELOG.md` |
| d5ad76495 | 2026-09-29 | APPEND-POINT | `internal/fusion/pipeline.go` |
| d6438fc78 | 2026-09-29 | APPEND-POINT | `.fathom/observations.md` |
| daba823bf | 2026-09-29 | APPEND-POINT | `CHANGELOG.md` |
| dc75e7a94 | 2026-09-29 | APPEND-POINT | `.fathom/observations.md`, `CHANGELOG.md` |
| e95a1da3b | 2026-09-29 | APPEND-POINT | `.fathom/observations.md` |
| fa240083e | 2026-09-29 | APPEND-POINT | `.fathom/observations.md` |
| fc96cc8b8 | 2026-09-29 | OTHER | `.agents/skills/fathom-activate/SKILL.md`, `.agents/skills/fathom-custodian/SKILL.md`, `.agents/skills/fathom-doctor/SKILL.md`, `.agents/skills/fathom-onboard/SKILL.md`, `.agents/skills/fathom-rewire/SKILL.md`, `.agents/skills~74ce6e9ce2abc4fb46c54830b680c6a93a7fe2da`, `docs/intent/invariants.md`, `docs/usage.md` |
| fc96cc8b8 | 2026-09-29 | SEMANTIC | `cmd/fathom/stanza_registered_surface_test.go` |
| fc96cc8b8 | 2026-09-29 | APPEND-POINT | `docs/plans/strengths-bench.md` |
| ff56e9e99 | 2026-09-29 | OTHER | `calibration/oracles/README.md` |
| ff56e9e99 | 2026-09-29 | APPEND-POINT | `docs/languages.md` |
| 0244e1b58 | 2026-09-30 | APPEND-POINT | `.fathom/observations.md` |
| 0dfb675ba | 2026-09-30 | APPEND-POINT | `.fathom/observations.md`, `CHANGELOG.md`, `docs/languages.md`, `docs/plans/language-breadth.md`, `internal/fusion/pipeline.go` |
| 0dfb675ba | 2026-09-30 | OTHER | `schema/dialect/testdata/invalid.cue` |
| 373d6545c | 2026-09-30 | APPEND-POINT | `.fathom/observations.md` |
| 89b7695c0 | 2026-09-30 | APPEND-POINT | `.fathom/observations.md` |
| b209f7bd4 | 2026-09-30 | APPEND-POINT | `CHANGELOG.md` |
| c975a2434 | 2026-09-30 | APPEND-POINT | `CHANGELOG.md` |
| 11a2be233 | 2026-10-01 | APPEND-POINT | `CHANGELOG.md`, `docs/languages.md`, `docs/plans/language-breadth.md`, `internal/fusion/extractor_epoch.go` |
| 24e6dce6f | 2026-10-01 | APPEND-POINT | `CHANGELOG.md` |
| 252bbf80f | 2026-10-01 | APPEND-POINT | `docs/plans/language-breadth.md`, `internal/fusion/extractor_epoch.go` |
| 308af73e4 | 2026-10-01 | APPEND-POINT | `CHANGELOG.md`, `docs/plans/language-breadth.md`, `internal/connector/typededges.go` |
| 308af73e4 | 2026-10-01 | OTHER | `calibration/oracles/README.md`, `schema/dialect/doc.cue` |
| 308af73e4 | 2026-10-01 | SEMANTIC | `internal/extractor/docdecode.go`, `internal/extractor/docengine.go` |
| 35dcc952b | 2026-10-01 | APPEND-POINT | `CHANGELOG.md`, `docs/languages.md`, `docs/plans/language-breadth.md`, `internal/fusion/extractor_epoch.go` |
| 35dcc952b | 2026-10-01 | ADJACENT | `internal/fusion/epoch_test.go` |
| 3b07f6bee | 2026-10-01 | APPEND-POINT | `CHANGELOG.md`, `docs/plans/language-breadth.md` |
| 4c5d04cbb | 2026-10-01 | APPEND-POINT | `.fathom/observations.md` |
| 4e0bd1d3d | 2026-10-01 | APPEND-POINT | `CHANGELOG.md` |
| 4f45f683d | 2026-10-01 | APPEND-POINT | `.fathom/observations.md`, `CHANGELOG.md` |
| 50d0b157b | 2026-10-01 | APPEND-POINT | `.fathom/observations.md` |
| 51cadd68a | 2026-10-01 | APPEND-POINT | `CHANGELOG.md`, `docs/languages.md`, `docs/plans/language-breadth.md`, `internal/core/model.go`, `internal/fusion/extractor_epoch.go` |
| 51cadd68a | 2026-10-01 | SEMANTIC | `internal/extractor/docengine.go`, `internal/extractor/tsengine/tsengine_test.go`, `internal/fusion/resolvestrategy_test.go` |
| 51cadd68a | 2026-10-01 | OTHER | `internal/extractor/tsengine/grammars/PROVENANCE.md`, `schema/dialect/testdata/valid.cue` |
| 67b7eda45 | 2026-10-01 | APPEND-POINT | `CHANGELOG.md` |
| 6d9ea08f9 | 2026-10-01 | APPEND-POINT | `.fathom/observations.md` |
| 731bafb9b | 2026-10-01 | APPEND-POINT | `internal/fusion/extractor_epoch.go` |
| 75c9f67c5 | 2026-10-01 | APPEND-POINT | `docs/intent/futures.md` |
| 83e99406c | 2026-10-01 | APPEND-POINT | `docs/plans/language-breadth.md` |
| 8e442ac11 | 2026-10-01 | APPEND-POINT | `.fathom/observations.md`, `CHANGELOG.md`, `internal/extractor/tsdialect_parity_test.go` |
| ae4937c47 | 2026-10-01 | APPEND-POINT | `.fathom/observations.md`, `docs/plans/language-breadth.md` |
| ae4937c47 | 2026-10-01 | SEMANTIC | `cmd/indexbench/langoracle.go` |
| ae4937c47 | 2026-10-01 | ADJACENT | `cmd/indexbench/langoracle_unmatched_test.go` |
| b39429aad | 2026-10-01 | APPEND-POINT | `.fathom/observations.md`, `CHANGELOG.md` |
| cc44bdd92 | 2026-10-01 | APPEND-POINT | `CHANGELOG.md` |
| d12af4a1f | 2026-10-01 | APPEND-POINT | `docs/plans/language-breadth.md` |
| dc711d1aa | 2026-10-01 | APPEND-POINT | `.fathom/observations.md` |
| eb900e7af | 2026-10-01 | OTHER | `calibration/oracles/README.md` |
| eb900e7af | 2026-10-01 | APPEND-POINT | `docs/languages.md`, `internal/fusion/extractor_epoch.go` |
| fa238962d | 2026-10-01 | APPEND-POINT | `.fathom/observations.md`, `docs/plans/language-breadth.md` |
| fa238962d | 2026-10-01 | SEMANTIC | `cmd/indexbench/langoracle.go` |
| fa238962d | 2026-10-01 | ADJACENT | `cmd/indexbench/langoracle_unmatched_test.go` |
| 0f9f26e44 | 2026-10-02 | APPEND-POINT | `CHANGELOG.md` |
| 1f8b8faaf | 2026-10-02 | APPEND-POINT | `CHANGELOG.md`, `docs/languages.md` |
| 1f8b8faaf | 2026-10-02 | OTHER | `docs/bench/readings/lang/footprint.md` |
| 344413603 | 2026-10-02 | APPEND-POINT | `docs/languages.md`, `docs/plans/language-breadth.md` |
| 47d77502d | 2026-10-02 | APPEND-POINT | `docs/plans/language-breadth.md` |
| 47d77502d | 2026-10-02 | SEMANTIC | `internal/langbench/scip.go` |
| 4a6e8693b | 2026-10-02 | APPEND-POINT | `docs/languages.md`, `docs/plans/language-breadth.md` |
| 565f04a31 | 2026-10-02 | SEMANTIC | `internal/fusion/classify.go` |
| 60b0168e6 | 2026-10-02 | APPEND-POINT | `docs/languages.md`, `docs/plans/language-breadth.md`, `internal/fusion/extractor_epoch.go` |
| 6f3b0986d | 2026-10-02 | APPEND-POINT | `docs/languages.md` |
| 77b06ee06 | 2026-10-02 | OTHER | `calibration/oracles/README.md` |
| 77b06ee06 | 2026-10-02 | APPEND-POINT | `docs/languages.md` |
| 8442dbaf9 | 2026-10-02 | APPEND-POINT | `docs/plans/language-breadth.md` |
| 8bf1486b9 | 2026-10-02 | APPEND-POINT | `docs/plans/language-breadth.md` |
| 91004999e | 2026-10-02 | OTHER | `docs/bench/readings/lang/footprint.md` |
| 91004999e | 2026-10-02 | APPEND-POINT | `docs/languages.md`, `internal/fusion/extractor_epoch.go` |
| 91004999e | 2026-10-02 | SEMANTIC | `internal/fusion/fileresolve.go`, `internal/fusion/resolvestrategy_test.go` |
| 939098ed5 | 2026-10-02 | APPEND-POINT | `CHANGELOG.md`, `docs/languages.md`, `docs/plans/language-breadth.md`, `internal/fusion/extractor_epoch.go` |
| 972cb0ffe | 2026-10-02 | APPEND-POINT | `CHANGELOG.md`, `docs/languages.md`, `docs/plans/language-breadth.md`, `internal/fusion/extractor_epoch.go` |
| 972cb0ffe | 2026-10-02 | SEMANTIC | `internal/langbench/scip.go` |
| 99faabe22 | 2026-10-02 | OTHER | `calibration/oracles/README.md` |
| 99faabe22 | 2026-10-02 | APPEND-POINT | `docs/languages.md` |
| 9ddf9f9fd | 2026-10-02 | APPEND-POINT | `CHANGELOG.md`, `docs/languages.md`, `docs/plans/language-breadth.md`, `internal/fusion/extractor_epoch.go` |
| 9ddf9f9fd | 2026-10-02 | SEMANTIC | `internal/extractor/gomoddialect.go`, `internal/fusion/docxref.go` |
| a5b0ee3da | 2026-10-02 | SEMANTIC | `cmd/indexbench/langoracle.go`, `internal/extractor/docengine.go`, `internal/extractor/tsengine/tsengine_test.go`, `internal/fusion/resolvestrategy_test.go` |
| a5b0ee3da | 2026-10-02 | APPEND-POINT | `docs/languages.md`, `docs/plans/language-breadth.md`, `internal/connector/languages_test.go` |
| a5b0ee3da | 2026-10-02 | OTHER | `internal/extractor/tsengine/grammars/PROVENANCE.md`, `internal/langbench/languages.tmpl.md`, `schema/dialect/doc.cue` |
| a63ae0a5b | 2026-10-02 | OTHER | `docs/bench/readings/lang/footprint.md` |
| a63ae0a5b | 2026-10-02 | APPEND-POINT | `internal/fusion/extractor_epoch.go` |
| a63ae0a5b | 2026-10-02 | SEMANTIC | `internal/fusion/specmemo_test.go` |
| aa5508555 | 2026-10-02 | APPEND-POINT | `CHANGELOG.md` |
| b363371c1 | 2026-10-02 | APPEND-POINT | `docs/languages.md`, `docs/plans/language-breadth.md` |
| b363371c1 | 2026-10-02 | SEMANTIC | `internal/fusion/register.go`, `internal/fusion/reingest_file.go` |
| c67183aad | 2026-10-02 | APPEND-POINT | `CHANGELOG.md`, `docs/plans/language-breadth.md` |
| c67183aad | 2026-10-02 | SEMANTIC | `internal/langbench/scip.go` |
| cba25df9d | 2026-10-02 | APPEND-POINT | `CHANGELOG.md` |
| cbe1c3817 | 2026-10-02 | APPEND-POINT | `docs/languages.md`, `docs/plans/language-breadth.md`, `internal/fusion/extractor_epoch.go` |
| cf838488f | 2026-10-02 | APPEND-POINT | `docs/languages.md`, `internal/fusion/extractor_epoch.go` |
| d4e495e6e | 2026-10-02 | APPEND-POINT | `docs/languages.md`, `docs/plans/language-breadth.md`, `internal/connector/connector.go`, `internal/connector/languages_test.go` |
| d4e495e6e | 2026-10-02 | OTHER | `internal/langbench/languages.tmpl.md` |
| e13bf27ba | 2026-10-02 | APPEND-POINT | `docs/languages.md` |
| e1cebfc32 | 2026-10-02 | APPEND-POINT | `docs/languages.md`, `docs/plans/language-breadth.md` |
| f9e5b63e9 | 2026-10-02 | APPEND-POINT | `CHANGELOG.md`, `docs/languages.md`, `docs/plans/language-breadth.md` |
| fdb65d6b0 | 2026-10-02 | APPEND-POINT | `docs/languages.md`, `docs/plans/language-breadth.md`, `internal/fusion/extractor_epoch.go` |
| 06037ecbb | 2026-10-09 | APPEND-POINT | `CHANGELOG.md` |
| 0a4ce8eb4 | 2026-10-09 | APPEND-POINT | `docs/languages.md`, `docs/plans/language-breadth.md` |
| 2184c1daf | 2026-10-09 | SEMANTIC | `cmd/indexbench/langoracle.go`, `internal/fusion/specmemo_test.go`, `internal/langbench/scip.go` |
| 2184c1daf | 2026-10-09 | APPEND-POINT | `docs/plans/language-breadth.md` |
| 27b03b10e | 2026-10-09 | APPEND-POINT | `docs/plans/language-breadth.md` |
| 3664b763a | 2026-10-09 | APPEND-POINT | `CHANGELOG.md` |
| 3d4ec6b86 | 2026-10-09 | APPEND-POINT | `docs/plans/language-breadth.md` |
| 630ebd12c | 2026-10-09 | APPEND-POINT | `CHANGELOG.md` |
| 7af4f4ae8 | 2026-10-09 | APPEND-POINT | `ADR.md` |
| a19bb036f | 2026-10-09 | APPEND-POINT | `CHANGELOG.md`, `docs/plans/language-breadth.md` |
| a19bb036f | 2026-10-09 | SEMANTIC | `internal/langbench/scip.go` |
| a23288ab8 | 2026-10-09 | APPEND-POINT | `docs/languages.md` |
| ab62a5e07 | 2026-10-09 | APPEND-POINT | `docs/plans/language-breadth.md` |
| b25bfb2fa | 2026-10-09 | APPEND-POINT | `CHANGELOG.md` |
| d7bee7c91 | 2026-10-09 | APPEND-POINT | `docs/plans/language-breadth.md` |
| daf36529e | 2026-10-09 | APPEND-POINT | `docs/plans/language-breadth.md` |
| def207ccb | 2026-10-09 | APPEND-POINT | `docs/plans/language-breadth.md` |
| f88d354d7 | 2026-10-09 | APPEND-POINT | `CHANGELOG.md` |
| 06de908b6 | 2026-10-10 | APPEND-POINT | `CHANGELOG.md` |
| 272972cc4 | 2026-10-10 | APPEND-POINT | `CHANGELOG.md`, `internal/fusion/extractor_epoch.go` |
| 2d24ee952 | 2026-10-10 | APPEND-POINT | `docs/plans/language-breadth.md` |
| 2dd05859a | 2026-10-10 | APPEND-POINT | `docs/plans/language-breadth.md` |
| 3d4633a5a | 2026-10-10 | APPEND-POINT | `docs/languages.md`, `docs/plans/language-breadth.md` |
| 3d4633a5a | 2026-10-10 | SEMANTIC | `internal/extractor/nativespecs_probe_test.go`, `internal/extractor/tsonepass_test.go`, `internal/fusion/dispatchtable_test.go` |
| 3d4633a5a | 2026-10-10 | OTHER | `internal/langbench/languages.tmpl.md` |
| 4229544b1 | 2026-10-10 | APPEND-POINT | `cmd/indexbench/langparity.go`, `docs/languages.md`, `docs/plans/language-breadth.md` |
| 4229544b1 | 2026-10-10 | OTHER | `docs/bench/readings/lang/parity/README.md`, `internal/langbench/languages.tmpl.md` |
| 4e416b1f5 | 2026-10-10 | APPEND-POINT | `docs/languages.md`, `docs/plans/language-breadth.md` |
| 4e906acf6 | 2026-10-10 | APPEND-POINT | `docs/languages.md`, `docs/plans/language-breadth.md` |
| 51a6910e6 | 2026-10-10 | APPEND-POINT | `docs/languages.md` |
| 5da0bb229 | 2026-10-10 | APPEND-POINT | `CHANGELOG.md`, `docs/languages.md`, `docs/plans/language-breadth.md`, `internal/fusion/extractor_epoch.go` |
| 5da0bb229 | 2026-10-10 | OTHER | `internal/langbench/languages.tmpl.md` |
| 5e47f683c | 2026-10-10 | APPEND-POINT | `CHANGELOG.md`, `docs/languages.md`, `docs/plans/language-breadth.md` |
| 5e47f683c | 2026-10-10 | OTHER | `docs/design/language-breadth.md`, `schema/dialect/doc.cue` |
| 5e47f683c | 2026-10-10 | SEMANTIC | `internal/fusion/docxref.go` |
| 6d4f97a82 | 2026-10-10 | APPEND-POINT | `docs/languages.md` |
| 859c735f5 | 2026-10-10 | OTHER | `calibration/oracles/README.md`, `schema/dialect/testdata/valid.cue` |
| 859c735f5 | 2026-10-10 | SEMANTIC | `cmd/indexbench/langoracle.go`, `internal/extractor/tsextract_module_test.go` |
| 859c735f5 | 2026-10-10 | APPEND-POINT | `docs/languages.md` |
| 99c3880b2 | 2026-10-10 | APPEND-POINT | `docs/plans/language-breadth.md` |
| 9e21be2b6 | 2026-10-10 | APPEND-POINT | `CHANGELOG.md` |
| b62edbc7c | 2026-10-10 | APPEND-POINT | `CHANGELOG.md`, `docs/languages.md`, `docs/plans/language-breadth.md` |
| b62edbc7c | 2026-10-10 | OTHER | `docs/design/language-breadth.md` |
| c095f29b6 | 2026-10-10 | APPEND-POINT | `docs/languages.md`, `docs/plans/language-breadth.md` |
| c095f29b6 | 2026-10-10 | SEMANTIC | `internal/fusion/docxref.go` |
| c495b89fa | 2026-10-10 | APPEND-POINT | `docs/plans/language-breadth.md` |
| c68b5097d | 2026-10-10 | APPEND-POINT | `CHANGELOG.md` |
| e9d5dde2e | 2026-10-10 | APPEND-POINT | `docs/languages.md`, `docs/plans/language-breadth.md` |
| 0e2c22fd8 | 2026-10-11 | OTHER | `docs/configuration.md`, `internal/config/config_schema.cue`, `schema/config/schema.cue`, `schema/config/testdata/invalid.cue`, `schema/config/testdata/valid.cue` |
| 0e2c22fd8 | 2026-10-11 | SEMANTIC | `internal/config/config.go`, `internal/config/provenance.go` |
| 0e2c22fd8 | 2026-10-11 | ADJACENT | `internal/config/config_test.go`, `internal/config/schema_test.go` |
| 4a5eba013 | 2026-10-11 | APPEND-POINT | `docs/plans/language-breadth.md` |
| 4a5eba013 | 2026-10-11 | SEMANTIC | `internal/extractor/tsextract_module_test.go`, `internal/fusion/resolvestrategy_test.go` |
| 4a5eba013 | 2026-10-11 | OTHER | `schema/dialect/testdata/valid.cue` |
| 4c413f988 | 2026-10-11 | OTHER | `docs/design/language-breadth.md` |
| 4c413f988 | 2026-10-11 | APPEND-POINT | `docs/plans/language-breadth.md` |
| 56746ba91 | 2026-10-11 | APPEND-POINT | `docs/languages.md`, `docs/plans/language-breadth.md` |
| 6bdb1df3a | 2026-10-11 | OTHER | `docs/design/language-breadth.md` |
| 6bdb1df3a | 2026-10-11 | APPEND-POINT | `docs/plans/language-breadth.md` |
| e1fe5b8f6 | 2026-10-11 | APPEND-POINT | `docs/plans/language-breadth.md` |
| ebc51e51e | 2026-10-11 | APPEND-POINT | `docs/plans/language-breadth.md`, `internal/connector/connector.go`, `internal/connector/languages_test.go` |
| ebc51e51e | 2026-10-11 | SEMANTIC | `internal/extractor/fileresolvelangs.go`, `internal/extractor/tsengine/tsengine_test.go`, `internal/extractor/tsextract.go`, `internal/extractor/tsextract_module_test.go`, `internal/fusion/resolvestrategy_test.go` |
| ebc51e51e | 2026-10-11 | OTHER | `internal/extractor/tsengine/grammars/PROVENANCE.md`, `schema/dialect/testdata/valid.cue` |
| ebc51e51e | 2026-10-11 | ADJACENT | `internal/extractor/tsextract_host.go`, `internal/extractor/tsextract_host_test.go` |
| 05a490ef6 | 2026-10-15 | APPEND-POINT | `CHANGELOG.md` |
| 4ffb40e6a | 2026-10-15 | APPEND-POINT | `CHANGELOG.md` |
| 6b5d8961b | 2026-10-15 | APPEND-POINT | `docs/plans/language-breadth.md` |
| 81bf0fe66 | 2026-10-15 | APPEND-POINT | `docs/plans/language-breadth.md` |
| 9dca8e12e | 2026-10-15 | APPEND-POINT | `CHANGELOG.md` |
| fd308eba6 | 2026-10-15 | APPEND-POINT | `docs/plans/language-breadth.md` |
| 1f1f74922 | 2026-10-16 | APPEND-POINT | `docs/plans/index-snapshots.md` |
| b0fd169fe | 2026-10-16 | APPEND-POINT | `CHANGELOG.md` |
| b4e2e263c | 2026-10-16 | APPEND-POINT | `docs/plans/distribution.md` |
| c175a45db | 2026-10-16 | APPEND-POINT | `CHANGELOG.md` |
| d56c3b575 | 2026-10-16 | APPEND-POINT | `CHANGELOG.md` |
| d9280dd2a | 2026-10-16 | APPEND-POINT | `docs/plans/resume-2026-10.md` |
| e7788c0a8 | 2026-10-16 | APPEND-POINT | `CHANGELOG.md` |
| f46a65119 | 2026-10-16 | APPEND-POINT | `CHANGELOG.md` |
| 409e9972b | 2026-10-17 | APPEND-POINT | `CHANGELOG.md` |
| 4595df4e3 | 2026-10-17 | APPEND-POINT | `CHANGELOG.md` |
| 486bb87a9 | 2026-10-17 | APPEND-POINT | `CHANGELOG.md` |
| 9409b51ea | 2026-10-17 | APPEND-POINT | `CHANGELOG.md` |
| 96343b4c0 | 2026-10-17 | APPEND-POINT | `CHANGELOG.md` |
| a27433447 | 2026-10-17 | APPEND-POINT | `CHANGELOG.md` |
| a27433447 | 2026-10-17 | OTHER | `Taskfile.yml`, `docs/usage/intent.md` |
| daabd8ace | 2026-10-17 | APPEND-POINT | `CHANGELOG.md` |
| df6a15563 | 2026-10-17 | APPEND-POINT | `CHANGELOG.md` |
| f0172a952 | 2026-10-17 | APPEND-POINT | `CHANGELOG.md` |
| 11d678c8e | 2026-10-18 | APPEND-POINT | `CHANGELOG.md` |
| 27f5076e1 | 2026-10-18 | APPEND-POINT | `docs/plans/readouts.md` |
| 339b7d73f | 2026-10-18 | APPEND-POINT | `CHANGELOG.md` |
| 3cdbf35ba | 2026-10-18 | APPEND-POINT | `CHANGELOG.md` |
| 4906042b9 | 2026-10-18 | APPEND-POINT | `docs/plans/responsible-tenant.md` |
| 58ac98589 | 2026-10-18 | APPEND-POINT | `CHANGELOG.md` |
| 5a5e7335f | 2026-10-18 | APPEND-POINT | `docs/plans/readouts.md` |
| 6ec3145cc | 2026-10-18 | SEMANTIC | `cmd/fathom/doctor.go`, `cmd/fathom/init.go`, `cmd/fathom/pluginrender.go`, `cmd/fathom/reconcile.go`, `internal/harness/codec_test.go`, `internal/harness/descriptors.go`, `internal/harness/harness.go`, `internal/harness/harness_test.go` |
| 6ec3145cc | 2026-10-18 | OTHER | `docs/adoption/codex.md`, `schema/harness/schema.cue`, `schema/harness/testdata/valid.cue` |
| 7ae2d09a9 | 2026-10-18 | APPEND-POINT | `CHANGELOG.md`, `docs/plans/claude-code-mods.md` |
| 8aa34daca | 2026-10-18 | APPEND-POINT | `CHANGELOG.md` |
| b73169661 | 2026-10-18 | APPEND-POINT | `CHANGELOG.md` |
| ea2dd51d7 | 2026-10-18 | APPEND-POINT | `CHANGELOG.md`, `docs/plans/readouts.md` |
| 0f0b017a7 | 2026-10-19 | APPEND-POINT | `CHANGELOG.md` |
| 1032f0ea8 | 2026-10-19 | APPEND-POINT | `CHANGELOG.md` |
| 1a1bdcba8 | 2026-10-19 | APPEND-POINT | `docs/plans/fathom-dir.md` |
| 2456dbc8e | 2026-10-19 | APPEND-POINT | `docs/plans/readouts.md` |
| 3c95c4864 | 2026-10-19 | SEMANTIC | `cmd/fathom/init.go` |
| 6265c74f8 | 2026-10-19 | APPEND-POINT | `CHANGELOG.md`, `docs/languages.md` |
| 6265c74f8 | 2026-10-19 | OTHER | `internal/extractor/tsengine/grammars/PROVENANCE.md` |
| 6265c74f8 | 2026-10-19 | SEMANTIC | `internal/extractor/tsengine/tsengine_test.go` |
| c9ea342e8 | 2026-10-19 | APPEND-POINT | `CHANGELOG.md` |
| d472c53be | 2026-10-19 | APPEND-POINT | `docs/languages.md` |
| d472c53be | 2026-10-19 | SEMANTIC | `internal/extractor/gomoddialect.go` |
| dae76235a | 2026-10-19 | APPEND-POINT | `CHANGELOG.md` |
| dea96a15b | 2026-10-19 | APPEND-POINT | `CHANGELOG.md` |
| dea96a15b | 2026-10-19 | OTHER | `docs/vision/38-proof-layer-roadmap.md` |
| f8a08ed0a | 2026-10-19 | APPEND-POINT | `CHANGELOG.md` |
