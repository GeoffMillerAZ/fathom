package proselab

import (
	"list"
	"struct"
	"strings"
	"time"
)

#Lane: close({
	provider:              "anthropic" | "openrouter-jev"
	model:                 #ModelID
	resolved_model?:       #ModelID | ""
	pricing_source:        string & !=""
	max_input_tokens:      int & >=1 & <=2000000
	max_output_tokens:     int & >=0 & <=131072
	input_rate:            int & >=1 & <=100000000
	output_rate:           int & >=0 & <=100000000
	cache_write_rate:      int & >=0 & <=100000000
	cache_read_rate:       int & >=0 & <=100000000
	reserve_input_rate:    int & >=input_rate & >=cache_write_rate & >=cache_read_rate & <=100000000
	reserve_output_rate:   int & >=output_rate & <=100000000
	tier_threshold:        int & >=0 & <=max_input_tokens
	high_input_rate:       int & >=0 & <=reserve_input_rate
	high_output_rate:      int & >=0 & <=reserve_output_rate
	high_cache_write_rate: int & >=0 & <=reserve_input_rate
	high_cache_read_rate:  int & >=0 & <=reserve_input_rate
	if tier_threshold > 0 {
		high_input_rate:       >=input_rate
		high_output_rate:      >=output_rate
		high_cache_write_rate: >=cache_write_rate
		high_cache_read_rate:  >=cache_read_rate
	}
	if tier_threshold == 0 {
		high_input_rate:       0
		high_output_rate:      0
		high_cache_write_rate: 0
		high_cache_read_rate:  0
	}
	if provider == "anthropic" {
		max_output_tokens: >=1
		output_rate:       >=1
	}
	if provider == "openrouter-jev" {
		output_rate:         0
		reserve_output_rate: 0
		high_output_rate:    0
	}
})

#ID:      string & =~"^[A-Za-z0-9][A-Za-z0-9._-]{0,95}$"
#ModelID: string & =~"^[A-Za-z0-9][A-Za-z0-9._:/-]{0,255}$"

#CanaryStudy: close({
	version:          "prose-lab.canary/1"
	id:               #ID
	dataset_kind:     "synthetic"
	budget_micro_usd: int & >=1 & <=10000000
	lanes: {[#ID]: #Lane} & struct.MinFields(1) & struct.MaxFields(4)
	expected_case_ids: [#ID, ...#ID] & list.MaxItems(256) & list.UniqueItems()
})

#Reservation: close({
	version:            "prose-lab.journal/1"
	sequence:           int & >=1 & <=4999
	case_id:            #ID
	provider:           "anthropic" | "openrouter-jev"
	model:              string & !=""
	request_digest:     string & =~"^[a-f0-9]{64}$"
	request_json:       string
	reserved_micro_usd: int & >=1 & <=10000000
	pricing:            #Lane
	at_unix_nano:       int & >=1
})

// Cost is computed from actual reported usage and the selected price data. It is
// not an invoice, safety certification or permission to publish generated prose.
#Outcome: close({
	version:                 "prose-lab.journal/1"
	sequence:                int & >=1 & <=4999
	reservation_digest:      string & =~"^[a-f0-9]{64}$"
	status:                  "completed" | "failed" | "unknown"
	resolved_model?:         string & !=""
	usage_present:           bool
	usage_priced_micro_usd?: int & >=0
	latency_nanos:           int & >=0
	http_status:             int & >=0 & <=599
	response_json?:          string
	at_unix_nano:            int & >=1
})

// The document cohort is source data. Its digest is bound to the private journal
// study header by the lab before any provider request.
// Calendar-valid, nonzero canonical UTC RFC3339Nano, matching Go intake.
#WorldTime: time.Time & !="0001-01-01T00:00:00Z" & =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]{0,8}[1-9])?Z$"

