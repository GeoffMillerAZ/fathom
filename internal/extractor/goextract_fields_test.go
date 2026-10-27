package extractor_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

func TestGoExtractorStructFields(t *testing.T) {
	src := `package sample
type Outer[T any] struct {
	Public, hidden *T
	_ int
	*Box[T]
	pkg.Item
	Nested struct { Secret int }
}
type Box[T any] struct{}
type Alias = struct { Lost int }
type Contract interface { Run() }
`
	nodes, err := extractor.NewGoExtractor().Extract("svc", "pkg/sample.go", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	type field struct {
		name, pkg, sig string
		line, end      int
	}
	var got []field
	for _, n := range nodes {
		if n.Kind != core.KindGoField {
			continue
		}
		if n.Service != "svc" || n.FilePath != "pkg/sample.go" || n.Tier != core.TierVerified {
			t.Fatalf("bad field metadata: %+v", n)
		}
		got = append(got, field{n.Name, n.Package, n.Signature, n.Line, n.EndLine})
	}
	want := []field{
		{"Outer.Public", "Outer", "Public *T", 3, 3},
		{"Outer.hidden", "Outer", "hidden *T", 3, 3},
		{"Outer.Box", "Outer", "*Box[T]", 5, 5},
		{"Outer.Item", "Outer", "pkg.Item", 6, 6},
		{"Outer.Nested", "Outer", "Nested struct{Secret int}", 7, 7},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fields = %#v, want %#v", got, want)
	}
}
