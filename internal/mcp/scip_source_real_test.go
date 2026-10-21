package mcp_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/memo"
	"github.com/geoffmilleraz/fathom/internal/refresh"
	"github.com/geoffmilleraz/fathom/internal/scip"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
	"google.golang.org/protobuf/proto"
)

func scipSourceMCPFixture(t *testing.T) (*client.Client, string) {
	t.Helper()
	root := t.TempDir()
	index := scip.BuildTSFixtureIndex()
	index.Metadata.ProjectRoot = "file://" + filepath.ToSlash(root)
	digest := func(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
	var inputs []scip.SourceInput
	var names []string
	for _, doc := range index.Documents {
		data, err := os.ReadFile(filepath.Join("..", "scip", "testdata", "tsfixture", doc.RelativePath))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, doc.RelativePath), data, 0644); err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, scip.SourceInput{Path: doc.RelativePath, SHA256: digest(data)})
		names = append(names, doc.RelativePath)
	}
	raw, err := proto.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "index.scip")
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}
	receipt := scip.SourceReceipt{Schema: scip.SourceReceiptSchema, Status: "complete", Root: root, IndexSHA256: digest(raw), Producer: scip.SourceProducer{Name: index.Metadata.ToolInfo.Name, Version: index.Metadata.ToolInfo.Version}, Inputs: inputs, Inventories: []scip.SourceInventory{{Root: ".", Pattern: "*.ts", Paths: names}}}
	body, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".receipt.json", body, 0644); err != nil {
		t.Fatal(err)
	}
	c := client.New(connector.NewLocalConnector(), store.NewMemoryStore())
	return c, root
}
func TestSourceReceiptMCPAndDaemonQueueClearRowsAndMemoOnRefusal(t *testing.T) {
	for _, route := range []string{"direct MCP", "daemon auto queue"} {
		for _, fault := range []string{"source edit", "missing receipt", "mixed index"} {
			t.Run(route+"/"+fault, func(t *testing.T) {
				c, root := scipSourceMCPFixture(t)
				srv := mcp.NewServer(c)
				seedIdentCache(t)
				ingestWith(t, srv, map[string]any{"service": "source-mcp", "path": root})
				// The upstream cold symbol_context path can answer before the
				// graph finishes. Prime its real memo stages synchronously so
				// the original stale-memo assertion remains deterministic.
				ctx := context.Background()
				watermark, err := c.StoreWatermark(ctx, "source-mcp")
				if err != nil {
					t.Fatal(err)
				}
				if token := c.SuppressionToken("source-mcp"); token != "" {
					watermark += "|suppressed:" + token
				}
				snap, err := memo.SnapshotFor(ctx, memo.Scope{Store: c.StoreID(), Service: "source-mcp", Watermark: watermark}, func(ctx context.Context) ([]core.SymbolNode, error) {
					return c.ListAllSymbols(ctx, "source-mcp")
				})
				if err != nil {
					t.Fatal(err)
				}
				for _, kind := range []memo.GraphKind{memo.GraphSyntax, memo.GraphBoth, memo.GraphResolved} {
					if _, err := memo.GraphFor(ctx, snap, kind, func(ctx context.Context) ([]core.Edge, error) {
						return c.TypedEdges(ctx, "source-mcp")
					}); err != nil {
						t.Fatal(err)
					}
				}
				callTool(t, srv, "symbol_context", map[string]any{"service": "source-mcp", "symbol": "Greeter.greet", "snippet": false})
				if !srv.HasCachedGraph("source-mcp") {
					t.Fatal("fixture did not build the served graph memo")
				}
				var faultErr error
				switch fault {
				case "source edit":
					faultErr = os.WriteFile(filepath.Join(root, "math.ts"), []byte("export const changed=1;"), 0644)
				case "missing receipt":
					faultErr = os.Remove(filepath.Join(root, "index.scip.receipt.json"))
				case "mixed index":
					faultErr = os.WriteFile(filepath.Join(root, "index.scip"), []byte("index from another attempt"), 0644)
				}
				if faultErr != nil {
					t.Fatal(faultErr)
				}
				if route == "direct MCP" {
					response := roundtrip(t, srv, request{JSONRPC: "2.0", ID: 9551, Method: "tools/call", Params: map[string]any{"name": "ingest", "arguments": map[string]any{"service": "source-mcp"}}})
					encoded, _ := json.Marshal(response)
					if response["error"] == nil || !strings.Contains(string(encoded), "receipt") {
						t.Fatalf("missing source refusal: %v", response)
					}
				} else {
					now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
					q := mcp.NewRefreshQueue(c, refresh.Config{Now: func() time.Time { return now }, StartWait: -1, Stagger: -1, Gate: func(string, bool, time.Time) refresh.Verdict { return refresh.Verdict{} }})
					ctx, cancel := context.WithCancel(context.Background())
					q.Start(ctx)
					defer func() { cancel(); q.Wait() }()
					waitCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
					defer stop()
					status := q.RequestRefresh(waitCtx, refresh.Request{Service: "source-mcp", Reason: refresh.ReasonAuto}, true)
					if !status.Done || status.Err == nil || !strings.Contains(status.Err.Error(), "receipt") {
						t.Fatalf("queue status=%+v", status)
					}
				}
				if srv.HasCachedGraph("source-mcp") {
					t.Fatal("refused source snapshot remains in the served graph memo")
				}
				edges, err := c.TypedEdges(context.Background(), "source-mcp")
				if err != nil {
					t.Fatal(err)
				}
				for _, edge := range edges {
					if edge.Kind == "direct" {
						t.Fatalf("stale typed row survives: %+v", edge)
					}
				}
			})
		}
	}
}