#DocumentFact: close({
	id:          #ID
	text:        string & !=""
	occurred_at: #WorldTime
	kind:        "observed" | "proposed" | "assumed" | "definition"
	required:    bool
})
#DocumentSection: close({
	id:          #ID
	title:       string & !=""
	instruction: string & !=""
	fact_ids: [#ID, ...#ID]
	required_fact_ids: [#ID, ...#ID]
})
#DocumentGoal: close({
	id:         #ID
	statement:  string & !=""
	acceptance: string & !=""
	method:     string & !=""
})
#DocumentNegative: close({
	id:                #ID
	fault:             string & !=""
	target_fact_id:    #ID
	mutated_assertion: string & !=""
	expected:          "hold"
	criterion_ids: [#ID, ...#ID]
})
#DocumentFixture: close({
	id:          #ID
	type:        #ID
	length_band: "short" | "medium" | "long"
	min_words:   int & >=100 & <=1600
	max_words:   int & >=100 & <=1600
	purpose:     string & !=""
	facts: [#DocumentFact, ...#DocumentFact] & list.MaxItems(256)
	sections: [#DocumentSection, #DocumentSection, ...#DocumentSection] & list.MaxItems(5)
	required_goals: [#DocumentGoal, #DocumentGoal, #DocumentGoal, ...#DocumentGoal] & list.MaxItems(5)
	negative_cases: [#DocumentNegative, #DocumentNegative, #DocumentNegative, #DocumentNegative, #DocumentNegative, #DocumentNegative, ...#DocumentNegative] & list.MaxItems(32)
})
#DocumentLengthGoal: close({
	criterion_id: #ID
	class: "required" | "advisory"
	min_words: int & >=100 & <=1600
	max_words: int & >=min_words & <=1600
	reason: string
	_reasonBytes: len(reason) & <=4096
	_nonblankReason: strings.TrimSpace(reason)
}) & ({class: "required", _nonblankReason: !=""} | {class: "advisory"})
#DocumentFixtureV2: close({
	version: "prose-lab.fixture/2"
	id: #ID
	type: #ID
	length_band: "short" | "medium" | "long"
	length_goal: #DocumentLengthGoal
	purpose: string & !=""
	facts: [#DocumentFact, ...#DocumentFact] & list.MaxItems(256)
	sections: [#DocumentSection, #DocumentSection, ...#DocumentSection] & list.MaxItems(5)
	required_goals: [#DocumentGoal, #DocumentGoal, #DocumentGoal, ...#DocumentGoal] & list.MaxItems(5)
	negative_cases: [#DocumentNegative, #DocumentNegative, #DocumentNegative, #DocumentNegative, #DocumentNegative, #DocumentNegative, ...#DocumentNegative] & list.MaxItems(32)
	if length_band == "short" { length_goal: {max_words: <=250} }
	if length_band == "medium" { length_goal: {min_words: >=400, max_words: <=750} }
	if length_band == "long" { length_goal: {min_words: >=1200, max_words: <=1600} }
})
#DocumentScopeExpectation: close({
	document_id: #ID
	required_semantic_goal_ids: [#ID, ...#ID] & list.UniqueItems()
	length_criterion_id: #ID
	length_class: "required" | "advisory"
	min_words: int & >=100 & <=1600
	max_words: int & >=min_words & <=1600
	reason_digest: string & =~"^[a-f0-9]{64}$"
})
#DocumentGoalScope: close({
	version: "prose-lab.goal-scope/1"
	id: #ID
	corpus_digest: string & =~"^[a-f0-9]{64}$"
	documents: [#DocumentScopeExpectation, #DocumentScopeExpectation, #DocumentScopeExpectation, #DocumentScopeExpectation, #DocumentScopeExpectation, #DocumentScopeExpectation]
	long_artifact: close({document_id: #ID, criterion_id: #ID, min_words: int & >=1200 & <=1600})
})
#DocumentCell: close({
	id:          #ID
	document_id: #ID
	audience:    "executive" | "technical"
	repeat:      int & >=1 & <=3
})
#DocumentManifest: close({
	version: 1 | 2
	audiences: ["executive", "technical"]
	repeats: 3
	cells: [...#DocumentCell] & list.MinItems(36) & list.MaxItems(36)
})

#DocumentRunReceipt: close({
	version:                       "prose-lab.document-run/1"
	cell_id:                       #ID
	corpus_digest:                 string & =~"^[a-f0-9]{64}$"
	study_digest:                  string & =~"^[a-f0-9]{64}$"
	prepared_digest:               string & =~"^[a-f0-9]{64}$"
	generation_lane:               #ID
	review_lane:                   #ID
	requested_generation_model:    #ModelID
	requested_review_model:        #ModelID
	text_projection:               "anthropic-thinking-text-projection/v1"
	status:                        "held" | "candidate_checked"
	document_digest?:              string & =~"^[a-f0-9]{64}$"
	markdown_digest?:              string & =~"^[a-f0-9]{64}$"
	charge_basis:                  "unavailable" | "journal-usage-priced-or-reserved-local-accounting"
	cumulative_charged_micro_usd?: int & >=0 & <=10000000
	cell_charged_micro_usd?:       int & >=0 & <=10000000
	if charge_basis == "journal-usage-priced-or-reserved-local-accounting" {
		cumulative_charged_micro_usd: int & >=0 & <=10000000
		cell_charged_micro_usd:       int & >=0 & <=10000000
	}
	if charge_basis == "unavailable" {
		cumulative_charged_micro_usd?: _|_
		cell_charged_micro_usd?:       _|_
	}
	negative_cases_evaluated: false
	independent_acceptance:   false
	publication_authority:    false
	error_class?:             string & !=""
	if status == "candidate_checked" {
		charge_basis:    "journal-usage-priced-or-reserved-local-accounting"
		document_digest: string & =~"^[a-f0-9]{64}$"
		markdown_digest: string & =~"^[a-f0-9]{64}$"
	}
	if status == "held" {
		error_class: string & !=""
	}
})

