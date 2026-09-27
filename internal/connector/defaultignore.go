package connector

// defaultignore.go — RT-145 (docs/design/responsible-tenant.md §4.2): the
// walk prunes the default cache directories on top of skipDir. They are
// build output, tool caches and virtualenvs, not source: app-kit's .cache/
// (19,659 fingerprinted files, ~2.3 GiB) and compute/.tools held 43% of
// its 5.3 GiB tree. A corpus that genuinely indexes one re-includes it
// with `[connector] include` in its own .fathom/config.toml, and every
// directory pruned here is named in Exclusions (I25/I38) — never a silent
// drop.

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	toml "github.com/pelletier/go-toml/v2"
)

// defaultIgnoredDirs is the default cache-directory list (§4.2).
var defaultIgnoredDirs = map[string]bool{
	".cache": true, ".tools": true, "target": true, "dist": true, "build": true,
	"__pycache__": true, ".venv": true, ".gradle": true, ".next": true,
	".turbo": true, "coverage": true,
}

// DefaultIgnoredDirNames returns the default cache-directory list, sorted.
func DefaultIgnoredDirNames() []string {
	out := make([]string, 0, len(defaultIgnoredDirs))
	for n := range defaultIgnoredDirs {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// defaultIgnoredDir reports whether the directory at rel (repo-relative,
// slash-separated) with base name name is pruned by the default list and
// not re-included. include holds `[connector] include` entries: a bare name
// re-includes every directory with that name, an entry with a slash
// re-includes that one repo-relative path. skipDir names are never
// re-included — include widens only the default list.
func defaultIgnoredDir(name, rel string, include map[string]bool) bool {
	if !defaultIgnoredDirs[name] {
		return false
	}
	return !include[name] && !include[rel]
}

// Exclusions is what the most recent Discover of a service left out of the
// corpus at the directory level: DefaultIgnored names each directory the
// default cache list pruned (repo-relative, sorted); SkippedDirs counts the
// directories skipDir pruned (vendor, .git, testdata, …); IgnoredDirs
// counts those an ignore file pruned.
type Exclusions struct {
	DefaultIgnored []string
	SkippedDirs    int
	IgnoredDirs    int
	// GitIgnored counts the untracked entries .gitignore excluded
	// (directories collapsed, as `git ls-files -o -i --directory` lists
	// them) when discovery went through git (RT-68); Git reports that it
	// did.
	GitIgnored int
	Git        bool
	// GitUnlisted counts directory entries git listed that discovery could
	// not list through their own git (a nested repository or submodule
	// with no readable work tree): excluded, and counted (I38).
	GitUnlisted int
}

// ExclusionReporter is implemented by connectors that record what their
// last Discover of a service excluded.
type ExclusionReporter interface {
	Exclusions(service string) Exclusions
}

// connectorConfigName is the corpus-local config file `[connector] include`
// is read from.
var connectorConfigName = filepath.Join(".fathom", "config.toml")

type includeMemo struct {
	size    int64
	mtimeNS int64
	include map[string]bool
	gitMode bool
}

var includeCache sync.Map // abs config path -> includeMemo

// loadConnectorInclude returns the `[connector] include` set of the corpus
// at root (nil when absent). It is read from the corpus's own
// .fathom/config.toml — a property of the corpus, like .fathomignore — and
// re-parsed only when the file's size or mtime changes, so a Discover
// costs a stat, not a parse. A malformed file includes nothing (the narrower
// default stands; `fathom config` is the loud surface for a broken file).
func loadConnectorInclude(root string) map[string]bool {
	inc, _ := loadConnectorConfig(root)
	return inc
}

// loadConnectorConfig is the corpus's `[connector]` table: include, and
// whether discovery respects .gitignore (RT-68).
func loadConnectorConfig(root string) (map[string]bool, bool) {
	p := filepath.Join(root, connectorConfigName)
	fi, err := os.Stat(p)
	if err != nil {
		return nil, false
	}
	if v, ok := includeCache.Load(p); ok {
		m := v.(includeMemo)
		if m.size == fi.Size() && m.mtimeNS == fi.ModTime().UnixNano() {
			return m.include, m.gitMode
		}
	}
	var include map[string]bool
	gitMode := false
	if fi.Size() <= 1<<20 {
		if data, rerr := os.ReadFile(p); rerr == nil {
			var doc struct {
				Connector struct {
					Include          []string `toml:"include"`
					RespectGitignore bool     `toml:"respect_gitignore"`
				} `toml:"connector"`
			}
			perr := toml.Unmarshal(data, &doc)
			gitMode = perr == nil && doc.Connector.RespectGitignore
			if perr == nil && len(doc.Connector.Include) > 0 {
				include = make(map[string]bool, len(doc.Connector.Include))
				for _, e := range doc.Connector.Include {
					e = path.Clean(filepath.ToSlash(e))
					if e != "." && e != "" {
						include[e] = true
					}
				}
			}
		}
	}
	includeCache.Store(p, includeMemo{size: fi.Size(), mtimeNS: fi.ModTime().UnixNano(), include: include, gitMode: gitMode})
	return include, gitMode
}

// ExcludedFromCorpus reports whether the repo-relative file rel under root
// is outside the corpus Discover would walk for serviceID — under an
// always-skipped directory, a default-ignored cache directory that
// `[connector] include` does not re-include, or a path the scoped ignore
// files match — and names why. The one-file refresh path (/refresh-file)
// asks it so the pre-edit hook never indexes a file the next ingest purges
// as an orphan. The corpus root itself is never excluded by name.
func ExcludedFromCorpus(root, serviceID, rel string) (reason string, excluded bool, err error) {
	rel = path.Clean(filepath.ToSlash(rel))
	ignore, _, err := LoadIgnoreScopedFor(root, serviceID)
	if err != nil {
		return "", false, err
	}
	include, gitMode := loadConnectorConfig(root)
	segs := strings.Split(rel, "/")
	for i := 0; i < len(segs)-1; i++ {
		name := segs[i]
		prefix := strings.Join(segs[:i+1], "/")
		switch {
		case skipDir(name):
			return fmt.Sprintf("under always-skipped directory %s", prefix), true, nil
		case defaultIgnoredDir(name, prefix, include):
			return fmt.Sprintf("under default-ignored cache directory %s (re-include with [connector] include)", prefix), true, nil
		case ignore != nil && ignore.Match(prefix+"/"):
			return fmt.Sprintf("under ignored directory %s", prefix), true, nil
		}
	}
	if ignore != nil && ignore.Match(rel) {
		return "matched by an ignore file", true, nil
	}
	// RT-68: a corpus discovered through git excludes what .gitignore
	// does (unless include re-includes its directory), so a refresh must
	// not index it either.
	if gitMode && !underInclude(rel, include) {
		if gitIgnored(root, rel) && !refreshRootIgnored(root) {
			return "ignored by .gitignore ([connector] respect_gitignore)", true, nil
		}
	}
	return "", false, nil
}

// underInclude reports whether rel lies under a `[connector] include`
// directory (a bare name anywhere, or a repo-relative path).
func underInclude(rel string, include map[string]bool) bool {
	segs := strings.Split(rel, "/")
	for i := 0; i < len(segs)-1; i++ {
		if include[segs[i]] || include[strings.Join(segs[:i+1], "/")] {
			return true
		}
	}
	return false
}
