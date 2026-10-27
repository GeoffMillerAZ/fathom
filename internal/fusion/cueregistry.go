package fusion

import (
	"cuelang.org/go/cue/load"

	"github.com/geoffmilleraz/fathom/internal/cuerefuse"
)

// cueTypedLoadConfig is the one load.Config the typed CUE pass uses.
func cueTypedLoadConfig(root string) *load.Config {
	return &load.Config{
		Dir:     root,
		Package: "*", // every package; files with no package clause -> "_"
		// Explicit registry: ambient CUE_REGISTRY and the default registry
		// are never consulted.
		Registry: cuerefuse.Registry{},
		Env:      []string{},
	}
}