#DocumentRunReceiptV2: close({
	version: "prose-lab.document-run/2"
	cell_id: #ID
	corpus_digest: string & =~"^[a-f0-9]{64}$"
	study_digest: string & =~"^[a-f0-9]{64}$"
	prepared_digest: string & =~"^[a-f0-9]{64}$"
	goal_scope_digest: string & =~"^[a-f0-9]{64}$"
	generation_lane: #ID
	review_lane: #ID
	requested_generation_model: #ModelID
	requested_review_model: #ModelID
	text_projection: "anthropic-thinking-text-projection/v1"
	status: "held" | "candidate_checked"
	document_digest?: string & =~"^[a-f0-9]{64}$"
	markdown_digest?: string & =~"^[a-f0-9]{64}$"
	goals_digest?: string & =~"^[a-f0-9]{64}$"
	length_criterion_id: #ID
	length_class: "required" | "advisory"
	length_min_words: int & >=100 & <=1600
	length_max_words: int & >=length_min_words & <=1600
	length_word_count?: int & >=0
	length_deviation?: int
	length_status?: "pass" | "warning" | "hold"
	count_rule: "markdown-whitespace-fields/v1"
	semantic_quality: "unassessed"
	charge_basis: "unavailable" | "journal-usage-priced-or-reserved-local-accounting"
	cumulative_charged_micro_usd?: int & >=0 & <=10000000
	cell_charged_micro_usd?: int & >=0 & <=10000000
	negative_cases_evaluated: false
	independent_acceptance: false
	publication_authority: false
	error_class?: string & !=""
}) & ({
	// Early failures have no length observation. Avoid testing an optional field
	// in a conditional: doing so creates incomplete/cyclic concrete validation.
	status: "held"
	error_class: string & !=""
	length_status?: _|_
	length_word_count?: _|_
	length_deviation?: _|_
	goals_digest?: _|_
} | ({
	charge_basis: "journal-usage-priced-or-reserved-local-accounting"
	document_digest: string & =~"^[a-f0-9]{64}$"
	markdown_digest: string & =~"^[a-f0-9]{64}$"
	goals_digest: string & =~"^[a-f0-9]{64}$"
	length_min_words: int & >=100 & <=1600
	length_max_words: int & >=length_min_words & <=1600
	length_word_count: int & >=0
	length_deviation: int
	if length_word_count < length_min_words {
		length_deviation: length_word_count - length_min_words
		length_status: "warning" | "hold"
	}
	if length_word_count > length_max_words {
		length_deviation: length_word_count - length_max_words
		length_status: "warning" | "hold"
	}
	if length_word_count >= length_min_words && length_word_count <= length_max_words {
		length_deviation: 0
		length_status: "pass"
	}
} & ({
	status: "held"
	length_status: "hold"
	length_class: "required"
	error_class: string & !=""
} | {
	status: "candidate_checked"
	length_status: "pass"
	length_deviation: 0
} | {
	status: "candidate_checked"
	length_status: "warning"
	length_class: "advisory"
})))

