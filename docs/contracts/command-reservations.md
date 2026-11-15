# Retained delivery command/hook reservations and reconstruction handoff

Status: DRAFT Author A for #267/#272/#273/#276/#296/#298/#304/#314/#322.
Base: d9eef4794afdbc200d1b8697ce121d9d688fedaf. Owns this document only. No product or hook implementation.
Normative parent CTR-12 d7dba4e681e910aa26a58adbb13dd7d50e491b28 is critic LAND
under W4. Reuse its complete typed ports/JSON/assertions; do not copy a second model.
FLT-3/10 are absent here and have their own later delivery-integration contract.

## Intent, decisions and prerequisites before design

Read intent router, I9/I12/I38/I53, P2/P12/P15/P18/P22, parallel-lanes §4 hook order,
realignment ENG-7/8/9/WIR-1/CHG-3/8/1, proof-program BEN-4 and fleet-picture FLT-0/1.
W3-2/4 and W4-2 settle implementing real RED, typed API mechanics, retained serial
grants, full CI/every bundle and 26/256 advisory-only holdout. No new policy/rate.
Unknown source or harness shape remains unknown, no guessed edit form. No provider/
test execution by product, future field, default value disclosure or new door package.

D1 settled parallel-lanes §4/W4-2: one shared-file owner in this order:
ENG-8 #273 -> ENG-9 #272 -> WIR-1 #298 -> CHG-3 #296 -> BEN-4 #267 -> CHG-8 #314
-> CHG-1 #322. Other new facade files never authorize concurrent hook edits.
D2 settled existing source/PL-D2: #164's actual command boundary is iac.go/iac_test.go,
not all cmd/fathom. A source claim is not a full-package grant.
D3 settled #304/FLT-0/1: real fleet report and registered /fleet page share model,
browser launch witness retained; a new facade's unit test alone is insufficient.
D4 settled CHG-8/ADR-100 d6: reconstruct proposed FULL file from actual edit form;
flattened new-string fragments are not the proposed file. Missing form refuses
advisory evaluation, never claims correct pre-write proof.
D5 settled CHG-1/LIB-13: admitted CODE replay card only, internal fact delta, no
public delta API or infrastructure-card admission. CHG-8 and replay bars remain.

Every issue retains its authored dependencies: #267 CHG-3/BEN-1/CTR-12/CTR-13/path
gate; #272 ENG-8; #298 ENG-9; #296 WIR-1/ENG-9; #276 ENG-9; #304 ENG-7/9/10+REC-8;
#314 CHG-M2/BEN-4; #322 CHG-8. Contracts are not landed implementations or RED logs.

## Exact command/MCP ownership and cross-author request

Frozen candidate serial grants below incorporate parent W4 choices. New extra
sites are explicitly identified for the one U1 grant choice, not inferred active.
REQUEST-A-command-reservations.md asks Author B for exact tools.go/IaC handback and
Author C for read-only CHG-M2 fixture/run provenance; never edit their clones.

| Exclusive exact paths | Serial turn / responsibility |
|---|---|
| cmd/fathom/iac.go, iac_test.go | #164 only; A reads, does not edit |
| internal/mcp/tools.go | B #167/#168 IaC context handback -> A #273 actual find_symbol handler/hookLookup integration |
| internal/mcp/hook_lookup.go, hook_lookup_test.go | #273 new bounded stored lookup, no backend interface expansion |
| cmd/fathom/sessionhook.go | #273 lookup -> #272 attribution -> #298 once/session stamp -> #296 test advice -> #267 delivery -> #314 reconstruction |
| cmd/fathom/sessionhook_events.go, edithook.go | #272 -> #296 -> #267 -> #314 -> #322 code-card call, one owner each |
| cmd/fathom/beats.go, discoveries.go, internal/mcp/calllog.go | #272 -> #298 -> #267 archive/attribution |
| cmd/fathom/doctor.go, doctor_harness_firing.go, doctor_wire_stamp.go | #298 WR-4/WR-12 reuse, no second ladder/ledger |
| cmd/fathom/testrunhook.go, testrunhook_delta.go, edithook_teststorun.go and their existing *_test.go files | #296 after #272/#298, actual cmdTestRunHook/testRunEntryHook -> testrun.FindCandidates; testrunhook*.go grants are U1 additions |
| cmd/fathom/fleetreport.go, fleetreport_test.go, console_fleet.go, console_test.go; internal/console/fleet.go | #276 typed presence adapter -> #304 actual cmdFleetReport/buildFleetReport and buildFleetPage /fleet parity |
| cmd/fathom/fleet_health.go/fleet_health_test.go, observatory_fleet_health.go/observatory_fleet_health_test.go | #304 new adapters, not a nonexistent observatory_fleet.go |
| cmd/fathom/hook_proposed_file.go, internal/hookpath/proposed_file.go/proposed_file_test.go | #314 actual pre-edit reconstruction; original owned paths |
| cmd/fathom/edithook_test.go, whatif_replay_test.go | #322 actual cmdEditHook/renderPostEdit replay after #314/BEN-4 |
| internal/factdelta/code.go, code_test.go (new exact directory slice) | #322 internal removed-symbol/route card delta only; no public library export |

