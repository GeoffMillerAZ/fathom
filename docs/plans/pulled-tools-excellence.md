# Wave 3a — Pulled-tools excellence (O12 + serving-layer honesty)

Fix the three measured product gaps from context-bench B1-T09 (dogfood O12),
close the disclosed C4 method-edge gap, and clear the serving-layer hygiene
queue. Proof standard: the bench's own cross-repo tasks re-run and the sign
flips — or the non-flip is recorded honestly.

Design (normative):
- **P1-1 parameter alias.** The four hot tools each name their primary
  argument differently (find_symbol `name`, references `query`,
  query_symbols `concern`, symbol_context `symbol`) — five error turns of
  -32602 guessing in the bench transcript. All four accept `symbol` as an
  additive alias (native name still primary; schemas document the alias;
  ambiguity — both provided, different values — is a loud error).
- **P1-2 cross-repo stitching.** `symbol_context` callees/callers and
  `find_callers` consult the cross-service graph (platformlink/platform)
  when the store is a fleet store, so fathom→kit edges appear instead of
  silently stopping at the service boundary. Results carry an edge-scope
  label (`same-service` | `cross-service`) — never silently merged.
- **P1-3 case-tier affinity.** The case-insensitive find_symbol tier
  returns a ranked multi-match (or affinity-ranked single) instead of
  steering to the first bare-name hit in the wrong package; `matched_via`
  stays. Bench reproduction: "heartbeat" must surface telemetry/heartbeat
  alongside worker-heartbeat, not hide it.
- **P1-4 method-edge normalization.** Both-sides symbol-key→declaring-
  package resolution for arch_conformance's edge build (closes the C4-4
  disclosed gap: method-origin AND method-target edges join the checked
  set). The pack's disclosure text updates to "closed" with the test that
  proves a method-body violation is caught.
- **P1-5 the proof re-run.** Re-run the three cross_repo bench tasks +
  B1-T09 with the fixed binary against a freshly-ingested scratch fleet
  store (NOT the live daemon): record per-task before/after tokens/turns.
  Honest either way — a non-flip is a finding, not a failure.
- **P1-6 O11.** migration_targets merges caller `deprecated_pkgs` with the
  default list per its own description (or the description changes —
  merge is the better contract); existing replace-semantics tests updated.
- **P1-7 catalog truth.** docs/mcp-tools.md gains its missing rows
  (spec_coverage, spec_status, traceability, pack_context at minimum;
  sweep extra_tools.go registrations for any other absentees).
- **P1-8 server-side required.** Dispatch validates declared InputSchema
  `required` fields before invoking handlers (ErrCodeParams, matching the
  Go-enforced tools' behavior). The C4-5 audit showed handlers are the only
  enforcement today; aligning the declared contract with behavior is the
  honesty fix. Existing lenient-caller tests updated deliberately, not
  worked around.
- **P1-9 vocabulary fix for the sweep tension.** `contradicted` (anchor-
  dead, machine-detected, served WITH label pending recompile) is not
  `refuted` (deliberate judgment, never served). The sweep's claim_events
  verdict becomes "contradicted" to match the status it sets; serve-layer
  doc + semantic-layers design note record the two-word distinction and why
  contradicted claims stay served-labeled.

## Tasks

- [ ] symbol alias accepted by all four hot tools; ambiguity errors loudly; schemas document it; fake-backend tests for alias, native, and conflict per tool {id: P1-1, symbol: registerFindSymbol, package: internal/mcp}
- [ ] Cross-service callees/callers in symbol_context + find_callers for fleet stores, edge-scope labeled; test proves the fathom→kit heartbeat edges surface {id: P1-2, symbol: registerSymbolContext, package: internal/mcp}
- [ ] Case-insensitive tier multi-match/affinity ranking; "heartbeat" reproduction test {id: P1-3, symbol: registerFindSymbol, package: internal/mcp, skip_test: true, skip: "anchor shared with P1-1 (same registration); the ranking has its own tests"}
- [ ] Method-edge both-sides package normalization in arch_conformance edge build; disclosed-gap text flipped to closed; method-body violation fixture caught {id: P1-4, symbol: registerArchConformance, package: internal/mcp}
- [ ] Cross-repo bench tasks re-run with the fixed binary (scratch store); before/after recorded here {id: P1-5, symbol: registerSymbolContext, package: internal/mcp, skip_test: true, skip: "measurement row; evidence is the recorded numbers + the P1-2 tests"}
- [x] migration_targets merges with defaults per its description; tests updated {id: P1-6, symbol: Find, package: internal/analysis/migrationtargets}
- [x] docs/mcp-tools.md missing rows added after a full registration sweep {id: P1-7, symbol: registerServerStats, package: internal/mcp, skip_test: true, skip: "docs row anchored to the served surface; accuracy checked against registrations by the critic pass"}
- [ ] Dispatch enforces InputSchema required (ErrCodeParams); per-tool behavior unchanged where handlers already enforced; tests cover a previously-lenient tool now refusing {id: P1-8, symbol: registerServerStats, package: internal/mcp, skip_test: true, skip: "dispatch-layer change anchored to a registered tool it protects; dispatch tests are the coverage — anchor updated if a dispatch-level symbol reads truer at landing"}
- [x] Sweep event verdict "contradicted"; served-with-label vs never-served distinction documented in serve layer + design doc {id: P1-9, symbol: SweepClaims, package: pkg/client}
