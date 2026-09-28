package mcp_test

// ingest_indexers_real_test.go — SR-65 end to end with the real store: the
// ingest tool, behind the auto-refresh wrapper the daemon serves through,
// imports the SCIP index a producer left under .fathom/scip/ as typed edges
// (the SAME hand-built TypeScript fixture SU-9's own tests pin), and a
// re-import reconciles rather than doubling.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/scip"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

func TestIngestIndexersImportsARealIndexThroughTheWrapper(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		"math.ts":    "export function add(a: number, b: number): number {\n    return a + b;\n}\n",
		"greeter.ts": "import { add } from \"./math\";\n\nexport class Greeter {\n    greet(name: string): number {\n        console.log(name);\n        return add(1, 2) + name.length;\n    }\n}\n",
		"main.ts":    "import { Greeter } from \"./greeter\";\n\nexport function run(): number {\n    const g = new Greeter();\n    return g.greet(\"hi\");\n}\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := proto.Marshal(scip.BuildTSFixtureIndex())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".fathom", "scip"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".fathom", "scip", "scip-typescript.scip"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := client.NewConstantClient(client.ModeLocal, filepath.Join(t.TempDir(), "s.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	srv := mcp.NewServer(mcp.NewAutoRefreshForTest(t, c))
	seedIdentCache(t)

	call := func() map[string]any {
		return ingestWith(t, srv, map[string]any{"service": "tsfixture", "path": root, "indexers": []string{"scip-typescript"}})
	}
	p := call()
	rows, _ := p["scip"].([]any)
	if len(rows) != 1 {
		t.Fatalf("scip = %v, want one row: %v", p["scip"], p)
	}
	row := rows[0].(map[string]any)
	if row["error"] != nil {
		t.Fatalf("import failed: %v", row["error"])
	}
	if row["producer"] != "scip-typescript 0.3.16" || row["edges"] != float64(4) || row["direct"] != float64(3) || row["unresolved"] != float64(1) {
		t.Errorf("scip row = %v, want the fixture's 4 edges (3 direct, 1 unresolved) from scip-typescript 0.3.16", row)
	}

	edges, err := c.TypedEdges(context.Background(), "tsfixture")
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 4 {
		t.Errorf("typed edges in the store = %d, want the import's 4", len(edges))
	}
	// A second call re-imports the same index: it replaces its own rows.
	call()
	again, err := c.TypedEdges(context.Background(), "tsfixture")
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != len(edges) {
		t.Errorf("typed edges after a re-import = %d, want %d (an import reconciles, it never doubles)", len(again), len(edges))
	}
}
