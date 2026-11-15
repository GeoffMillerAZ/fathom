# Records integration: schema fixtures, REC-8 backfill and vet gate

Status: Approved 2026-11-15 under W5-4. Contract only; implementation proof pending.
Base: d9eef4794afdbc200d1b8697ce121d9d688fedaf, freshly fetched origin/main.
Owns only this document and records-integration-backfill.md. No product/schema/
gate/plan-title edits. Companion is a frozen candidate manifest, not an executed tool.

## Governing intent and reuse before design

Read intent router, P2/P3/P17/P22, I38, compatibility plan-v2/verify0/1/2, docs/schema
instructions, typed-records CU-D2/5/7/8 and REC-2/6/7/8/10; parallel-lanes PL-D1/2/5.
W4-2 settles CTR-11's exact serial schema/parser/CLI grants and separate backfill,
five-trial median/all-trials reporting with >2x flags. W3-2/4 settles implementing
real RED/assertion-by-assertion critics and typed additive API mechanics.
No syntax-trial or decision-file split trigger fired. Do not invent extra record
fields, change current author syntax, conflate vet with verify or add a store.

This reuses, never duplicates the initial contract:
- rec-1 approved W2-2, final exact source 5ef375e4bb44364bbc1c703aff234f76c9383cce.
- ctr-11 W4-2 compliant/critic LAND source 7db03afbda7fb935b331b3b20b226b1a6b8e4ebf.
- ctr-13 reviewed source 7c67d10654566da08a77c52413b9e5b3227bf0ae, with exact
  serial integration grants approved W5-3. No simultaneous schema writer inferred.
They are dependencies for integration; independent branch includes no copied schema.

D1 settled CU-D2/REC-2/schema rules: eight current heads, labelled Markdown source,
positive/live and negative/commented CUE pairs, Go rejection checker.
Alternative migrating authored syntax before trial is excluded.
D2 settled REC-8/CU-D8/W4-2: one serial parser/schema/actual-console turn and
separate frozen backfill; alternative concurrent plan/schema writers excluded.
D3 settled REC-7/CU-D7: separate vet gate, existing verify exits unchanged.
Alternative record-shape failures as verify exit1 violates compatibility.
D4 settled W4-2 T: all five cold and changed-file trials, median, >2x flags;
alternative dropping outlier/only reporting median excluded.
No generic API-spelling question. Exact initial typed heads/conditions/loader stay.

## Exact ownership sequence

1. REC-1 releases lane.cue/schema.cue and testdata/{valid,invalid}.cue at approved
   implementation handback, then #294 owns ONLY heads.cue and sixteen heads files
   named by CTR-11. Lane schema is read-only, not replaced.
2. #294 transfers schema read-only to #293 parser, then #292 evaluator; #318 owns
   LoadRecords/fusion adapter. #319 receives completed port before CLI/gate wiring.
3. #317 one architect turn owns internal/verify/{annotation_keys.go,
   annotation_keys_test.go,markdown.go,verify.go}, schema/plan/schema.cue,
   cmd/fathom/{plans_facts.go,plans_facts_test.go}, internal/console/{plans.go,
   plans_test.go}. It uses actual planTaskFact -> PlanRowFact/RollupPlan/ProofBarOf,
   not a speculative project.go edit. Current IsOpTitle is retired after callers
   transfer to Task.Gate. Five fields/JSON map exactly CTR-11; no new model here.
4. Backfill is a distinct exclusive tool turn after parser/schema/consumer proof:
   W5-4 approved cmd/recordbackfill/main.go and main_test.go plus ONLY the exact
   plan files/rows in companion manifest. No permission from a directory wildcard.
5. #319 owns cmd/fathom/{rec_vet.go,rec_vet_test.go}, Taskfile.yml and lefthook.yml
   in one gate-owner turn, after current owner committed handback. Existing
   cmd/fathom/iac.go/iac_test.go remain #164's actual command boundary; no overlap.
   rec registration uses existing declareVerb, no rewrite of another verb.
