package conserve

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

func TestDeclarations_ExactPriorSelfNone(t *testing.T) {
	basePlans := []verify.Plan{{Tasks: []verify.Task{
		{ID: "R-1", Retires: []string{"api:foo.Old"}},
	}}}
	headPlans := []verify.Plan{{Tasks: []verify.Task{
		{ID: "R-1", Retires: []string{"api:foo.Old"}},
		{ID: "R-2", SurfaceChange: []string{"json:foo.Bar.tag"}},
	}}}
	d := NewDeclarations(basePlans, headPlans)

	if got := d.Declared("api:foo.Old"); got != DeclaredPrior {
		t.Errorf("api:foo.Old = %q, want prior (declared at base)", got)
	}
	if got := d.Declared("json:foo.Bar.tag"); got != DeclaredSelf {
		t.Errorf("json:foo.Bar.tag = %q, want self (only at head)", got)
	}
	if got := d.Declared("api:foo.Untouched"); got != DeclaredNone {
		t.Errorf("api:foo.Untouched = %q, want none", got)
	}
}

func TestDeclarations_WildcardPrefix(t *testing.T) {
	basePlans := []verify.Plan{{Tasks: []verify.Task{
		{ID: "R-1", Retires: []string{"api:foo.Old*"}},
	}}}
	d := NewDeclarations(basePlans, nil)

	if got := d.Declared("api:foo.OldThing"); got != DeclaredPrior {
		t.Errorf("api:foo.OldThing = %q, want prior (wildcard prefix match)", got)
	}
	if got := d.Declared("api:foo.OldThing.Method"); got != DeclaredPrior {
		t.Errorf("api:foo.OldThing.Method = %q, want prior", got)
	}
	if got := d.Declared("api:foo.New"); got != DeclaredNone {
		t.Errorf("api:foo.New = %q, want none (does not share the prefix)", got)
	}
	// "<prefix>*" is an ordinary glob-shaped wildcard: * matches zero or
	// more characters, so it also matches the bare prefix itself.
	if got := d.Declared("api:foo.Old"); got != DeclaredPrior {
		t.Errorf("api:foo.Old (the bare prefix) = %q, want prior — %q matches zero-or-more trailing characters too", got, "api:foo.Old*")
	}
}

func TestDeclarations_PresentAtBothIsPrior(t *testing.T) {
	basePlans := []verify.Plan{{Tasks: []verify.Task{{ID: "R-1", Retires: []string{"api:foo.X"}}}}}
	headPlans := []verify.Plan{{Tasks: []verify.Task{{ID: "R-1", Retires: []string{"api:foo.X"}}}}}
	d := NewDeclarations(basePlans, headPlans)
	if got := d.Declared("api:foo.X"); got != DeclaredPrior {
		t.Errorf("declared at both base and head = %q, want prior (the stronger of the two)", got)
	}
}

func TestDeclarations_EmptyPlansIsAlwaysNone(t *testing.T) {
	d := NewDeclarations(nil, nil)
	if got := d.Declared("anything"); got != DeclaredNone {
		t.Errorf("empty Declarations = %q, want none", got)
	}
}
