package modserve

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/modulehost"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// fakeFederator is a deterministic ModuleFederator test double: it answers
// per-module, canned FederatedObligations results (or an error, simulating
// "not mounted") without spinning up a real module host or store.
type fakeFederator struct {
	byModule map[string][]modulehost.FederatedObligation
	errs     map[string]error
	calls    []string // records every (module) FederatedObligations was called with, in order

	// staleness (finding 2a) is keyed by module name, mirroring
	// modulehost.Host.ModuleStaleness's own per-module report shape.
	// staleErr, when non-nil, simulates a staleness-lookup failure (which
	// must never block the obligations payload itself).
	staleness map[string][]modulehost.ModuleStaleness
	staleErr  error
}

func (f *fakeFederator) FederatedObligations(ctx context.Context, moduleName, file string, max int) ([]modulehost.FederatedObligation, error) {
	f.calls = append(f.calls, moduleName)
	if err, ok := f.errs[moduleName]; ok {
		return nil, err
	}
	return f.byModule[moduleName], nil
}

func (f *fakeFederator) ModuleStaleness(ctx context.Context, moduleName string) ([]modulehost.ModuleStaleness, bool, error) {
	if f.staleErr != nil {
		return nil, false, f.staleErr
	}
	reports, ok := f.staleness[moduleName]
	if !ok {
		return nil, false, nil
	}
	return reports, true, nil
}

