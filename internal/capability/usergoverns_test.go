package capability

import (
	"errors"
	"testing"
)

// TestUserGovernsCapability pins ADR-083's inversion of law 13 for the
// egress guard: the user decides; a repository may switch it on, never off.
func TestUserGovernsCapability(t *testing.T) {
	r := Default()
	c, ok := r.Get("egress-guard")
	if !ok || !c.Switchable || !c.Default || c.Scopes != ScopeUserGoverns {
		t.Fatalf("egress-guard = %+v, want a switchable, default-on, user-governed capability", c)
	}
	for _, tc := range []struct {
		name string
		res  Resolution
		on   bool
		from Scope
	}{
		{"built-in", Resolution{}, true, ScopeBuiltIn},
		{"user off", Resolution{User: Settings{c.ID: false}}, false, ScopeUser},
		{"repository off never loosens the user", Resolution{User: Settings{c.ID: true}, Project: Settings{c.ID: false}}, true, ScopeUser},
		{"repository off never loosens the default", Resolution{Project: Settings{c.ID: false}}, true, ScopeBuiltIn},
		{"repository on tightens the user's off", Resolution{User: Settings{c.ID: false}, Project: Settings{c.ID: true}}, true, ScopeProject},
		{"config off", Resolution{Config: Settings{c.ID: false}}, false, ScopeConfig},
	} {
		if got := tc.res.For(c); got.On != tc.on || got.Scope != tc.from {
			t.Errorf("%s: %+v, want on=%v from %s", tc.name, got, tc.on, tc.from)
		}
	}
	errs := r.ValidateSettings(Settings{c.ID: false}, ScopeProject)
	var le *LoosenError
	if len(errs) != 1 || !errors.As(errs[0], &le) || le.ID != c.ID {
		t.Errorf("a repository switching egress-guard off: %v, want one LoosenError naming it", errs)
	}
	if errs := r.ValidateSettings(Settings{c.ID: true}, ScopeProject); len(errs) != 0 {
		t.Errorf("a repository switching egress-guard on: %v, want no error", errs)
	}
	if errs := r.ValidateSettings(Settings{c.ID: false}, ScopeUser); len(errs) != 0 {
		t.Errorf("the user switching egress-guard off: %v, want no error", errs)
	}
}