// The closed support-review scope is independently authored and raw-pinned.
#ReviewPositiveControl: close({
 id: #ID
 document_id: #ID
 kind: "exact" | "paraphrase" | "synthesis"
 fact_ids: [#ID, ...#ID] & list.MaxItems(8) & list.UniqueItems()
 assertion: string & !=""
 _trimmedAssertion: strings.TrimSpace(assertion) & !=""
 _assertionBytes: len(assertion) & <=4096
 if kind == "exact" { fact_ids: [#ID] }
 if kind == "synthesis" { fact_ids: [#ID, #ID, ...#ID] }
})
#ReviewScope: close({
 version: "prose-lab.review-scope/1"
 id: #ID
 corpus_digest: string & =~"^[a-f0-9]{64}$"
 goal_scope_digest: string & =~"^[a-f0-9]{64}$"
 negative_case_ids: [#ID, ...#ID] & list.MinItems(43) & list.MaxItems(43) & list.UniqueItems()
 positive_controls: [...#ReviewPositiveControl] & list.MinItems(24) & list.MaxItems(24)
})
#ReviewPrepared: close({
 version: "prose-lab.review-prepared/1"
 case_id: #ID
 case_kind: "negative" | "exact" | "paraphrase" | "synthesis"
 expected: "supported" | "unsupported"
 study_digest: string & =~"^[a-f0-9]{64}$"
 corpus_digest: string & =~"^[a-f0-9]{64}$"
 goal_scope_digest: string & =~"^[a-f0-9]{64}$"
 review_scope_digest: string & =~"^[a-f0-9]{64}$"
 review_lane: #ID
 requested_review_model: #ModelID
 native_request_digest: string & =~"^[a-f0-9]{64}$"
 prepared_digest: string & =~"^[a-f0-9]{64}$"
 reservation_micro_usd: int & >=1 & <=10000000
 planned: 67
 independent_acceptance: false
 publication_authority: false
 live: false
})
#ReviewCapture: close({
 version: "prose-lab.review-capture/1"
 case_id: #ID
 study_digest: string & =~"^[a-f0-9]{64}$"
 prepared_digest: string & =~"^[a-f0-9]{64}$"
 corpus_digest: string & =~"^[a-f0-9]{64}$"
 goal_scope_digest: string & =~"^[a-f0-9]{64}$"
 review_scope_digest: string & =~"^[a-f0-9]{64}$"
 review_lane: #ID
 requested_review_model: #ModelID
 native_request_digest: string & =~"^[a-f0-9]{64}$"
 status: "supported" | "unsupported" | "inconclusive" | "held"
 expected: "supported" | "unsupported"
 matched_fixture_expectation: bool
 charge_basis: "unavailable" | "journal-usage-priced-or-reserved-local-accounting"
 cumulative_charged_micro_usd?: int & >=0 & <=10000000
 case_charged_micro_usd?: int & >=0 & <=10000000
 independent_acceptance: false
 publication_authority: false
 error_class?: string & !=""
}) & ({status:"supported",expected:"supported",matched_fixture_expectation:true} | {status:"unsupported",expected:"unsupported",matched_fixture_expectation:true} | {status:"supported",expected:"unsupported",matched_fixture_expectation:false} | {status:"unsupported",expected:"supported",matched_fixture_expectation:false} | {status:"inconclusive",matched_fixture_expectation:false} | {status:"held",matched_fixture_expectation:false}) & ({status:"held",error_class:string & !=""} | {status:"supported" | "unsupported" | "inconclusive",charge_basis:"journal-usage-priced-or-reserved-local-accounting",cumulative_charged_micro_usd:int & >=0,case_charged_micro_usd:int & >=0,error_class?:_|_})
#ReviewCaseReport: close({case_id: #ID, expected: "supported" | "unsupported", status: "supported" | "unsupported" | "inconclusive" | "pending" | "failed" | "unknown" | "missing" | "invalid", matched: bool}) & ({status:"supported",expected:"supported",matched:true} | {status:"unsupported",expected:"unsupported",matched:true} | {status:"supported",expected:"unsupported",matched:false} | {status:"unsupported",expected:"supported",matched:false} | {status:"inconclusive" | "pending" | "failed" | "unknown" | "missing" | "invalid",matched:false})
#MatchedReviewCase: #ReviewCaseReport & {matched:true}
#ReviewReport: close({
 version: "prose-lab.review-report/1"
 study_digest: string & =~"^[a-f0-9]{64}$"
 corpus_digest: string & =~"^[a-f0-9]{64}$"
 goal_scope_digest: string & =~"^[a-f0-9]{64}$"
 review_scope_digest: string & =~"^[a-f0-9]{64}$"
 review_lane: #ID
 planned: 67
 completed: int & >=0 & <=67
 missing: int & >=0 & <=67
 false_support: int & >=0 & <=43
 false_hold: int & >=0 & <=24
 inconclusive: int & >=0 & <=67
 pending: int & >=0 & <=67
 failed: int & >=0 & <=67
 unknown: int & >=0 & <=67
 invalid_count: int & >=0 & <=4999
 total_charged_micro_usd: int & >=0 & <=10000000
 charge_basis: "journal-usage-priced-or-reserved-local-accounting"
 matched_fixture_expectations: bool
 cases: [...#ReviewCaseReport] & list.MinItems(67) & list.MaxItems(67)
 independent_acceptance: false
 publication_authority: false
}) & ({matched_fixture_expectations:true,completed:67,missing:0,pending:0,failed:0,unknown:0,invalid_count:0,false_support:0,false_hold:0,inconclusive:0,cases:[...#MatchedReviewCase]} | {matched_fixture_expectations:false})

#PairedVariant: close({id: "baseline" | "candidate", protocol: "prose-lab.support-review/1" | "prose-lab.support-review/2"})
#PairedStudy: close({
 version: "prose-lab.paired-review-study/1"
 id: #ID
 dataset_kind: "synthetic"
 budget_micro_usd: int & >=1 & <=5000000
 prior_direct_micro_usd: int & >=0
 direct_stage_budget_micro_usd: int & >=1 & <=10000000
 review: #Lane & {provider:"anthropic",max_output_tokens:int & >=1 & <=8192}
 variants: [close({id:"baseline",protocol:"prose-lab.support-review/1"}),close({id:"candidate",protocol:"prose-lab.support-review/2"})]
 expected_execution_ids: [...#ID] & list.MinItems(140) & list.MaxItems(140) & list.UniqueItems()
 _stageTotal: prior_direct_micro_usd + budget_micro_usd
 direct_stage_budget_micro_usd: >=_stageTotal
})
#PairedRegressionFact: close({id:#ID,text:string & !="",kind:string & !="",required:bool,occurred_at:string & !=""})
#PairedRegressionCase: close({id:#ID,assertion:string & !="",fact_ids:[#ID,...#ID],facts:[#PairedRegressionFact,...#PairedRegressionFact],observed_support_status:"unsupported" | "not_reached",reference_judgment:"supported" | "unsupported",method:string & !="",basis:string & !=""})
#PairedRegressionSource: close({version:"prose-lab.observed-review-regressions/1",provenance:string & !="",fixture_digest:#DocumentReviewDigest,cases:[#PairedRegressionCase,#PairedRegressionCase],human_labeling:false,changes_pilot_status:false,site_promotion:false})
#PairedProvenance: close({source_digest: #DocumentReviewDigest, method: string & !="", reason: string & !=""})
#PairedExtension: close({
 id: #ID
 document_id: #ID
 kind: "observed-development" | "authored-contrast"
 role: "rhetorical-framing" | "document-scope-overclaim" | "material-scope-exclusion"
 fact_ids: [#ID, ...#ID] & list.MaxItems(8) & list.UniqueItems()
 assertion: string & !=""
 expected: "supported" | "unsupported"
 provenance: #PairedProvenance
})
#PairedScope: close({
 version: "prose-lab.review-scope/2"
 id: #ID
 corpus_digest: #DocumentReviewDigest
 goal_scope_digest: #DocumentReviewDigest
 base_scope_digest: #DocumentReviewDigest
 extensions: [#PairedExtension,#PairedExtension,#PairedExtension]
})
#PairedTrust: close({independent_acceptance:false,publication_authority:false,model_site_adoption:false})
#PairedContent: close({assertion:string & !="",fact_ids:[#ID,...#ID],facts:{[#ID]:string & !=""},kind:string & !="",role?:"rhetorical-framing" | "document-scope-overclaim" | "material-scope-exclusion",expected:"supported" | "unsupported",reason?:string & !="",provenance?:#PairedProvenance})
#PairedPrepared: close({
 version:"prose-lab.review-pair-prepared/1"
 execution_id:#ID
 case_id:#ID
 variant:"baseline" | "candidate"
 study_digest:#DocumentReviewDigest
 native_request_digest:#DocumentReviewDigest
 prepared_digest:#DocumentReviewDigest
 reservation_micro_usd:int & >=1 & <=5000000
 requested_review_model: string & !=""
 review_protocol: "prose-lab.support-review/1" | "prose-lab.support-review/2"
 if variant == "baseline" {review_protocol:"prose-lab.support-review/1"}
 if variant == "candidate" {review_protocol:"prose-lab.support-review/2"}
 corpus_digest:#DocumentReviewDigest
 goal_scope_digest:#DocumentReviewDigest
 base_review_scope_digest:#DocumentReviewDigest
 review_scope_digest:#DocumentReviewDigest
 regression_source_digest:#DocumentReviewDigest
 budget_micro_usd:int & >=1 & <=5000000
 prior_direct_micro_usd:int & >=0
 direct_stage_budget_micro_usd:int & >=(prior_direct_micro_usd+budget_micro_usd) & <=10000000
 planned:140
 live:false
 independent_acceptance:false
 publication_authority:false
 model_site_adoption:false
 content?:#PairedContent
})
#PairedCapture: close({
 version:"prose-lab.review-pair-capture/1"
 execution_id:#ID
 case_id:#ID
 variant:"baseline" | "candidate"
 study_digest:#DocumentReviewDigest
 prepared_digest:#DocumentReviewDigest
 native_request_digest:#DocumentReviewDigest
 status:"supported" | "unsupported" | "inconclusive" | "held"
 charge_basis:"journal-usage-priced-or-reserved-local-accounting"
 cumulative_charged_micro_usd:int & >=0 & <=5000000
 case_charged_micro_usd:int & >=0 & <=5000000
 independent_acceptance:false
 publication_authority:false
 model_site_adoption:false
 error_class?:string & !=""
 content?:#PairedContent
})
#PairedCounts: close({planned:int & >=0 & <=140,complete:int & >=0 & <=140,missing:int & >=0 & <=140,false_support:int & >=0 & <=140,false_hold:int & >=0 & <=140,inconclusive:int & >=0 & <=140,pending:int & >=0 & <=140,failed:int & >=0 & <=140,unknown:int & >=0 & <=140,invalid:int & >=0 & <=140,input_tokens:int & >=0,output_tokens:int & >=0,cache_write_tokens:int & >=0,cache_read_tokens:int & >=0,charged_micro_usd:int & >=0 & <=5000000,latency_nanos:int & >=0})
#PairedRow: close({position:int & >=1 & <=140,case_id:#ID,variant:"baseline" | "candidate",execution_id:#ID,state:"supported" | "unsupported" | "inconclusive" | "missing" | "pending" | "failed" | "unknown" | "invalid",content?:#PairedContent})
#PairedKnownRow: #PairedRow & {state:"supported" | "unsupported"}
#PairedReport: close({version:"prose-lab.review-pair-report/1",study_digest:#DocumentReviewDigest,corpus_digest:#DocumentReviewDigest,goal_scope_digest:#DocumentReviewDigest,review_scope_digest:#DocumentReviewDigest,total:#PairedCounts,baseline:#PairedCounts,candidate:#PairedCounts,rows:[...#PairedRow] & list.MinItems(140) & list.MaxItems(140),charge_basis:"journal-usage-priced-or-reserved-local-accounting",qualified:bool,independent_acceptance:false,publication_authority:false,model_site_adoption:false}) & ({qualified:true,total:{planned:140,complete:140,missing:0,inconclusive:0,pending:0,failed:0,unknown:0,invalid:0},baseline:{planned:70,complete:70,missing:0,inconclusive:0,pending:0,failed:0,unknown:0,invalid:0},candidate:{planned:70,complete:70,missing:0,false_support:0,false_hold:0,inconclusive:0,pending:0,failed:0,unknown:0,invalid:0},rows:[...#PairedKnownRow] & list.MinItems(140) & list.MaxItems(140)} | {qualified:false})

