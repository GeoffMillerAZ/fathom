package mcp

// references_scope.go — which services `references` searches (RT-86,
// RT-159; docs/design/responsible-tenant.md §4.5). The default is the
// caller's own service when its identity resolves; `services: ["*"]` is
// the fleet (fleet cost class, costClassOf); with neither, the call is
// refused by name (-32602) listing what is indexed — never a silent fleet
// scan and never an empty result.

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// referencesScanBytes and referencesScanWall bound one references call's
// reads across every service it searches (§4.5): the text scan and the
// interface files a bare query reads. Variables only so a test can shrink
// them.
var (
	referencesScanBytes int64 = 64 << 20
	referencesScanWall        = 5 * time.Second
)

// Scope labels on the response.
const (
	referencesScopeFleet    = "fleet"
	referencesScopeServices = "services"
	referencesScopeCaller   = "caller-service"
)

// referencesScope resolves requested (the call's `services`) to the
// sorted services to search and the scope label. An empty request resolves
// to the caller's own service (callerService); when none resolves it is a
// -32602 naming the indexed services and the ["*"] opt-in.
func referencesScope(ctx context.Context, backend QueryInterface, requested []string) ([]string, string, *rpcError) {
	for _, s := range requested {
		if s == "*" {
			all, err := backend.Services(ctx)
			if err != nil {
				return nil, "", &rpcError{Code: ErrCodeInternal, Message: "references: list services: " + err.Error()}
			}
			out := append([]string(nil), all...)
			sort.Strings(out)
			return out, referencesScopeFleet, nil
		}
	}
	if len(requested) > 0 {
		out := append([]string(nil), requested...)
		sort.Strings(out)
		return out, referencesScopeServices, nil
	}
	all, err := backend.Services(ctx)
	if err != nil {
		return nil, "", &rpcError{Code: ErrCodeInternal, Message: "references: list services: " + err.Error()}
	}
	if svc := callerService(ctx, backend, all); svc != "" {
		return []string{svc}, referencesScopeCaller, nil
	}
	sorted := append([]string(nil), all...)
	sort.Strings(sorted)
	indexed := strings.Join(sorted, ", ")
	if indexed == "" {
		indexed = "(none)"
	}
	return nil, "", &rpcError{
		Code:    ErrCodeParams,
		Message: "references: name the services to search — indexed: " + indexed + `; or pass services: ["*"] for the fleet`,
		Data:    map[string]any{"indexed_services": sorted, "fleet_opt_in": []string{"*"}},
	}
}

// callerService is the indexed service whose root holds the caller's
// working directory (X-Fathom-Caller cwd=, or serve-mcp's own), the
// deepest root winning; "" when the caller sent no cwd or no root holds it.
func callerService(ctx context.Context, backend QueryInterface, services []string) string {
	c, ok := callerFrom(ctx)
	if !ok || c.Cwd == "" || !filepath.IsAbs(c.Cwd) {
		return ""
	}
	cwds := []string{filepath.Clean(c.Cwd)}
	if real, err := filepath.EvalSymlinks(c.Cwd); err == nil && real != cwds[0] {
		cwds = append(cwds, real)
	}
	best, bestLen := "", -1
	for _, svc := range services {
		root, _, found, err := backend.ServiceMeta(ctx, svc)
		if err != nil || !found || root == "" || !filepath.IsAbs(root) {
			continue
		}
		roots := []string{filepath.Clean(root)}
		if real, err := filepath.EvalSymlinks(root); err == nil && real != roots[0] {
			roots = append(roots, real)
		}
		for _, r := range roots {
			for _, cwd := range cwds {
				if (cwd == r || strings.HasPrefix(cwd, r+string(filepath.Separator))) && len(r) > bestLen {
					best, bestLen = svc, len(r)
				}
			}
		}
	}
	return best
}
