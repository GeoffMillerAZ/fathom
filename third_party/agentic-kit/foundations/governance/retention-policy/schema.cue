// Package retentionpolicy defines the CUE contract for data-retention rules.
// Go types in types.go and retentionpolicy.go are the counterparts; drift is a bug.
//
// WASM note: this schema is pure data; it contains no import that would make it
// non-portable. The Go implementation (retentionpolicy.go) is WASM-safe.
package retentionpolicy

// #ActionKind enumerates the permitted retention actions.
#ActionKind: "shred" | "flag" | "report"

// #Policy is a single retention rule binding a namespace/class filter to a
// maximum age and a prescribed action. MaxAgeSec is the max age in seconds
// (Go's time.Duration maps to int64 nanoseconds; CUE fixtures use seconds for
// human readability and Go tests use time.Duration directly).
#Policy: {
	// Name is a human-readable label. Must be non-empty.
	Name: string & !=""

	// NamespacePrefix, if non-empty, restricts the policy to namespaces with
	// this prefix. Empty string matches all namespaces.
	NamespacePrefix: string

	// Class, if non-empty, restricts to inventory entries of this class.
	// Empty string matches all classes.
	Class: string

	// MaxAgeSec is the maximum allowed data age in seconds (> 0).
	MaxAgeSec: >0 & number

	// Action is the prescribed retention action.
	Action: #ActionKind
}

// #NamespaceEntry is a caller-supplied fact about one namespace.
#NamespaceEntry: {
	// Namespace is the namespace identifier. Must be non-empty.
	Namespace: string & !=""

	// Class is the data sensitivity class (e.g. "pii"). May be empty.
	Class: string

	// CreatedAtRFC3339 is the creation time as an RFC3339 string.
	CreatedAtRFC3339: string & !=""
}
