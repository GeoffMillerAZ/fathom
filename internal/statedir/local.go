package statedir

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// LocalPath is where fathom writes the per-checkout state file or directory
// f (owned.json, leases.jsonl, scip/): under the state directory, never
// under <root>/.fathom (docs/design/fathom-dir.md M4).
func LocalPath(root string, f projectdir.File, elem ...string) string {
	return filepath.Join(append([]string{Resolve(root), filepath.FromSlash(string(f))}, elem...)...)
}

// ReadPath is where to read f from: the state-directory path when it exists,
// else the legacy <root>/.fathom copy an older fathom left. The legacy path
// is returned even when it is absent too, so a caller's not-exist handling
// is one case.
func ReadPath(root string, f projectdir.File, elem ...string) string {
	if p := LocalPath(root, f, elem...); exists(p) {
		return p
	}
	return f.Path(root, elem...)
}

// LegacyStale reports whether the legacy <root>/.fathom copy of f exists
// beside a state-directory copy that supersedes it. fathom deletes neither
// (I63); doctor names the old one.
func LegacyStale(root string, f projectdir.File) bool {
	return exists(f.Path(root)) && exists(LocalPath(root, f))
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// SCIPIndexes lists the imported SCIP index files (*.scip) for root, sorted
// by name. The state-directory copy of a name wins; a legacy
// <root>/.fathom/scip file is listed only for a name the state directory
// does not hold.
func SCIPIndexes(root string) []string {
	byName := map[string]string{}
	for _, dir := range []string{projectdir.SCIP.Path(root), LocalPath(root, projectdir.SCIP)} {
		matches, err := filepath.Glob(filepath.Join(dir, "*.scip"))
		if err != nil {
			continue
		}
		for _, m := range matches {
			byName[filepath.Base(m)] = m // the state directory is visited last and wins
		}
	}
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, byName[n])
	}
	return out
}
