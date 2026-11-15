# CHG-10 served-rule ranking and proposer join

W4-3 draft, gap #277. Base d9eef4794afdbc200d1b8697ce121d9d688fedaf.
Own only docs/bench/readings/realignment/chg-10-rules-compiled-once.md in its
later L-R lane, plus workspace scratch. RL-9 and RL-8 depend on this reading.
No rule is authored/accepted, no population measured and no prerequisite claimed.

## Choices and governing text

| ID | Recommendation / citation | Alternative |
|---|---|---|
| D1 | Rank served prose rules by actual rule-ID firing counts in the frozen 168h interval from #277, with complete raw coverage. realignment CHG-10, I38/P7/P22. | Rank hook channel totals or call counts. Rejected: neither names every served rule. |
| D2 | Count qualifying RULES among top20, not distinct kinds. Kill at fewer than5 mapped rules, exactly5 survives. CHG-10 wording: fewer than five of the top twenty map to a kind that exists or a lane could write. | Demand five different kinds or infer a percent. Rejected: changes bar. |
| D3 | Explicit claim/citation-to-proposer join, retaining no-match/ambiguity and advisory-only. CHG-10 names intent adr-rules, intent export --guard, findings candidates; ADR-106 acceptance still separate. | Text resemblance or a candidate's label means enforced check. Rejected. |
| D4 | Top-only log cannot establish all-served population; incomplete retained, no silent rescope. I38, ruleoutcome_track.go intentional top-only scope. | Call top-rule exposure a complete ranked rule census. Rejected. |

No initial CTR-10..13 draft supplies this ranked population. CTR-12 delivery rows
(91860745...) are not per-rule firing history. W3-4 creates no telemetry grant.
No applicable future is prebuilt, and no new evidence-trigger fired by this draft.

## Grounding: current records, writers and consumers

cmd/fathom/ruleoutcome.go ruleOutcomeEvent v2 has schema_version,fathom_version,
ts,rule_id,event,session,file,output,sources. recordRuleOutcome(ruleID,event,
session,file string) appends JSONL with failures swallowed. ruleOutcomeLogPath()
uses FATHOM_RULE_OUTCOME_LOG or the user default. readRuleOutcomeSignal reads
only latest bounded 16MiB, not rotations, returns console.RuleSignal and retains
Skipped; its result is not a complete historical population by itself.

ruleoutcome_track.go trackRuleOutcome(sessionID,service,repoRelFile,claimID string,
agentID ...string) records served for the TOP claim, plus still_served/top_changed.
Callers: edithook.go's post-edit path and edithook_bashwrite.go reconstructed
writes. ruleoutcome_blocked.go records blocked; claim.go records refuted/corroborated;
ruleoutcome_generated.go records generated-source events. Do not combine those
kinds as separate firings of a served prose rule. Old violated/complied are
CanonicalRuleEvent persistence/change, not actual violation/success labels.

hookdedupe.go (*hookDedupeCheck).record emits hook-dedupe beat metadata channel,
emitted,bytes,suppressed_bytes,key,first_touch,state_error. It has NO rule-ID list.
metrics_channels.go cmdMetricsChannels accepts --hooks-log --from --to --json;
internal/metrics.AggregateChannels returns channel/guard/module totals. This is
context and coverage evidence, not the CHG-10 ranking source. The copied CB-M2
session>=50 and byte/suppression criteria in #277 do not apply to CHG-10.

Proposer sites:

* cmd/fathom/intentadrrules.go cmdIntentADRRules --root --json returns root,
  decisions,rule_pack,proposals. adrRuleRow embeds review.RuleProposal and adds
  in_rule_pack. internal/review/adrrules.go ProposeADRRules extracts actual ADR
  dependency statements with their source line and from/to package constraint.
  Its rule-pack identity is archconformance.RuleKey, not a semantic claim ID.
* cmd/fathom/intentexport.go cmdIntentExport --root --guard prints generated
  CUE scopes/notes from the intent router. No --json flag; --write is forbidden
  in this reading. intentGuardEntry is Scopes,Note, not a check evaluator. A
  pointer-only guard proposal does not by itself prove a computable predicate.
