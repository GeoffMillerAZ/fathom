package extractor

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type simulinkGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Line      int      `json:"line"`
	End       int      `json:"end"`
	Signature string   `json:"signature"`
	Calls     []string `json:"calls,omitempty"`
}

// simulinkFixture reads a fixture: a .mdl as written, a `<name>.slx.d` directory zipped
// into the package `<name>.slx` (its files the parts; no binary is committed).
func simulinkFixture(t *testing.T, dir, name string) (string, []byte) {
	t.Helper()
	p := filepath.Join(dir, name)
	if !strings.HasSuffix(name, ".slx.d") {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		return name, b
	}
	var parts []string
	err := filepath.WalkDir(p, func(f string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(p, f)
		parts = append(parts, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(parts)
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, part := range parts {
		b, err := os.ReadFile(filepath.Join(p, filepath.FromSlash(part)))
		if err != nil {
			t.Fatal(err)
		}
		fw, err := w.Create(part)
		if err != nil {
			t.Fatal(err)
		}
		fw.Write(b)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(name, ".d"), buf.Bytes()
}

// TestSimulinkDialectGoldens: each fixture's symbols, read as sitting at its own basename,
// equal its hand-written golden (LB-Q-simulink).
func TestSimulinkDialectGoldens(t *testing.T) {
	dir := filepath.Join("dialects", "simulink", "testdata")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []string
	for _, e := range ents {
		if n := e.Name(); strings.HasSuffix(n, ".mdl") || (e.IsDir() && strings.HasSuffix(n, ".slx.d")) {
			fixtures = append(fixtures, n)
		}
	}
	sort.Strings(fixtures)
	if len(fixtures) < 3 {
		t.Fatalf("%s: %d fixtures", dir, len(fixtures))
	}
	ex, ok := NativeDialectExtractor("simulink")
	if !ok {
		t.Fatal("no native extractor for simulink")
	}
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			rel, src := simulinkFixture(t, dir, name)
			nodes, err := ex.Extract("svc", rel, src)
			if err != nil {
				t.Fatal(err)
			}
			got := []simulinkGoldenSymbol{}
			for _, n := range nodes {
				got = append(got, simulinkGoldenSymbol{n.Name, string(n.Kind), n.Line, n.EndLine, n.Signature, n.Calls})
			}
			raw, err := os.ReadFile(filepath.Join(dir, rel+".golden.json"))
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want struct {
				Symbols []simulinkGoldenSymbol `json:"symbols"`
			}
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want.Symbols) {
				t.Errorf("symbols\n got: %+v\nwant: %+v", got, want.Symbols)
			}
		})
	}
}
