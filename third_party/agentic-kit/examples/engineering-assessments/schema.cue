package engineeringassessments

import fitness "github.com/geoffmilleraz/agentic-kit/foundations/quality/fitness-function-registry:fitnessfunctionregistry"

// #Report is the portable reference envelope, not a mandatory evidence store.
#Report: {
	profile: fitness.#Profile
	subject_digest: fitness.#AssessmentDigest
	observations: [...fitness.#Observation]
	assessment: fitness.#Assessment
	artifacts: [string]: string
}
