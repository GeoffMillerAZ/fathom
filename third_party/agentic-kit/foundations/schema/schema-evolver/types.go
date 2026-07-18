// Package schemaevolver provides controlled migration paths between schema
// versions so that consumers are never silently broken by schema changes.
//
// The Evolver maintains a directed graph of registered Migration steps. Given
// a source and destination version, Path performs a breadth-first search to
// find the shortest sequence of migrations that connects them. If no path
// exists the call returns an error, making the gap explicit rather than silent.
//
// Concurrency: Evolver is not safe for concurrent use; guard with your own synchronisation.
package schemaevolver

// Version is a two-part schema version. Major increments signal
// backward-incompatible changes; minor increments are backward-compatible
// additions.
type Version struct {
	// Major is the major version component.
	Major int
	// Minor is the minor version component.
	Minor int
}

// Migration describes a single controlled step between two schema versions.
// The Description field is required and must explain what changed.
type Migration struct {
	// From is the source version for this migration step.
	From Version

	// To is the destination version for this migration step.
	To Version

	// Description is a human-readable explanation of what changed between
	// From and To. Required; an empty description is rejected by Register.
	Description string
}
