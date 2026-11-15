# BEN-1 consolidation and card/schema boundary coverage

Status: Approved 2026-11-15 under W5-3. Contract only; implementation proof pending.
Base: d9eef4794afdbc200d1b8697ce121d9d688fedaf. No product code or new API shape. Owns this document only.
Depends on reviewed CTR-13 source 7c67d10654566da08a77c52413b9e5b3227bf0ae and
CTR-11 W4 source 7db03afbda7fb935b331b3b20b226b1a6b8e4ebf. These exact immutable
documents are normative dependencies, not copied implementations or dispatch proof.

## Intent and decisions before design

Read intent router, P3/P12/P20/P22, BEN-1/3 and REC-10, proof-program BN-D1/2/6,
ADR-101/109/110, CU-D2/5, W3-2/4 and W4-2 serial schema rule.
One command/ledger and three ordered proofs are current rows. No additional runner,
statistics library, store table, paid cell, syntax migration or future data field.
Cell and artifact provenance are exactly CTR-13's acyclic raw->envelope->normalized
contract; PairPreflight is its required typed disposition, never two booleans
guessed from free Reason. Own-fleet smoke does not become independent outcome.

D1 settled BEN-1/ADR-101: cell-key-only #266 cannot claim consolidation. Keep three
one-day serial slices and BEN-1 open until every slice passes. Alternative calling
a new key helper the whole row is excluded.
D2 settled BEN-3/BN-D1/2: card closure covers exact shipped surface identities,
fact->delivery->outcome proof order; registry configuration detection is not proof.
D3 settled W4-2/CU-D2: separate exclusive schema/testdata turns after REC-1 and
head schemas; no two writers receive one directory. Alternative package-wide
parallel ownership violates PL-D2.
D4 settled W3-2/4: implementing-lane real failing tests, assertion-by-assertion
critic; exact existing-type safe projections, named enums/wire names/typed errors/
nonnil slices/no future fields stay in CTR-13. No generic API question reopened.

## Exact current writer inventory and serial grant

W5-3 settles CTR-13/U2 and this exact serial refinement as recommended: one
integration owner and B1-B3 in order. These are approved file/grant boundaries,
not simultaneous ownership or proof a prerequisite implementation landed.

- B1 #266 current two files cellkey.go/cellkey_test.go, plus explicit proposed
  internal/benchledger/ledger.go/ledger_test.go. CellKey reuses contextbench base
  invariants; CellRow persists ONLY ScoredCell, never SessionResult telemetry.
- B2 exclusive cmd/scenariobench/{main.go,main_test.go,cells.go,cells_test.go,
  readiness.go,readiness_test.go}; current cmdLedger's UpsertScenarioLedgerRows
  redirects through same canonical scored-cell adapter as cells. External-run
  CLI sets legacy LedgerPath empty and appends canonical cells at command layer.
  No scenariobench -> benchledger import cycle.
- B2 exclusive cmd/contextbenchscore/{main.go,freeze_test.go,import.go,import_test.go}:
  cmdLedger and cmdBackfill's two AppendLedger calls refuse new outcomes/rows.
  cmdImport is corpus derivation, NOT a ledger writer; preserve that artifact,
  reject it as a scored cell. Existing historical sources are not deleted.
- B2 test-only cmd/benchledgerrender/freeze_test.go preserves read-only renderer;
  main.go already reads/renders only. No needless implementation change there.
- B2 internal/scenariobench/external_corpus_test.go asserts command's canonical
  routing; old internal serializers remain historical format adapters, not another
  published live command writer.
- B3 docs/bench/readings/proof-program/ben-1-consolidation.md and
  docs/bench/proof-cells.jsonl, exact source/commands/old and new bytes published.
  No receipt-presence-only BEN-1 claim. Other intent/flywheel ledgers excluded.

No shared test file simultaneously owned by contract batch and implementing lane.
Transfer at exact landed parent implementation commit. Source branch base is
frozen above; W5-3 grants are approved, prerequisite rows still precede dispatch.

## #299 namespace and row-claim boundary

REC-1 owns schema/records/lane.cue/schema.cue and testdata/{valid,invalid}.cue.
Then #294 exclusively owns schema/records/heads.cue and the sixteen heads fixtures
named by CTR-11. After those handbacks, #299 alone owns:
internal/capability/card_closure.go/card_closure_test.go,
schema/records/data.cue,
schema/records/testdata/cards/valid.cue,
schema/records/testdata/cards/invalid.cue,
calibration/capabilities/demo.cue.
No broad testdata/ grant, no edits to heads/ or lane fixtures. Standard data has
no guessed new fields or invented future schema. Reserved calibration/standards/
is not permission to author an unknown Standard model.

