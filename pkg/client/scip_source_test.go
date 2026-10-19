package client_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/scip"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
	"google.golang.org/protobuf/proto"
)

func scipSourceFixture(t *testing.T) (*client.Client, store.Store, string, string) {
	t.Helper()
	root := t.TempDir()
	st := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), st)
	for name, body := range scipO116TSFixtureSources {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	idx := scip.BuildTSFixtureIndex()
	idx.Metadata.ProjectRoot = "file://" + filepath.ToSlash(root)
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "index.scip")
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}
	hash := func(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
	var inputs []scip.SourceInput
	var names []string
	for _, name := range []string{"greeter.ts", "main.ts", "math.ts"} {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, scip.SourceInput{Path: name, SHA256: hash(body)})
		names = append(names, name)
	}
	receipt := scip.SourceReceipt{Schema: scip.SourceReceiptSchema, Status: "complete", Root: root, IndexSHA256: hash(raw), Producer: scip.SourceProducer{Name: idx.Metadata.ToolInfo.Name, Version: idx.Metadata.ToolInfo.Version}, Inputs: inputs, Inventories: []scip.SourceInventory{{Root: ".", Pattern: "*.ts", Paths: names}}}
	data, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".receipt.json", data, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Ingest(context.Background(), client.ServiceRef{ID: scipO116Service, Path: root}); err != nil {
		t.Fatal(err)
	}
	return c, st, root, path
}
func scipSourceEdges(t *testing.T, c *client.Client) []core.Edge {
	t.Helper()
	edges, err := c.TypedEdges(context.Background(), scipO116Service)
	if err != nil {
		t.Fatal(err)
	}
	var out []core.Edge
	for _, e := range edges {
		if e.Kind == core.EdgeDirect {
			out = append(out, e)
		}
	}
	return out
}
func TestSCIPSourceReceiptFreshAutoImportAcrossClientPaths(t *testing.T) {
	c, _, _, _ := scipSourceFixture(t)
	if got := len(scipSourceEdges(t, c)); got != 3 {
		t.Fatalf("fresh direct SCIP edges=%d, want3", got)
	}
}
func TestSCIPSourceRefusalClearsOnlyItsOwnPriorRows(t *testing.T) {
	for _, entry := range []string{"import", "ingest", "daemon", "onboard", "file-refresh"} {
		t.Run(entry, func(t *testing.T) {
			c, st, root, path := scipSourceFixture(t)
			ctx := context.Background()
			if got := len(scipSourceEdges(t, c)); got != 3 {
				t.Fatalf("initial edges=%d", got)
			}
			if err := st.PutTypedEdges(ctx, scipO116Service, "scip:other.scip::independent", []core.Edge{{FromSymbol: "unrelated", ToSymbol: "safe", Kind: core.EdgeDirect}}); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "math.ts"), []byte("export function add(a:number,b:number){return a-b;}\n"), 0644); err != nil {
				t.Fatal(err)
			}
			var err error
			switch entry {
			case "import":
				_, err = c.ImportSCIP(ctx, scipO116Service, path)
			case "ingest":
				_, err = c.Ingest(ctx, client.ServiceRef{ID: scipO116Service, Path: root})
			case "daemon":
				err = c.IngestService(ctx, scipO116Service)
			case "onboard":
				err = c.IngestServiceAt(ctx, scipO116Service, root)
			case "file-refresh":
				_, err = c.ReingestFile(ctx, scipO116Service, "math.ts")
			}
			if err == nil || !strings.Contains(err.Error(), "receipt") {
				t.Fatalf("err=%v, want receipt refusal", err)
			}
			edges := scipSourceEdges(t, c)
			if len(edges) != 1 || edges[0].FromSymbol != "unrelated" {
				t.Fatalf("stale edges survived or unrelated rows removed: %+v", edges)
			}
		})
	}
}
func TestSCIPSourceMissingReceiptAndIndexCannotKeepOldRows(t *testing.T) {
	for _, deleted := range []string{"receipt", "index"} {
		t.Run(deleted, func(t *testing.T) {
			c, _, _, path := scipSourceFixture(t)
			victim := path
			if deleted == "receipt" {
				victim += ".receipt.json"
			}
			if err := os.Remove(victim); err != nil {
				t.Fatal(err)
			}
			if err := c.IngestService(context.Background(), scipO116Service); err == nil {
				t.Fatal("missing bound artifact accepted")
			}
			if edges := scipSourceEdges(t, c); len(edges) != 0 {
				t.Fatalf("stale edges survived: %+v", edges)
			}
		})
	}
}