Parent's other exact issue-local new adapters/tests remain their own issues'
allowlists: hook_attribution.go/test, wire_stamp.go/test, suspects.go/test,
delivery.go/test, hook_presence.go/test. Current whole-command package ownership
does not override this file manifest. All listed predecessor handbacks must name
commit/base before next owner writes. No touch to B's iac_facts/iaccatalog/binding/
public conversion/review_tool.go files. No broad schema/tool-registry grant.

U1 (<300 chars): approve the additional exact testrunhook/codec-reconstruction
sites below as one serial integration turn. Recommend actual source sites and
existing codecs; alternative keep leaf-only issues unready until a separate
architect adapter lane lands. Parent W4 grants are not re-escalated.

## Source-grounded reconstruction adapter (#314)

Current HookInput.NewContent/OldContent concatenate Edit/MultiEdit fragments;
Codex ApplyPatchFirstFile keeps only first file, ApplyPatchContent concatenates
patch lines. Those fields cannot prove a full-file or multi-file proposal.
cmdEditHook retains raw JSON before codec.DecodeInput: use that exact raw input
at its pre-edit boundary, not a post-edit file mistaken for proposed text.

Keep parent's pure ReconstructProposedFile(base, []Edit) helper/offset law unchanged.
Add pure adapters in internal/hookpath/proposed_file.go:
~~~go
func AffectedProposalPaths(codecID string, event harness.Event, raw []byte) ([]string, error)
func ReconstructHarnessFile(codecID string, event harness.Event,
    raw, base []byte, path string) ([]byte, error)
type ProposalReason string
const (
    ProposalUnsupported ProposalReason = "unsupported-shape"
    ProposalAmbiguous ProposalReason = "ambiguous-match"
    ProposalMissingSource ProposalReason = "missing-source"
    ProposalOutsidePath ProposalReason = "outside-path"
    ProposalMalformed ProposalReason = "malformed"
)
type ProposalRefusal struct { Reason ProposalReason `json:"reason"` }
func (e *ProposalRefusal) Error() string
type ProposalLimit struct { Limit int64 `json:"limit"`; Observed int64 `json:"observed"` }
func (e *ProposalLimit) Error() string
~~~

Codec registry id is existing open registry identity, not a new closed string enum.
Event is existing harness.Event. No context on pure adapters. Paths nonnil and
sorted unique; raw/base/output source bytes never serialized or echoed by errors.
No new raw-content field on exported HookInput and no body value in a product
answer. Current helper caps match cmdEditHook's 1 MiB raw-input bound; cap+1 typed
limit before partial parsing. Path must be the exact normalized repo-relative
target selected by decoded envelope, not an arbitrary caller override.

Additional proposed U1 exact path grant:
internal/harness/proposal.go/proposal_test.go (shared private-envelope access helper),
internal/hookpath/proposed_file.go/proposed_file_test.go (adapter/application),
cmd/fathom/hook_proposed_file.go/hook_proposed_file_test.go (I/O facade and test),
cmd/fathom/edithook.go/edithook_test.go (real call, existing serial turn).
Existing Codec interface, descriptors and EncodeOutput remain unchanged.
Codec-specific decoding reuses the current package's private envelope helpers;
no copying a second identity/session/path parser into cmd.

Frozen supported form/assertion matrix:
- Claude Write: current claudeToolInput.content is the WHOLE proposed body,
  including valid empty write. Never append it to old body.
- Claude Edit: exact old_string -> new_string on a unique match in current base;
  zero/multiple match returns typed refusal, not approximate matching.
- Claude MultiEdit: current edits[] old_string/new_string pairs apply in supplied
  sequence to intermediate body. Concatenated HookInput.NewContent is never used
  as a full-file body or paired with unrelated old text.
- Codex apply_patch: use codexToolCommand's actual patch bytes, parse ALL file
  sections, exact context hunks, add/delete/move headers. Affected paths includes
  every source/destination; unique context match required, no first-file-only
  claim. Unsupported/binary/malformed/ambiguous hunk refuses, not partial success.
- Gemini current codec only retains file_path/command; no full-content/edit body
  contract is present in the shipped source. Copilot's source explicitly calls its
  tool_input shape an assumption and retains file_path/path/command only.
  Therefore no invented Edit/Write body fields: these unproven forms return
  ProposalUnsupported and no what-if verdict. An actual pinned payload fixture
  contract is required before claiming CHG-8 parity for them. A unit fixture
  labelled invented cannot clear that requirement. Harness registry/data-only
  or unknown codec similarly reports unsupported and fail-open output.

