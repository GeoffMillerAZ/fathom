package extractor

import (
	"bufio"
	"bytes"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// TemplExtractor implements Extractor for a-h/templ template files (.templ).
// It uses line-based heuristics to find top-level declarations and outgoing
// component call edges.
//
// Detected declaration forms (must start at column 0):
//   - templ ComponentName(params...) {  → KindTemplComponent
//   - script Name(params...) {          → KindTemplComponent
//   - css Name(params...) {             → KindTemplComponent
//   - func Name(...) ...                → KindTemplComponent (Go func inside .templ)
//
// Call edges: @OtherComponent( and @pkg.OtherComponent( references within a
// component body are recorded in that component's Calls slice (token stored as
// written minus the @ and open paren, e.g. "OtherComponent" or "pkg.OtherComponent").
//
// It is deterministic, uses only stdlib regexp, and is safe for concurrent use.
type TemplExtractor struct{}

// NewTemplExtractor returns a new TemplExtractor.
func NewTemplExtractor() *TemplExtractor { return &TemplExtractor{} }

var (
	// Top-level block declarations (must start at column 0).
	templDeclRe = regexp.MustCompile(`^(templ|script|css)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	templFuncRe = regexp.MustCompile(`^func\s+([A-Za-z_][A-Za-z0-9_]*)`)

	// Call-edge references: @Name( or @pkg.Name(
	templCallRe = regexp.MustCompile(`@([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)?)\(`)
)

// Extract parses a .templ file and returns SymbolNodes for each top-level
// declaration. Output preserves source order (sorted by Line then Name).
func (t *TemplExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode

	pkg := pkgFromPath(relPath)

	type declEntry struct {
		nodeIdx int // index into nodes slice
		startAt int // line number where the decl was found
	}

	// decls tracks open top-level declarations in source order so we can
	// accumulate call edges until the next top-level decl begins.
	var decls []declEntry

	scanner := bufio.NewScanner(bytes.NewReader(src))
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		line := scanner.Text()

		// Skip blank lines and comment lines for decl detection.
		stripped := strings.TrimSpace(line)
		if stripped == "" || strings.HasPrefix(stripped, "//") {
			continue
		}

		// Only top-level decls start at column 0 (no leading whitespace).
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			// Inside a body: scan for call edges to attribute to the current decl.
			if len(decls) > 0 {
				currentIdx := decls[len(decls)-1].nodeIdx
				addCallEdges(&nodes[currentIdx], line)
			}
			continue
		}

		// Attempt to match a templ/script/css block declaration.
		if m := templDeclRe.FindStringSubmatch(line); m != nil {
			// m[1] = keyword (templ|script|css), m[2] = name
			sig := strings.TrimSpace(line)
			node := core.SymbolNode{
				Service:   service,
				Package:   pkg,
				Name:      m[2],
				Kind:      core.KindTemplComponent,
				FilePath:  relPath,
				Line:      lineNo,
				Signature: sig,
				Tier:      core.TierVerified,
			}
			nodes = append(nodes, node)
			decls = append(decls, declEntry{nodeIdx: len(nodes) - 1, startAt: lineNo})
			continue
		}

		// Attempt to match a plain Go func declaration.
		if m := templFuncRe.FindStringSubmatch(line); m != nil {
			sig := strings.TrimSpace(line)
			node := core.SymbolNode{
				Service:   service,
				Package:   pkg,
				Name:      m[1],
				Kind:      core.KindTemplComponent,
				FilePath:  relPath,
				Line:      lineNo,
				Signature: sig,
				Tier:      core.TierVerified,
			}
			nodes = append(nodes, node)
			decls = append(decls, declEntry{nodeIdx: len(nodes) - 1, startAt: lineNo})
			continue
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

// addCallEdges scans a body line for @Name( and @pkg.Name( references and
// appends them to node.Calls (deduped, first-seen order preserved).
func addCallEdges(node *core.SymbolNode, line string) {
	matches := templCallRe.FindAllStringSubmatch(line, -1)
	for _, m := range matches {
		callee := m[1]
		if !containsCall(node.Calls, callee) {
			node.Calls = append(node.Calls, callee)
		}
	}
}

// containsCall reports whether callee is already in the calls slice.
func containsCall(calls []string, callee string) bool {
	for _, c := range calls {
		if c == callee {
			return true
		}
	}
	return false
}
