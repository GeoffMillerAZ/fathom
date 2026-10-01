// i42_obligations_golden_test.go — I42 (judgment-foundations design §4, JF-5):
// the obligations envelope served with no [decide] block (config.Load of no
// layer, the kit's decision.None, as servehttp.go wires an unconfigured
// daemon) is byte-identical to what it was before the judgment-foundations
// rows. The golden was captured at the pre-change HEAD (2026-10-01); a diff
// here is an I42 break, never a golden to refresh.
package mcp_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

var updateI42 = flag.Bool("update-i42", false, "capture the I42 obligations golden (only ever at the pre-change HEAD)")

const i42ObligationsGolden = "testdata/i42/obligations-none.golden"

// TestI42ObligationsEnvelopeWithNoneIsByteIdentical serves the obligations
// tool over the real JSON-RPC surface with an unconfigured [decide] block and
// compares the envelope's text with the pre-change golden.
func TestI42ObligationsEnvelopeWithNoneIsByteIdentical(t *testing.T) {
	loaded, err := config.Load("", "")
	if err != nil {
		t.Fatalf("config.Load with no layer: %v", err)
	}
	st := store.NewMemoryStore()
	seedWireClaim(t, st, "i42-1", "i42-1: requires review before any change.")
	seedWireClaim(t, st, "i42-2", "i42-2: requires review before any change too.")

	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	c.WithDecision(decision.None{}, loaded.Config.Decide, nil)

	resp := roundtrip(t, mcp.NewServer(c), request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{
			"name":      "obligations",
			"arguments": map[string]any{"service": "svc", "file": rerankWireTestFile},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("obligations tool error: %v", resp["error"])
	}
	result, _ := resp["result"].(map[string]any)
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		t.Fatalf("no content: %v", resp)
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	got := []byte(text + "\n")

	if *updateI42 {
		if err := os.MkdirAll(filepath.Dir(i42ObligationsGolden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(i42ObligationsGolden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(i42ObligationsGolden)
	if err != nil {
		t.Fatalf("reading %s: %v", i42ObligationsGolden, err)
	}
	if string(got) != string(want) {
		t.Errorf("unconfigured obligations envelope changed (I42); never refresh this golden to pass:\n--- got\n%s\n--- want\n%s", got, want)
	}
}