* cmd/fathom/findings.go cmdFindingsCandidates --store --service --min-sites 3
  --json calls findings.RuleCandidates(claims,events,minSites). RuleCandidate
  carries class,shape,sites,members {claim_id,subject,scope,producer},producers,
  code_change_resolutions,draft,check_family,score,pattern_id,disputes.
  internal/analysis/findings/candidates.go checkFamilyFor maps classes to
  secret_literal/import_direction/usecase_maintained/workspace_output or advisory.
  Advisory is not an implemented check. The coarse family suggestion is not
  a correctness oracle for the original rule.

No proposed API or function changes. Existing tests are reference semantics only:
cmd/fathom/ruleoutcome_track_test.go and ruleoutcome_read_test.go; findings
candidates_test.go; intentadrrules_test.go; intentexport_test.go. Their passing
would not prove a historical operator reading. No tests ran here.

## Frozen manifest, population and ranking

Before outcome access pin exact UTC [from,to) of length168h, binary hashes,
all raw signal/beat log generations and byte ranges, service/repository aliases,
claim snapshots (including superseded versions and citations), source/ADR/router
commits, proposer binaries/options/configuration, checker hash and independent
logging-coverage witness. Publish digests and opaque aliases; private paths/names
stay local. A seven-day file timestamp span is not complete delivery coverage.

Population is every actually delivered prose rule firing in the chosen indexed
repository population, each delivered rule in a multi-rule turn once. Suppressed
payloads and mere eligibility do not count. Resolve identical rules by actual claim
ID and source/revision, not normalized text. Same ID across services is joined
only with service/source evidence. ruleOutcomeEvent lacks service: file/session
alone may not resolve fleet collisions. Keep those as unknown. Repeated exposures
on different turns count; do not de-duplicate by rule ID before counting frequency.

Use event=served only if that record proves delivery, not merely selection prior
to suppression. Validate placement against the two hook callers and raw output or
independent delivery accounting. Top-only logs cannot expose remaining capped
rules, and non-top rules must not be invented. If no complete all-served export
exists, record top-only ranking as a named lower-bound diagnostic; main ranking
and verdict are INCOMPLETE. A separately reserved instrumentation lane would be
needed to close this, not an edit in this reading lane.

Rank complete eligible rule IDs by count descending, tie by comparable
(service_alias,rule_id,source_commit,citation_path,citation_line) ascending.
Take first20, publish all counts and the cutoff tie set. Fewer than20 actually
exposed rules does not authorize padding or scaling five down. Report exposure
insufficient and INCOMPLETE rather than make the kill easier with a shorter list.
Every malformed/unreadable/rotation-lost/unjoinable row is counted and named;
unknown rows that could affect top20 preclude an exact ranking. Do not replace
population errors with a top20 from a convenient surviving session.

## Join and exact grader

For each ranked row retain claim ID, governing source and text digest, cited
source range at the served revision, scope, count, all three proposer outputs
(raw hash + indexed row/line) and kind mapping. Produce these using pinned local
snapshots, never compile with a paid model or fetch at evaluation time:

```sh
fathom intent adr-rules --root "$SNAPSHOT_ROOT" --json
fathom intent export --root "$SNAPSHOT_ROOT" --guard
fathom findings candidates --store "$SNAPSHOT_STORE" --service "$SERVICE_ALIAS" --min-sites 3 --json
```

Proposals must be bound to that rule's actual source/citation/claim. ADR proposal
join is cited ADR range + from/to package pair + stable rule key. Findings join
is members.claim_id or its bound citation chain, with pattern_id and check_family.
Guard export join is exact router row/scopes+note source, preserving that it only
exports a pointer. No exact join => no_match; multiple unsupported matches =>
ambiguous; unread source/output => unknown. Similar prose alone never auto-joins.

