package extractor

import (
	"bytes"
	"sort"

	"github.com/emicklei/proto"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// ProtoExtractor implements Extractor for Protocol Buffer (.proto) files using
// github.com/emicklei/proto for real AST parsing. It emits:
//   - KindProtoMessage for each message definition
//   - KindProtoService for each service definition
//   - KindProtoRPC    for each RPC method (Signature = "RequestType -> ReturnsType")
//   - KindProtoEnum   for each enum definition
//
// Parsing is best-effort: on error the extractor returns whatever nodes were
// collected before the failure, never a non-nil error.
type ProtoExtractor struct{}

// NewProtoExtractor returns a new ProtoExtractor.
func NewProtoExtractor() *ProtoExtractor { return &ProtoExtractor{} }

// Extract parses Protocol Buffer source bytes and returns SymbolNodes.
// Output is sorted by Line then Name for determinism.
func (p *ProtoExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	pkg := pkgFromPath(relPath)
	var nodes []core.SymbolNode

	def, _ := proto.NewParser(bytes.NewReader(src)).Parse()
	if def == nil {
		return nodes, nil
	}

	proto.Walk(
		def,
		proto.WithMessage(func(m *proto.Message) {
			nodes = append(nodes, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     m.Name,
				Kind:     core.KindProtoMessage,
				FilePath: relPath,
				Line:     m.Position.Line,
				Tier:     core.TierVerified,
			})
		}),
		proto.WithService(func(s *proto.Service) {
			nodes = append(nodes, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     s.Name,
				Kind:     core.KindProtoService,
				FilePath: relPath,
				Line:     s.Position.Line,
				Tier:     core.TierVerified,
			})
		}),
		proto.WithRPC(func(r *proto.RPC) {
			nodes = append(nodes, core.SymbolNode{
				Service:   service,
				Package:   pkg,
				Name:      r.Name,
				Kind:      core.KindProtoRPC,
				FilePath:  relPath,
				Line:      r.Position.Line,
				Signature: r.RequestType + " -> " + r.ReturnsType,
				Tier:      core.TierVerified,
			})
		}),
		proto.WithEnum(func(e *proto.Enum) {
			nodes = append(nodes, core.SymbolNode{
				Service:  service,
				Package:  pkg,
				Name:     e.Name,
				Kind:     core.KindProtoEnum,
				FilePath: relPath,
				Line:     e.Position.Line,
				Tier:     core.TierVerified,
			})
		}),
	)

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Line != nodes[j].Line {
			return nodes[i].Line < nodes[j].Line
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}
