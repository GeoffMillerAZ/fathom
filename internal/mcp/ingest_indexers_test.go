package mcp_test

// ingest_indexers_test.go — SR-65 (surface reckoning F-74, owner decision):
// the ingest tool gains an additive optional `indexers` parameter. Naming
// scip-typescript and/or scip-python asks the daemon to import the SCIP
// index that producer left under the service root's .fathom/scip/ (`fathom
// init` runs the toolchain when it is installed and never installs one),
// after the syntax pass and under the ingest slot and the admission budget.
// An index that cannot be imported is disclosed per indexer, never silently
// dropped and never a failed ingest: the syntax index is already written.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/scip"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// scipImportBackend is a backend whose ImportSCIP records what it was asked
// to import and whether the ingest slot was held while it ran.
type scipImportBackend struct {
	extendedFakeBackend
	imported []string
	slotFree bool
	err      error
}

func (b *scipImportBackend) IngestService(context.Context, string) error { return nil }

func (b *scipImportBackend) ImportSCIP(_ context.Context, _ string, path string) (scip.Import, error) {
	b.imported = append(b.imported, path)
	if release, ok := tenancy.TryIngestSlot(); ok {
		b.slotFree = true
		release()
	}
	if b.err != nil {
		return scip.Import{}, b.err
	}
	return scip.Import{Report: scip.Report{
		Producer: scip.Producer{Name: "scip-typescript", Version: "0.3.16"},
		Edges:    12, Direct: 9, Unresolved: 3, Unmapped: 1,
	}}, nil
}

func scipRoot(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		p := filepath.Join(root, ".fathom", "scip", name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func ingestWith(t *testing.T, srv *mcp.Server, args map[string]any) map[string]any {
	t.Helper()
	return extToolPayload(t, roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 6501, Method: "tools/call",
		Params: map[string]any{"name": "ingest", "arguments": args},
	}))
}

func TestIngestImportsTheNamedIndexersProducedIndex(t *testing.T) {
	root := scipRoot(t, map[string]string{"scip-typescript.scip": "not-really-protobuf"})
	b := &scipImportBackend{extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}, svcPath: root}}
	seedIdentCache(t)

	p := ingestWith(t, mcp.NewServer(b), map[string]any{"service": "svc", "indexers": []string{"scip-typescript"}})
	if p["ok"] != true {
		t.Fatalf("ok = %v: %v", p["ok"], p)
	}
	if len(b.imported) != 1 || b.imported[0] != filepath.Join(root, ".fathom", "scip", "scip-typescript.scip") {
		t.Fatalf("imported %v, want exactly the produced .fathom/scip/scip-typescript.scip", b.imported)
	}
	if b.slotFree {
		t.Error("the import ran with the ingest slot free; it must hold it (RT-160) like the ingest itself")
	}
	rows, _ := p["scip"].([]any)
	if len(rows) != 1 {
		t.Fatalf("scip = %v, want one row per named indexer", p["scip"])
	}
	row := rows[0].(map[string]any)
	if row["indexer"] != "scip-typescript" || row["edges"] != float64(12) || row["direct"] != float64(9) || row["unresolved"] != float64(3) {
		t.Errorf("scip row = %v, want the import's counts", row)
	}
	if row["unmapped"] != float64(1) {
		t.Errorf("scip row = %v, want the unmapped definition count disclosed (O115)", row)
	}
	if row["producer"] != "scip-typescript 0.3.16" {
		t.Errorf("producer = %v", row["producer"])
	}
}

func TestIngestWithoutIndexersImportsNothing(t *testing.T) {
	root := scipRoot(t, map[string]string{"scip-typescript.scip": "x"})
	b := &scipImportBackend{extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}, svcPath: root}}
	seedIdentCache(t)
	p := ingestWith(t, mcp.NewServer(b), map[string]any{"service": "svc"})
	if len(b.imported) != 0 {
		t.Errorf("imported %v without an indexers parameter", b.imported)
	}
	if _, present := p["scip"]; present {
		t.Errorf("a call that named no indexer carries a scip block: %v", p["scip"])
	}
}