A qualifying existing kind must name the actual evaluator, input shape, matching
predicate and fixture demonstrating it can enforce THIS rule without narrowing
it. A lane-writable kind must have a concrete bounded predicate, existing input
source, proposed exact ownership/test assertions and independent label/acceptance
prerequisites for a future lane. It counts as feasibility, not implemented or
accepted. A generic claim that a lane could write any arbitrary rule never counts.
Document no-match/ambiguous/advisory/not-computable separately, never erase them.
No newly invented kind is asserted as existing. A person signs each feasibility
mapping, with source citation; missing signature leaves it unknown.

Let k=number of top20 RULES with validated existing-or-lane-writable mappings.
Complete top20 and complete mappings with k<5 => KILL. k>=5 => reading PASS for
this entry only, not implemented rules or accepted predicates. If unknowns could
change the decision => INCOMPLETE; publish known k and possible bounds. A known
maximum k<5 can establish KILL even with named missingness, but no incomplete
ranking is treated as complete. RL-9 still requires port parity and RL-8 still
requires incident/CHG-11/CHG-12 and personal acceptance. No file written by fathom.

## Prospective exact scratch-checker tests

Implementing lane writes these before grading, critic checks every assertion.
No RED, GREEN or measurements have been observed by this author.

| Test | Assertions |
|---|---|
| TestServedPopulationCoverage | from included/to excluded,168h exact; current-only rotated log insufficient; top-only source cannot claim all-served; ambiguous service collision retained; selected-but-suppressed not delivered; same turn multiple rules each count; repeat turns count again; malformed/unreadable/missing generation prevents exact rank. |
| TestServedRanking | descending frequency; tie composite fields order stable; cutoff tie disclosed; 19 unique rules not padded; counts cannot come from channel bytes or CB-M2 session filter. |
| TestRankedRuleProposerJoin | exact ADR citation+pair joins; findings members.claim_id joins with provenance; router scope-note join retains pointer-only nature; prose resemblance no-match; ambiguous/foreign-source/recompiled-source mismatch unknown; advisory never existing check. |
| TestRuleKindFeasibility | evaluator+predicate+input+fixture required for existing; bounded predicate+exact future ownership/tests+independent label prerequisites+person signoff required for lane-writable; missing component unknown; candidate check_family alone insufficient. |
| TestCHG10FiveOfTwenty | k4 kills,k5 passes same-kind rules can supply all5; never require5 distinct kinds; missing mapping retains min/max and INCOMPLETE when decision straddles5; insufficient population no scaled threshold; PASS does not claim RL-9/RL-8 implemented or accepted. |

## Decision models

None in the reader. Deterministic joins and independent personal feasibility labels;
provider none. Rule drafting/acceptance is a separate downstream lane under ADR-106.

## Executable scratch handoff (critic correction)

Frozen PROPOSED interface, not implemented tooling or run evidence. W3-2 leaves
real failing tests to the implementing reader. All earlier assertions remain
binding. No product/telemetry/API grant. Synthetic test success cannot close
missing raw inputs, policies or dispatch prerequisites. READY is for document
review only. Resource-windows is final FIX round2; another unresolved finding is
HOLD. The other seven reading drafts are FIX round1 and have one round remaining.

Exact layout: workspace/tmp/reading-served-rules/checker.py (normalize/grade),
contracts.py (schema validation/declaration guard), test_checker.py (unittest
ContractTests), manifest.json, normalized.json, result.json and the three schemas
below. Fixture directory is fixtures/<exact-test-name>/ with input.json and
expected.json for EVERY assertion case in the earlier table. Implementing reader
creates these workspace-only files. Copy each JSON code block to its named file.

contracts.py implements only the closed JSON-Schema subset below (objects/
required/additionalProperties, arrays/items, enum/const, anyOf/null, minimum,
string pattern, date-time). Standard library only, no network schema fetch or
dependency. Check RFC3339 timestamps, unique artifact IDs, SHA256 and foreign
references, non-nil arrays, nonfinite numbers rejected. Quantity decimals use
canonical base10 strings; null upper bound is unbounded, never zero.
contracts.py guard parses the three Python files with ast without importing them,
requires checker normalize/grade entrypoints and ContractTests(unittest.TestCase)
with EACH exact method selected below declared once; checks the three schema
hashes pinned by the contract commit and fixtures for every case. Declaration
presence is never a substitute for running each assertion.

