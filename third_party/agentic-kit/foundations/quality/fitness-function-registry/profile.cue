package fitnessfunctionregistry

import (
	"list"
)

// Assessment-profile CUE contract. profile.go owns cross-record validation,
// canonical identity, freshness, and required/scored decision semantics.

#AssessmentDigest: string & =~"^[0-9a-f]{64}$"
#AssessmentID:     string & =~"^[A-Za-z][A-Za-z0-9._/-]{0,127}$"
#AssessmentText:   string & !=""
#AssessmentTime:   string & =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?Z$"
#ArtifactDigests: [...#AssessmentDigest] & list.MaxItems(16) & list.UniqueItems()
#NullableArtifactDigests: null | #ArtifactDigests

#EvidenceMethod: "deterministic" | "empirical" | "heuristic" | "human"
#FindingOutcome: "pass" | "fail" | "unknown" | "not-applicable"
#CriterionKind:  "required" | "scored"
#RequiredStatus: "pass" | "fail" | "unknown"
#AssessmentRule: "observed" | "observed-unknown" | "observed-not-applicable" |
	"missing-observation" | "definition-mismatch" | "subject-mismatch" |
	"method-disallowed" | "observed-in-future" | "observation-stale" |
	"not-applicable-disallowed"

#Criterion: {
	id:                #AssessmentID
	definition_digest: #AssessmentDigest
	kind:              #CriterionKind
	allowed_methods: [...#EvidenceMethod] & list.MinItems(1) & list.MaxItems(4) & list.UniqueItems()
	allow_not_applicable: bool
	weight:               number & >=0 & <=1000000
	if kind == "required" {
		weight: 0
	}
	if kind == "scored" {
		weight: >0
	}
}

#Profile: {
	version: 1
	id:      #AssessmentID
	domain:  #AssessmentID
	criteria: [...#Criterion] & list.MinItems(1) & list.MaxItems(256) & list.UniqueItems()
}

#Observation: {
	criterion_id:      #AssessmentID
	definition_digest: #AssessmentDigest
	subject_digest:    #AssessmentDigest
	outcome:           #FindingOutcome
	method:            #EvidenceMethod
	reason:            #AssessmentText
	artifact_digests:  #NullableArtifactDigests
	observed_at:       #AssessmentTime
	valid_until:       #AssessmentTime
	if outcome != "unknown" {
		artifact_digests: #ArtifactDigests & list.MinItems(1)
	}
	if len(reason) > 2048 {
		_|_
	}
}

#AssessmentTrace: {
	criterion_id:       #AssessmentID
	definition_digest:  #AssessmentDigest
	outcome:            #FindingOutcome
	rule:               #AssessmentRule
	reason:             #AssessmentText
	artifact_digests:   #ArtifactDigests
	observation_digest: null | #AssessmentDigest
	if len(reason) > 2048 {
		_|_
	}
}

#Assessment: {
	trust:           "caller-declared"
	profile_digest:  #AssessmentDigest
	subject_digest:  #AssessmentDigest
	evidence_digest: #AssessmentDigest
	evaluated_at:    #AssessmentTime
	required_status: #RequiredStatus
	soft_score:      null | (number & >=0 & <=1)
	trace: [...#AssessmentTrace] & list.MaxItems(256)
}
