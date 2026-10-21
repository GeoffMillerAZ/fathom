package fusion

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/mod/modconfig"
	"cuelang.org/go/mod/module"

	"github.com/geoffmilleraz/fathom/internal/store"
)

const cueRegistryRefusalReason = "CUE native index refuses registry access: local module and builtins only"

// cueLocalOnlyRegistry closes cue/load's external-module boundary. In particular,
// CUE_REGISTRY=inline:{} is not an offline switch: a nil Config.Registry creates
// the default registry and its persistent module cache. This implementation never
// constructs or delegates to a registry, cache, credential reader or transport.
// It intentionally refuses external modules even if an operator cached them.
// Local module imports and builtins are resolved by cue/load without this port.
// This is not a filesystem sandbox or a per-file loader read boundary.
type cueLocalOnlyRegistry struct{ denied atomic.Bool }

var _ modconfig.Registry = (*cueLocalOnlyRegistry)(nil)

func (r *cueLocalOnlyRegistry) Requirements(_ context.Context, target module.Version) ([]module.Version, error) {
	return nil, r.refuse("Requirements", target.String())
}

func (r *cueLocalOnlyRegistry) Fetch(_ context.Context, target module.Version) (module.SourceLoc, error) {
	return module.SourceLoc{}, r.refuse("Fetch", target.String())
}

func (r *cueLocalOnlyRegistry) ModuleVersions(_ context.Context, target string) ([]string, error) {
	return nil, r.refuse("ModuleVersions", target)
}

func (r *cueLocalOnlyRegistry) refuse(method, target string) error {
	r.denied.Store(true)
	return fmt.Errorf("%s (%s %s)", cueRegistryRefusalReason, method, target)
}

func cueLocalLoadConfig(root string, registry *cueLocalOnlyRegistry) *load.Config {
	return &load.Config{
		Dir: root, ModuleRoot: root,
		Package: "*", // every package; files with no package clause -> "_"
		// An inline-empty CUE_REGISTRY still enables the default registry.
		Registry: registry,
		Env:      []string{},
	}
}

// invalidateCUEDeniedPackages uses native package loads to attribute policy
// denial. Pinned cue/load formats away the registry's error identity and can
// collapse ./... into one failed synthetic instance without package metadata.
// A fresh, pass-local denial flag therefore observes each prior CUE package's
// own native load. No error-string matching or all-producer purge is involved.
// Work is at most one retry per stored CUE package, never recursive. cue/load has
// no cancellation API; cancellation is checked between its bounded invocations.
func invalidateCUEDeniedPackages(ctx context.Context, st store.Store, service, root string, loadInstances func([]string, *load.Config) []*build.Instance) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("CUE registry denial inspection incomplete: %w", err)
	}
	packages, err := st.ListTypedEdgePackages(ctx, service)
	if err != nil {
		return fmt.Errorf("CUE registry denial inspection incomplete: list prior packages: %w", err)
	}
	for _, key := range packages { // store contract returns a sorted distinct list
		if !strings.HasPrefix(key, cuePackageKeyPrefix) {
			continue
		}
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("CUE registry denial inspection incomplete: %w", err)
		}
		arg, err := cueRegistryInspectionArg(root, strings.TrimPrefix(key, cuePackageKeyPrefix))
		if err != nil {
			return fmt.Errorf("CUE registry denial inspection incomplete for %q: %w", key, err)
		}
		registry := &cueLocalOnlyRegistry{}
		instances := loadInstances([]string{arg}, cueLocalLoadConfig(root, registry))
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("CUE registry denial inspection incomplete for %q: %w", key, err)
		}
		if len(instances) == 0 {
			return fmt.Errorf("CUE registry denial inspection incomplete for %q: native loader returned no instance", key)
		}
		if !registry.denied.Load() {
			continue // preserve healthy packages and the ordinary failure policy
		}
		if err := st.DeleteTypedEdgePackage(ctx, service, key); err != nil {
			return fmt.Errorf("CUE registry denial invalidation failed for %q: %w", key, err)
		}
	}
	return nil
}

// Native package IDs normally are import paths. Legacy/anonymous IDs instead
// use absolute-dir:PkgName; convert only an existing, physically in-root directory
// into a relative package selection. Never pass a stored absolute path to load.
func cueRegistryInspectionArg(root, id string) (string, error) {
	if colon := strings.LastIndexByte(id, ':'); colon >= 0 && filepath.IsAbs(id[:colon]) {
		dir, name := id[:colon], id[colon+1:]
		if !ast.IsValidIdent(name) {
			return "", fmt.Errorf("unsupported fallback package name %q", name)
		}
		rel, err := filepath.Rel(root, dir)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("fallback directory is outside the module root")
		}
		realRoot, rootErr := filepath.EvalSymlinks(root)
		realDir, dirErr := filepath.EvalSymlinks(dir)
		if rootErr != nil || dirErr != nil {
			return "", fmt.Errorf("fallback directory cannot be resolved")
		}
		if info, err := os.Stat(realDir); err != nil || !info.IsDir() {
			return "", fmt.Errorf("fallback directory is not an existing directory")
		}
		physicalRel, err := filepath.Rel(realRoot, realDir)
		if err != nil || physicalRel == ".." || strings.HasPrefix(physicalRel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("fallback directory resolves outside the module root")
		}
		return "./" + filepath.ToSlash(rel) + ":" + name, nil
	}
	if err := module.CheckImportPath(id); err != nil {
		return "", fmt.Errorf("invalid stored native import identity: %w", err)
	}
	if name := ast.ParseImportPath(id).Qualifier; name != "" && !ast.IsValidIdent(name) {
		return "", fmt.Errorf("invalid stored native package qualifier %q", name)
	}
	return id, nil
}