checker.py normalize --manifest PATH --output PATH validates manifest, reads ONLY
listed artifacts under this scratch root (reject absolute paths, traversal and
symlink escape), verifies raw hashes/lengths and uses only the named adapters.
SourceRef positions are half-open original byte offsets, record_index zero-based.
Every normalized row retains native lineage. Preserve identical distinct rows,
stack/frame/parent order and copy-overlap identity. Never synthesize missing facts.
Unreadable/absent native data appears as coverage gaps/nulls. No polling, capture,
network, provider, ingest, Compile/Sweep or live configuration mutation.

checker.py grade --manifest PATH --normalized PATH --output PATH revalidates
schemas, hashes, lineages, then recomputes all earlier criterion assertions from
native evidence, independent labels and bound policies. Declared PASS or approval
fields cannot supply evidence. Emit canonical UTF-8 JSON, sorted object keys;
criteria sorted by row_id,criterion_id,group_key and quantities by name. Every
authored clause and group appears, including missing required groups as INCOMPLETE.
Coverage includes all expected/read/malformed/torn/skipped rows and gaps.
complete=true needs independent accounting, not artifact presence. Pending policies
remain pending_policy_ids. Null/absent facts never become zero or empty success.
Unused group_key dimensions are empty strings. All result arrays are non-nil.

dispatch_state is BLOCKED for this draft interface; foreman approval plus real
input/prerequisite handoff is separate. Grade exit0 means valid computation,
including a FAIL/KILL/INCOMPLETE result, not measurement PASS. guard/validate/
normalize/grade exit1 means assertion/schema/identity/limit/refusal error; exit2
operational inability to read/write requested paths. Where output is writable,
errors have typed grader_errors kind and SourceRefs. TestCase exits nonzero on
any missing/weakened assertion. No new numerical bar or policy from this interface.

D-HANDOFF (W3-2, I38, L-R/parallel-lanes §3): recommend this exact standard-library
scratch layout, closed schemas and all tests; alternative ad-hoc reader protocol
is rejected because it leaves the handed contract unspecified. Proposed commands
below run from the clone root AFTER the implementing lane creates these files.
None ran in the author lane.

```sh
set -eu
cd "workspace/tmp/reading-served-rules"
test -f checker.py
test -f contracts.py
test -f test_checker.py
test -f manifest.schema.json
test -f normalized.schema.json
test -f result.schema.json
python3 contracts.py guard
python3 -m unittest test_checker.ContractTests.TestServedPopulationCoverage
python3 -m unittest test_checker.ContractTests.TestServedRanking
python3 -m unittest test_checker.ContractTests.TestRankedRuleProposerJoin
python3 -m unittest test_checker.ContractTests.TestRuleKindFeasibility
python3 -m unittest test_checker.ContractTests.TestCHG10FiveOfTwenty
python3 contracts.py validate --kind manifest --input manifest.json
python3 checker.py normalize --manifest manifest.json --output normalized.json
python3 contracts.py validate --kind normalized --input normalized.json
python3 checker.py grade --manifest manifest.json --normalized normalized.json --output result.json
python3 contracts.py validate --kind result --input result.json
```

### Ranked output binding

result.ranked_rules is the deterministic frequency ranking, rank1-based and at
most20 rows, retaining the cutoff tie flag. It joins normalized mappings by the
same structured rule_key, retains exact proposer IDs and independent label IDs.
An incomplete full population yields only labelled diagnostic rows with main
criterion INCOMPLETE, never a fabricated complete ranking. TestServedRanking and
TestRankedRuleProposerJoin also check this wire projection and its identity/count/
tie/unknown correspondence, preserving all earlier assertions.

### Native adapters

