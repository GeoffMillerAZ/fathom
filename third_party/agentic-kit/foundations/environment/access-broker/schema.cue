// Package accessbroker defines the CUE schema for credential negotiation types.
// These constraints are the authoritative shape specification; the Go types
// in types.go mirror them. Evaluated with `cue vet ./...`.
package accessbroker

// #Level enumerates the valid access levels for a credential scope.
#Level: "read" | "write" | "admin"

// #CredentialScope describes a resource and the desired access level.
#CredentialScope: {
	// Resource is the logical identifier of the resource being accessed.
	// Must be non-empty.
	Resource!: string & !=""

	// Level is the access level requested.
	Level!: #Level
}
