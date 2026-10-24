# Landing evidence

Grammar: v2. Prefix: LE-.
Scope: ADR-081 and `docs/design/landing-evidence.md`; file-based landing evidence extending ADR-078. No table, column, cache, background job, transport, signing, dispatch, network fetch or model spend; no tenancy/governor change. Plan grammar v2 and verify exit semantics stay unchanged.

- [x] Receipt write: record git tree hash, dirty flag (refuse dirty), validate task identity plus Taskfile/lefthook digest, toolchain, os/arch, operator host label, exit, log digest and start/end in files under the fathom home or a repo-configured directory. {id: LE-1, symbol: Write, package: internal/gatereceipt}
- [x] Receipt check and opt-in [gate] accept_receipts pre-push acceptance for a successful exact-tree identity match; accept only with the gate log present beside the receipt, re-hashing to its recorded log digest and ending with the gate's own success line; never accept dirty trees. Keep acceptance off in this repository until BE-11 is decided; otherwise retain the existing gate path, with CI the independent witness. {id: LE-2, symbol: Check, package: internal/gatereceipt}
- [ ] `fathom provenance note` writes harness, model, operator host label, session and lane under refs/notes/fathom/provenance; no trailer and no note push. {id: LE-3, symbol: ProvenanceNote, package: internal/provenance, reachable_from: cmdProvenanceNote}
- [ ] `fathom tests flaky` reads hook logs and imported receipt files across sessions, listing failed-then-passed tests on the same git tree hash and ranking by recorded gate time lost; unknown identity is not a flake. {id: LE-4}
- [ ] Culprit candidates rank a red range by failing-test reach plus blast radius and print git bisect commands; never run a build or test, and name missing reach evidence. {id: LE-5, symbol: RankCulprits, package: internal/landing, reachable_from: runCulprit}
- [x] Record-id uniqueness gate over configurable ADR/invariant/future/incident/per-plan task families; cross-plan opt-in grandfathers the existing 20 shared ids by explicit list and rejects new duplicates without changing grammar v2. Exclude docs/plans/archive/** from the scan; AD-1 through AD-4 recur there. {id: LE-6, symbol: Check, package: internal/recordids}
- [ ] (op) A helper-produced receipt is accepted for a real exact-tree push with CI independently agreeing; record source/command/toolchain/platform identity, receipt/log evidence, push and CI results before verdict. {id: LE-M1, oracle: lexical, path: docs/bench/readings/landing-evidence/round-trip.md, pattern: "^LE-M1 verdict: (PASS|FAIL)"}

Dependencies: LE-2 follows LE-1; LE-4 follows LE-1's receipt representation and needs per-test evidence, never an aggregate gate pass as a test pass. LE-M1 follows LE-1 and LE-2 and requires a real push plus CI, not a synthetic reading. LE-3 uses file/note fields without new columns; if it needs stored host fields, that portion follows `multi-machine.md` MM-3 after RT-M1 PASS. MM-3 remains deferred. LE-6 reuses existing intent-index and per-plan lint checks; it does not waive them. Add product symbol contracts when implementing and verifying each row; all boxes remain open for the controller.

Verify before claiming any row: `fathom verify --service fathom --plan docs/plans/landing-evidence.md`.

## Decision models

None applies: identity matching, duplicate detection and reach ranking are computed with provider none. A future flake-versus-real text audit would need sampled opt-in, a calibrated band, deterministic unknown fallback, opt-out and a labelled keep/kill reading. It cannot certify evidence or run builds/tests. No model spend is authorized.
