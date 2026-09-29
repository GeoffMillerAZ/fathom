package explmap

import (
	"errors"
	"strings"
	"testing"
)

// TestNamedMapWithoutPurposeRefused pins XM-7: a named map must carry its
// purpose line, the question it answers; an unnamed ephemeral map need not.
func TestNamedMapWithoutPurposeRefused(t *testing.T) {
	refused := []struct {
		name, purpose string
	}{
		{"auth-surface", ""},
		{"auth-surface", "   "},
		{"auth-surface", "which handlers check auth?\nand which do not"},
		{"auth-surface", strings.Repeat("which handler checks auth ", 12)},
	}
	for _, c := range refused {
		m := Map{ID: "run-1", Kind: KindEphemeral, Name: c.name, Purpose: c.purpose}
		err := Validate(m)
		if !errors.Is(err, ErrNoPurpose) {
			t.Errorf("named %q with purpose %q: err = %v, want ErrNoPurpose", c.name, c.purpose, err)
		}
		if err := validateMapPurpose(m); err == nil {
			t.Errorf("validateMapPurpose accepted purpose %q", c.purpose)
		}
	}

	ok := Map{ID: "run-1", Kind: KindEphemeral, Name: "auth-surface", Purpose: "Which handlers reach the session store without the auth middleware?"}
	if err := Validate(ok); err != nil {
		t.Errorf("a named map with a purpose line was refused: %v", err)
	}
	if err := Validate(Map{ID: "run-2", Kind: KindEphemeral}); err != nil {
		t.Errorf("an unnamed ephemeral map needs no purpose, got %v", err)
	}

	// The structural rules Validate carries beside the purpose.
	for _, bad := range []Map{
		{ID: "has space", Kind: KindEphemeral},
		{ID: CompositePrefix + "0123456789", Kind: KindEphemeral},
		{ID: CompositePrefix + "0123456789", Kind: KindComposite},
		{ID: "run-3", Kind: "weird"},
	} {
		if err := Validate(bad); err == nil {
			t.Errorf("Validate accepted %+v", bad)
		}
	}
}
