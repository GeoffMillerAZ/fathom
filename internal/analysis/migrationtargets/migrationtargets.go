// Package migrationtargets identifies symbols that depend on deprecated or
// outdated packages and should be migrated to modern alternatives.
//
// Find scans a []core.SymbolNode and returns a sorted []Target for every node
// whose Package field matches any entry in the deprecatedPkgs list.  The Reason
// field explains which deprecated package triggered the match.
//
// The check is package-level: a node is flagged when its Package contains (or
// equals) any deprecated package string.  This handles both exact matches
// ("io/ioutil") and sub-package matches.
//
// All results are sorted by (Symbol, Reason) for determinism.
package migrationtargets

import (
	"fmt"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// Target is a symbol that should be migrated away from a deprecated package.
type Target struct {
	Symbol string
	Reason string
}

// DefaultDeprecatedPkgs is the built-in set of Go packages that have been
// deprecated or superseded.
var DefaultDeprecatedPkgs = []string{
	"io/ioutil",
	"golang.org/x/net/context",
	"golang.org/x/oauth2/internal",
	"github.com/golang/protobuf",
}

// Find returns the sorted slice of Targets: symbols whose Package is (or
// contains) any of the deprecated packages.
//
// If deprecatedPkgs is nil or empty, DefaultDeprecatedPkgs is used.
//
// The Symbol field of each Target is the node's canonical SymbolKey.
func Find(nodes []core.SymbolNode, deprecatedPkgs []string) []Target {
	if len(deprecatedPkgs) == 0 {
		deprecatedPkgs = DefaultDeprecatedPkgs
	}

	var targets []Target
	seen := make(map[string]bool)

	for _, node := range nodes {
		for _, pkg := range deprecatedPkgs {
			if matchesPkg(node.Package, pkg) {
				key := node.Key()
				dedupeKey := key + "\x00" + pkg
				if seen[dedupeKey] {
					continue
				}
				seen[dedupeKey] = true
				targets = append(targets, Target{
					Symbol: key,
					Reason: fmt.Sprintf("uses deprecated package %q", pkg),
				})
			}
		}
	}

	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Symbol != targets[j].Symbol {
			return targets[i].Symbol < targets[j].Symbol
		}
		return targets[i].Reason < targets[j].Reason
	})
	return targets
}

// matchesPkg reports whether nodePkg is deprecated according to depPkg.
// It matches when nodePkg equals depPkg or nodePkg starts with depPkg+"/".
func matchesPkg(nodePkg, depPkg string) bool {
	return nodePkg == depPkg ||
		strings.HasPrefix(nodePkg, depPkg+"/") ||
		strings.Contains(nodePkg, depPkg)
}
