package symbolnav_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/symbolnav"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// A Go struct field never shadows the method its member spelling also names
// (core.WithoutShadowingFields): symbol_context `Record` resolves the one
// method Recorder.Record, not an ambiguity with the field DecidedBy.Record.
// The field's own Owner.Field spelling still resolves the field.
func TestResolveTieredGoFieldNeverShadowsMethod(t *testing.T) {
	method := core.SymbolNode{Service: "s", Package: "Recorder", Name: "Recorder.Record", Kind: core.KindGoMethod, FilePath: "internal/decide/record.go", Line: 216}
	field := core.SymbolNode{Service: "s", Package: "DecidedBy", Name: "DecidedBy.Record", Kind: core.KindGoField, FilePath: "internal/decide/decidedby.go", Line: 54}
	nodes := []core.SymbolNode{field, method}

	key, cands, ok := symbolnav.ResolveTiered(nodes, "Record")
	if !ok || key != method.Key() || len(cands) != 1 {
		t.Fatalf("ResolveTiered(Record) = %q, %v, %v; want the method %q alone", key, cands, ok, method.Key())
	}
	key, _, ok = symbolnav.ResolveTiered(nodes, "DecidedBy.Record")
	if !ok || key != field.Key() {
		t.Fatalf("ResolveTiered(DecidedBy.Record) = %q, %v; want the field %q", key, ok, field.Key())
	}
}