Card/#Capability shape and CheckCardClosure are exactly CTR-13, not a second
definition in this document. Current capability.Registry remains switch policy.
Positive card references current proof records and exact verb:/tool:/hook: keys;
pending records do not mean proof PASS. Negative removes a shipped key's card or
adds duplicate/orphan/conflicting identity. Data kind's phase/live/proven values
are derived, never authored.

BEN-3 can be proven separately when its whole closure bar holds. REC-10 names six
current definitions, not just Capability; a card-only #299 subtask cannot claim
whole REC-10. Remaining Standard/NamedQuery/ReadingHead/Knob contracts are explicit
unimplemented/unspecified current work, not fields prebuilt here. If current issue
claim scope demands all REC-10, it remains blocked rather than pretending namespace
reservation proves it. This coverage record closes the #299 handoff gap only,
not the broader record-data program.

## Exact named test/assertion coverage

All are prospective implementing-lane tests; use the parent contract's complete
assertions, with the following concrete minimums retained, never weakened:

| Test and exact file | Required assertion |
|---|---|
| TestLaneBenchCells, internal/benchledger/cellkey_test.go | each invariant/trial changes key; treatment commit alone does not invalidate daemon-free control; line-break ambiguity rejects; duplicate/wrong stored key refuses |
| TestScoredCellSafeProjection, same file | exact safe keys; seeded raw/context/path/error canaries never serialize; independent oracle/assigned arm/failed zero-use admitted result retained |
| TestPairPreflightAssignedArms, same file | definite pre-agent fail voids both; unknown/missing/late unknown; admitted zero-use failure scored; no W4/W5 admission or Reason parsing |
| TestProjectCellPreflightDisposition, same file | every disposition maps both arms identically, inconsistent booleans refuse; varying Reason alone cannot change class |
| TestBenchDriverConsolidation, cmd/scenariobench/cells_test.go | real kept cmdLedger/external-run route writes one canonical row, no new legacy ledger mutation, imported corpus is not a scored cell |
| TestLegacyLedgerAndBackfillFrozen, cmd/contextbenchscore/freeze_test.go | real two writers refuse new rows and leave original bytes intact, diagnostic names kept command |
| TestCorpusImportIsNotOutcomeWriter, cmd/contextbenchscore/import_test.go | original corpus selection/derivation bytes preserved, cannot masquerade as outcome/canonical cell |
| TestLegacyLedgerRenderParity, cmd/benchledgerrender/freeze_test.go | read-only renderer output unchanged, no append path introduced |
| TestLaneBenchCards, internal/capability/card_closure_test.go | every shipped exact surface has card; missing/orphan/duplicate/conflicting identities fail; valid concrete schema accepted, invalid fields rejected; pending reference never proof |
| TestBenchContractsAPIHouseRules, internal/benchledger/lane_contract_test.go | existing parent wire/enums/errors/[]/I/O context contract, safe outcome keys and no future fields |

For each row: matching rg '^func TESTNAME\(' at EXACT file must succeed before
go test -race EXACT_PACKAGE -run '^TESTNAME$' -count=1. These literal path/name
pairs are the guard contract. Missing executable/dependency/package/test or empty
-run is absence, never observed RED. Record commands/exits and actual assertion
or compile failure independently. Implementing critic compares parent and table
assertion by assertion, weaker test FIX. #262 steward guarded-proof proposal is
read and source-consistent. W5-3 approves the exact serial manifest, not
activation/prerequisite proof or a new batch-RED mandate.

No test, readiness harness, paid cell, ledger append, schema generation or gate
ran here. Draft diff check/secret scan and both git identities are the evidence.
No row checkbox or forge state changed. W5-3 settles the parent and this
consolidation grant choice; no contract choice remains. No schema implementation.
Decision models: none; identity/provenance/closure are deterministic.

W5-3 public status: approved contract only. Exact immutable parent semantics are
CTR-13 at 7c67d10654566da08a77c52413b9e5b3227bf0ae with W5-3 grants finalized in
its own append-only compliance commit. No new model, writer, API or assertion is
introduced here. BEN-1 requires every B1-B3 proof; BEN-3 is capability closure,
not all REC-10. Implementation/RED/readings remain pending.
