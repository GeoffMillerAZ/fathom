package records

import (
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/recordids"
)

func fixtureLane() Lane {
	return Lane{Kind: "lane", ID: "demo", Goal: "test allocation", Class: "L-P", BaseCommit: strings.Repeat("a", 40), Rows: []string{"REC-1"}, OwnedPaths: []string{"ADR.md", "invariants.md", "futures.md", "incidents.md", "observations.md", "docs/plans/demo.md"}, ReadOnlyPaths: []string{}, ContractFiles: []string{"contract.md"}, ProofCommand: "test", Budget: "fixture", HarnessClass: "test", IDRanges: []IDRange{
		{Family: "adr", Prefix: "ADR-", First: 10000, Last: 10001}, {Family: "invariant", Prefix: "I", First: 10000, Last: 10001}, {Family: "future", Prefix: "F", First: 10000, Last: 10001}, {Family: "incident", Prefix: "INC-", First: 10000, Last: 10001}, {Family: "observation", Prefix: "O", First: 10000, Last: 10001}, {Family: "plan", Prefix: "DEMO-", First: 10000, Last: 10001}}}
}
func validated(t *testing.T, l Lane, defs []recordids.Definition, cfg recordids.Config) recordids.ValidatedInventory {
	t.Helper()
	trust := recordids.TrustedInventory{Revision: strings.Repeat("a", 40), Lanes: []recordids.LaneGrant{{ID: l.ID, OwnedPaths: l.OwnedPaths, IDRanges: l.IDRanges}}, Legacy: []recordids.Definition{}}
	// Grandfathered legacy definitions retain their original paths.
	for _, d := range defs {
		if strings.HasPrefix(d.ID, "PAIR-") {
			trust.Legacy = append(trust.Legacy, d)
		}
	}
	v, e := recordids.ValidateInventory(defs, cfg, trust)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func TestLaneLaneRecords(t *testing.T) {
	l := fixtureLane()
	cfg := recordids.Defaults()
	cfg.CrossPlan = true
	t.Run("shape", func(t *testing.T) {
		raw, _ := json.Marshal(l)
		var fields map[string]any
		_ = json.Unmarshal(raw, &fields)
		fields["phase"] = "active"
		raw, _ = json.Marshal(fields)
		if _, e := ParseLaneJSON(raw); e == nil {
			t.Fatal("unknown phase accepted")
		}
		for _, mutate := range []func(*Lane){func(x *Lane) { x.Kind = "other" }, func(x *Lane) { x.OwnedPaths = []string{"../escape"} }, func(x *Lane) { x.IDRanges[0].Last = 9999 }, func(x *Lane) { x.IDRanges = append(x.IDRanges, x.IDRanges[0]) }, func(x *Lane) { x.Class = "unknown" }, func(x *Lane) { x.Budget = "" }} {
			x := fixtureLane()
			mutate(&x)
			if e := x.Validate(); e == nil {
				t.Fatalf("invalid lane accepted: %+v", x)
			}
		}
	})
	for _, tc := range []struct {
		name   string
		family recordids.FamilyName
		id     string
	}{{"duplicate-key", "adr", "ADR-10000"}, {"duplicate-observation", "observation", "O10000"}} {
		t.Run(tc.name, func(t *testing.T) {
			defs := []recordids.Definition{{Family: tc.family, ID: tc.id, At: recordids.Location{Path: "a.md", Line: 1}}, {Family: tc.family, ID: tc.id, At: recordids.Location{Path: "b.md", Line: 2}}}
			trust := recordids.TrustedInventory{Revision: l.BaseCommit, Lanes: []recordids.LaneGrant{{ID: l.ID, OwnedPaths: l.OwnedPaths, IDRanges: l.IDRanges}}}
			_, e := recordids.ValidateInventory(defs, cfg, trust)
			if e == nil || !strings.Contains(e.Error(), "a.md:1") || !strings.Contains(e.Error(), "b.md:2") {
				t.Fatalf("duplicate not diagnosed: %v", e)
			}
			slices.Reverse(defs)
			_, e2 := recordids.ValidateInventory(defs, cfg, trust)
			if e2 == nil || e.Error() != e2.Error() {
				t.Fatalf("unstable errors %v / %v", e, e2)
			}
		})
	}
	t.Run("repeat", func(t *testing.T) {
		v := validated(t, l, nil, cfg)
		id, e := AllocateRecordID(l, "adr", "ADR.md", v)
		if e != nil || id != "ADR-10000" {
			t.Fatalf("%q %v", id, e)
		}
		again, _ := AllocateRecordID(l, "adr", "ADR.md", v)
		if again != id {
			t.Fatal("pure call reserved an id")
		}
		defs := append(v.Definitions(), recordids.Definition{Family: "adr", ID: id, At: recordids.Location{Path: "ADR.md", Line: 1}})
		v = validated(t, l, defs, cfg)
		id, e = AllocateRecordID(l, "adr", "ADR.md", v)
		if e != nil || id != "ADR-10001" {
			t.Fatalf("%q %v", id, e)
		}
		copyDefs := v.Definitions()
		copyDefs[0].ID = "changed"
		if v.Definitions()[0].ID == "changed" {
			t.Fatal("inventory alias")
		}
	})
	t.Run("exhausted-range", func(t *testing.T) {
		defs := []recordids.Definition{{Family: "adr", ID: "ADR-10000", At: recordids.Location{Path: "ADR.md", Line: 1}}, {Family: "adr", ID: "ADR-10001", At: recordids.Location{Path: "ADR.md", Line: 2}}}
		id, e := AllocateRecordID(l, "adr", "ADR.md", validated(t, l, defs, cfg))
		var refusal *recordids.RefusalError
		if id != "" || !errors.As(e, &refusal) || refusal.Code != recordids.ErrExhausted {
			t.Fatalf("%q %v", id, e)
		}
	})
	t.Run("outside-path", func(t *testing.T) {
		v := validated(t, l, nil, cfg)
		before := v.Definitions()
		for _, path := range []string{"elsewhere.md", "../ADR.md", "/ADR.md", "a/../ADR.md", "./ADR.md"} {
			id, e := AllocateRecordID(l, "adr", path, v)
			if id != "" || e == nil {
				t.Fatalf("%q: %q %v", path, id, e)
			}
		}
		if !reflect.DeepEqual(before, v.Definitions()) {
			t.Fatal("mutated inventory")
		}
	})
	t.Run("families", func(t *testing.T) {
		v := validated(t, l, nil, cfg)
		for i, want := range []string{"ADR-10000", "I10000", "F10000", "INC-10000", "O10000", "DEMO-10000"} {
			id, e := AllocateRecordID(l, string(l.IDRanges[i].Family), l.OwnedPaths[i], v)
			if e != nil || id != want {
				t.Fatalf("%q %v", id, e)
			}
		}
	})
	t.Run("reordered", func(t *testing.T) {
		defs := []recordids.Definition{{Family: "adr", ID: "ADR-10000", At: recordids.Location{Path: "ADR.md", Line: 1}}, {Family: "future", ID: "F10000", At: recordids.Location{Path: "futures.md", Line: 1}}}
		a, e := AllocateRecordID(l, "adr", "ADR.md", validated(t, l, defs, cfg))
		slices.Reverse(defs)
		b, f := AllocateRecordID(l, "adr", "ADR.md", validated(t, l, defs, cfg))
		if a != "ADR-10001" || a != b || e != nil || f != nil {
			t.Fatalf("%q %q %v %v", a, b, e, f)
		}
	})
	for _, name := range []string{"grandfather-pair", "grandfather-third-path", "grandfather-local-duplicate", "grandfather-reuse", "stale-grandfather"} {
		t.Run(name, func(t *testing.T) {
			x := fixtureLane()
			x.IDRanges[5] = IDRange{Family: "plan", Prefix: "PAIR-", First: 10000, Last: 10001}
			defs := []recordids.Definition{{Family: "plan", ID: "PAIR-10000", At: recordids.Location{Path: "docs/plans/a.md", Line: 1}}, {Family: "plan", ID: "PAIR-10000", At: recordids.Location{Path: "docs/plans/b.md", Line: 1}}}
			policy := recordids.Defaults()
			policy.CrossPlan = true
			policy.Grandfather = []recordids.Exception{{ID: "PAIR-10000", Paths: []string{"docs/plans/a.md", "docs/plans/b.md"}}}
			trust := recordids.TrustedInventory{Revision: l.BaseCommit, Lanes: []recordids.LaneGrant{{ID: x.ID, OwnedPaths: x.OwnedPaths, IDRanges: x.IDRanges}}, Legacy: slices.Clone(defs)}
			switch name {
			case "grandfather-third-path":
				defs = append(defs, recordids.Definition{Family: "plan", ID: "PAIR-10000", At: recordids.Location{Path: "docs/plans/c.md", Line: 1}})
			case "grandfather-local-duplicate":
				d := defs[0]
				d.At.Line = 2
				defs = append(defs, d)
			case "stale-grandfather":
				defs = defs[:1]
			}
			v, e := recordids.ValidateInventory(defs, policy, trust)
			if name == "grandfather-third-path" || name == "grandfather-local-duplicate" || name == "stale-grandfather" {
				if e == nil {
					t.Fatal("bad grandfather policy accepted")
				}
				if name == "stale-grandfather" && !strings.Contains(e.Error(), "stale exception") {
					t.Fatal(e)
				}
				return
			}
			if e != nil || len(v.Definitions()) != 2 {
				t.Fatalf("lost valid pair: %v", e)
			}
			family, target, want := "adr", "ADR.md", "ADR-10000"
			if name == "grandfather-reuse" {
				family, target, want = "plan", "docs/plans/demo.md", "PAIR-10001"
			}
			id, e := AllocateRecordID(x, family, target, v)
			if id != want || e != nil {
				t.Fatalf("%q %v", id, e)
			}
		})
	}
	t.Run("ambiguous-family", func(t *testing.T) {
		x := fixtureLane()
		x.IDRanges = append(x.IDRanges, IDRange{Family: "adr", Prefix: "ADR-", First: 10002, Last: 10003})
		v := validated(t, x, nil, cfg)
		id, e := AllocateRecordID(x, "adr", "ADR.md", v)
		if id != "" || e == nil {
			t.Fatal("ambiguous family accepted")
		}
	})
	t.Run("readonly-is-not-owned", func(t *testing.T) {
		x := fixtureLane()
		x.ReadOnlyPaths = []string{"readonly.md"}
		id, e := AllocateRecordID(x, "adr", "readonly.md", validated(t, x, nil, cfg))
		if id != "" || e == nil {
			t.Fatal("read-only path grants writing")
		}
	})
	t.Run("adr-padding", func(t *testing.T) {
		x := fixtureLane()
		x.IDRanges[0] = IDRange{Family: "adr", Prefix: "ADR-", First: 1, Last: 2}
		id, e := AllocateRecordID(x, "adr", "ADR.md", validated(t, x, nil, cfg))
		if id != "ADR-001" || e != nil {
			t.Fatalf("%q %v", id, e)
		}
	})

	t.Run("invalid-inventory", func(t *testing.T) {
		id, e := AllocateRecordID(l, "adr", "ADR.md", recordids.ValidatedInventory{})
		if id != "" || e == nil {
			t.Fatal("zero inventory accepted")
		}
	})
	t.Run("grant-tampering", func(t *testing.T) {
		v := validated(t, l, nil, cfg)
		x := fixtureLane()
		x.IDRanges[0].First = 9000
		id, e := AllocateRecordID(x, "adr", "ADR.md", v)
		if id != "" || e == nil {
			t.Fatal("untrusted interval accepted")
		}
	})
	t.Run("absent-family", func(t *testing.T) {
		id, e := AllocateRecordID(l, "unknown", "ADR.md", validated(t, l, nil, cfg))
		if id != "" || e == nil {
			t.Fatal("unknown family accepted")
		}
	})
	t.Run("json", func(t *testing.T) {
		v := validated(t, l, nil, cfg)
		b, _ := json.Marshal(v.Definitions())
		if string(b) != "[]" {
			t.Fatalf("%s", b)
		}
	})
}