#DocumentReviewDigest: string & =~"^[a-f0-9]{64}$"
#DocumentReviewLength: close({
 criterion_id: #ID
 class: "required" | "advisory"
 min_words: int & >=100 & <=1600
 max_words: int & >=min_words & <=1600
 count_rule: "markdown-whitespace-fields/v1"
 word_count: int & >=0
 deviation: int
 status: "pass" | "warning" | "hold"
 definition_digest: #DocumentReviewDigest
 profile_digest: #DocumentReviewDigest
 if word_count < min_words { deviation: word_count - min_words }
 if word_count > max_words { deviation: word_count - max_words }
 if word_count >= min_words && word_count <= max_words { deviation: 0, status: "pass" }
 if class == "required" && deviation != 0 { status: "hold" }
 if class == "advisory" && deviation != 0 { status: "warning" }
})
#DocumentReviewGoal: close({criterion_id: #ID, definition_digest: #DocumentReviewDigest, profile_digest: #DocumentReviewDigest})
#DocumentReviewTrust: {
 trust: "caller-declared"
 identity_verification: "not_performed"
 source_revalidation: "not_performed"
 provider_history_verification: "not_performed"
 independent_acceptance: false
 publication_authority: false
}
#DocumentReviewClaimID: string & =~"^[A-Za-z0-9][A-Za-z0-9._-]{0,95}#c([1-9]|[1-9][0-9]|1[01][0-9]|12[0-8])$"
#DocumentReviewClaim: close({node_id: #ID, claim_id: #DocumentReviewClaimID, coordinate_basis: "node-text-utf8-byte-range-half-open/v1", start: int & >=0, end: int & >start, assertion: string & !="", fact_ids: [...#ID], contradictions: [...#ID]})
#DocumentReviewSpan: close({node_id: #ID, start: int & >=0, end: int & >=start})
#DocumentReviewContent: close({
 purpose: string & !=""
 audience: "executive" | "technical"
 voice: string & !=""
 technical_level: string & !=""
 markdown: string
 span_map: [...#DocumentReviewSpan]
 facts: [...#DocumentFact]
 sections: [...#DocumentSection]
 required_goals: [...#DocumentGoal]
 claims: [...#DocumentReviewClaim]
})
#DocumentReviewPacket: close({
 #DocumentReviewTrust
 version: "prose-lab.document-review-packet/1"
 cell_id: #ID
 document_id: #ID
 corpus_digest: #DocumentReviewDigest
 goal_scope_digest: #DocumentReviewDigest
 study_digest: #DocumentReviewDigest
 prepared_digest: #DocumentReviewDigest
 receipt_digest: #DocumentReviewDigest
 version_file_digest: #DocumentReviewDigest
 markdown_file_digest: #DocumentReviewDigest
 goals_file_digest: #DocumentReviewDigest
 document_digest: #DocumentReviewDigest
 markdown_digest: #DocumentReviewDigest
 subject_digest: #DocumentReviewDigest
 candidate_status: "candidate_checked" | "held"
 length: #DocumentReviewLength
 semantic_goals: [#DocumentReviewGoal, #DocumentReviewGoal, #DocumentReviewGoal, ...#DocumentReviewGoal] & list.MaxItems(5)
 packet_digest: #DocumentReviewDigest
 content?: #DocumentReviewContent
})
#DocumentAdjudicationObservation: close({
 criterion_id: #ID
 definition_digest: #DocumentReviewDigest
 subject_digest: #DocumentReviewDigest
 outcome: "pass" | "fail" | "unknown" | "not-applicable"
 method: "deterministic" | "empirical" | "heuristic" | "human"
 reason: string & !=""
 _reasonBytes: len(reason) & <=2048
 _trimmedReason: strings.TrimSpace(reason) & !=""
 artifact_digests: [...#DocumentReviewDigest] & list.MaxItems(16) & list.UniqueItems()
 observed_at: #WorldTime
 valid_until: #WorldTime
})
#DocumentAdjudication: close({
 version: "prose-lab.document-adjudication/1"
 packet_digest: #DocumentReviewDigest
 reviewer: close({id: #ID, kind: "human" | "agent", relationship: "independent" | "producer" | "unknown"})
 observations: [...#DocumentAdjudicationObservation] & list.MaxItems(32)
 if reviewer.kind == "agent" {
  observations: [...(#DocumentAdjudicationObservation & {method: "deterministic" | "empirical" | "heuristic"})]
 }
})
#DocumentReviewRule: "observed" | "observed-unknown" | "observed-not-applicable" | "missing-observation" | "definition-mismatch" | "subject-mismatch" | "method-disallowed" | "observed-in-future" | "observation-stale" | "not-applicable-disallowed"
#DocumentReviewEligibilityRule: #DocumentReviewRule | "independence-not-declared"
#DocumentReviewFinding: close({criterion_id: #ID, outcome: "pass" | "fail" | "unknown" | "not-applicable", rule: #DocumentReviewRule, eligible: bool, eligibility_rule: #DocumentReviewEligibilityRule, reason?: string})
#DocumentReviewTrace: close({criterion_id: #ID, definition_digest: #DocumentReviewDigest, outcome: "pass" | "fail" | "unknown" | "not-applicable", rule: #DocumentReviewRule, reason: string, artifact_digests: [...#DocumentReviewDigest], observation_digest: null | #DocumentReviewDigest})
#DocumentReviewReport: close({
 #DocumentReviewTrust
 version: "prose-lab.document-review-report/1"
 packet_digest: #DocumentReviewDigest
 adjudication_digest?: #DocumentReviewDigest
 declared_goals_status: "pass" | "fail" | "unknown"
 declared_criteria_status: "pass" | "fail" | "unknown"
 semantic_goals: [#DocumentReviewFinding, #DocumentReviewFinding, #DocumentReviewFinding, ...#DocumentReviewFinding] & list.MaxItems(5)
 length: #DocumentReviewLength
 if length.status == "hold" { declared_criteria_status: "fail" }
 if length.status != "hold" { declared_criteria_status: declared_goals_status }
 content?: close({adjudication?: #DocumentAdjudication, traces: [...#DocumentReviewTrace], packet: #DocumentReviewContent})
})

// Oracle evidence is declared coverage. Go checks source, digest and roster binding.
#OracleLineage: close({
 artifact_kind: "base-review-scope" | "paired-review-scope" | "corpus-fixture" | "observed-regression-source"
 artifact_digest: #DocumentReviewDigest
 json_pointer: string & =~"^/.*"
 mapping_rule?: "positive-control-means-supported/1" | "negative-control-means-unsupported/1"
 source_digest?: #DocumentReviewDigest
 method?: string & !=""
 reason?: string & !=""
 regression_source?: #OracleLineage
 fixture_expected?: #OracleLineage
 fixture_expected_value?: "hold"
 target_fact_id?: #ID
})
#OracleFact: close({
 id: #ID
 text: string & !=""
 occurred_at: #WorldTime
 kind: "observed" | "proposed" | "assumed" | "definition"
 required: bool
 lineage: #OracleLineage
})
#OracleCase: close({
 case_id: #ID
 document_id: #ID
 historical_expected: "supported" | "unsupported"
 assertion: string & !=""
 cited_fact_ids: [#ID, ...#ID] & list.MaxItems(8) & list.UniqueItems()
 cited_facts: [#OracleFact, ...#OracleFact] & list.MaxItems(8)
 context_facts: [#OracleFact, ...#OracleFact] & list.MaxItems(256)
 _citedIDs: [for f in cited_facts {f.id}] & list.UniqueItems()
 _contextIDs: [for f in context_facts {f.id}] & list.UniqueItems()
 _citedCount: len(cited_fact_ids)
 _citedCount: len(cited_facts)
 for id in cited_fact_ids {
  _hasCitedRecord: list.Contains(_citedIDs, id) & true
  _hasContextRecord: list.Contains(_contextIDs, id) & true
 }
 label_lineage: #OracleLineage
 assertion_lineage: #OracleLineage
})
#OracleCanonicalPacket: close({
 version: "prose-lab.oracle-canonical-packet/1"
 criterion_id: "cited-entailment/1"
 criterion_sha256: "bb12ed77442b05cc9912247881fce489a35d5de4749fb1efe364c5c2011acaac"
 corpus_digest: #DocumentReviewDigest
 goal_scope_digest: #DocumentReviewDigest
 base_review_scope_digest: #DocumentReviewDigest
 review_scope_digest: #DocumentReviewDigest
 regression_source_digest: #DocumentReviewDigest
 rows: [...#OracleCase] & list.MinItems(70) & list.MaxItems(70)
 _caseIDs: [for r in rows {r.case_id}] & list.UniqueItems()
})
#OracleBinding: close({
 case_id:#ID
 document_id:#ID
 case_digest:#DocumentReviewDigest
 let cid = case_id
 let did = document_id
 content?: #OracleCase & {case_id:cid, document_id:did}
})
#OraclePacket: close({
 version: "prose-lab.oracle-packet/1"
 packet_digest: #DocumentReviewDigest
 study_digest: #DocumentReviewDigest
 corpus_digest: #DocumentReviewDigest
 goal_scope_digest: #DocumentReviewDigest
 base_review_scope_digest: #DocumentReviewDigest
 review_scope_digest: #DocumentReviewDigest
 regression_source_digest: #DocumentReviewDigest
 criterion_id: "cited-entailment/1"
 criterion_sha256: "bb12ed77442b05cc9912247881fce489a35d5de4749fb1efe364c5c2011acaac"
 planned: 70
 rows: [...#OracleBinding] & list.MinItems(70) & list.MaxItems(70)
 _caseIDs: [for r in rows {r.case_id}] & list.UniqueItems()
 human_identity_authenticated:false
 source_authenticated:false
 model_site_adoption:false
 independent_acceptance:false
 publication_authority:false
 new_study_admission:false
 criterion_text?:string & !=""
})
#OracleAssessment: close({
 case_id: #ID
 case_digest: #DocumentReviewDigest
 disposition: "affirmed" | "ambiguous" | "revise"
 interpretation: "literal" | "faithful-paraphrase" | "normative-direction" | "unresolved"
 reason_code: "sufficient-premises" | "missing-premise" | "scope-ambiguity" | "modality-ambiguity" | "contradiction" | "citation-mismatch"
 premise_fact_ids: [...#ID] & list.MaxItems(256) & list.UniqueItems()
 counter_fact_ids: [...#ID] & list.MaxItems(256) & list.UniqueItems()
 basis: string & !=""
 _basisBytes: len(basis) & >=1 & <=4096
 _trimmedBasis: strings.TrimSpace(basis) & !=""
 assessor_kind: "human" | "agent"
 assessor_ref: string & !=""
 _assessorBytes: len(assessor_ref) & >=1 & <=256
 _trimmedAssessor: strings.TrimSpace(assessor_ref) & !=""
 proposed_fact_ids: [...#ID] & list.MaxItems(256) & list.UniqueItems()
 if disposition == "affirmed" {
  interpretation: "literal" | "faithful-paraphrase" | "normative-direction"
  reason_code: "sufficient-premises" | "missing-premise" | "contradiction" | "citation-mismatch"
  proposed_fact_ids: []
 }
 if disposition == "ambiguous" {
  interpretation: "unresolved"
  reason_code: "missing-premise" | "scope-ambiguity" | "modality-ambiguity"
 }
 if disposition == "revise" {
  reason_code: "missing-premise" | "contradiction" | "citation-mismatch"
 }
 if reason_code == "citation-mismatch" && disposition == "revise" { proposed_fact_ids: [#ID, ...#ID] }
 if reason_code == "contradiction" { counter_fact_ids: [#ID, ...#ID] }
 if disposition == "affirmed" && reason_code == "sufficient-premises" { premise_fact_ids: [#ID, ...#ID] }
 _sourceBasis: len(premise_fact_ids) + len(counter_fact_ids)
 _sourceBasis: >=1
})
#OracleAssessments: close({
 version: "prose-lab.oracle-assessments/1"
 packet_digest: #DocumentReviewDigest
 criterion_id: "cited-entailment/1"
 criterion_sha256: "bb12ed77442b05cc9912247881fce489a35d5de4749fb1efe364c5c2011acaac"
 rows: [...#OracleAssessment] & list.MaxItems(70)
 _caseIDs: [for r in rows {r.case_id}] & list.UniqueItems()
})
#OracleUnreviewedRow: close({
 case_id:#ID
 document_id:#ID
 case_digest:#DocumentReviewDigest
 let cid = case_id
 let did = document_id
 state:"unreviewed"
 content?:close({case:#OracleCase & {case_id:cid,document_id:did},assessment?:_|_})
})
#OracleAssessedRow: close({
 case_id:#ID
 document_id:#ID
 case_digest:#DocumentReviewDigest
 let cid = case_id
 let did = document_id
 let dig = case_digest
 state:"affirmed" | "ambiguous" | "revise"
 let st = state
 content?:close({case:#OracleCase & {case_id:cid,document_id:did},assessment:#OracleAssessment & {case_id:cid,case_digest:dig,disposition:st}})
})
#OracleReportRow: #OracleUnreviewedRow | #OracleAssessedRow
#OracleReport: close({
 version:"prose-lab.oracle-report/1"
 packet_digest:#DocumentReviewDigest
 study_digest:#DocumentReviewDigest
 assessments_digest:#DocumentReviewDigest
 corpus_digest:#DocumentReviewDigest
 goal_scope_digest:#DocumentReviewDigest
 base_review_scope_digest:#DocumentReviewDigest
 review_scope_digest:#DocumentReviewDigest
 regression_source_digest:#DocumentReviewDigest
 criterion_id:"cited-entailment/1"
 criterion_sha256:"bb12ed77442b05cc9912247881fce489a35d5de4749fb1efe364c5c2011acaac"
 planned:70
 assessed:int & >=0 & <=70
 unreviewed:int & >=0 & <=70
 affirmed:int & >=0 & <=70
 ambiguous:int & >=0 & <=70
 revise:int & >=0 & <=70
 _coverage: assessed + unreviewed
 _assessed: affirmed + ambiguous + revise
 _coverage: 70
 _assessed: assessed
 rows:[...#OracleReportRow] & list.MinItems(70) & list.MaxItems(70)
 _caseIDs: [for r in rows {r.case_id}] & list.UniqueItems()
 _affirmedRows: list.Sum([for r in rows if r.state == "affirmed" {1}])
 _ambiguousRows: list.Sum([for r in rows if r.state == "ambiguous" {1}])
 _reviseRows: list.Sum([for r in rows if r.state == "revise" {1}])
 _unreviewedRows: list.Sum([for r in rows if r.state == "unreviewed" {1}])
 affirmed:_affirmedRows
 ambiguous:_ambiguousRows
 revise:_reviseRows
 unreviewed:_unreviewedRows
 declared_assessments_complete:bool
 floor_declared_ready:bool
 human_identity_authenticated:false
 source_authenticated:false
 model_site_adoption:false
 independent_acceptance:false
 publication_authority:false
 new_study_admission:false
 criterion_text?:string & !=""
 if declared_assessments_complete { assessed:70, unreviewed:0 }
 if !declared_assessments_complete { unreviewed:>=1 }
 if floor_declared_ready { declared_assessments_complete:true, affirmed:70, ambiguous:0, revise:0 }
 if !floor_declared_ready { affirmed:<70 }
})