| Adapter ID | Frozen normalization contract |
|---|---|
| native_rule_outcome | ruleOutcomeEvent v2/all generations/canonical events; top-only served not guaranteed delivered. |
| native_delivery | Independent all-rule output per turn after suppression, tied to rule/source/session/service; absence keeps main population INCOMPLETE. |
| native_proposers | adrRuleRow/RuleProposal, guard CUE scopes/notes, findings.RuleCandidate; no fictitious --json guard/paid compilation. |
| independent_mapping | Exact source/citation/member joins + independent feasibility label, not text similarity. |

All input roles must appear in manifest.inputs. Missing roles use a named absent
Artifact with null hash/size and named coverage, never a placeholder evidence file.
Adapter IDs are dispatch keys of checker.py, not product commands. Independent
labels/derived classifications are checked against native source refs, not trusted
booleans. Prerequisite and policy authority must resolve to immutable receipts;
pending fields cannot be promoted by the reader. Unsupported native conversion
or missing capture contract keeps the corresponding consumer BLOCKED.

### manifest.schema.json

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {"schema":{"const":"fathom.reading.scratch/manifest-v1"},"protocol":{"const":"reading-served-rules"},"source_base":{"type":"string"},"preregistered_at":{"$ref":"#/$defs/Timestamp"},"window":{"type":"object","properties":{"from":{"anyOf":[{"$ref":"#/$defs/Timestamp"},{"type":"null"}]},"to":{"anyOf":[{"$ref":"#/$defs/Timestamp"},{"type":"null"}]}},"required":["from","to"],"additionalProperties":false},"artifacts":{"type":"array","items":{"$ref":"#/$defs/Artifact"}},"inputs":{"type":"array","items":{"$ref":"#/$defs/Input"}},"policies":{"type":"array","items":{"$ref":"#/$defs/Policy"}},"prerequisites":{"type":"array","items":{"$ref":"#/$defs/Prerequisite"}}},
  "required": ["schema","protocol","source_base","preregistered_at","window","artifacts","inputs","policies","prerequisites"],
  "additionalProperties": false,
  "$defs": {"Artifact":{"type":"object","properties":{"artifact_id":{"type":"string"},"relative_path":{"type":"string"},"sha256":{"anyOf":[{"$ref":"#/$defs/Hash"},{"type":"null"}]},"size_bytes":{"anyOf":[{"type":"integer","minimum":0},{"type":"null"}]},"state":{"$ref":"#/$defs/ArtifactState"},"error":{"anyOf":[{"type":"string"},{"type":"null"}]}},"required":["artifact_id","relative_path","sha256","size_bytes","state","error"],"additionalProperties":false},"ArtifactState":{"type":"string","enum":["available","absent","unreadable"]},"Hash":{"type":"string","pattern":"^[0-9a-f]{64}$"},"Input":{"type":"object","properties":{"role":{"type":"string","enum":["rule_outcome","hook_beats","delivery_accounting","claim_snapshot","source_snapshot","adr_proposals","guard_export","finding_candidates","feasibility_labels"]},"artifact_ids":{"type":"array","items":{"type":"string"}},"adapter":{"type":"string","enum":["native_rule_outcome","native_delivery","native_proposers","independent_mapping"]}},"required":["role","artifact_ids","adapter"],"additionalProperties":false},"Policy":{"type":"object","properties":{"policy_id":{"type":"string"},"status":{"type":"string","enum":["approved","pending","not_applicable"]},"authority_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"parameters_artifact_id":{"anyOf":[{"type":"string"},{"type":"null"}]}},"required":["policy_id","status","authority_refs","parameters_artifact_id"],"additionalProperties":false},"Prerequisite":{"type":"object","properties":{"id":{"type":"string","enum":["all_delivered_population"]},"state":{"type":"string","enum":["proven","missing","unproven"]},"evidence_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}}},"required":["id","state","evidence_refs"],"additionalProperties":false},"SourceRef":{"type":"object","properties":{"artifact_id":{"type":"string"},"record_index":{"type":"integer","minimum":0},"byte_start":{"type":"integer","minimum":0},"byte_end":{"type":"integer","minimum":0}},"required":["artifact_id","record_index","byte_start","byte_end"],"additionalProperties":false},"Timestamp":{"type":"string","format":"date-time"}}
}
```

### normalized.schema.json

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {"schema":{"const":"fathom.reading.scratch/normalized-v1"},"protocol":{"const":"reading-served-rules"},"manifest_sha256":{"$ref":"#/$defs/Hash"},"coverage":{"type":"array","items":{"$ref":"#/$defs/Coverage"}},"records":{"type":"object","properties":{"firings":{"type":"array","items":{"type":"object","properties":{"source":{"$ref":"#/$defs/SourceRef"},"at":{"$ref":"#/$defs/Timestamp"},"turn_id":{"type":"string"},"service_alias":{"anyOf":[{"type":"string"},{"type":"null"}]},"session":{"type":"string"},"rule_id":{"type":"string"},"source_commit":{"type":"string"},"citation_path":{"type":"string"},"citation_line":{"type":"integer","minimum":0},"event":{"type":"string"},"selected":{"type":"boolean"},"delivered":{"anyOf":[{"type":"boolean"},{"type":"null"}]},"prose":{"anyOf":[{"type":"boolean"},{"type":"null"}]},"delivery_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}}},"required":["source","at","turn_id","service_alias","session","rule_id","source_commit","citation_path","citation_line","event","selected","delivered","prose","delivery_refs"],"additionalProperties":false}},"proposals":{"type":"array","items":{"type":"object","properties":{"source":{"$ref":"#/$defs/SourceRef"},"proposer":{"type":"string","enum":["adr_rules","guard_export","findings_candidates"]},"proposal_id":{"type":"string"},"service_alias":{"type":"string"},"claim_ids":{"type":"array","items":{"type":"string"}},"citation_path":{"type":"string"},"citation_line":{"type":"integer","minimum":0},"from_pkg":{"anyOf":[{"type":"string"},{"type":"null"}]},"to_pkg":{"anyOf":[{"type":"string"},{"type":"null"}]},"scopes":{"type":"array","items":{"type":"string"}},"check_family":{"anyOf":[{"type":"string"},{"type":"null"}]},"pattern_id":{"anyOf":[{"type":"string"},{"type":"null"}]}},"required":["source","proposer","proposal_id","service_alias","claim_ids","citation_path","citation_line","from_pkg","to_pkg","scopes","check_family","pattern_id"],"additionalProperties":false}},"mappings":{"type":"array","items":{"type":"object","properties":{"rule_key":{"type":"object","properties":{"service_alias":{"type":"string"},"rule_id":{"type":"string"},"source_commit":{"type":"string"},"citation_path":{"type":"string"},"citation_line":{"type":"integer","minimum":0}},"required":["service_alias","rule_id","source_commit","citation_path","citation_line"],"additionalProperties":false},"proposal_ids":{"type":"array","items":{"type":"string"}},"join":{"type":"string","enum":["exact","no_match","ambiguous","unknown"]},"feasibility":{"type":"string","enum":["existing","lane_writable","advisory","not_computable","unknown"]},"predicate_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"input_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"evaluator_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"fixture_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"future_ownership_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"test_assertion_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"label_ids":{"type":"array","items":{"type":"string"}}},"required":["rule_key","proposal_ids","join","feasibility","predicate_refs","input_refs","evaluator_refs","fixture_refs","future_ownership_refs","test_assertion_refs","label_ids"],"additionalProperties":false}},"labels":{"type":"array","items":{"$ref":"#/$defs/Label"}}},"required":["firings","proposals","mappings","labels"],"additionalProperties":false}},
  "required": ["schema","protocol","manifest_sha256","coverage","records"],
  "additionalProperties": false,
  "$defs": {"Coverage":{"type":"object","properties":{"input_role":{"type":"string","enum":["rule_outcome","hook_beats","delivery_accounting","claim_snapshot","source_snapshot","adr_proposals","guard_export","finding_candidates","feasibility_labels"]},"artifact_ids":{"type":"array","items":{"type":"string"}},"expected_records":{"anyOf":[{"type":"integer","minimum":0},{"type":"null"}]},"read_records":{"type":"integer","minimum":0},"malformed_records":{"type":"integer","minimum":0},"torn_records":{"type":"integer","minimum":0},"skipped_records":{"type":"integer","minimum":0},"missing_intervals":{"type":"array","items":{"type":"object","properties":{"from":{"$ref":"#/$defs/Timestamp"},"to":{"$ref":"#/$defs/Timestamp"},"reason":{"type":"string"}},"required":["from","to","reason"],"additionalProperties":false}},"complete":{"type":"boolean"},"reason":{"anyOf":[{"type":"string"},{"type":"null"}]}},"required":["input_role","artifact_ids","expected_records","read_records","malformed_records","torn_records","skipped_records","missing_intervals","complete","reason"],"additionalProperties":false},"Hash":{"type":"string","pattern":"^[0-9a-f]{64}$"},"Label":{"type":"object","properties":{"label_id":{"type":"string"},"sources":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"labeler_role":{"type":"string","enum":["native_oracle","independent_person","architect"]},"labelled_at":{"$ref":"#/$defs/Timestamp"},"outcome_blind":{"type":"boolean"},"value":{"type":"string"}},"required":["label_id","sources","labeler_role","labelled_at","outcome_blind","value"],"additionalProperties":false},"SourceRef":{"type":"object","properties":{"artifact_id":{"type":"string"},"record_index":{"type":"integer","minimum":0},"byte_start":{"type":"integer","minimum":0},"byte_end":{"type":"integer","minimum":0}},"required":["artifact_id","record_index","byte_start","byte_end"],"additionalProperties":false},"Timestamp":{"type":"string","format":"date-time"}}
}
```

