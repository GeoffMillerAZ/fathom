package anchor

import "fmt"

// validStatuses is the set of recognised AnchorStatus values.
var validStatuses = map[AnchorStatus]struct{}{
	StatusPending:   {},
	StatusConfirmed: {},
}

// Validate returns an error if any invariant of the Anchor is violated.
//
// Checked invariants:
//   - Root must be non-empty.
//   - LeafCount must be >= 1.
//   - CreatedAt must be non-zero.
//   - Authority must be non-empty.
//   - Status must be StatusPending or StatusConfirmed.
func (a Anchor) Validate() error {
	if a.Root == "" {
		return fmt.Errorf("anchor: Anchor.Root must not be empty")
	}
	if a.LeafCount < 1 {
		return fmt.Errorf("anchor: Anchor.LeafCount must be >= 1, got %d", a.LeafCount)
	}
	if a.CreatedAt.IsZero() {
		return fmt.Errorf("anchor: Anchor.CreatedAt must not be zero")
	}
	if a.Authority == "" {
		return fmt.Errorf("anchor: Anchor.Authority must not be empty")
	}
	if _, ok := validStatuses[a.Status]; !ok {
		return fmt.Errorf("anchor: Anchor.Status %q is not valid; must be %q or %q",
			a.Status, StatusPending, StatusConfirmed)
	}
	return nil
}
