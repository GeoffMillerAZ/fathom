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
	"os"
	"path"
	"path/filepath"
	"sort"
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
}

var includeCache sync.Map // abs config path -> includeMemo

// loadConnectorInclude returns the `[connector] include` set of the corpus
// at root (nil when absent). It is read from the corpus's own
// .fathom/config.toml — a property of the corpus, like .fathomignore — and
// re-parsed only when the file's size or mtime changes, so a Discover
// costs a stat, not a parse. A malformed file includes nothing (the narrower
// default stands; `fathom config` is the loud surface for a broken file).
func loadConnectorInclude(root string) map[string]bool {
	p := filepath.Join(root, connectorConfigName)
	fi, err := os.Stat(p)
	if err != nil {
		return nil
	}
	if v, ok := includeCache.Load(p); ok {
		m := v.(includeMemo)
		if m.size == fi.Size() && m.mtimeNS == fi.ModTime().UnixNano() {
			return m.include
		}
	}
	var include map[string]bool
	if fi.Size() <= 1<<20 {
		if data, rerr := os.ReadFile(p); rerr == nil {
			var doc struct {
				Connector struct {
					Include []string `toml:"include"`
				} `toml:"connector"`
			}
			if toml.Unmarshal(data, &doc) == nil && len(doc.Connector.Include) > 0 {
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
	includeCache.Store(p, includeMemo{size: fi.Size(), mtimeNS: fi.ModTime().UnixNano(), include: include})
	return include
}
