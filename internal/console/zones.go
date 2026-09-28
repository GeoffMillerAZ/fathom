package console

// zones.go — WI-1 (docs/design/work-intelligence.md §2): the zone is the
// unit every work-intelligence view aggregates to. By default a Go file's
// zone is its package directory; any other file's is the deepest directory
// at or above it holding indexed symbols (its own directory when none
// does). `.fathom/config.toml` `[zones]` names coarser zones by path
// globs, which override the default. A change belongs to the distinct
// zones of the files it touched, split evenly across them. Pure: the
// caller supplies the config and the symbol directories (a stored fact),
// never a tree walk.

import (
	"path"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/pathglob"
)

// ZoneRule is one `[zones]` entry: a zone name and its path globs.
type ZoneRule struct {
	Name  string   `json:"name"`
	Globs []string `json:"globs"`
}

// Zones resolves files to zones.
type Zones struct {
	Rules      []ZoneRule
	symbolDirs map[string]bool
}

// NewZones builds the resolver from the `[zones]` table (name to globs)
// and the repository-relative directories holding indexed symbols. Rules
// are ordered by name, so a file two zones' globs both match resolves
// deterministically to the first name.
func NewZones(cfg map[string][]string, symbolDirs []string) Zones {
	z := Zones{symbolDirs: make(map[string]bool, len(symbolDirs))}
	for name, globs := range cfg {
		z.Rules = append(z.Rules, ZoneRule{Name: name, Globs: append([]string(nil), globs...)})
	}
	sort.Slice(z.Rules, func(i, j int) bool { return z.Rules[i].Name < z.Rules[j].Name })
	for _, d := range symbolDirs {
		z.symbolDirs[cleanRel(d)] = true
	}
	return z
}

func cleanRel(p string) string {
	p = path.Clean(strings.ReplaceAll(p, "\\", "/"))
	return strings.TrimPrefix(p, "/")
}

// ZoneOf is file's zone under z.
func ZoneOf(file string, z Zones) string {
	file = cleanRel(file)
	for _, r := range z.Rules {
		if pathglob.MatchAny(r.Globs, file) {
			return r.Name
		}
	}
	dir := path.Dir(file)
	if strings.HasSuffix(file, ".go") {
		return dir
	}
	for d := dir; ; d = path.Dir(d) {
		if z.symbolDirs[d] {
			return d
		}
		if d == "." || d == "/" {
			break
		}
	}
	return dir
}

// ZoneShare is one zone's share of a change.
type ZoneShare struct {
	Zone  string  `json:"zone"`
	Share float64 `json:"share"`
}

// ChangeZones splits a change touching files evenly across its distinct
// zones, sorted by zone.
func ChangeZones(files []string, z Zones) []ZoneShare {
	set := map[string]bool{}
	for _, f := range files {
		set[ZoneOf(f, z)] = true
	}
	out := make([]ZoneShare, 0, len(set))
	for _, name := range sortedKeys(set) {
		out = append(out, ZoneShare{Zone: name, Share: 1 / float64(len(set))})
	}
	return out
}
