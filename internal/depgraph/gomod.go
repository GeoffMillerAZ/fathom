package depgraph

import (
	"errors"
	"fmt"
	"strings"
)

// Require represents a single require directive in a go.mod file.
type Require struct {
	Path    string // module path, e.g. "github.com/foo/bar"
	Version string // semver string, e.g. "v1.2.3"
}

// ParseGoMod parses the content of a go.mod file and returns the module path
// and the list of require directives in declaration order.  Indirect
// dependencies (marked with "// indirect") are included.  Returns an error if
// the module declaration is absent or if a require line is malformed.
func ParseGoMod(content []byte) (module string, requires []Require, err error) {
	lines := strings.Split(string(content), "\n")

	inBlock := false
	for _, raw := range lines {
		line := strings.TrimSpace(raw)

		// Strip inline comments.
		if idx := strings.Index(line, "//"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}

		if line == "" {
			continue
		}

		// Handle block require close.
		if inBlock {
			if line == ")" {
				inBlock = false
				continue
			}
			// Inside require ( ... ) block: "path version"
			r, parseErr := parseRequireLine(line)
			if parseErr != nil {
				return "", nil, parseErr
			}
			requires = append(requires, r)
			continue
		}

		// module directive.
		if strings.HasPrefix(line, "module ") {
			module = strings.TrimSpace(strings.TrimPrefix(line, "module "))
			continue
		}

		// require block opener.
		if line == "require (" {
			inBlock = true
			continue
		}

		// Single-line require.
		if strings.HasPrefix(line, "require ") {
			rest := strings.TrimSpace(strings.TrimPrefix(line, "require "))
			// rest might be "(" — treat as block opener.
			if rest == "(" {
				inBlock = true
				continue
			}
			r, parseErr := parseRequireLine(rest)
			if parseErr != nil {
				return "", nil, parseErr
			}
			requires = append(requires, r)
			continue
		}
	}

	if module == "" {
		return "", nil, errors.New("depgraph: go.mod missing module declaration")
	}
	return module, requires, nil
}

// parseRequireLine parses a single "path version" string.
func parseRequireLine(s string) (Require, error) {
	fields := strings.Fields(s)
	if len(fields) < 2 {
		return Require{}, fmt.Errorf("depgraph: malformed require line: %q", s)
	}
	return Require{Path: fields[0], Version: fields[1]}, nil
}

// BuildModuleGraph builds a module dependency Graph from a map of go.mod file
// contents keyed by their logical name (typically the module path, but the
// key is only used for error messages).  Each module's go.mod is parsed; an
// edge module→require is added for every require directive.  External modules
// that appear only as dependencies are still added as nodes (no outbound
// edges).
//
// Returns an error if any go.mod cannot be parsed.
func BuildModuleGraph(mods map[string][]byte) (*Graph, error) {
	g := NewGraph()

	for key, content := range mods {
		modPath, reqs, err := ParseGoMod(content)
		if err != nil {
			return nil, fmt.Errorf("depgraph: BuildModuleGraph: key %q: %w", key, err)
		}
		g.AddNode(modPath)
		for _, r := range reqs {
			g.AddEdge(modPath, r.Path)
		}
	}
	return g, nil
}
