package extractor

import (
	"sort"

	ts "github.com/tree-sitter/go-tree-sitter"
	tsruby "github.com/tree-sitter/tree-sitter-ruby/bindings/go"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor/treesitter"
)

// rubyLang is initialised once and shared across all RubyExtractor calls.
// tree-sitter Language objects are immutable and safe for concurrent use;
// only the Parser (created fresh per call) carries mutable state.
var rubyLang = ts.NewLanguage(tsruby.Language())

// RubyExtractor implements Extractor for Ruby source files using
// tree-sitter CGO bindings (ADR-013). It emits:
//
//   - KindRubyModule for module declarations.
//   - KindRubyClass for class declarations.
//   - KindRubyMethod for instance method definitions (def foo) and singleton
//     method definitions (def self.foo), qualified as "Owner.method".
//     Lexically nested class/module names are joined with "::".
//
// All symbols carry TierVerified.
type RubyExtractor struct{}

// NewRubyExtractor returns a new RubyExtractor.
func NewRubyExtractor() *RubyExtractor { return &RubyExtractor{} }

// Extract parses Ruby source bytes with tree-sitter and returns SymbolNodes
// for modules, classes, and methods. Output is sorted by Line then Name for
// determinism.
func (r *RubyExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode

	pkg := pkgFromPath(relPath)

	err := treesitter.WithTree(rubyLang, src, func(root *ts.Node) {
		walkRuby(root, src, service, pkg, relPath, nil, &nodes)
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Line != nodes[j].Line {
			return nodes[i].Line < nodes[j].Line
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}

// walkRuby recursively descends the tree-sitter CST.
//
// containerStack tracks the qualified names of lexically enclosing modules and
// classes so that nested names use "::" and method names use ".".
func walkRuby(
	n *ts.Node,
	src []byte,
	service, pkg, relPath string,
	containerStack []string,
	out *[]core.SymbolNode,
) {
	if n == nil {
		return
	}

	kind := n.Kind()

	switch kind {
	case "module", "class":
		name := treesitter.FieldText(n, "name", src)
		if name == "" {
			break
		}
		qualName := qualifyRubyContainer(containerStack, name)
		symKind := core.KindRubyModule
		if kind == "class" {
			symKind = core.KindRubyClass
		}
		*out = append(*out, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     qualName,
			Kind:     symKind,
			FilePath: relPath,
			Line:     treesitter.Line(n),
			EndLine:  treesitter.EndLine(n),
			Tier:     core.TierVerified,
		})
		// Recurse into the body with this container pushed.
		newStack := append(append([]string{}, containerStack...), qualName)
		body := n.ChildByFieldName("body")
		if body != nil {
			walkRuby(body, src, service, pkg, relPath, newStack, out)
		}
		return

	case "method", "singleton_method":
		name := treesitter.FieldText(n, "name", src)
		if name == "" {
			break
		}
		qualName := qualifyRubyMethod(containerStack, name)
		*out = append(*out, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     qualName,
			Kind:     core.KindRubyMethod,
			FilePath: relPath,
			Line:     treesitter.Line(n),
			EndLine:  treesitter.EndLine(n),
			Tier:     core.TierVerified,
			Calls:    treesitter.CollectCalls(n, src, map[string]string{"call": "method"}),
		})
		// Do not descend into method bodies — methods don't alter container scope.
		return
	}

	// Generic descent for all other node types (program, body_statement, …).
	count := n.NamedChildCount()
	for i := uint(0); i < count; i++ {
		child := n.NamedChild(i)
		walkRuby(child, src, service, pkg, relPath, containerStack, out)
	}
}

// qualifyRubyContainer builds "Outer::Inner" style names for nested modules
// and classes.
func qualifyRubyContainer(stack []string, name string) string {
	if len(stack) == 0 {
		return name
	}
	return stack[len(stack)-1] + "::" + name
}

// qualifyRubyMethod builds "Owner.method" for methods inside a container.
func qualifyRubyMethod(stack []string, name string) string {
	if len(stack) == 0 {
		return name
	}
	return stack[len(stack)-1] + "." + name
}
