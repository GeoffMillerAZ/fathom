package explmap

import (
	"errors"
	"fmt"
	"strings"
)

// PurposeMaxBytes bounds a purpose line: it is one line naming the question a
// map answers, the first thing a reader of a pointer or a brief sees, not a
// description (fathom computes the rest, XM-15).
const PurposeMaxBytes = 200

// ErrNoPurpose is a named map without a usable purpose line.
var ErrNoPurpose = errors.New("a named map needs a purpose line: the question it answers")

// Validate checks a map before it is stored or served: a well-formed id, the
// composite prefix reserved for minted composites, a known kind, a composite
// made only of references, and a named map's purpose line (XM-7).
func Validate(m Map) error {
	if !ValidID(m.ID) {
		return fmt.Errorf("map id %q: want a letter or digit, then up to 63 letters, digits, '.', '_' or '-'", m.ID)
	}
	switch m.Kind {
	case KindComposite:
		if !strings.HasPrefix(m.ID, CompositePrefix) {
			return fmt.Errorf("composite %q: a composite's id is minted with the %q prefix", m.ID, CompositePrefix)
		}
		if len(m.Entries) > 0 {
			return fmt.Errorf("composite %q carries %d entries: a composite is made only of references", m.ID, len(m.Entries))
		}
		if len(m.Includes) == 0 {
			return fmt.Errorf("composite %q references nothing", m.ID)
		}
	case KindEphemeral, KindLive:
		if strings.HasPrefix(m.ID, CompositePrefix) {
			return fmt.Errorf("map id %q: the %q prefix is reserved for minted composites", m.ID, CompositePrefix)
		}
	default:
		return fmt.Errorf("map %q: unknown kind %q", m.ID, m.Kind)
	}
	return validateMapPurpose(m)
}

// validateMapPurpose refuses a named map without a purpose line (XM-7): the
// purpose is the question the map answers, the one thing about a map fathom
// cannot compute, so a name without it would advertise a map nobody can judge
// the relevance of. It must be one non-blank line of at most PurposeMaxBytes.
// An unnamed map is a run's scratch and needs none, but a purpose it does carry
// is held to the same one-line rule, so what a brief or a pointer line quotes is
// always a line.
func validateMapPurpose(m Map) error {
	if !m.Named() && strings.TrimSpace(m.Purpose) == "" {
		return nil
	}
	if err := CheckPurpose(m.Purpose); err != nil {
		if m.Named() {
			return fmt.Errorf("map %q (named %q): %w", m.ID, m.Name, err)
		}
		return fmt.Errorf("map %q: %w", m.ID, err)
	}
	return nil
}

// CheckPurpose is the one-line rule for a purpose line: non-blank, one line, at
// most PurposeMaxBytes. The error wraps ErrNoPurpose and says which part failed.
func CheckPurpose(purpose string) error {
	p := strings.TrimSpace(purpose)
	switch {
	case p == "":
		return ErrNoPurpose
	case strings.ContainsAny(p, "\r\n"):
		return fmt.Errorf("the purpose is one line: %w", ErrNoPurpose)
	case len(p) > PurposeMaxBytes:
		return fmt.Errorf("the purpose is %d bytes, over the %d-byte line: %w", len(p), PurposeMaxBytes, ErrNoPurpose)
	}
	return nil
}
