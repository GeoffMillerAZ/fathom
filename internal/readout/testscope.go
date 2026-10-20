package readout

import "strings"

// ScopeAdmits reports whether a test-run row's scope (RO-35's text,
// `<patterns> run=<re> skip=<re>`, patterns comma-joined) ran the test name
// in the package at repository directory dir ("." for the root): one of its
// package patterns names dir and its -run and -skip filters admit name. The
// row keeps no working directory, so a relative pattern is read against the
// repository root, and a pattern that cannot be placed there (a module-path
// wildcard, a "../" path) admits nothing. ok is false when the scope cannot
// say: it is empty, or a filter does not compile.
func ScopeAdmits(scope, dir, name string) (admits, ok bool) {
	if strings.TrimSpace(scope) == "" {
		return false, false
	}
	patterns := []string{"."} // `go test` with no pattern tests the current package
	if f, _, _ := strings.Cut(scope, " "); f != "" && !strings.HasPrefix(f, "run=") && !strings.HasPrefix(f, "skip=") {
		patterns = strings.Split(f, ",")
	}
	named := false
	for _, p := range patterns {
		if packagePatternNames(p, dir) {
			named = true
			break
		}
	}
	if !named {
		return false, true
	}
	run, skip := scopeFilters(scope)
	if run != "" {
		m, ok := goTestFilterMatch(run, name, false)
		if !ok {
			return false, false
		}
		if !m {
			return false, true
		}
	}
	if skip != "" {
		m, ok := goTestFilterMatch(skip, name, true)
		if !ok {
			return false, false
		}
		if m {
			return false, true
		}
	}
	return true, true
}

// packagePatternNames reports whether one `go test` package pattern names
// the package at repository directory dir, the pattern read against the
// repository root.
func packagePatternNames(p, dir string) bool {
	d := strings.Trim(dir, "/")
	if d == "." {
		d = ""
	}
	p = strings.TrimSuffix(p, "/")
	if p == "..." {
		return true
	}
	wild := strings.HasSuffix(p, "/...")
	base := strings.TrimSuffix(p, "/...")
	switch {
	case base == "." || base == "":
		base = ""
	case strings.HasPrefix(base, "./"):
		base = strings.TrimPrefix(base, "./")
	case strings.HasPrefix(base, "../") || base == "..":
		return false
	default:
		// An import path: its tail names dir; a module-path wildcard
		// cannot be placed without the module's own path.
		if wild || d == "" {
			return false
		}
		return base == d || strings.HasSuffix(base, "/"+d)
	}
	if wild {
		return base == "" || d == base || strings.HasPrefix(d, base+"/")
	}
	return base == d
}
