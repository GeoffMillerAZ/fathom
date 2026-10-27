// Package cuerefuse holds the registry that closes cue/load's external-module
// boundary for every in-process CUE load.
package cuerefuse

import (
	"context"
	"fmt"

	"cuelang.org/go/mod/modconfig"
	"cuelang.org/go/mod/module"
)

const refusalReason = "CUE typed pass refuses registry access: local module and builtins only"

// Registry closes cue/load's external-module boundary. A nil
// load.Config.Registry builds the default registry and module cache even with
// CUE_REGISTRY=inline:{}, because the resolver still names the public default
// host. This implementation names no host and never opens a cache, credential
// store or transport: an external dependency reads as an unresolved import.
// Local module imports and builtins are resolved by cue/load without it.
type Registry struct{}

var _ modconfig.Registry = Registry{}

func (Registry) Requirements(_ context.Context, v module.Version) ([]module.Version, error) {
	return nil, fmt.Errorf("%s (Requirements %s)", refusalReason, v)
}

func (Registry) Fetch(_ context.Context, v module.Version) (module.SourceLoc, error) {
	return module.SourceLoc{}, fmt.Errorf("%s (Fetch %s)", refusalReason, v)
}

func (Registry) ModuleVersions(_ context.Context, path string) ([]string, error) {
	return nil, fmt.Errorf("%s (ModuleVersions %s)", refusalReason, path)
}
