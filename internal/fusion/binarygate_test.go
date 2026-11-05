package fusion

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
)

// TestBinaryGateExemptsClaimedContainers: the one binary rule (ADR-021 (1), amended
// 2026-10-24) excludes NUL-sniffed content unless a dialect claims its extension as an
// OPC zip model container and the bytes start with the zip local-file signature (the gate
// checks those four bytes, never that the package opens).
func TestBinaryGateExemptsClaimedContainers(t *testing.T) {
	rules := []connector.DialectRule{{ID: "m", Extensions: []string{".slx", ".mdl"}, Containers: []string{".slx"}}}
	zipBytes := []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00")
	cases := []struct {
		path string
		data []byte
		want bool
	}{
		{"m/plant.slx", zipBytes, false},
		{"m/plant.mdl", zipBytes, true},
		{"m/tool.bin", []byte("\x7fELF\x00\x00"), true},
		{"m/plant.slx", []byte("\x7fELF\x00\x00"), true},
		{"m/plant.slx", []byte("Model {\n}\n"), false}, // text is never binary
	}
	for _, c := range cases {
		if got := binaryExcludedWith(rules, c.path, c.data); got != c.want {
			t.Errorf("binaryExcludedWith(%s) = %v, want %v", c.path, got, c.want)
		}
	}
	// The shipped manifests: simulink claims .slx, nothing claims .zip.
	if binaryExcluded("m/plant.slx", zipBytes) || !binaryExcluded("m/bundle.zip", zipBytes) {
		t.Error("the shipped gate's container claims are not simulink's .slx alone")
	}
}

// TestExtractForBenchReadsSimulinkPackage: a .slx clears ingest's gates and dispatches to
// the simulink dialect (LB-Q-simulink), where it was fingerprinted as binary before.
func TestExtractForBenchReadsSimulinkPackage(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("simulink/systems/system_root.xml")
	f.Write([]byte(`<System><Block BlockType="Gain" Name="K" SID="1"/></System>`))
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	p := New(nil, nil, Config{})
	defer p.Close()
	ex, err := p.ExtractForBench("svc", nil, "models/plant.slx", buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if ex.Skipped != "" || len(ex.Nodes) != 2 || ex.Nodes[1].Name != "plant/K" {
		t.Errorf("skipped %q, nodes %+v", ex.Skipped, ex.Nodes)
	}
}

// TestBinaryGateIsTheOnlySniff: every read path in the routing and fusion
// packages asks the gate (langroute.binaryExcluded), never connector.IsBinary or
// langclass.IsBinary directly, so the container exemption is one rule, not
// nineteen. langroute/binarygate.go is the one place that sniffs.
func TestBinaryGateIsTheOnlySniff(t *testing.T) {
	var files []string
	for _, dir := range []string{".", filepath.Join("..", "langroute")} {
		fs, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, fs...)
	}
	if len(files) == 0 {
		t.Fatal("no source files found to scan")
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") || (filepath.Base(f) == "binarygate.go" && filepath.Base(filepath.Dir(f)) == "langroute") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, call := range []string{"connector.IsBinary(", "langclass.IsBinary("} {
			if strings.Contains(string(b), call) {
				t.Errorf("%s calls %s; use the gate in internal/langroute/binarygate.go", f, call)
			}
		}
	}
}
