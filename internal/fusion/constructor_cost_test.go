package fusion_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// BenchmarkPipelineConstruction isolates repeated pipeline construction from
// source discovery, parsing, typed producers and storage. The warmup excludes
// the process's once-only dialect registry. The normal per-pipeline work is
// deliberately left inside the timer. This is a reading, not a timing gate.
func BenchmarkPipelineConstruction(b *testing.B) {
	conn := connector.NewLocalConnector()
	st := store.NewMemoryStore()
	cfg := fusion.Config{Concurrency: 1}
	p := fusion.New(conn, st, cfg)
	p.Close()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := fusion.New(conn, st, cfg)
		p.Close()
	}
}

// BenchmarkEmbeddedDocumentConstruction measures the embedded document
// constructors used by the pipeline, independent of tree-sitter and producers.
// Their input manifests are immutable embedded bytes. Any future sharing must
// preserve decoder validation and extraction, not bypass either for a benchmark.
func BenchmarkEmbeddedDocumentConstruction(b *testing.B) {
	for _, id := range []string{
		"unity-asmdef", "msbuild", "taskfile", "git-hooks-config", "compile-commands",
	} {
		b.Run(id, func(b *testing.B) {
			if _, err := extractor.NewDocDialectExtractor(id); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				e, err := extractor.NewDocDialectExtractor(id)
				if err != nil || e == nil {
					b.Fatalf("construct %s: extractor=%v, error=%v", id, e, err)
				}
			}
		})
	}
}

// BenchmarkEmbeddedDocumentExtraction is a comparison reading: valid declared
// facts are extracted with one constructed decoder. It is not a compiler or
// runtime oracle. Check the fixture before timing so an empty result cannot
// appear to be a cheap successful extraction.
func BenchmarkEmbeddedDocumentExtraction(b *testing.B) {
	e, err := extractor.NewDocDialectExtractor("unity-asmdef")
	if err != nil {
		b.Fatal(err)
	}
	src := []byte(`{"name":"Example.Rules","noEngineReferences":true,"references":["Example.Contracts"]}`)
	nodes, err := e.Extract("fixture", "Assets/Rules.asmdef", src)
	if err != nil {
		b.Fatal(err)
	}
	definition, reference := false, false
	for _, n := range nodes {
		definition = definition || (n.Name == "Example.Rules" && string(n.Kind) == "UnityAssemblyDefinition")
		reference = reference || (n.Name == "Example.Contracts" && string(n.Kind) == "UnityAssemblyReference")
		if len(n.Calls) != 0 {
			b.Fatal("document declarations became calls")
		}
	}
	if !definition || !reference {
		b.Fatalf("fixture lost declared definition/reference: %+v", nodes)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := e.Extract("fixture", "Assets/Rules.asmdef", src); err != nil {
			b.Fatal(err)
		}
	}
}
