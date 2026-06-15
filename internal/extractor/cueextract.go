package extractor

import (
	"sort"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/parser"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// CUEExtractor implements Extractor for CUE language files. It uses the
// official cuelang.org/go parser to detect top-level field declarations.
// Fields whose label starts with "#" → KindCUEDefinition; all other
// top-level fields → KindCUEField. Tier is TierVerified (real parse).
type CUEExtractor struct{}

// NewCUEExtractor returns a new CUEExtractor.
func NewCUEExtractor() *CUEExtractor { return &CUEExtractor{} }

// Extract parses CUE source bytes and returns SymbolNodes for top-level
// field declarations. Parse errors are treated as best-effort: if the
// parser returns a nil File we return nil,nil; otherwise we use whatever
// partial AST the parser produced.
func (c *CUEExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	f, _ := parser.ParseFile(relPath, src)
	if f == nil {
		return nil, nil
	}

	pkg := pkgFromPath(relPath)
	var nodes []core.SymbolNode

	for _, d := range f.Decls {
		field, ok := d.(*ast.Field)
		if !ok {
			continue
		}

		// Extract label name. The label is an ast.Label interface; the most
		// common case is *ast.Ident (covers both #Def: and plain field:).
		ident, ok := field.Label.(*ast.Ident)
		if !ok {
			continue
		}
		name := ident.Name
		if name == "" {
			continue
		}

		kind := core.KindCUEField
		if strings.HasPrefix(name, "#") {
			kind = core.KindCUEDefinition
		}

		line := field.Pos().Line()

		nodes = append(nodes, core.SymbolNode{
			Service:  service,
			Package:  pkg,
			Name:     name,
			Kind:     kind,
			FilePath: relPath,
			Line:     line,
			Tier:     core.TierVerified,
		})
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Line != nodes[j].Line {
			return nodes[i].Line < nodes[j].Line
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}
