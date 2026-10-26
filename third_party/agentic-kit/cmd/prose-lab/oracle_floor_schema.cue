package proselab

import (
	"list"
	"strings"
)

#FloorText: string & !=""
#FloorFact: close({
	id:             #ID
	text:           #FloorText
	_textBytes:     len(text) & <=4096
	_textNonblank:  strings.TrimSpace(text) & !=""
	occurred_at:    #WorldTime
	_occurredBytes: len(occurred_at) & <=256
	kind:           "observed" | "proposed" | "assumed" | "definition"
	required:       bool
})
#FloorCaseReviewBase: {
	reviewer_ref:     #ID
	criterion_id:     "cited-entailment/1" | "cited-entailment/2"
	criterion_sha256: "bb12ed77442b05cc9912247881fce489a35d5de4749fb1efe364c5c2011acaac" | "ecdecdb37fac788f2d95919c540c397d547c14f353d76d9df55ae4fac2ed4e2d"
	case_digest:      #DocumentReviewDigest
	status:           "determinate" | "unresolved"
	expected:         "supported" | "unsupported" | "inconclusive"
	premise_fact_ids: [...#ID] & list.MaxItems(8) & list.UniqueItems()
	counter_fact_ids: [...#ID] & list.MaxItems(256) & list.UniqueItems()
	_sourceBasis: len(premise_fact_ids)+len(counter_fact_ids) & >=1
	if expected == "supported" {premise_fact_ids: [#ID, ...#ID]}
	basis:          #FloorText
	_basisBytes:    len(basis) & <=4096
	_basisNonblank: strings.TrimSpace(basis) & !=""
}
#FloorCaseReview: (#FloorCaseReviewBase & {criterion_id: "cited-entailment/1", criterion_sha256: "bb12ed77442b05cc9912247881fce489a35d5de4749fb1efe364c5c2011acaac"}) | (#FloorCaseReviewBase & {criterion_id: "cited-entailment/2", criterion_sha256: "ecdecdb37fac788f2d95919c540c397d547c14f353d76d9df55ae4fac2ed4e2d"})
#FloorCase: {
	case_id:            #ID
	document_id:        #ID
	assertion:          #FloorText
	_assertionBytes:    len(assertion) & <=4096
	_assertionNonblank: strings.TrimSpace(assertion) & !=""
	cited_fact_ids: [#ID, ...#ID] & list.MaxItems(8) & list.UniqueItems()
	facts: [#FloorFact, ...#FloorFact] & list.MaxItems(256)
	expected: "supported" | "unsupported" | "inconclusive"
	role:     "ordinary" | "uncertainty-control" | "modal-control"
	old_case_ids: [...#ID] & list.MaxItems(70) & list.UniqueItems()
	basis:          #FloorText
	_basisBytes:    len(basis) & <=4096
	_basisNonblank: strings.TrimSpace(basis) & !=""
	reviews: [#FloorCaseReview, #FloorCaseReview, ...#FloorCaseReview] & list.MaxItems(8)
	_factIDs: [for f in facts {f.id}] & list.UniqueItems()
	_reviewerRefs: [for r in reviews {r.reviewer_ref}] & list.UniqueItems()
	for id in cited_fact_ids {_citedExists: list.Contains(_factIDs, id) & true}
	for r in reviews {
		if r.expected == "supported" {_positivePremise: len(r.premise_fact_ids) & >=1}
		for id in r.premise_fact_ids {_premiseCited: list.Contains(cited_fact_ids, id) & true}
		for id in r.counter_fact_ids {_counterExists: list.Contains(_factIDs, id) & true}
	}
	if role == "uncertainty-control" {expected: "unsupported"}
}
#FloorFixture: close({
	version: "prose-lab.oracle-proposed-fixture/1"
	id:      #ID
	cases: [#FloorCase, ...#FloorCase] & list.MaxItems(128)
	_caseIDs: [for c in cases {c.case_id}] & list.UniqueItems()
})
#FloorFixturePin: close({id: #ID, sha256: #DocumentReviewDigest})
#FloorScopeBase: {
	version:          "prose-lab.oracle-floor-scope/1"
	scope_id:         #ID
	criterion_id:     "cited-entailment/1" | "cited-entailment/2"
	criterion_sha256: #DocumentReviewDigest
	expected_case_ids: [#ID, ...#ID] & list.MaxItems(128) & list.UniqueItems()
	fixtures: [#FloorFixturePin, ...#FloorFixturePin] & list.MaxItems(32)
	_fixtureIDs: [for f in fixtures {f.id}] & list.UniqueItems()
}
#FloorScope: (#FloorScopeBase & {criterion_id: "cited-entailment/1", criterion_sha256: "bb12ed77442b05cc9912247881fce489a35d5de4749fb1efe364c5c2011acaac"}) | (#FloorScopeBase & {criterion_id: "cited-entailment/2", criterion_sha256: "ecdecdb37fac788f2d95919c540c397d547c14f353d76d9df55ae4fac2ed4e2d"})
#FloorReviewer: close({
	ref:                      #ID
	kind:                     "human" | "agent"
	role:                     "independent-source-reviewer"
	exposure:                 #FloorText
	_exposureBytes:           len(exposure) & <=4096
	_exposureNonblank:        strings.TrimSpace(exposure) & !=""
	assessments_digest:       #DocumentReviewDigest
	assessment_report_digest: #DocumentReviewDigest
})
#FloorDisposition: close({
	case_id:     #ID
	case_digest: #DocumentReviewDigest
	disposition: "retain" | "uncertainty" | "replace"
	prospective_ids: [#ID, ...#ID] & list.MaxItems(4) & list.UniqueItems()
	changed_fields: [...("label" | "citation" | "criterion" | "source" | "assertion")] & list.MaxItems(5) & list.UniqueItems()
	basis:          #FloorText
	_basisBytes:    len(basis) & <=4096
	_basisNonblank: strings.TrimSpace(basis) & !=""
	reviewer_refs: [#ID, #ID, ...#ID] & list.MaxItems(8) & list.UniqueItems()
	if disposition == "retain" {prospective_ids: [#ID], changed_fields: []}
	if disposition != "retain" {changed_fields: [string, ...string]}
})
#FloorCaseRef: close({case_id: #ID, case_digest: #DocumentReviewDigest})
#FloorContrast: close({
	id: "OQ-C01" | "OQ-C02" | "OQ-C03" | "OQ-C04" | "OQ-C05" | "OQ-C06" | "OQ-C07"
	cases: [#FloorCaseRef, #FloorCaseRef, ...#FloorCaseRef] & list.MaxItems(4)
	reviewer_refs: [#ID, #ID, ...#ID] & list.MaxItems(8) & list.UniqueItems()
	_caseIDs: [for c in cases {c.case_id}] & list.UniqueItems()
})
#FloorAmbiguity: close({case_id: #ID, basis: #FloorText, _basisBytes: len(basis) & <=4096, _basisNonblank: strings.TrimSpace(basis) & !=""})
#FloorReview: close({
	version:                  "prose-lab.oracle-floor-review/1"
	packet_digest:            #DocumentReviewDigest
	criterion_id:             "cited-entailment/1"
	criterion_sha256:         "bb12ed77442b05cc9912247881fce489a35d5de4749fb1efe364c5c2011acaac"
	assessments_digest:       #DocumentReviewDigest
	assessment_report_digest: #DocumentReviewDigest
	future_scope_digest:      #DocumentReviewDigest
	reviewers: [#FloorReviewer, #FloorReviewer, ...#FloorReviewer] & list.MaxItems(8)
	old_case_dispositions: [...#FloorDisposition] & list.MinItems(70) & list.MaxItems(70)
	contrasts: [...#FloorContrast] & list.MinItems(7) & list.MaxItems(7)
	remaining_ambiguities: [...#FloorAmbiguity] & list.MaxItems(128)
	verdict:                "held" | "ready-for-new-study-design"
	provider_qualified:     false
	identity_authenticated: false
	publication_authority:  false
	new_study_admission:    false
	_reviewerRefs: [for r in reviewers {r.ref}] & list.UniqueItems()
	_oldIDs: [for d in old_case_dispositions {d.case_id}] & list.UniqueItems()
	_contrastIDs: [for c in contrasts {c.id}] & list.UniqueItems()
	_ambiguityIDs: [for a in remaining_ambiguities {a.case_id}] & list.UniqueItems()
	if verdict == "ready-for-new-study-design" {remaining_ambiguities: []}
})
#FloorReport: close({
	version:                  "prose-lab.oracle-floor-report/1"
	packet_digest:            #DocumentReviewDigest
	assessments_digest:       #DocumentReviewDigest
	assessment_report_digest: #DocumentReviewDigest
	future_scope_digest:      #DocumentReviewDigest
	floor_review_digest:      #DocumentReviewDigest
	counts: close({old_cases: 70, prospective_cases: int & >=1 & <=128, contrasts: 7, reviewers: int & >=2 & <=8, remaining_ambiguities: int & >=0 & <=128})
	declared_floor_ready:   bool
	provider_qualified:     false
	identity_authenticated: false
	publication_authority:  false
	new_study_admission:    false
	content?: close({scope: #FloorScope, receipt: #FloorReview, fixtures: [#FloorFixture, ...#FloorFixture] & list.MaxItems(32)})
	if declared_floor_ready {counts: remaining_ambiguities: 0}
})
