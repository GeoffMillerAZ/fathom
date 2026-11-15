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
