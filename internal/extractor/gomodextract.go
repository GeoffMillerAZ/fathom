package extractor

import (
	"bufio"
	"bytes"
	"sort"
	"strings"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// GoModExtractor implements Extractor for go.mod files. It parses require
// directives (both single-line and block form) and emits one KindModuleDep
// per dependency. The symbol Name is the module path; Signature contains the
// version string.
type GoModExtractor struct{}

// NewGoModExtractor returns a new GoModExtractor.
func NewGoModExtractor() *GoModExtractor { return &GoModExtractor{} }

// Extract parses go.mod bytes and returns SymbolNodes for every require entry.
func (g *GoModExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode

	scanner := bufio.NewScanner(bytes.NewReader(src))
	lineNo := 0
	inRequireBlock := false
	pkg := pkgFromPath(relPath)

	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}

		// Detect require block start.
		if trimmed == "require (" || trimmed == "require(" {
			inRequireBlock = true
			continue
		}
		// Detect block end.
		if inRequireBlock && trimmed == ")" {
			inRequireBlock = false
			continue
		}

		// Single-line require: require module/path v1.2.3
		if strings.HasPrefix(trimmed, "require ") {
			rest := strings.TrimPrefix(trimmed, "require ")
			rest = strings.TrimSpace(rest)
			// strip surrounding parens if accidentally included
			rest = strings.Trim(rest, "()")
			modPath, version := parseModLine(rest)
			if modPath != "" {
				nodes = append(nodes, core.SymbolNode{
					Service:   service,
					Package:   pkg,
					Name:      modPath,
					Kind:      core.KindModuleDep,
					FilePath:  relPath,
					Line:      lineNo,
					Signature: version,
					Tier:      core.TierPreScan,
				})
			}
			continue
		}

		// Inside a require block: each line is "module/path vX.Y.Z [// indirect]"
		if inRequireBlock {
			modPath, version := parseModLine(trimmed)
			if modPath != "" {
				nodes = append(nodes, core.SymbolNode{
					Service:   service,
					Package:   pkg,
					Name:      modPath,
					Kind:      core.KindModuleDep,
					FilePath:  relPath,
					Line:      lineNo,
					Signature: version,
					Tier:      core.TierPreScan,
				})
			}
		}
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Line != nodes[j].Line {
			return nodes[i].Line < nodes[j].Line
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}

// parseModLine parses a go.mod module line like:
//
//	github.com/foo/bar v1.2.3
//	github.com/foo/bar v1.2.3 // indirect
//
// and returns (modulePath, version). Returns ("", "") for blank/comment lines.
func parseModLine(line string) (modPath, version string) {
	// Strip inline comments.
	if idx := strings.Index(line, "//"); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return "", ""
	}
	return parts[0], parts[1]
}
