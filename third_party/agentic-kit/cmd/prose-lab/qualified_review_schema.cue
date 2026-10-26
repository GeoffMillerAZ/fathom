package proselab

import (
	"list"
	"strings"
)

// New qualified source-support trial. The Go intake also checks original JSON
// field presence, byte limits, lexical roster, floor readiness and full cost.
#QualifiedReviewTrial: close({
	version:                       "prose-lab.qualified-review-study/1"
	id:                            #ID
	dataset_kind:                  "synthetic"
	budget_micro_usd:              int & >=1 & <=5000000
	prior_direct_micro_usd:        int & >=0
	direct_stage_budget_micro_usd: int & >=1 & <=10000000
	_stageTotal:                   prior_direct_micro_usd + budget_micro_usd
	_stageTotal:                   <=direct_stage_budget_micro_usd
	review: #Lane & {
		provider:          "anthropic"
		max_output_tokens: >=1 & <=8192
	}
	expected_case_ids: [...#ID] & list.MinItems(85) & list.MaxItems(85) & list.UniqueItems()
})

#QualifiedReviewTrialV2: close({
	version:                       "prose-lab.qualified-review-study/2"
	review_protocol:               "prose-lab.support-review/4"
	id:                            #ID
	dataset_kind:                  "synthetic"
	budget_micro_usd:              int & >=1 & <=5000000
	prior_direct_micro_usd:        int & >=0
	direct_stage_budget_micro_usd: int & >=1 & <=10000000
	_stageTotal:                   prior_direct_micro_usd + budget_micro_usd
	_stageTotal:                   <=direct_stage_budget_micro_usd
	review: #Lane & {
		provider:          "anthropic"
		max_output_tokens: >=1 & <=8192
	}
	expected_case_ids: [...#ID] & list.MinItems(85) & list.MaxItems(85) & list.UniqueItems()
})

#QualifiedReviewDecision: close({
	status:          "supported" | "unsupported" | "inconclusive"
	reason:          string
	_reasonBytes:    len(reason) & <=4096
	_nonblankReason: strings.TrimSpace(reason) & !=""
	premise_fact_ids: [...#ID] & list.UniqueItems()
	counter_fact_ids: [...#ID] & list.UniqueItems()
	_sourceBasis: len(premise_fact_ids) + len(counter_fact_ids)
	_sourceBasis: >=1
	if status == "supported" {
		premise_fact_ids: [#ID, ...#ID]
	}
})

#QualifiedReviewDecisionV4: close({
	status:          "supported" | "unsupported" | "inconclusive"
	reason:          string
	_reasonBytes:    len(reason) & <=4096
	_nonblankReason: strings.TrimSpace(reason) & !=""
	examined_fact_ids: [...#ID] & list.MinItems(1) & list.MaxItems(256) & list.UniqueItems()
	premise_fact_ids:  [...#ID] & list.MaxItems(8) & list.UniqueItems()
	counter_fact_ids:  [...#ID] & list.MaxItems(256) & list.UniqueItems()
	_examinedPremises: [for id in premise_fact_ids {list.Contains(examined_fact_ids, id)}] & [...true]
	_examinedCounters: [for id in counter_fact_ids {list.Contains(examined_fact_ids, id)}] & [...true]
	if status == "supported" {
		premise_fact_ids: [#ID, ...#ID]
	}
})

// Source membership, original JSON field presence, and UTF-8 byte fidelity
// require the Go intake and decoder; list overlap is valid for composite facts.
#QualifiedPriorSpend: close({
	version:                      "prose-lab.prior-spend/1"
	stage_id:                     #ID
	charged_micro_usd:            int & >=0 & <=10000000
	uncertain_reserved_micro_usd: int & >=0 & <=10000000
	source_receipts: [...close({
		id:                           #ID
		sha256:                       string & =~"^[a-f0-9]{64}$"
		charged_micro_usd:            int & >=0 & <=10000000
		uncertain_reserved_micro_usd: int & >=0 & <=10000000
	})] & list.MaxItems(32)
	_sourceIDs: [for source in source_receipts {source.id}] & list.UniqueItems()
	_chargedTotal: list.Sum([for source in source_receipts {source.charged_micro_usd}])
	_chargedTotal: charged_micro_usd
	_uncertainTotal: list.Sum([for source in source_receipts {source.uncertain_reserved_micro_usd}])
	_uncertainTotal: uncertain_reserved_micro_usd
	_priorTotal:     charged_micro_usd + uncertain_reserved_micro_usd
	_priorTotal:     <=10000000
})
