package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/scipexport"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

const exportSCIPFixtureSrc = `package widget

// Greet returns a greeting.
func Greet(name string) string {
	return prefix() + name
}

// prefix is called by Greet.
func prefix() string {
	return "hello, "
}
`

func TestExportSCIP_WritesValidIndex(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "widget.go"), []byte(exportSCIPFixtureSrc), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	storePath := filepath.Join(t.TempDir(), "s.sqlite")

	var ingestOut, ingestErr bytes.Buffer
	if code := run([]string{"ingest", "--service", "export-scip-svc", "--path", root, "--store", storePath}, &ingestOut, &ingestErr); code != 0 {
		t.Fatalf("ingest exit = %d, stderr: %s", code, ingestErr.String())
	}

	outPath := filepath.Join(t.TempDir(), "index.scip")
	var stdout, stderr bytes.Buffer
	code := run([]string{"export", "scip", "--service", "export-scip-svc", "--store", storePath, "-o", outPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("export scip exit = %d, stderr: %s", code, stderr.String())
	}

	if !bytes.Contains(stdout.Bytes(), []byte("wrote "+outPath)) {
		t.Errorf("stdout missing 'wrote %s' line: %s", outPath, stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("round-trip (bindings-decode)")) {
		t.Errorf("stdout missing round-trip summary line (bindings-decode expected: scip-go is not installed in CI): %s", stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(" — ok")) {
		t.Errorf("round-trip did not report ok: %s", stdout.String())
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read %s: %v", outPath, err)
	}
	idx, err := scipexport.DecodeIndex(data)
	if err != nil {
		t.Fatalf("decode written index: %v", err)
	}
	if len(idx.Documents) != 1 || idx.Documents[0].RelativePath != "widget.go" {
		t.Fatalf("Documents = %+v, want one document for widget.go", idx.Documents)
	}
	if idx.Metadata == nil || idx.Metadata.ToolInfo == nil || idx.Metadata.ToolInfo.Name != "fathom" {
		t.Errorf("Metadata.ToolInfo = %+v, want Name fathom", idx.Metadata)
	}

	// The decoded symbol table must match what the store itself reports —
	// the actual round-trip proof, independent of scipexport's own
	// RoundTrip() bookkeeping.
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()
	nodes, err := c.ListAllSymbols(context.Background(), "export-scip-svc")
	if err != nil {
		t.Fatalf("list symbols: %v", err)
	}
	want := scipexport.ExpectedSymbolSet("export-scip-svc", nodes)
	got := scipexport.SymbolSet(idx)
	if len(want) == 0 {
		t.Fatal("fixture produced zero symbols — test fixture is broken")
	}
	for w := range want {
		if !got[w] {
			t.Errorf("decoded index missing expected symbol %q", w)
		}
	}
	for g := range got {
		if !want[g] {
			t.Errorf("decoded index has unexpected symbol %q not in the store's own list", g)
		}
	}

	// A typed call edge (Greet -> prefix) should have become a Relationship
	// carrying a resolution-tier Documentation line, IF the typed pass ran
	// for this service; best-effort — assert only when typed edges exist,
	// since whether the Go typed pass ran is an ingest-time property this
	// test does not control.
	typed, _ := c.TypedEdges(context.Background(), "export-scip-svc")
	if len(typed) > 0 {
		foundRelationship := false
		for _, d := range idx.Documents {
			for _, s := range d.Symbols {
				if len(s.Relationships) > 0 {
					foundRelationship = true
				}
			}
		}
		if !foundRelationship {
			t.Errorf("service has %d typed edges but no SymbolInformation carries a Relationship", len(typed))
		}
	}
}

func TestExportSCIP_RequiresService(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"export", "scip", "--store", filepath.Join(t.TempDir(), "s.sqlite")}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (missing --service)", code)
	}
}

func TestExportSCIP_UnknownServiceErrors(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	// An empty-but-valid store: opening it succeeds, but the requested
	// service has never been ingested.
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	st.Close()

	var stdout, stderr bytes.Buffer
	code := run([]string{"export", "scip", "--service", "nowhere", "--store", storePath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (unindexed service)", code)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("no indexed symbols")) {
		t.Errorf("stderr = %q, want a 'no indexed symbols' message", stderr.String())
	}
}

func TestExportSCIP_UnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"export", "bogus"}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("unknown export subcommand")) {
		t.Errorf("stderr = %q, want an 'unknown export subcommand' message", stderr.String())
	}
}

func TestExportSCIP_NoSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"export"}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
}