6. #299 card-only data.cue and cards/{valid,invalid}.cue namespace follows heads;
   capability shape is CTR-13's existing Card/#Capability, never a duplicate model.
   REC-10 also names Lane/Standard/NamedQuery/ReadingHead/Knob; capability closure
   alone CANNOT claim entire REC-10. Those other kind contracts remain unclaimed
   if absent. #299 may complete BEN-3 plus its capability subtask only. No
   speculative standard/query/knob fields or blanket testdata co-ownership.

W5-4 settles U1: cmd/recordbackfill/main.go and main_test.go are committed with
their real tests in a separate exclusive tool turn. Run the tool once in ONE
integration batch against the frozen companion manifest. The backfill commit
names the exact tool commit id. Only AFTER the backfill has been verified, remove
the tool and its tests in the immediately following commit. No product
rec-backfill verb, permanent migration runner or broad plan-file grant.

A refresh that ONLY ADDS rows landed by this wave may be re-approved by the
contract critic. Publish original/refreshed manifest identities and exact wave
commit for each added row; retain all original entries/transformations unchanged.
Updated file hashes must be explained solely by those additions. A changed,
removed, retargeted or otherwise modified existing row, non-wave addition or
other manifest change returns to the architect. No self-approved moving allowlist.
This contract does not refresh the existing 72-file/661-row candidate manifest.

## Eight explicit fixture recipes and real CUE checker

CTR-11's exact schemas are the authority. Fixture examples below are local test
records, not allocated/live ids. For each listed kind, positive candidate is its
schema applied to this exact JSON data; negative pair is the same bytes with id
removed. Each also gets derived-field/enum/missing-label subtests in the Go checker.
All conditions lists deliberately [] because a field fixture is not an expiry.

~~~json
{
 "decision":{"kind":"decision","id":"ADR-10000","title":"Example","ruling":"proposed","supersedes":[],"conditions":[]},
 "invariant":{"kind":"invariant","id":"I10000","statement":"Example","enforced_by":[],"conditions":[]},
 "goal":{"kind":"goal","id":"G10000","title":"Example","bet":"example","instrument":"task example","threshold":">= 1","review":"2026-12-01","conditions":[]},
 "future":{"kind":"future","id":"F10000","title":"Example","confidence":"plausible","horizon":"later","conditions":[]},
 "question":{"kind":"question","id":"Q10000","title":"Example","conditions":[]},
 "incident":{"kind":"incident","id":"INC-10000","title":"Example","outcome":[],"conditions":[]},
 "observation":{"kind":"observation","id":"O10000","title":"Example","outcome":[],"conditions":[]},
 "ask":{"kind":"ask","id":"ask-example","title":"Example","conditions":[]}
}
~~~

Generated CUE positive file: package records, then candidate: #<Kind>Head & JSON
object. Negative file is fully commented, named missing-id case. Reuse existing
internal/modspec/schema_test.go's checker pattern: load schema bytes, strip only
fixture package declaration, cuecontext.CompileString and Validate(cue.Concrete(true)).
Assert negative error mentions id, not just any compile error. Every kind rejects
authored phase/condition-result, bad kind and its invalid closed enum; required
label deletions name the actual field. Future measured basis without cite and real
calendar-invalid review date test Go law as well as schema. No live invalid CUE
breaks cue-vet. All output slices nonnil and wire enums/errors already CTR-11.

Existing/proposed exact #294 proof remains TestLaneHeadSchemas in
internal/records/lane_contract_test.go: eight valid concrete cases, eight missing-id
negatives, all derived-field/enum/label negatives and exact field diagnostics.
Command go test -race ./internal/records -run '^TestLaneHeadSchemas$' -count=1,
preceded by rg declaration guard. No task test:affected substitute. Proposed tests
are implementing-lane real RED under W3-2, not executed by this draft.

## Frozen backfill transformation and proof

Companion lists exact base/file SHA/line SHA/row id/desired gate/lane. It selects
only checkbox titles starting "(op) " and actual "(lane: slug)" metadata outside
backticks, top-level plans only. This lexical candidate selection is NOT native
parser proof: implementation first verifies every entry against
verify.ParseMarkdownPlan/ParseAnnotation, and rejects unmatched/missing/extra
metadata candidates or source/hash drift. Never dynamically broaden manifest.

