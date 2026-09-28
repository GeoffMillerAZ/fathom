package mcp_test

// module_backend_test.go — the scripted module backend the obligations
// binding tests (binding_obligations_test.go) and the registry tests use.
// The four module MCP tools it once fed were removed by SR-17 (surface
// reckoning F-68); a server built with a module backend still federates
// bound modules' rules into obligations (docs/design/platform-modules.md
// §3.3), so the fake stays.

import (
	"context"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/modulehost"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// fakeModuleBackend is a scripted moduleBackend fake — internal/mcp's
// moduleBackend interface is unexported, but this package (mcp_test) can
// still satisfy it structurally by implementing the same three methods.
type fakeModuleBackend struct {
	modules     []modulehost.ModuleInfo
	obligations []modulehost.FederatedObligation
	obligErr    error
	queryResult []modulehost.FederatedQueryEntry
	staleness   map[string][]modulehost.ModuleStaleness
}

func (f *fakeModuleBackend) ListModules(context.Context) ([]modulehost.ModuleInfo, error) {
	return f.modules, nil
}

func (f *fakeModuleBackend) FederatedObligations(_ context.Context, module, _ string, _ int) ([]modulehost.FederatedObligation, error) {
	if f.obligErr != nil {
		return nil, f.obligErr
	}
	if module == "" {
		return f.obligations, nil
	}
	var out []modulehost.FederatedObligation
	for _, o := range f.obligations {
		if o.Module == module {
			out = append(out, o)
		}
	}
	return out, nil
}

func (f *fakeModuleBackend) FederatedQuery(_ context.Context, module, _ string, _ int) ([]modulehost.FederatedQueryEntry, error) {
	if module == "" {
		return f.queryResult, nil
	}
	var out []modulehost.FederatedQueryEntry
	for _, e := range f.queryResult {
		if e.Module == module {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeModuleBackend) ModuleStaleness(_ context.Context, module string) ([]modulehost.ModuleStaleness, bool, error) {
	report, ok := f.staleness[module]
	return report, ok, nil
}

func newModuleTestServer() (*mcp.Server, *fakeModuleBackend) {
	fb := &fakeModuleBackend{
		modules: []modulehost.ModuleInfo{
			{Module: "alpha", Path: "/mods/alpha.sqlite", Service: "svc-a", IndexedCommit: "deadbeef"},
		},
		obligations: []modulehost.FederatedObligation{
			{Module: "alpha", Service: "svc-a", Obligations: semantic.Obligations{
				Service: "svc-a", File: "x.go",
				Rules: []semantic.Obligation{{Rule: "do the thing", Src: "RULES.md#L1-L1", Tier: "doc-claimed", Layer: "obligations"}},
			}},
		},
		queryResult: []modulehost.FederatedQueryEntry{
			{Module: "alpha", Service: "svc-a", Watermark: "deadbeef", Claims: []modulehost.QueryClaim{
				{ClaimID: "claim-abc123", Text: "matched rule", Src: "RULES.md#L1-L1", Anchor: "the cited text", Tier: "doc-claimed"},
			}},
		},
		staleness: map[string][]modulehost.ModuleStaleness{
			"alpha": {{Module: "alpha", Service: "svc-a", IndexedCommit: "deadbeef", Status: modulehost.StatusUnknownDetached}},
		},
	}
	srv := mcp.NewServerWithModules(&fakeBackend{}, nil, nil, fb)
	return srv, fb
}
