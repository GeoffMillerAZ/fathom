package extractor

import (
	"context"
	"os"
	"sync/atomic"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// TestDialectQueriesCompile (§3.2) compiles every tags dialect's query for
// every grammar variant — a predicate the engine does not support fails
// here, by name — and runs it over the dialect's fixtures, holding the
// host's pattern numbering (predicates.go) to the one the C core reports in
// each match.
func TestDialectQueriesCompile(t *testing.T) {
	if _, err := tsRegistry(); err != nil {
		t.Fatal(err)
	}
	var active atomic.Int32
	slots := make(chan struct{}, 4)
	for _, id := range TagsDialects() {
		spec := tsDialectMap[id]
		variants := map[string]tsVariant{spec.defaultVariant.key(): spec.defaultVariant}
		for _, v := range spec.variantForExt {
			variants[v.key()] = v
		}
		for key, v := range variants {
			t.Run(key, func(t *testing.T) {
				// Bound native parser admission so parallel package gates do not
				// fan out a full WASM VM for every grammar variant at once.
				t.Parallel()
				slots <- struct{}{}
				defer func() { <-slots }()
				n := active.Add(1)
				defer active.Add(-1)
				if n > 4 {
					t.Fatalf("%d concurrent native parsers, want at most 4", n)
				}
				p, err := tsengine.NewParser(context.Background(), v.blob, v.entryPoint)
				if err != nil {
					t.Fatal(err)
				}
				defer p.Close()
				q, err := p.CompileQuery(v.rules.query)
				if err != nil {
					t.Fatalf("%s: %v", v.rules.queryFile, err)
				}
				if v.rules.injectionsQuery != "" {
					iq, err := p.CompileQuery(v.rules.injectionsQuery)
					if err != nil {
						t.Fatalf("%s injections: %v", id, err)
					}
					iq.Close()
				}
				if _, err := os.Stat("dialects/" + id + "/testdata"); err != nil {
					return
				}
				for _, fx := range dialectFixtures(t, id) {
					// A tags+doc dialect's document fixtures (Chart.yaml,
					// values.yaml) belong to its doc engine: no variant.
					if sv, ok := spec.variantForExt[extOf(fx)]; !ok || sv.key() != key {
						continue
					}
					src, _ := os.ReadFile(fx)
					tree, err := p.Parse(src)
					if err != nil {
						t.Fatal(err)
					}
					ms, err := q.Run(tree)
					tree.Close()
					if err != nil {
						t.Fatal(err)
					}
					for _, m := range ms {
						if int(m.PatternIndex) >= q.PatternCount() {
							t.Fatalf("%s: match pattern %d, host read %d patterns", fx, m.PatternIndex, q.PatternCount())
						}
					}
				}
			})
		}
	}
}
