package semantic

// defaultlayers.go — CV-41 (docs/plans/refactor-conservation.md,
// docs/design/refactor-conservation.md §6 "Activation", ADR-044): when a
// caller of `fathom compile` names no layer, DefaultLayersFor picks the
// compile set instead of the caller silently getting whatever single layer
// happened to be hardcoded. "obligations" is always in the set — its own
// catalog entry (docs/layers.md) says "always on, every fleet repo" — and
// "adr-rules"/"spec-bindings" join it whenever their OWN corpus globs (read
// from each pack's layer.cue via LoadLayerIn, never duplicated here) match
// at least one file under root. A layer whose corpus matches nothing is
// left out: compiling it would run zero sections and write zero claims
// every time, so naming it in a "default" set would just print an empty
// run on every repo that doesn't have it.

import (
	"io/fs"
	"path/filepath"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/filter"
)

// activatedByCorpus names the catalog layers (beyond the always-on
// "obligations") whose presence in the default compile set is decided by
// corpus presence rather than being unconditional.
var activatedByCorpus = []string{"adr-rules", "spec-bindings"}

// DefaultLayersFor returns the layer names `fathom compile` should target
// when the caller names none: LayerObligations always, plus every name in
// activatedByCorpus whose LoadLayerIn(root, name) corpus matches at least
// one file under root. Sorted, deterministic. root == "", an unreadable
// root, or a layer this build does not carry are never fatal — they simply
// leave that layer out, never a guess at a corpus that cannot be read.
func DefaultLayersFor(root string) []string {
	names := []string{LayerObligations}
	for _, name := range activatedByCorpus {
		def, err := LoadLayerIn(root, name)
		if err != nil {
			continue
		}
		if corpusHasAnyFile(root, def.CorpusInclude, def.CorpusExclude) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// corpusHasAnyFile reports whether at least one regular file under root
// matches include (and not exclude) — connector.IsSkippedDir's exclusions
// applied so this agrees with what a real `fathom ingest`/compile corpus
// scan would surface (vendor, .git, testdata, generated, node_modules,
// .idea, .vscode, .claude are never corpus). It short-circuits on the
// first match: this is a presence check, not a corpus listing — the real
// corpus at compile time is still SelectCorpus over the service's actually
// indexed sources. A missing or unreadable root reads false, never panics
// or guesses.
func corpusHasAnyFile(root string, include, exclude []string) bool {
	if root == "" || len(include) == 0 {
		return false
	}
	inc := filter.New(include)
	var excl *filter.Matcher
	if len(exclude) > 0 {
		excl = filter.New(exclude)
	}
	found := false
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if found {
			return filepath.SkipAll
		}
		if err != nil {
			return nil // an unreadable entry is skipped, never aborts the whole walk
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel != "." && connector.IsSkippedDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if rel == "" || !d.Type().IsRegular() {
			return nil
		}
		if !inc.Match(rel) {
			return nil
		}
		if excl != nil && excl.Match(rel) {
			return nil
		}
		found = true
		return filepath.SkipAll
	})
	return found
}