### result.schema.json

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {"schema":{"const":"fathom.reading.scratch/result-v1"},"protocol":{"const":"reading-served-rules"},"manifest_sha256":{"$ref":"#/$defs/Hash"},"normalized_sha256":{"$ref":"#/$defs/Hash"},"state":{"$ref":"#/$defs/EvidenceState"},"dispatch_state":{"const":"BLOCKED"},"criteria":{"type":"array","items":{"$ref":"#/$defs/Criterion"}},"coverage":{"type":"array","items":{"$ref":"#/$defs/Coverage"}},"unknown_inputs":{"type":"array","items":{"$ref":"#/$defs/Unknown"}},"pending_policy_ids":{"type":"array","items":{"type":"string"}},"grader_errors":{"type":"array","items":{"type":"object","properties":{"kind":{"type":"string","enum":["schema","identity","limit","refusal","decode","coverage"]},"source_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"message":{"type":"string"}},"required":["kind","source_refs","message"],"additionalProperties":false}},"ranked_rules":{"type":"array","items":{"type":"object","properties":{"rank":{"type":"integer","minimum":0},"rule_key":{"type":"object","properties":{"service_alias":{"type":"string"},"rule_id":{"type":"string"},"source_commit":{"type":"string"},"citation_path":{"type":"string"},"citation_line":{"type":"integer","minimum":0}},"required":["service_alias","rule_id","source_commit","citation_path","citation_line"],"additionalProperties":false},"delivered_firings":{"type":"integer","minimum":0},"proposer_ids":{"type":"array","items":{"type":"string"}},"mapping_label_ids":{"type":"array","items":{"type":"string"}},"mapping_state":{"type":"string","enum":["qualifying_existing","qualifying_lane_writable","not_qualifying","unknown"]},"cutoff_tie":{"type":"boolean"}},"required":["rank","rule_key","delivered_firings","proposer_ids","mapping_label_ids","mapping_state","cutoff_tie"],"additionalProperties":false}}},
  "required": ["schema","protocol","manifest_sha256","normalized_sha256","state","dispatch_state","criteria","coverage","unknown_inputs","pending_policy_ids","grader_errors","ranked_rules"],
  "additionalProperties": false,
  "$defs": {"Coverage":{"type":"object","properties":{"input_role":{"type":"string","enum":["rule_outcome","hook_beats","delivery_accounting","claim_snapshot","source_snapshot","adr_proposals","guard_export","finding_candidates","feasibility_labels"]},"artifact_ids":{"type":"array","items":{"type":"string"}},"expected_records":{"anyOf":[{"type":"integer","minimum":0},{"type":"null"}]},"read_records":{"type":"integer","minimum":0},"malformed_records":{"type":"integer","minimum":0},"torn_records":{"type":"integer","minimum":0},"skipped_records":{"type":"integer","minimum":0},"missing_intervals":{"type":"array","items":{"type":"object","properties":{"from":{"$ref":"#/$defs/Timestamp"},"to":{"$ref":"#/$defs/Timestamp"},"reason":{"type":"string"}},"required":["from","to","reason"],"additionalProperties":false}},"complete":{"type":"boolean"},"reason":{"anyOf":[{"type":"string"},{"type":"null"}]}},"required":["input_role","artifact_ids","expected_records","read_records","malformed_records","torn_records","skipped_records","missing_intervals","complete","reason"],"additionalProperties":false},"Criterion":{"type":"object","properties":{"row_id":{"type":"string","enum":["CHG-10"]},"criterion_id":{"type":"string","enum":["all_served_coverage","top_twenty_rank","proposer_join","existing_or_feasible_mapping","five_of_twenty"]},"group_key":{"type":"object","properties":{"service_alias":{"type":"string"},"population_id":{"type":"string"},"dialect_id":{"type":"string"},"utc_day":{"type":"string"},"channel":{"type":"string"}},"required":["service_alias","population_id","dialect_id","utc_day","channel"],"additionalProperties":false},"state":{"$ref":"#/$defs/EvidenceState"},"quantities":{"type":"array","items":{"$ref":"#/$defs/Quantity"}},"evidence_refs":{"type":"array","items":{"$ref":"#/$defs/SourceRef"}},"unknowns":{"type":"array","items":{"$ref":"#/$defs/Unknown"}}},"required":["row_id","criterion_id","group_key","state","quantities","evidence_refs","unknowns"],"additionalProperties":false},"EvidenceState":{"type":"string","enum":["PASS","FAIL","KILL","INCOMPLETE","INCONCLUSIVE","HOLD"]},"Hash":{"type":"string","pattern":"^[0-9a-f]{64}$"},"Quantity":{"type":"object","properties":{"name":{"type":"string"},"unit":{"type":"string"},"value":{"anyOf":[{"type":"string"},{"type":"null"}]},"lower_bound":{"anyOf":[{"type":"string"},{"type":"null"}]},"upper_bound":{"anyOf":[{"type":"string"},{"type":"null"}]}},"required":["name","unit","value","lower_bound","upper_bound"],"additionalProperties":false},"SourceRef":{"type":"object","properties":{"artifact_id":{"type":"string"},"record_index":{"type":"integer","minimum":0},"byte_start":{"type":"integer","minimum":0},"byte_end":{"type":"integer","minimum":0}},"required":["artifact_id","record_index","byte_start","byte_end"],"additionalProperties":false},"Timestamp":{"type":"string","format":"date-time"},"Unknown":{"type":"object","properties":{"input_role":{"type":"string","enum":["rule_outcome","hook_beats","delivery_accounting","claim_snapshot","source_snapshot","adr_proposals","guard_export","finding_candidates","feasibility_labels"]},"artifact_ids":{"type":"array","items":{"type":"string"}},"reason":{"type":"string"}},"required":["input_role","artifact_ids","reason"],"additionalProperties":false}}
}
```

### Pinned scratch schema digests

SHA256 of each JSON code block plus one terminal newline. contracts.py guard
compares exact file bytes to these constants, not a newly generated policy.

- manifest.schema.json: b13a476781fe73920eaa7b12a0b6f374a53378b98c959e8c378554168070068d
- normalized.schema.json: 7b5cd6f0ae015463f8aecf8ba401b8ea6031ff70d422d1744ed25477664bcc56
- result.schema.json: 9d47276a21624c0eb9beb131af4e2162fa4b414a04bfd89be6ec044ac8935cb5
