// valid.cue — positive fixtures for every type in schema.cue.
// Every value here must unify cleanly on `cue vet ./...`.
package accessbroker

import "github.com/geoffmilleraz/agentic-kit/foundations/environment/access-broker:accessbroker"

// readScope demonstrates a read-only credential scope.
readScope: accessbroker.#CredentialScope & {
	Resource: "db/analytics"
	Level:    "read"
}

// writeScope demonstrates a write credential scope.
writeScope: accessbroker.#CredentialScope & {
	Resource: "bucket/reports"
	Level:    "write"
}

// adminScope demonstrates an admin credential scope.
adminScope: accessbroker.#CredentialScope & {
	Resource: "cluster/control-plane"
	Level:    "admin"
}

// allLevels verifies all #Level enum values are accepted.
_l1: accessbroker.#Level & "read"
_l2: accessbroker.#Level & "write"
_l3: accessbroker.#Level & "admin"