I/O facade in hook_proposed_file.go (unexported cmd helper) receives raw bytes and
selected codec/Event from cmdEditHook, obtains complete original files only after
AffectedProposalPaths validates them, resolves within checkout/no outside symlink,
reads under current byte/deadline bounds, reconstructs, then passes whole proposed
content to admitted card reader. It writes no user file and executes no edit/shell.
Post-edit cannot be used as pre-edit proof. Source/codec gap -> named unknown/no
advisory card, mandatory guards still deliver. CHG-8 stays unclaimed for unsupported
harness forms until real fixture/wiring proofs exist; no approval can invent data.

## Exact test/fixture and route handoff

Proposed fixture files under cmd/fathom/testdata/proposed/:
claude-write.json, claude-empty-write.json, claude-edit.json, claude-multiedit.json,
codex-multifile.json, codex-add-delete-move.json, gemini-no-body.json,
copilot-no-body.json, base.go. Exact added grant U1; no copied real transcript.
Use source fields above, fixed fixture cwd resolved in test root, no private path.

TestLaneHookEditRebuild in internal/hookpath/proposed_file_test.go must assert all
matrix cases, sequential edits, empty vs delete distinction, all Codex paths,
duplicate context/old text and missing base refusals, byte limit+1, traversal/
symlink outside root, unknown codec, unsupported Gemini/Copilot no verdict. Keep
parent offset helper overlap/original-base assertions; do not replace them.
TestEditHookReconstructsWholeProposal in cmd/fathom/hook_proposed_file_test.go calls
actual cmdEditHook pre-edit with fixtures and fake downstream reader, asserts
whole body received, no host file written, missing/unsupported never evaluates a
partial proposed file. Mandatory error/guard payload remains fail-open/full.
TestProposalCodecPaths in internal/harness/proposal_test.go asserts identity/path
parity with existing DecodeInput for known forms and all Codex files retained.

Guard exact declarations then commands (each one line):
go test -race ./internal/hookpath -run '^TestLaneHookEditRebuild$' -count=1
go test -race ./internal/harness -run '^TestProposalCodecPaths$' -count=1
go test -race ./cmd/fathom -run '^TestEditHookReconstructsWholeProposal$' -count=1
These are proposed real RED tests for implementing lanes, unrun here.

Retained parent tests/assertions, no weakening:
TestLaneHookLookup (bounded stored lookup and no off-surface graph liveness);
TestLaneHookAttribution (actual session/lane/caller rows >=95% complete denominator);
TestLaneWireStamp (four revision inputs/once-session/idx/doctor unknown);
TestLaneRedTestSuspects (real testrunhook wrapped failing test -> FindCandidates,
unresolved tests retained); TestLaneDeliveryLog plus archive/mandatory-channel/
call-order/preflight tests (exact raw generations/coverage/26-of-256 exclusions);
TestLanePresenceMetrics (eligible/fired/delivered/unknown distinct);
TestLaneFleetGround and TestFleetReportPanelPresence (real CLI + registered /fleet
same value/unknown reasons); TestEditHookCodeCardReplay/TestLaneWhatifDelta (actual
cmdEditHook whole-file input, admitted code card only, no public/infra delta).
Files/packages/complete assertions are incorporated from immutable CTR-12, never
a generic “run affected tests” substitute. Declaration guards before anchored
race invocation; absent test/tool/package is not observed RED.

#304 FLT-0 witness uses actual launch path and registered /fleet with pinned browser,
both themes/narrow/wide plus keyboard/error states per app-kit and fleet design.
No new page markup authored; existing buildFleetReport/buildFleetPage parity is
the source. A screenshot filename or facade test alone cannot prove launch.
No browser launched or visual/runtime claim by this author.

## Proof and coordination limits

#314 waits on Author C's CHG-M2 fixture/run receipt at unchanged 500 ms bar and
BEN-4 archive/delivery implementation; unsupported payload shapes remain missing
evidence, not authorized future fields. REQUEST file records cross-author order.
#322 reads only approved internal code delta/card fixtures after CHG-8; its two
reading files stay honest about replay eligibility/noise vs post-write baseline.
Neither source presence nor this contract claims those readings or prerequisites.

Executed source/history review and git diff --check/secret scan, both identities.
No new code/dependency/schema, RED/race/CLI/hook/browser/latency/gate execution,
live grant, source-state change or row/dispatch claim. U1 names only actual extra
sites; parent settled choices and missing raw evidence are not generic escalations.
Decision models: none; reconstruction and factual route joins are deterministic.
