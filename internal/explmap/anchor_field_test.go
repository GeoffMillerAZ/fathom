package explmap

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// A map anchor's member tier never lets a Go struct field shadow the method
// it shares a name with (core.WithoutShadowingFields); the field's own
// Owner.Field spelling still anchors the field.
func TestResolveAnchorGoFieldNeverShadowsMethod(t *testing.T) {
	method := core.SymbolNode{Service: "s", Package: "Recorder", Name: "Recorder.Record", Kind: core.KindGoMethod, FilePath: "internal/decide/record.go", Line: 216}
	field := core.SymbolNode{Service: "s", Package: "DecidedBy", Name: "DecidedBy.Record", Kind: core.KindGoField, FilePath: "internal/decide/decidedby.go", Line: 54}
	nodes := []core.SymbolNode{field, method}

	m, err := ResolveAnchorSymbol(nodes, "Record")
	if err != nil || m.Node.Name != method.Name {
		t.Fatalf("ResolveAnchorSymbol(Record) = %+v, %v; want the method %s", m.Node, err, method.Name)
	}
	m, err = ResolveAnchorSymbol(nodes, "DecidedBy.Record")
	if err != nil || m.Node.Name != field.Name {
		t.Fatalf("ResolveAnchorSymbol(DecidedBy.Record) = %+v, %v; want the field %s", m.Node, err, field.Name)
	}
}