Tool uses existing verify.UpsertAnnotationField for lane/gate keys, not an
independent brace/quote writer. Only strip the exact frozen title marker and add
gate:true/lane:"slug"; preserve checkbox, id, all proof keys/needs/dispositions/
waivers, body prose and line endings. Existing conflicting metadata refuses, not
overwrites. Idempotent second run yields no diff. Default --check writes nothing;
explicit --write requires the exact reviewed target inventory from the manifest
base (or its W5-4 critic-reapproved additive refresh) and all target hashes match.
The separate tool commit may add ONLY its approved tool/test paths on top of that
source state; it does not authorize unrelated tree drift. Bind tool commit and
manifest identity before writing, rather than require the pre-tool whole tree to
equal a post-tool tree. Atomic per-file writes; failure before all
validation writes nothing. No third-party tool/dependency or global config.

Proposed cmd/recordbackfill/main_test.go::TestRecordBackfillExactManifest:
all frozen rows resolve native parser, source hashes/ids match, marker removal
and typed metadata preserve every other Task field and verdict on fixed corpus;
crossed titles/plain gate key follow actual planTaskFact; state/needs/waiver never
change; conflicting key/source drift/extra candidate/outside path refuses with
zero writes; second run no diff; quotes/escaped annotation string preserved.
Command go test -race ./cmd/recordbackfill -run '^TestRecordBackfillExactManifest$'
-count=1, one line, declaration guard first. Run current verify parser parity
fixtures under -race; a checksum/list presence is not proof a backfill is safe.

## Vet gate and W4 timing proof

#319 TestLaneRecordVet and TestRecordVetTimingReport remain exactly CTR-11.
Real command cmdRecVet -> LoadRecords -> shared REC-1 ValidateInventory/ownership/
effective grandfather policy + schemas/links/cycles. Taskfile/lefthook invoke this
separately; no existing gate removed. Actual CLI runs on valid/invalid/unreadable
fixture repo give 0/1/2. Record malformed/unknown corpus is not healthy.
Native parser/source coverage, not a fabricated registry list, determines corpus.

REC-M3: five complete cold-process trials and five single-changed-file trials on
the exact frozen full corpus, same binary/clock/command. Publish every duration,
two medians, units and source/digests. Bars <2s/<200ms. Flag any single >4s/>400ms;
exact twice is not over. Missing trial is incomplete, never drop an outlier.
Author C owns measurement populations outside this grant; request handoff rather
than alter thresholds. No timing or passing gate is measured here.

## Evidence limits and decisions

Evidence source sites: intent/corpus+family+futures+GoalBet, verify parser/upsert,
modspec concrete schema tests, actual plans_facts/console reader, current command
registration and shared gate config. Source hashes in companion computed read-only;
native-parser/backfill/CUE/RED/race/gate execution remains pending.
No plan edits or claims in this branch. W5-4 settles tool ownership/lifecycle;
all contract decisions cite settled texts/rulings. No executed backfill implied.
Decision models: none; deterministic parsing, identity and timing reports.

## W5-4 lifecycle acceptance

Extend TestRecordBackfillExactManifest in cmd/recordbackfill/main_test.go with:
a fixture tool-only commit above the frozen source state is admitted; unrelated
source/target drift refuses before writes; exact tool commit id and manifest
identity appear in the generated integration receipt. A critic-approved refresh
adds only wave-landed rows and preserves every old entry; missing wave identity,
old-row edit/removal or non-wave addition cannot use the critic-only path.
Existing parser parity, zero-write refusals, byte-preservation and idempotence
assertions remain. Proposed/unrun; same exact guarded race command above.

Controller verifies the commit sequence: T contains only tool plus real tests;
B is the one integration backfill batch and names T; its actual parser/consumer/
gate/verify proof is bound to B; the immediately following cleanup commit removes
the tool/tests only AFTER that verification. Missing/failed verification prevents
cleanup and completion. Keep T/B/proof/cleanup identities in the handback, not a
fiction that tool deletion itself proves migration. No native parser or backfill
has run in this author lane.

The REC-1/#252 cross-family corpus conflict is an external blocked prerequisite.
This contract grants no migration or checker-policy exception, never weakens
global uniqueness and never treats a stopped #252 implementation as completed.
