package explmap

import (
	"errors"
	"strings"
	"testing"
)

// TestCheckPurposeIsTheOneLineRule pins the rule `fathom map new` shares with a
// named map (XM-7): a purpose is one non-blank line of at most PurposeMaxBytes,
// and an unnamed map that carries a purpose is held to it too, so a purpose that
// reaches a brief or a pointer line is always a line.
func TestCheckPurposeIsTheOneLineRule(t *testing.T) {
	for _, p := range []string{"", "  \t ", "two\nlines", "cr\rhere", strings.Repeat("x", PurposeMaxBytes+1)} {
		if err := CheckPurpose(p); !errors.Is(err, ErrNoPurpose) {
			t.Errorf("CheckPurpose(%q) = %v, want ErrNoPurpose", p, err)
		}
	}
	for _, p := range []string{"Which handlers reach the store?", "  padded  ", strings.Repeat("x", PurposeMaxBytes)} {
		if err := CheckPurpose(p); err != nil {
			t.Errorf("CheckPurpose(%q) = %v", p, err)
		}
	}

	// An unnamed map needs no purpose, but one it carries must be a line.
	if err := Validate(Map{ID: "run-1", Kind: KindEphemeral, Purpose: "which helper backs it?"}); err != nil {
		t.Errorf("an unnamed map with a one-line purpose was refused: %v", err)
	}
	if err := Validate(Map{ID: "run-1", Kind: KindEphemeral, Purpose: "two\nlines"}); !errors.Is(err, ErrNoPurpose) {
		t.Errorf("an unnamed map with a two-line purpose: err = %v, want ErrNoPurpose", err)
	}
	if err := Validate(Map{ID: "run-1", Kind: KindEphemeral}); err != nil {
		t.Errorf("an unnamed map without a purpose: %v", err)
	}
}
