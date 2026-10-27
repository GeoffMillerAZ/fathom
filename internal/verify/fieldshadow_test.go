package verify_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// Go struct fields are symbols (GoField, named Owner.Field), so a bare
// `symbol:` contract also suffix-matches every same-named field. A field
// never shadows the func, method or type the row names: with one left, the
// fields drop out of the match set and reachability is asked about the one
// remaining canonical key, not the ambiguous bare name (CR-1, DP-3 and CV-61
// failed on exactly this once fields were indexed).
func TestVerifyPlan_GoFieldNeverShadowsCallable(t *testing.T) {
	fn := core.SymbolNode{Service: "svc", Package: "capability", Name: "Default", Kind: core.KindGoFunc, FilePath: "internal/capability/registry.go", Line: 40, Tier: core.TierVerified}
	field := core.SymbolNode{Service: "svc", Package: "Capability", Name: "Capability.Default", Kind: core.KindGoField, FilePath: "internal/capability/capability.go", Line: 154, Tier: core.TierVerified}
	other := core.SymbolNode{Service: "svc", Package: "Options", Name: "Options.Default", Kind: core.KindGoField, FilePath: "internal/config/options.go", Line: 12, Tier: core.TierVerified}
	method := core.SymbolNode{Service: "svc", Package: "Recorder", Name: "Recorder.Record", Kind: core.KindGoMethod, FilePath: "internal/decide/record.go", Line: 216, Tier: core.TierVerified}
	recField := core.SymbolNode{Service: "svc", Package: "DecidedBy", Name: "DecidedBy.Record", Kind: core.KindGoField, FilePath: "internal/decide/decidedby.go", Line: 54, Tier: core.TierVerified}
	typ := core.SymbolNode{Service: "svc", Package: "conserve", Name: "Residue", Kind: core.KindGoType, FilePath: "internal/conserve/types.go", Line: 9, Tier: core.TierVerified}
	resField := core.SymbolNode{Service: "svc", Package: "Report", Name: "Report.Residue", Kind: core.KindGoField, FilePath: "internal/conserve/conserve.go", Line: 195, Tier: core.TierVerified}
	const test = "svc::internal/capability.TestRegistryEveryEntryExplainsItsAbsence"
	ev := fakeEvidence{
		// FindSymbol sorts by Name, so the field (Capability.Default) is
		// first: the shape the real index answers.
		symbols: map[string][]core.SymbolNode{
			"Default":            {field, fn, other},
			"Record":             {recField, method},
			"Residue":            {resField, typ},
			"Capability.Default": {field},
		},
		// Only canonical keys reach: the bare name resolves to nothing in
		// a graph where several definitions share it.
		tests: map[string][]string{
			fn.Key():     {test},
			method.Key(): {test},
			typ.Key():    {test},
			field.Key():  {test},
		},
		reach: map[string]bool{
			"TestRegistryEveryEntryExplainsItsAbsence->" + fn.Key():     true,
			"TestRegistryEveryEntryExplainsItsAbsence->" + method.Key(): true,
			"TestRegistryEveryEntryExplainsItsAbsence->" + typ.Key():    true,
		},
	}
	const from = "TestRegistryEveryEntryExplainsItsAbsence"
	cases := []struct {
		name string
		e    verify.Expect
		file string
	}{
		{"func beside a same-named field, package filter", verify.Expect{Symbol: "Default", Package: "internal/capability", ReachableFrom: from}, fn.FilePath},
		{"func beside fields in two packages, bare", verify.Expect{Symbol: "Default", ReachableFrom: from}, fn.FilePath},
		{"method beside a same-named field", verify.Expect{Symbol: "Record", Package: "internal/decide", ReachableFrom: from}, method.FilePath},
		{"type beside a same-named field", verify.Expect{Symbol: "Residue", Package: "internal/conserve", ReachableFrom: from}, typ.FilePath},
		{"owner-qualified field is still the field", verify.Expect{Symbol: "Capability.Default", Package: "internal/capability"}, field.FilePath},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := verify.Plan{Service: "svc", Tasks: []verify.Task{{ID: "CR-1", Title: "registry", Claimed: true, Expect: tc.e}}}
			v := verify.VerifyPlan(plan, ev).Tasks[0]
			if v.Status != verify.Pass {
				t.Fatalf("want Pass, got %s (%s): %+v", v.Status, v.Label, v.Criteria)
			}
			if v.EvidenceFile != tc.file {
				t.Fatalf("symbol_present cited %q, want %q", v.EvidenceFile, tc.file)
			}
			if c, ok := findCrit(v, "test_reaches"); !ok || c.Status != verify.Pass {
				t.Fatalf("test_reaches must pass through the canonical key, got %+v", c)
			}
			if tc.e.ReachableFrom != "" {
				if c, ok := findCrit(v, "edge_wired"); !ok || c.Status != verify.Pass {
					t.Fatalf("edge_wired must pass through the canonical key, got %+v", c)
				}
			}
		})
	}
}
