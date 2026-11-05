// Package langlink is the registry of optional extractor families for the
// language library (ADR-092). A family is a set of dialects whose extractors
// pull in a heavy dependency (the CUE family needs cuelang.org/go), so pkg/lang
// does not link them: a sibling package registers its family from init, and a
// Reader asks for it by name.
package langlink

import (
	"sort"
	"sync"

	"github.com/geoffmilleraz/fathom/internal/langroute"
)

// CUE is the name of the CUE family.
const CUE = "cue"

// FamilyDialects names the dialects each family reads. It is static, so a
// Reader that did not link a family can still leave its dialects out of
// Dialects.
var FamilyDialects = map[string][]string{
	CUE: {"cue", "fathom-concerns", "fathom-layer-packs", "timoni"},
}

// Family is a registered family: the router options that add its extractors.
type Family struct {
	Name    string
	Options func() []langroute.Option
}

var (
	mu       sync.RWMutex
	registry = map[string]Family{}
)

// Register makes f available to Lookup. A second registration of a name
// replaces the first.
func Register(f Family) {
	mu.Lock()
	defer mu.Unlock()
	registry[f.Name] = f
}

// Lookup is the registered family called name.
func Lookup(name string) (Family, bool) {
	mu.RLock()
	defer mu.RUnlock()
	f, ok := registry[name]
	return f, ok
}

// Names is every family FamilyDialects knows, sorted.
func Names() []string {
	out := make([]string, 0, len(FamilyDialects))
	for n := range FamilyDialects {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
