package fusion

import (
	"context"
	"fmt"

	"cuelang.org/go/cue/load"
	"cuelang.org/go/mod/modconfig"
	"cuelang.org/go/mod/module"
)

const cueRegistryRefusalReason = "CUE typed pass refuses registry access: local module and builtins only"

// cueRefusingRegistry closes cue/load's external-module boundary. A nil
// load.Config.Registry builds the default registry and module cache even with
// CUE_REGISTRY=inline:{}, because the resolver still names the public default
// host. This implementation names no host and never opens a cache, credential
// store or transport: an external dependency reads as an unresolved import.
// Local module imports and builtins are resolved by cue/load without it.
type cueRefusingRegistry struct{}

var _ modconfig.Registry = cueRefusingRegistry{}

func (cueRefusingRegistry) Requirements(_ context.Context, v module.Version) ([]module.Version, error) {
	return nil, fmt.Errorf("%s (Requirements %s)", cueRegistryRefusalReason, v)
}

func (cueRefusingRegistry) Fetch(_ context.Context, v module.Version) (module.SourceLoc, error) {
	return module.SourceLoc{}, fmt.Errorf("%s (Fetch %s)", cueRegistryRefusalReason, v)
}

func (cueRefusingRegistry) ModuleVersions(_ context.Context, path string) ([]string, error) {
	return nil, fmt.Errorf("%s (ModuleVersions %s)", cueRegistryRefusalReason, path)
}

// cueTypedLoadConfig is the one load.Config the typed CUE pass uses.
func cueTypedLoadConfig(root string) *load.Config {
	return &load.Config{
		Dir:     root,
		Package: "*", // every package; files with no package clause -> "_"
		// Explicit registry: ambient CUE_REGISTRY and the default registry
		// are never consulted.
		Registry: cueRefusingRegistry{},
		Env:      []string{},
	}
}
