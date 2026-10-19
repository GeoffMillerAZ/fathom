package mcp_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

func TestSymbolContextCSharpPartialTypeLocations(t *testing.T) {
	e, err := extractor.NewTSExtractor("csharp")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	var nodes []core.SymbolNode
	for _, file := range []string{"UI.cs", "Preview.cs", "Panel.cs"} {
		part, err := e.Extract("svc", "Editor/"+file, []byte("namespace Workshop { public partial class Panel { } }"))
		if err != nil {
			t.Fatal(err)
		}
		nodes = append(nodes, part...)
	}
	for _, query := range []string{"Panel", "svc::Editor/Workshop.Panel"} {
		srv := mcp.NewServer(&extendedFakeBackend{nodes: nodes, services: []string{"svc"}})
		p := callTool(t, srv, "symbol_context", map[string]any{"service": "svc", "symbol": query, "snippet": false})
		if p["ambiguous"] != nil {
			t.Fatalf("%q falsely ambiguous: %v", query, p)
		}
		if p["definition_count"] != float64(3) {
			t.Fatalf("%q count = %v, want3: %v", query, p["definition_count"], p)
		}
		rows, _ := p["definitions"].([]any)
		var files []string
		for _, row := range rows {
			files = append(files, row.(map[string]any)["file"].(string))
		}
		if !reflect.DeepEqual(files, []string{"Editor/Panel.cs", "Editor/Preview.cs", "Editor/UI.cs"}) {
			t.Fatalf("definitions = %v", rows)
		}
		if p["def"].(map[string]any)["file_path"] != "Editor/Panel.cs" {
			t.Fatalf("unstable primary definition: %v", p["def"])
		}
	}
}

func TestSymbolContextRepeatedDefinitionsBounded(t *testing.T) {
	var nodes []core.SymbolNode
	for i := 14; i >= 0; i-- {
		nodes = append(nodes, core.SymbolNode{Service: "svc", Package: "Workshop", Name: "Panel", Kind: core.KindCSharpClass, FilePath: fmt.Sprintf("Editor/Part%02d.cs", i), Line: 1})
	}
	srv := mcp.NewServer(&extendedFakeBackend{nodes: nodes, services: []string{"svc"}})
	p := callTool(t, srv, "symbol_context", map[string]any{"service": "svc", "symbol": "Panel", "snippet": false})
	rows, _ := p["definitions"].([]any)
	if len(rows) != 10 || p["definition_count"] != float64(15) || p["definitions_truncated"] != true {
		t.Fatalf("unbounded or undisclosed parts: %v", p)
	}
}

func TestSymbolContextPartialTypeDoesNotHideNamespaceAmbiguity(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "Alpha", Name: "Panel", Kind: core.KindCSharpClass, FilePath: "Editor/A.cs", Line: 1},
		{Service: "svc", Package: "Alpha", Name: "Panel", Kind: core.KindCSharpClass, FilePath: "Editor/B.cs", Line: 2},
		{Service: "svc", Package: "Other", Name: "Panel", Kind: core.KindCSharpClass, FilePath: "Editor/C.cs", Line: 3},
	}
	srv := mcp.NewServer(&extendedFakeBackend{nodes: nodes, services: []string{"svc"}})
	p := callTool(t, srv, "symbol_context", map[string]any{"service": "svc", "symbol": "Panel", "snippet": false})
	if p["ambiguous"] != true || p["count"] != float64(2) {
		t.Fatalf("distinct namespace keys must remain ambiguous: %v", p)
	}
	for _, field := range []string{"def", "definitions", "callers", "callees", "snippet"} {
		if _, ok := p[field]; ok {
			t.Fatalf("ambiguous response leaks %s: %v", field, p)
		}
	}
}
