package conserve

// util.go — tiny helpers shared by more than one plane file, kept
// separate so each plane's own file stays focused on its own mechanics
// (conserve.go's own doc comment: planes land from different workers in
// the same wave without touching each other's files; this one is neutral
// ground for the handful of one-liners more than one plane needs).

// toSet turns ss into a membership set — used by every plane that needs
// an O(1) "is this file in the changed set" check.
func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}
