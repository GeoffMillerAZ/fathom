// Package toolexecutionpolicy defines the CUE schema for tool execution rules.
// These constraints are the authoritative shape specification; the Go types
// in types.go mirror them. Evaluated with `cue vet ./...`.
package toolexecutionpolicy

// #ToolPolicy is a single rule that allows or denies a named tool in one
// or more named runtime environments.
#ToolPolicy: {
	// Tool is the tool name this rule governs. Must be non-empty.
	Tool!: string & !=""

	// Allow specifies whether this rule permits (true) or denies (false)
	// tool execution when matched.
	Allow!: bool

	// Environments lists the environment names to which this rule applies.
	// Each name must be non-empty.
	Environments!: [...string & !=""]
}
