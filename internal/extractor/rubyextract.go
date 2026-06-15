package extractor

import (
	"bufio"
	"bytes"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// RubyExtractor implements Extractor for Ruby source files using line-based
// heuristics. It detects:
//   - class declarations → KindRubyClass
//   - module declarations → KindRubyModule
//   - def declarations → KindRubyMethod (qualified as "ClassName.method" when inside a class/module)
type RubyExtractor struct{}

// NewRubyExtractor returns a new RubyExtractor.
func NewRubyExtractor() *RubyExtractor { return &RubyExtractor{} }

var (
	rubyClassRe  = regexp.MustCompile(`^\s*class\s+([A-Z][A-Za-z0-9_:]*)`)
	rubyModuleRe = regexp.MustCompile(`^\s*module\s+([A-Z][A-Za-z0-9_:]*)`)
	rubyDefRe    = regexp.MustCompile(`^\s*def\s+(self\.)?([a-z_][A-Za-z0-9_?!]*)`)
	rubyEndRe    = regexp.MustCompile(`^\s*end\s*$`)
)

// Extract parses Ruby source bytes and returns SymbolNodes.
func (r *RubyExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode

	scanner := bufio.NewScanner(bytes.NewReader(src))
	lineNo := 0

	// Ruby uses keyword-balanced blocks: class/module/def ... end.
	// We track a scope stack of (name, kind, depth).
	type scopeFrame struct {
		name  string
		kind  core.SourceKind
		depth int // nesting depth at entry
	}
	var scopeStack []scopeFrame
	depth := 0 // keyword block depth

	// Keywords that open a new depth level.
	rubyBlockOpener := func(s string) bool {
		kw := strings.Fields(s)
		if len(kw) == 0 {
			return false
		}
		switch kw[0] {
		case "if", "unless", "while", "until", "for", "begin", "do", "case":
			return true
		case "def", "class", "module":
			return true
		}
		// Single-line if/unless modifiers do NOT open a block.
		return false
	}

	pkg := pkgFromPath(relPath)

	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		stripped := strings.TrimSpace(line)

		if stripped == "" || strings.HasPrefix(stripped, "#") {
			continue
		}

		// 'end' closes the most recent block.
		if rubyEndRe.MatchString(line) {
			depth--
			// Pop scope frames whose depth exceeds current depth.
			for len(scopeStack) > 0 && scopeStack[len(scopeStack)-1].depth >= depth+1 {
				scopeStack = scopeStack[:len(scopeStack)-1]
			}
			continue
		}

		// class declaration
		if m := rubyClassRe.FindStringSubmatch(line); m != nil {
			name := m[1]
			// If inside another class/module, qualify the name.
			qualName := name
			if len(scopeStack) > 0 {
				qualName = scopeStack[len(scopeStack)-1].name + "::" + name
			}
			depth++
			nodes = append(nodes, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     qualName,
				Kind:     core.KindRubyClass,
				FilePath: relPath,
				Line:     lineNo,
				Tier:     core.TierPreScan,
			})
			scopeStack = append(scopeStack, scopeFrame{name: qualName, kind: core.KindRubyClass, depth: depth})
			continue
		}

		// module declaration
		if m := rubyModuleRe.FindStringSubmatch(line); m != nil {
			name := m[1]
			qualName := name
			if len(scopeStack) > 0 {
				qualName = scopeStack[len(scopeStack)-1].name + "::" + name
			}
			depth++
			nodes = append(nodes, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     qualName,
				Kind:     core.KindRubyModule,
				FilePath: relPath,
				Line:     lineNo,
				Tier:     core.TierPreScan,
			})
			scopeStack = append(scopeStack, scopeFrame{name: qualName, kind: core.KindRubyModule, depth: depth})
			continue
		}

		// def declaration
		if m := rubyDefRe.FindStringSubmatch(line); m != nil {
			methodName := m[2]
			qualName := methodName
			if len(scopeStack) > 0 {
				qualName = scopeStack[len(scopeStack)-1].name + "." + methodName
			}
			depth++
			nodes = append(nodes, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     qualName,
				Kind:     core.KindRubyMethod,
				FilePath: relPath,
				Line:     lineNo,
				Tier:     core.TierPreScan,
			})
			// defs are scopes too (they have their own 'end'), but we don't
			// push them as class/module scopes for method-qualification purposes.
			scopeStack = append(scopeStack, scopeFrame{name: qualName, kind: core.KindRubyMethod, depth: depth})
			continue
		}

		// Track other block-openers (do, begin, if, etc.) for depth balance.
		if rubyBlockOpener(stripped) {
			depth++
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
