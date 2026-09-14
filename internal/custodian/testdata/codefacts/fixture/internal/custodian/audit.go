package custodian

// A tiny stand-in for the real internal/custodian/audit.go: enough
// declarations for the symbol-anchor checker to find, none of the real
// logic.

// Concern is a falsifiable want with provenance.
type Concern struct {
	ID string
}

// Finding is one concern's audit outcome.
type Finding struct {
	Status string
}

// Audit evaluates every concern against the repository at root.
func Audit(root string, concerns []Concern) []Finding {
	return nil
}

// LoadFile reads one scope's concerns file.
func LoadFile(path string) ([]Concern, error) {
	return nil, nil
}

// Merge resolves the two scopes: the union by ID, project governs.
func Merge(user, project []Concern) []Concern {
	return nil
}

// ExampleTemplate is what `fathom init` scaffolds.
func ExampleTemplate() []byte {
	return nil
}
