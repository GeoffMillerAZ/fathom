package qualitygate

import (
	"list"
	"strings"
	"time"
)

// #Policy is the authoring contract for quality-gate/policy.json. Runtime
// validation is stdlib-only and must mirror the bounds and cross-field rules.
#Policy: {
	version: 1
	id:      #ID
	limits:  #Limits
	rules: [...#Rule] & list.MaxItems(1000)
	model: #Model
	#ruleIDs: [for r in rules {r.id}]
	#siteIDs: [for s in model.sites {s.id}]
	#allIDs: list.UniqueItems() & list.Concat([#ruleIDs, #siteIDs])
	if len(model.sites) == 0 {
		rules: list.MinItems(1)
	}
}

#ID:       string & =~"^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$"
#Env:      string & =~"^[A-Za-z_][A-Za-z0-9_]{0,127}$"
#Boundary: "pre-commit" | "pre-push" | "ci" | "pre-publish"
#DoctorCheck: {
	id:     string & !=""
	state:  "pass" | "unknown" | "advisory"
	reason: string & !=""
	action: string & !=""
	ids?:  [...#ID]
}
#DoctorReport: {
	version:       1
	scope:         "configuration-only"
	evaluated_at:  time.Time
	boundary:      #Boundary
	state:         "pass" | "unknown"
	policy_source: "committed" | "local-untrusted"
	policy_id?:    #ID
	policy_commit?: string & =~"^([a-fA-F0-9]{40}|[a-fA-F0-9]{64})$"
	policy_sha256?: string & =~"^[a-fA-F0-9]{64}$"
	caveats:       [...string] & list.MinItems(2)
	checks:        [...#DoctorCheck] & list.MinItems(1)
}
#Boundaries: [...#Boundary] & list.MinItems(1) & list.MaxItems(4) & list.UniqueItems()
#PathPart: string & !="" & !~"^/" & !~"[\\\\:]" & !~"//" & !~"(^|/)\\.{1,2}(/|$)" & !~"@\\{" & !~"[\\x00-\\x1f\\x7f]"
#Selector: {
	prefixes: [...(#PathPart | "")] & list.MinItems(1) & list.MaxItems(1000) & list.UniqueItems()
	suffixes?: [...#PathPart] & list.MaxItems(1000) & list.UniqueItems()
	#prefixLengths: [for p in prefixes {len(p) & <=1024}]
	if suffixes != _|_ {
		#suffixLengths: [for s in suffixes {len(s) & <=1024}]
	}
}
#Limits: {
	max_files:       int & >=1 & <=10000
	max_file_bytes:  int & >=1 & <=16777216
	max_total_bytes: int & >=1 & <=16777216
	timeout_seconds: int & >=1 & <=300
}
#Rule: {
	id:         #ID
	kind:       "conflict-markers" | "forbidden-path" | "max-bytes" | "json-valid" | "require-companion"
	boundaries: #Boundaries
	select:     #Selector
	companion?: #Selector
	max_bytes?: int & >=1 & <=16777216
	if kind == "max-bytes" {
		max_bytes: int & >=1
	}
	if kind != "max-bytes" {
		max_bytes?: _|_
	}
	if kind == "require-companion" {
		companion: #Selector
	}
	if kind != "require-companion" {
		companion?: _|_
	}
}
#LabeledCase: {
	id:         #ID
	expected:   "clear" | "concern" | "insufficient"
	actual:     "clear" | "concern" | "insufficient"
	confidence: number & >=0 & <=1
	provenance: "human" | "computed"
}
#Calibration: {
	site:            string & =~"^[A-Za-z0-9][A-Za-z0-9._-]{0,99}/(pre-commit|pre-push|ci|pre-publish)/[A-Za-z0-9][A-Za-z0-9._-]{0,99}$"
	question_sha256: string & =~"^[a-fA-F0-9]{64}$"
	provider:        "none" | "openai" | "typesafe" | "typesafe-openrouter"
	resolved_model:  string & !="" & =~"^[^\\x00-\\x1f\\x7f]{1,256}$"
	#modelText:      strings.TrimSpace(resolved_model) & !=""
	#modelLength:    len(resolved_model) & <=256
	valid_until:     time.Time
	#nonzeroDate:    time.Parse(time.RFC3339, valid_until) & !="0001-01-01T00:00:00Z"
	min_per_class:   int & >=1 & <=10000
	min_n:           int & >=1 & <=10000
	min_accuracy:    number & >0 & <=1
	min_wilson_lo:   number & >0 & <=1
	cases: [...#LabeledCase] & list.MaxItems(10000)
	#caseIDs: list.UniqueItems() & [for c in cases {c.id}]
}
#Site: {
	id:              #ID
	version:         #ID
	question:        string & !="" & !~"[\\x00-\\x08\\x0b-\\x1f\\x7f]"
	#questionText:   strings.TrimSpace(question) & !=""
	#questionLength: len(question) & <=16384
	boundaries:      #Boundaries
	select:          #Selector
	mode:            "shadow" | "hold"
	min_confidence:  number & >0 & <=1
	calibration?:    #Calibration
}
#Model: {
	provider:            "none" | "openai" | "typesafe" | "openrouter"
	model_env?:          #Env | ""
	expected_model_env?: #Env | ""
	api_key_env?:        #Env | ""
	max_input_bytes:     int & >=1 & <=1048576
	timeout_seconds:     int & >=1 & <=120
	cache_ttl_seconds:   int & >=0 & <=86400
	max_batches?:        int & >=1 & <=32 // absent means one for the invocation
	sites: [...#Site] & list.MaxItems(100)
	if provider != "none" {
		model_env:          #Env
		expected_model_env: #Env
		api_key_env:        #Env
	}
	if provider == "openai" {
		sites: [...#Site & {calibration?: {provider: "openai"}}]
	}
	if provider == "typesafe" {
		sites: [...#Site & {calibration?: {provider: "typesafe"}}]
	}
	if provider == "openrouter" {
		sites: [...#Site & {calibration?: {provider: "typesafe-openrouter"}}]
	}
	if provider == "none" {
		sites: [...#Site & {calibration?: {provider: "none"}}]
	}
}
