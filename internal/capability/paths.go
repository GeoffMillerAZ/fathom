package capability

import (
	"os"
	"path/filepath"

	"github.com/geoffmilleraz/fathom/internal/fathomhome"
)

// ProjectPath returns "<root>/.fathom/capabilities.cue" for a repository
// root — the ONE spelling of the project-scope location. The daemon
// reaches it through [DefaultProjectPath]; every cmd/fathom verb that
// takes an explicit --repo reaches it here, so the two can never drift.
func ProjectPath(root string) string {
	return filepath.Join(root, ".fathom", ProjectFileName)
}

// DefaultUserPath returns "~/.fathom/capabilities.cue", or "" when the
// home directory cannot be resolved — which [LoadBudgets] treats as
// absent, and which a writer must refuse rather than write to.
func DefaultUserPath() string {
	return fathomhome.Path(UserFileName)
}

// DefaultProjectPath returns [ProjectPath] of the working directory — the
// daemon's working repository, the same root internal/config resolves its
// project layer against — or "" when the working directory cannot be
// resolved.
func DefaultProjectPath() string {
	wd, err := os.Getwd()
	if err != nil || wd == "" {
		return ""
	}
	return ProjectPath(wd)
}
