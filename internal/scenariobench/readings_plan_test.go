package scenariobench

import (
	"reflect"
	"testing"
)

func TestPlanCoverage_V2Grammar(t *testing.T) {
	plan := []byte(`# Plan
- [ ] Do the thing {id: T1, symbol: Foo, package: internal/pkg}
- [x] Do the other thing {id: T2, symbol: Qux, package: internal/pkg}
`)
	gold := Gold{Functions: []string{"Foo", "Type.Bar"}}
	got := PlanCoverage(plan, gold)
	if got.GoldFunctions != 2 || got.Named != 1 {
		t.Fatalf("got %+v", got)
	}
	if got.Fraction != 0.5 {
		t.Fatalf("fraction = %v, want 0.5", got.Fraction)
	}
	if !reflect.DeepEqual(got.Spurious, []string{"Qux"}) {
		t.Fatalf("spurious = %v", got.Spurious)
	}
	if got.NotMeasurable != "" {
		t.Fatalf("unexpected not_measurable: %s", got.NotMeasurable)
	}
}

func TestPlanCoverage_BacktickChecklist(t *testing.T) {
	plan := []byte(`# Plan
- [ ] Implement ` + "`Foo`" + ` next to ` + "`Type.Bar`" + `
- [ ] Also touch ` + "`Unrelated`" + `
`)
	gold := Gold{Functions: []string{"Foo", "Type.Bar"}}
	got := PlanCoverage(plan, gold)
	if got.Named != 2 {
		t.Fatalf("named = %d, want 2 (%+v)", got.Named, got)
	}
	if got.Fraction != 1.0 {
		t.Fatalf("fraction = %v, want 1.0", got.Fraction)
	}
	if !reflect.DeepEqual(got.Spurious, []string{"Unrelated"}) {
		t.Fatalf("spurious = %v", got.Spurious)
	}
}

func TestPlanCoverage_AnnotationWinsOverBackticks(t *testing.T) {
	// A v2-grammar row whose derivation prose backticks an unrelated name
	// must not count that name as spurious: the annotation, not the
	// backtick, is the row's named symbol.
	plan := []byte("- [ ] mirrors `SomeOtherThing` {symbol: Foo, package: internal/pkg}\n")
	gold := Gold{Functions: []string{"Foo"}}
	got := PlanCoverage(plan, gold)
	if got.Named != 1 || len(got.Spurious) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestPlanCoverage_NoRows(t *testing.T) {
	got := PlanCoverage([]byte("just prose, no checklist"), Gold{Functions: []string{"Foo"}})
	if got.NotMeasurable != "no checklist rows found in plan" {
		t.Fatalf("not_measurable = %q", got.NotMeasurable)
	}
}

func TestPlanCoverage_NoGoldFunctions(t *testing.T) {
	got := PlanCoverage([]byte("- [ ] `Foo`\n"), Gold{})
	if got.NotMeasurable != "gold has no functions" {
		t.Fatalf("not_measurable = %q", got.NotMeasurable)
	}
}