func writeBindingsFile(t *testing.T, root, content string) {
	t.Helper()
	dir := filepath.Join(root, ".fathom")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "modules.cue"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBind_NoBindingFile_ReturnsUnchanged(t *testing.T) {
	root := t.TempDir()
	base := semantic.Obligations{Service: "svc", File: "a.go", Rules: []semantic.Obligation{{Rule: "r1"}}}
	fed := &fakeFederator{}
	out := Bind(context.Background(), fed, root, "a.go", 6, base)
	if len(out.Rules) != 1 || out.Rules[0].Rule != "r1" {
		t.Fatalf("Rules = %+v, want unchanged", out.Rules)
	}
	if len(fed.calls) != 0 {
		t.Errorf("expected zero federator calls with no binding file, got %v", fed.calls)
	}
	if out.DisplacedModule != 0 || len(out.ModuleNotes) != 0 {
		t.Errorf("out = %+v, want no module contribution at all", out)
	}
}

func TestBind_NilHost_ReturnsUnchanged(t *testing.T) {
	root := t.TempDir()
	writeBindingsFile(t, root, `bindings: [{module: "platform-standards"}]`)
	base := semantic.Obligations{Rules: []semantic.Obligation{{Rule: "r1"}}}
	out := Bind(context.Background(), nil, root, "a.go", 6, base)
	if len(out.Rules) != 1 {
		t.Fatalf("Rules = %+v, want unchanged", out.Rules)
	}
}

func TestBind_UnionsModuleRules_TaggedWithModuleName(t *testing.T) {
	root := t.TempDir()
	writeBindingsFile(t, root, `bindings: [{module: "platform-standards"}]`)
	base := semantic.Obligations{Rules: []semantic.Obligation{{Rule: "repo-rule"}}}
	fed := &fakeFederator{byModule: map[string][]modulehost.FederatedObligation{
		"platform-standards": {{
			Module:  "platform-standards",
			Service: "standards-corpus",
			Obligations: semantic.Obligations{
				Rules: []semantic.Obligation{{Rule: "mod-rule", Tier: "doc-claimed"}},
			},
		}},
	}}
	out := Bind(context.Background(), fed, root, "a.go", 6, base)
	if len(out.Rules) != 2 {
		t.Fatalf("Rules = %+v, want repo rule + module rule", out.Rules)
	}
	if out.Rules[0].Rule != "repo-rule" || out.Rules[0].Module != "" {
		t.Errorf("repo rule should keep priority position and carry no Module tag: %+v", out.Rules[0])
	}
	if out.Rules[1].Rule != "mod-rule" || out.Rules[1].Module != "platform-standards" {
		t.Errorf("module rule should be tagged with its binding's module name: %+v", out.Rules[1])
	}
}

func TestBind_UnmountedModule_ProducesExplicitNote(t *testing.T) {
	root := t.TempDir()
	writeBindingsFile(t, root, `bindings: [{module: "platform-standards"}]`)
	base := semantic.Obligations{}
	fed := &fakeFederator{errs: map[string]error{"platform-standards": errors.New(`modulehost: module "platform-standards" is not mounted`)}}
	out := Bind(context.Background(), fed, root, "a.go", 6, base)
	if len(out.ModuleNotes) != 1 {
		t.Fatalf("ModuleNotes = %v, want exactly one note", out.ModuleNotes)
	}
	if out.ModuleNotes[0].Module != "platform-standards" {
		t.Errorf("ModuleNotes[0].Module = %q", out.ModuleNotes[0].Module)
	}
	if out.ModuleNotes[0].Note == "" {
		t.Error("note text must not be empty -- never silence")
	}
}

func TestBind_ScopeIntersection_ConsumerScopeExcludesFile(t *testing.T) {
	root := t.TempDir()
	writeBindingsFile(t, root, `bindings: [{module: "platform-standards", scopes: ["infra/**"]}]`)
	base := semantic.Obligations{}
	fed := &fakeFederator{byModule: map[string][]modulehost.FederatedObligation{
		"platform-standards": {{Obligations: semantic.Obligations{Rules: []semantic.Obligation{{Rule: "mod-rule"}}}}},
	}}
	// a.go is outside infra/** -- the binding's own scope restriction
	// excludes it before the module is ever consulted.
	out := Bind(context.Background(), fed, root, "a.go", 6, base)
	if len(out.Rules) != 0 {
		t.Fatalf("Rules = %+v, want none (scope excluded)", out.Rules)
	}
	if len(fed.calls) != 0 {
		t.Errorf("expected zero federator calls when binding scope excludes the file, got %v", fed.calls)
	}

	out2 := Bind(context.Background(), fed, root, "infra/main.tf", 6, base)
	if len(out2.Rules) != 1 {
		t.Fatalf("Rules = %+v, want the module rule for an in-scope file", out2.Rules)
	}
}

func TestBind_MinTierFloor(t *testing.T) {
	root := t.TempDir()
	writeBindingsFile(t, root, `bindings: [{module: "platform-standards", min_tier: "verified"}]`)
	base := semantic.Obligations{}
	fed := &fakeFederator{byModule: map[string][]modulehost.FederatedObligation{
		"platform-standards": {{Obligations: semantic.Obligations{Rules: []semantic.Obligation{
			{Rule: "low-tier", Tier: "doc-claimed"},
			{Rule: "high-tier", Tier: "verified"},
		}}}},
	}}
	out := Bind(context.Background(), fed, root, "a.go", 6, base)
	if len(out.Rules) != 1 || out.Rules[0].Rule != "high-tier" {
		t.Fatalf("Rules = %+v, want only the verified-tier rule", out.Rules)
	}
}

func TestBind_RepoRulesKeepPriorityWithinSharedBudget(t *testing.T) {
	root := t.TempDir()
	writeBindingsFile(t, root, `bindings: [{module: "platform-standards", max_rules: 5}]`)
	// repoObl already fills 2 of a 3-rule total budget.
	base := semantic.Obligations{Rules: []semantic.Obligation{{Rule: "r1"}, {Rule: "r2"}}}
	fed := &fakeFederator{byModule: map[string][]modulehost.FederatedObligation{
		"platform-standards": {{Obligations: semantic.Obligations{Rules: []semantic.Obligation{
			{Rule: "m1"}, {Rule: "m2"}, {Rule: "m3"},
		}}}},
	}}
	out := Bind(context.Background(), fed, root, "a.go", 3, base)
	if len(out.Rules) != 3 {
		t.Fatalf("Rules = %+v, want exactly 3 (the shared budget), not %d + module rules on top", out.Rules, len(base.Rules))
	}
	if out.Rules[2].Rule != "m1" {
		t.Errorf("Rules[2] = %+v, want the first module rule to fill the one remaining slot", out.Rules[2])
	}
	if out.DisplacedModule != 2 {
		t.Errorf("DisplacedModule = %d, want 2 (m2, m3 didn't fit the shared budget)", out.DisplacedModule)
	}
	if out.Displaced != out.DisplacedRepo+out.DisplacedModule {
		t.Errorf("Displaced = %d, want DisplacedRepo(%d)+DisplacedModule(%d)", out.Displaced, out.DisplacedRepo, out.DisplacedModule)
	}
}

func TestBind_MultipleBindings_Deterministic(t *testing.T) {
	root := t.TempDir()
	writeBindingsFile(t, root, `
bindings: [
	{module: "alpha-standards"},
	{module: "beta-standards"},
]
`)
	base := semantic.Obligations{}
	fed := &fakeFederator{byModule: map[string][]modulehost.FederatedObligation{
		"alpha-standards": {{Obligations: semantic.Obligations{Rules: []semantic.Obligation{{Rule: "a1"}}}}},
		"beta-standards":  {{Obligations: semantic.Obligations{Rules: []semantic.Obligation{{Rule: "b1"}}}}},
	}}
	out1 := Bind(context.Background(), fed, root, "a.go", 6, base)
	out2 := Bind(context.Background(), fed, root, "a.go", 6, base)
	if len(out1.Rules) != 2 || len(out2.Rules) != 2 {
		t.Fatalf("out1=%+v out2=%+v", out1.Rules, out2.Rules)
	}
	for i := range out1.Rules {
		if out1.Rules[i] != out2.Rules[i] {
			t.Errorf("non-deterministic: out1.Rules[%d]=%+v out2.Rules[%d]=%+v", i, out1.Rules[i], i, out2.Rules[i])
		}
	}
	// Binding file order: alpha before beta.
	if out1.Rules[0].Module != "alpha-standards" || out1.Rules[1].Module != "beta-standards" {
		t.Errorf("Rules = %+v, want alpha before beta (binding file order)", out1.Rules)
	}
}

// ─── finding 2a (critic, phase-O merge, BLOCKING): staleness threading ────

func TestBind_DriftedModule_FlagsRules(t *testing.T) {
	root := t.TempDir()
	writeBindingsFile(t, root, `bindings: [{module: "platform-standards"}]`)
	base := semantic.Obligations{}
	fed := &fakeFederator{
		byModule: map[string][]modulehost.FederatedObligation{
			"platform-standards": {{
				Module: "platform-standards", Service: "standards-corpus",
				Obligations: semantic.Obligations{Rules: []semantic.Obligation{{Rule: "mod-rule"}}},
			}},
		},
		staleness: map[string][]modulehost.ModuleStaleness{
			"platform-standards": {{Module: "platform-standards", Service: "standards-corpus", Status: modulehost.StatusDrifted}},
		},
	}
	out := Bind(context.Background(), fed, root, "a.go", 6, base)
	if len(out.Rules) != 1 || out.Rules[0].Status != modulehost.StatusDrifted {
		t.Fatalf("Rules = %+v, want the module rule flagged %q", out.Rules, modulehost.StatusDrifted)
	}
}

func TestBind_UnknownDetachedModule_FlagsRules(t *testing.T) {
	root := t.TempDir()
	writeBindingsFile(t, root, `bindings: [{module: "platform-standards"}]`)
	base := semantic.Obligations{}
	fed := &fakeFederator{
		byModule: map[string][]modulehost.FederatedObligation{
			"platform-standards": {{
				Module: "platform-standards", Service: "standards-corpus",
				Obligations: semantic.Obligations{Rules: []semantic.Obligation{{Rule: "mod-rule"}}},
			}},
		},
		staleness: map[string][]modulehost.ModuleStaleness{
			"platform-standards": {{Module: "platform-standards", Service: "standards-corpus", Status: modulehost.StatusUnknownDetached}},
		},
	}
	out := Bind(context.Background(), fed, root, "a.go", 6, base)
	if len(out.Rules) != 1 || out.Rules[0].Status != modulehost.StatusUnknownDetached {
		t.Fatalf("Rules = %+v, want the module rule flagged %q (stated, not hidden)", out.Rules, modulehost.StatusUnknownDetached)
	}
}

func TestBind_FreshModule_NotFlagged(t *testing.T) {
	root := t.TempDir()
	writeBindingsFile(t, root, `bindings: [{module: "platform-standards"}]`)
	base := semantic.Obligations{}
	fed := &fakeFederator{
		byModule: map[string][]modulehost.FederatedObligation{
			"platform-standards": {{
				Module: "platform-standards", Service: "standards-corpus",
				Obligations: semantic.Obligations{Rules: []semantic.Obligation{{Rule: "mod-rule"}}},
			}},
		},
		staleness: map[string][]modulehost.ModuleStaleness{
			"platform-standards": {{Module: "platform-standards", Service: "standards-corpus", Status: modulehost.StatusFresh}},
		},
	}
	out := Bind(context.Background(), fed, root, "a.go", 6, base)
	if len(out.Rules) != 1 || out.Rules[0].Status != "" {
		t.Fatalf("Rules = %+v, want the module rule UNFLAGGED (fresh)", out.Rules)
	}
}

// TestBind_StalenessNeverClobbersExistingStatus proves the module-staleness
// flag only FILLS an empty Status — it must never overwrite a status the
// module's own store already computed (e.g. a contradicted claim), which
// would silently downgrade a stronger, more specific drift signal to a
// weaker corpus-level one.
func TestBind_StalenessNeverClobbersExistingStatus(t *testing.T) {
	root := t.TempDir()
	writeBindingsFile(t, root, `bindings: [{module: "platform-standards"}]`)
	base := semantic.Obligations{}
	fed := &fakeFederator{
		byModule: map[string][]modulehost.FederatedObligation{
			"platform-standards": {{
				Module: "platform-standards", Service: "standards-corpus",
				Obligations: semantic.Obligations{Rules: []semantic.Obligation{{Rule: "mod-rule", Status: "contradicted"}}},
			}},
		},
		staleness: map[string][]modulehost.ModuleStaleness{
			"platform-standards": {{Module: "platform-standards", Service: "standards-corpus", Status: modulehost.StatusDrifted}},
		},
	}
	out := Bind(context.Background(), fed, root, "a.go", 6, base)
	if len(out.Rules) != 1 || out.Rules[0].Status != "contradicted" {
		t.Fatalf("Rules = %+v, want the module store's own \"contradicted\" status preserved, not overwritten", out.Rules)
	}
}

// TestBind_StalenessLookupFailure_FailsOpen proves a staleness-lookup
// failure never blocks (or errors) the obligations payload itself — the
// rule is served unflagged rather than the whole call degrading.
func TestBind_StalenessLookupFailure_FailsOpen(t *testing.T) {
	root := t.TempDir()
	writeBindingsFile(t, root, `bindings: [{module: "platform-standards"}]`)
	base := semantic.Obligations{}
	fed := &fakeFederator{
		byModule: map[string][]modulehost.FederatedObligation{
			"platform-standards": {{
				Module: "platform-standards", Service: "standards-corpus",
				Obligations: semantic.Obligations{Rules: []semantic.Obligation{{Rule: "mod-rule"}}},
			}},
		},
		staleErr: errors.New("staleness lookup exploded"),
	}
	out := Bind(context.Background(), fed, root, "a.go", 6, base)
	if len(out.Rules) != 1 || out.Rules[0].Status != "" {
		t.Fatalf("Rules = %+v, want the module rule served unflagged despite the staleness-lookup failure", out.Rules)
	}
}

func TestBind_MalformedBindingFile_FailsOpen(t *testing.T) {
	root := t.TempDir()
	writeBindingsFile(t, root, `bindings: [{module: "Not_Valid"}]`) // fails LoadBindings
	base := semantic.Obligations{Rules: []semantic.Obligation{{Rule: "r1"}}}
	fed := &fakeFederator{}
	out := Bind(context.Background(), fed, root, "a.go", 6, base)
	if len(out.Rules) != 1 {
		t.Fatalf("Rules = %+v, want repoObl unchanged on a malformed binding file", out.Rules)
	}
	if len(fed.calls) != 0 {
		t.Errorf("expected zero federator calls on a malformed binding file, got %v", fed.calls)
	}
}
