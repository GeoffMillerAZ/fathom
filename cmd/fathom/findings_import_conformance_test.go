package main

// findings_import_conformance_test.go — plan FL-14: every fixture under
// testdata/contracts/findings-import/ runs through the CLI (fathom findings
// import --json) AND through pkg/client.Client.ImportFindings, and the two
// receipts must be identical — the one code path both share
// (pkg/client/findings.go's own doc comment). A fixture whose expected.json
// says parseError:true must refuse on BOTH paths, writing nothing.

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/findings/external"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// findingsImportFixture is the input manifest shape testdata/contracts/
// findings-import/<name>.json carries.
type findingsImportFixture struct {
	Service    string          `json:"service"`
	Subject    string          `json:"subject"`
	Producer   string          `json:"producer"`
	DryRun     bool            `json:"dryRun"`
	Projection json.RawMessage `json:"projection"`
}

// findingsImportExpected is testdata/contracts/findings-import/
// <name>.expected.json's shape.
type findingsImportExpected struct {
	ParseError bool              `json:"parseError"`
	Receipt    *external.Receipt `json:"receipt"`
}

func findingsImportFixtureDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "contracts", "findings-import")
}

// findingsImportFixtureNames lists every fixture base name (the "<name>" in
// "<name>.json" / "<name>.expected.json"), sorted.
func findingsImportFixtureNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir %s: %v", dir, err)
	}
	var names []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".expected.json") || !strings.HasSuffix(name, ".json") {
			continue
		}
		names = append(names, strings.TrimSuffix(name, ".json"))
	}
	sort.Strings(names)
	return names
}

// writeConformanceCorpus writes the small fixed corpus every fixture's
// anchors point into.
func writeConformanceCorpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	content := []byte("line1\nline2\nline3\nline4\nline5\n")
	for _, rel := range []string{"internal/foo/bar.go", "internal/foo/baz.go"} {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// seedConformanceStore creates a fresh SQLite store at path, records
// service_meta pointing at corpusRoot, and pre-seeds fingerprints for the
// fixture corpus (so every fixture's NoFingerprint stays 0 on both the CLI
// and the client path — the run is deterministic, not a coin flip on
// whether ingest happened to run first).
func seedConformanceStore(t *testing.T, path, service, corpusRoot string) {
	t.Helper()
	st, err := store.NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	ctx := context.Background()
	if err := st.SetServiceMeta(ctx, store.ServiceMeta{ServiceID: service, Path: corpusRoot}); err != nil {
		t.Fatalf("SetServiceMeta: %v", err)
	}
	for _, rel := range []string{"internal/foo/bar.go", "internal/foo/baz.go"} {
		if err := st.SetFingerprint(ctx, service, rel, "fp-"+rel); err != nil {
			t.Fatalf("SetFingerprint: %v", err)
		}
	}
	if err := st.Close(); err != nil {
		t.Fatalf("close seeded store: %v", err)
	}
}

func TestFindingsImportConformance(t *testing.T) {
	dir := findingsImportFixtureDir(t)
	names := findingsImportFixtureNames(t, dir)
	if len(names) == 0 {
		t.Fatalf("no fixtures found under %s", dir)
	}

	for _, name := range names {
		name := name
		t.Run(name, func(t *testing.T) {
			rawFixture, err := os.ReadFile(filepath.Join(dir, name+".json"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var fx findingsImportFixture
			if err := json.Unmarshal(rawFixture, &fx); err != nil {
				t.Fatalf("decode fixture: %v", err)
			}
			rawExpected, err := os.ReadFile(filepath.Join(dir, name+".expected.json"))
			if err != nil {
				t.Fatalf("read expected: %v", err)
			}
			var expected findingsImportExpected
			if err := json.Unmarshal(rawExpected, &expected); err != nil {
				t.Fatalf("decode expected: %v", err)
			}

			corpusRoot := writeConformanceCorpus(t)
			projectionBytes, err := json.Marshal(fx.Projection)
			if err != nil {
				t.Fatalf("marshal projection: %v", err)
			}

			// ─── CLI path ───────────────────────────────────────────────
			cliStorePath := filepath.Join(t.TempDir(), "cli.sqlite")
			seedConformanceStore(t, cliStorePath, fx.Service, corpusRoot)
			projFile := filepath.Join(t.TempDir(), "projection.json")
			if err := os.WriteFile(projFile, projectionBytes, 0o644); err != nil {
				t.Fatal(err)
			}
			args := []string{
				"--store", cliStorePath, "--service", fx.Service, "--subject", fx.Subject,
				"--file", projFile, "--json",
			}
			if fx.Producer != "" {
				args = append(args, "--producer", fx.Producer)
			}
			if fx.DryRun {
				args = append(args, "--dry-run")
			}
			var cliOut, cliErr bytes.Buffer
			cliCode := cmdFindingsImport(args, &cliOut, &cliErr)

			// ─── client path ────────────────────────────────────────────
			clientStorePath := filepath.Join(t.TempDir(), "client.sqlite")
			seedConformanceStore(t, clientStorePath, fx.Service, corpusRoot)
			clientSt, err := store.Open(clientStorePath)
			if err != nil {
				t.Fatalf("open client store: %v", err)
			}
			defer clientSt.Close()
			c := client.New(connector.NewLocalConnector(), clientSt)
			clientReceipt, clientErr := c.ImportFindings(context.Background(), client.ImportFindingsRequest{
				Service: fx.Service, Subject: fx.Subject, Producer: fx.Producer,
				Projection: projectionBytes, DryRun: fx.DryRun,
			})

			if expected.ParseError {
				if cliCode != 2 {
					t.Errorf("CLI exit code = %d, want 2 (parse refusal)", cliCode)
				}
				if cliErr.Len() == 0 {
					t.Error("CLI stderr empty, want a refusal message")
				}
				if clientErr == nil {
					t.Error("client.ImportFindings: expected an error, got nil")
				}
				return
			}

			if clientErr != nil {
				t.Fatalf("client.ImportFindings: %v", clientErr)
			}
			wantCode := 0
			if len(clientReceipt.Rejected) > 0 {
				wantCode = 1
			}
			if cliCode != wantCode {
				t.Errorf("CLI exit code = %d, want %d\nstderr: %s", cliCode, wantCode, cliErr.String())
			}

			var cliReceipt external.Receipt
			if err := json.Unmarshal(cliOut.Bytes(), &cliReceipt); err != nil {
				t.Fatalf("decode CLI --json output: %v\noutput: %s", err, cliOut.String())
			}

			if !reflect.DeepEqual(cliReceipt, clientReceipt) {
				t.Errorf("CLI and client receipts differ:\nCLI:    %+v\nclient: %+v", cliReceipt, clientReceipt)
			}
			if expected.Receipt != nil && !reflect.DeepEqual(clientReceipt, *expected.Receipt) {
				t.Errorf("receipt does not match fixture's expected.json:\ngot:      %+v\nexpected: %+v", clientReceipt, *expected.Receipt)
			}
		})
	}
}
