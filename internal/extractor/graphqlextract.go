package extractor

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

// GraphQLExtractor implements Extractor for GraphQL schema files (.graphql, .gql)
// using github.com/vektah/gqlparser/v2 for real AST parsing. It emits:
//   - KindGraphQLType  for each type definition (Signature = lowercased kind, e.g. "object")
//   - KindGraphQLField for each field on OBJECT, INPUT_OBJECT, and INTERFACE definitions
//     (Name = "TypeName.fieldName", Signature = field type string)
//
// Parsing is best-effort: on error the extractor returns whatever nodes were
// collected before the failure, never a non-nil error.
type GraphQLExtractor struct{}

// NewGraphQLExtractor returns a new GraphQLExtractor.
func NewGraphQLExtractor() *GraphQLExtractor { return &GraphQLExtractor{} }

// Extract parses GraphQL schema bytes and returns SymbolNodes.
// Output is sorted by Line then Name for determinism.
func (g *GraphQLExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	pkg := pkgFromPath(relPath)
	var nodes []core.SymbolNode

	doc, err := parser.ParseSchema(&ast.Source{Name: relPath, Input: string(src)})
	if err != nil || doc == nil {
		return nodes, nil
	}

	for _, def := range doc.Definitions {
		if def == nil {
			continue
		}

		line := 0
		if def.Position != nil {
			line = def.Position.Line
		}

		// Emit one KindGraphQLType node per definition.
		nodes = append(nodes, core.SymbolNode{
			Service:   service,
			Package:   pkg,
			Name:      def.Name,
			Kind:      core.KindGraphQLType,
			FilePath:  relPath,
			Line:      line,
			Signature: strings.ToLower(string(def.Kind)),
			Tier:      core.TierVerified,
		})

		// Emit one KindGraphQLField per field for OBJECT, INPUT_OBJECT, and INTERFACE.
		if def.Kind == ast.Object || def.Kind == ast.InputObject || def.Kind == ast.Interface {
			for _, field := range def.Fields {
				if field == nil {
					continue
				}
				fieldLine := 0
				if field.Position != nil {
					fieldLine = field.Position.Line
				}
				typeSig := ""
				if field.Type != nil {
					typeSig = field.Type.String()
				}
				nodes = append(nodes, core.SymbolNode{
					Service:   service,
					Package:   pkg,
					Name:      def.Name + "." + field.Name,
					Kind:      core.KindGraphQLField,
					FilePath:  relPath,
					Line:      fieldLine,
					Signature: typeSig,
					Tier:      core.TierVerified,
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
