// valid.cue — positive fixtures for every type in schema.cue.
// Every value here must unify cleanly on `cue vet ./...`.
package environmentregistry

import "github.com/geoffmilleraz/agentic-kit/foundations/environment/environment-registry:environmentregistry"

// devEnv demonstrates a well-formed dev-tier environment.
devEnv: environmentregistry.#Environment & {
	Name: "local-dev"
	Kind: "dev"
}

// stagingEnv demonstrates a well-formed staging-tier environment.
stagingEnv: environmentregistry.#Environment & {
	Name: "ci-staging"
	Kind: "staging"
}

// prodEnv demonstrates a well-formed production-tier environment.
prodEnv: environmentregistry.#Environment & {
	Name: "production"
	Kind: "prod"
}

// allKinds verifies that all #Kind enum values are accepted.
_k1: environmentregistry.#Kind & "dev"
_k2: environmentregistry.#Kind & "staging"
_k3: environmentregistry.#Kind & "prod"
