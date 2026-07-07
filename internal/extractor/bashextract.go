package extractor

import (
	"sort"

	ts "github.com/tree-sitter/go-tree-sitter"
	tsbash "github.com/tree-sitter/tree-sitter-bash/bindings/go"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor/treesitter"
)

// bashLang is initialised once and shared across all BashExtractor calls.
// tree-sitter Language objects are immutable and safe for concurrent use;
// only the Parser (created fresh per call) carries mutable state.
var bashLang = ts.NewLanguage(tsbash.Language())

// BashExtractor implements Extractor for Bash/shell source files using
// tree-sitter CGO bindings (ADR-013). It emits:
//
//   - KindBashFunction for function definitions in both forms:
//     "function name() { }" and "name() { }".
//
// All symbols carry TierVerified.
type BashExtractor struct{}

// NewBashExtractor returns a new BashExtractor.
func NewBashExtractor() *BashExtractor { return &BashExtractor{} }

// Extract parses src with tree-sitter and returns SymbolNodes for Bash
// function definitions. Output is sorted by Line then Name for determinism.
func (b *BashExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	var nodes []core.SymbolNode

	pkg := pkgFromPath(relPath)

	err := treesitter.WithTree(bashLang, src, func(root *ts.Node) {
		walkBash(root, src, service, pkg, relPath, &nodes)
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

// walkBash recursively descends the tree-sitter CST, collecting
// function_definition nodes.
func walkBash(
	n *ts.Node,
	src []byte,
	service, pkg, relPath string,
	out *[]core.SymbolNode,
) {
	if n == nil {
		return
	}

	if n.Kind() == "function_definition" {
		name := treesitter.FieldText(n, "name", src)
		if name != "" {
			*out = append(*out, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     name,
				Kind:     core.KindBashFunction,
				FilePath: relPath,
				Line:     treesitter.Line(n),
				Tier:     core.TierVerified,
			})
		}
		// Do not recurse into function bodies — nested functions are uncommon
		// and would be redundant without a meaningful scope qualifier.
		return
	}

	// Generic descent for all other node types.
	count := n.NamedChildCount()
	for i := uint(0); i < count; i++ {
		child := n.NamedChild(i)
		walkBash(child, src, service, pkg, relPath, out)
	}
}
