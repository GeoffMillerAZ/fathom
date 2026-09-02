package lib

// Parse parses.
func Parse(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	return len(s), nil
}

// Format formats.
func Format(n int) string {
	return "n"
}
