package callgraph_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

func TestReplacePackageRealGoFieldDoesNotHideMethod(t *testing.T) {
	ex := extractor.NewGoExtractor()
	methodNodes, err := ex.Extract("svc", "a/a.go", []byte("package a\ntype Config struct{}\nfunc (c *Config) Load() {}\n"))
	if err != nil {
		t.Fatal(err)
	}
	fieldNodes, err := ex.Extract("svc", "b/b.go", []byte("package b\ntype Config struct{ Load func() }\n"))
	if err != nil {
		t.Fatal(err)
	}
	nodes := append(append([]core.SymbolNode{}, methodNodes...), fieldNodes...)
	fresh := callgraph.Build(nodes)
	want := callgraph.Canonical(fresh, "Config.Load")
	if want != "svc::a/Config.Config.Load" {
		t.Fatalf("Build Config.Load = %q, want method", want)
	}
	var ownerNodes []core.SymbolNode
	for _, n := range nodes {
		if n.Package == "Config" {
			ownerNodes = append(ownerNodes, n)
		}
	}
	if len(ownerNodes) < 2 {
		t.Fatalf("extractor produced %d Config members, want method and field", len(ownerNodes))
	}
	patched := fresh.ReplacePackage("Config", ownerNodes, nil)
	if got := callgraph.Canonical(patched, "Config.Load"); got != want {
		t.Errorf("Config.Load after unchanged refresh = %q, Build = %q", got, want)
	}
}