func TestIngestIndexerDisclosures(t *testing.T) {
	seedIdentCache(t)
	t.Run("missing index", func(t *testing.T) {
		root := scipRoot(t, nil)
		b := &scipImportBackend{extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}, svcPath: root}}
		p := ingestWith(t, mcp.NewServer(b), map[string]any{"service": "svc", "indexers": []string{"scip-python"}})
		row := p["scip"].([]any)[0].(map[string]any)
		if row["indexer"] != "scip-python" || !strings.Contains(row["error"].(string), ".fathom/scip/scip-python.scip") {
			t.Errorf("row = %v, want the missing index named", row)
		}
		if len(b.imported) != 0 || p["ok"] != true {
			t.Errorf("imported %v ok %v: a missing index must not fail the ingest or import anything", b.imported, p["ok"])
		}
	})
	t.Run("failing import", func(t *testing.T) {
		root := scipRoot(t, map[string]string{"scip-python.scip": "broken"})
		b := &scipImportBackend{extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}, svcPath: root}, err: os.ErrInvalid}
		p := ingestWith(t, mcp.NewServer(b), map[string]any{"service": "svc", "indexers": []string{"scip-python"}})
		row := p["scip"].([]any)[0].(map[string]any)
		if !strings.Contains(row["error"].(string), "invalid argument") {
			t.Errorf("row = %v, want the import's own error", row)
		}
		if p["ok"] != true {
			t.Errorf("ok = %v: the syntax index is written, so a failed enrichment is disclosed per indexer, not a failed ingest", p["ok"])
		}
	})
	t.Run("over the import budget", func(t *testing.T) {
		root := scipRoot(t, map[string]string{"scip-typescript.scip": strings.Repeat("x", 4096)})
		b := &scipImportBackend{extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}, svcPath: root}}
		srv := mcp.NewServer(b)
		srv.SetSCIPImportBudget(1024)
		p := ingestWith(t, srv, map[string]any{"service": "svc", "indexers": []string{"scip-typescript"}})
		row := p["scip"].([]any)[0].(map[string]any)
		if !strings.Contains(row["error"].(string), "budget") {
			t.Errorf("row = %v, want the budget named", row)
		}
		if len(b.imported) != 0 {
			t.Errorf("an index over the budget was imported: %v", b.imported)
		}
	})
	t.Run("backend cannot import", func(t *testing.T) {
		root := scipRoot(t, map[string]string{"scip-typescript.scip": "x"})
		b := &extendedFakeBackend{services: []string{"svc"}, svcPath: root}
		p := ingestWith(t, mcp.NewServer(b), map[string]any{"service": "svc", "indexers": []string{"scip-typescript"}})
		row := p["scip"].([]any)[0].(map[string]any)
		if !strings.Contains(row["error"].(string), "does not import") {
			t.Errorf("row = %v, want the unsupported backend named", row)
		}
	})
}

func TestIngestRefusesAnUnknownIndexer(t *testing.T) {
	root := scipRoot(t, nil)
	b := &scipImportBackend{extendedFakeBackend: extendedFakeBackend{services: []string{"svc"}, svcPath: root}}
	resp := roundtrip(t, mcp.NewServer(b), request{
		JSONRPC: "2.0", ID: 6502, Method: "tools/call",
		Params: map[string]any{"name": "ingest", "arguments": map[string]any{"service": "svc", "indexers": []string{"../../etc/passwd"}}},
	})
	if resp["error"] == nil {
		t.Fatalf("an indexer outside the allowlist was accepted: %v", resp)
	}
	msg := resp["error"].(map[string]any)["message"].(string)
	if !strings.Contains(msg, "scip-typescript") || !strings.Contains(msg, "scip-python") {
		t.Errorf("message %q must name the two producers that are allowed", msg)
	}
}
