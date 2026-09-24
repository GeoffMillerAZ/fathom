package conserve

import (
	"context"
	"testing"
)

// registerFakePlane is a Plane used only to test Register/Planes, kept
// distinct from conserve_test.go's fakePlane so the two test files can
// each own their own zero-value-friendly type without colliding.
type registerFakePlane struct{ name string }

func (p registerFakePlane) Name() string                                     { return p.name }
func (registerFakePlane) Check(context.Context, Input) ([]Finding, []string) { return nil, nil }

func TestRegister_PlanesReturnsSortedByName(t *testing.T) {
	saved := registry
	registry = nil
	defer func() { registry = saved }()

	Register(registerFakePlane{name: "zeta"})
	Register(registerFakePlane{name: "alpha"})
	Register(registerFakePlane{name: "mu"})

	got := Planes()
	if len(got) != 3 {
		t.Fatalf("Planes() returned %d, want 3", len(got))
	}
	want := []string{"alpha", "mu", "zeta"}
	for i, w := range want {
		if got[i].Name() != w {
			t.Errorf("Planes()[%d].Name() = %q, want %q", i, got[i].Name(), w)
		}
	}
}

func TestPlanes_ReturnsACopyNeverTheLiveRegistry(t *testing.T) {
	saved := registry
	registry = nil
	defer func() { registry = saved }()

	Register(registerFakePlane{name: "one"})
	got := Planes()
	got[0] = registerFakePlane{name: "mutated"}

	got2 := Planes()
	if got2[0].Name() != "one" {
		t.Errorf("mutating a Planes() result affected the registry: %q", got2[0].Name())
	}
}
