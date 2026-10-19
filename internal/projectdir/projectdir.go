// Package projectdir resolves the project-scope fathom directory,
// <repo>/.fathom, the twin of internal/fathomhome (the user scope, ~/.fathom).
//
// Every project-scope name is spelled here once, as a File, and joined under
// a repository root by File.Path (an on-disk path) or File.Rel (the
// repository-relative, slash-separated spelling a citation or a git path
// uses). A hand join of the literal directory name anywhere else in cmd/ or
// internal/ fails TestProjectDirIsTheOneSpelling, so a reader and the file it
// reads cannot drift apart (docs/design/fathom-dir.md section 5: the Config
// panel once checked a binding file name nothing wrote). Nothing here
// touches the disk.
package projectdir

import (
	"path"
	"path/filepath"
)

// Dir is the project directory's name under a repository root, and its own
// repository-relative spelling.
const Dir = ".fathom"

// File names one project-scope file or directory, relative to Dir and
// slash-separated.
type File string

// The project-scope names (docs/design/fathom-dir.md sections 2.1 and 4).
const (
	// Config holds the repository's parameters (TOML).
	Config File = "config.toml"
	// Capabilities is the capability switchboard.
	Capabilities File = "capabilities.cue"
	// Modules holds the platform-module bindings.
	Modules File = "modules.cue"
	// Guard holds the guards and the layer rules.
	Guard File = "guard.cue"
	// Concerns holds the agent-configuration contracts.
	Concerns File = "concerns.cue"
	// Risk is the review risk table.
	Risk File = "risk.cue"
	// Learning is the learned-rule table.
	Learning File = "learning.cue"
	// Layers is the directory of project semantic layers, one per subdirectory.
	Layers File = "layers"
	// Rules is the legacy architecture-rule directory.
	Rules File = "rules"
	// ArchConformance is the architecture-conformance rule pack under Rules.
	ArchConformance File = "rules/archconformance.cue"
	// Trace is the directory of standard mappings, one <standard>.cue each.
	Trace File = "trace"
	// Observations is the friction journal, one O<n>.md per entry.
	Observations File = "observations"
	// ObservationLog is the legacy single-file friction log.
	ObservationLog File = "observations.md"
	// Owned is the wiring ledger: what fathom added to files it does not own.
	Owned File = "owned.json"
	// Leases holds the repository's plan-row leases.
	Leases File = "leases.jsonl"
	// SCIP is the directory of imported SCIP indexes.
	SCIP File = "scip"
	// Memories is the retired anchored-memory file, read only to report a
	// copy left over from before its retirement.
	Memories File = "memories.jsonl"
)

// Under is root's project directory, <root>/.fathom.
func Under(root string) string {
	return filepath.Join(root, Dir)
}

// Path is f under root's project directory, joined with any further
// elements: <root>/.fathom/<f>/<elem...>.
func (f File) Path(root string, elem ...string) string {
	return filepath.Join(append([]string{root, Dir, filepath.FromSlash(string(f))}, elem...)...)
}

// Rel is f's repository-relative, slash-separated spelling, joined with any
// further elements: .fathom/<f>/<elem...>.
func (f File) Rel(elem ...string) string {
	return path.Join(append([]string{Dir, string(f)}, elem...)...)
}
