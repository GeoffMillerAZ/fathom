package main

// ingest_scip_test.go — ADR-041 decision 5, SU-9 (docs/plans/substrate.md):
// `fathom ingest --scip PATH` and auto-detection of index.scip at the
// service root, end to end through the real CLI entry point.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/scip"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// writeTSFixture copies internal/scip's committed TypeScript fixture (and
// its hand-built SCIP index) into a temp service root, so this test scores
// against the SAME fixture SU-9's own tests pin — one fixture, one truth.
func writeTSFixture(t *testing.T, root string, indexAtRoot bool) {
	t.Helper()
	idx := scip.BuildTSFixtureIndex()
	data, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"math.ts":    "export function add(a: number, b: number): number {\n    return a + b;\n}\n",
		"greeter.ts": "import { add } from \"./math\";\n\nexport class Greeter {\n    greet(name: string): number {\n        console.log(name);\n        return add(1, 2) + name.length;\n    }\n}\n",
		"main.ts":    "import { Greeter } from \"./greeter\";\n\nexport function run(): number {\n    const g = new Greeter();\n    return g.greet(\"hi\");\n}\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var scipPath string
	if indexAtRoot {
		scipPath = filepath.Join(root, "index.scip")
	} else {
		if err := os.MkdirAll(filepath.Join(root, ".fathom", "scip"), 0o755); err != nil {
			t.Fatal(err)
		}
		scipPath = filepath.Join(root, ".fathom", "scip", "tsfixture.scip")
	}
	if err := os.WriteFile(scipPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCmdIngest_ScipAutoDetectAtRoot(t *testing.T) {
	root := t.TempDir()
	writeTSFixture(t, root, true)
	store := filepath.Join(t.TempDir(), "s.sqlite")
	var stdout, stderr bytes.Buffer
	code := cmdIngest([]string{"--service", "scipautoroot", "--path", root, "--store", store}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "typed_edges:") || !strings.Contains(stdout.String(), "via scip-typescript 0.3.16") {
		t.Errorf("expected an auto-detected scip import summary line, got:\n%s", stdout.String())
	}
}

func TestCmdIngest_ScipAutoDetectFathomDir(t *testing.T) {
	root := t.TempDir()
	writeTSFixture(t, root, false)
	store := filepath.Join(t.TempDir(), "s.sqlite")
	var stdout, stderr bytes.Buffer
	code := cmdIngest([]string{"--service", "scipautodir", "--path", root, "--store", store}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "via scip-typescript 0.3.16") {
		t.Errorf("expected .fathom/scip/*.scip to be auto-detected, got:\n%s", stdout.String())
	}
}

func TestCmdIngest_ScipExplicitFlagJSON(t *testing.T) {
	root := t.TempDir()
	writeTSFixture(t, root, true)
	explicitPath := filepath.Join(root, "index.scip")
	store := filepath.Join(t.TempDir(), "s.sqlite")
	var stdout, stderr bytes.Buffer
	code := cmdIngest([]string{"--service", "scipexplicit", "--path", root, "--store", store, "--scip", explicitPath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var out ingestJSON
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, stdout.String())
	}
	if len(out.ScipImports) != 1 {
		t.Fatalf("ScipImports = %+v, want exactly one entry", out.ScipImports)
	}
	si := out.ScipImports[0]
	if si.Producer != "scip-typescript 0.3.16" {
		t.Errorf("Producer = %q, want %q", si.Producer, "scip-typescript 0.3.16")
	}
	if si.Direct != 3 || si.Unresolved != 1 || si.Edges != 4 {
		t.Errorf("ScipImports[0] = %+v, want Direct=3 Unresolved=1 Edges=4", si)
	}
	if si.Unmapped != 0 {
		t.Errorf("ScipImports[0].Unmapped = %d, want 0", si.Unmapped)
	}
}

// unmappableScipIndex returns internal/scip's no-DisplayName fixture with
// one extra definition appended that can be mapped by NEITHER path: a
// "local" symbol carries no package/descriptor path at all (SCIP's own
// grammar for a symbol with no cross-reference identity), so it cannot be
// mapped by position (nothing real sits at line 999) or by descriptor
// fallback (there is no descriptor to parse) — it must be counted
// unmapped, never guessed.
func unmappableScipIndex(t *testing.T) []byte {
	t.Helper()
	idx := scip.BuildTSFixtureIndexNoDisplayNames()
	idx.Documents = append(idx.Documents, &scippb.Document{
		RelativePath: "math.ts",
		Language:     "typescript",
		Occurrences: []*scippb.Occurrence{
			{Range: []int32{998, 0, 998, 5}, EnclosingRange: []int32{998, 0, 999, 1}, Symbol: "local 1", SymbolRoles: int32(scippb.SymbolRole_Definition)},
		},
	})
	data, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestCmdIngest_ScipUnmappedDisclosed is O115's end-to-end disclosure test
// (text output): a SCIP index naming a definition that cannot be joined to
// ANY symbol this ingest extracted must be counted and disclosed on
// stderr — never silently folded into "direct" or "unresolved".
func TestCmdIngest_ScipUnmappedDisclosed(t *testing.T) {
	root := t.TempDir()
	writeTSFixture(t, root, true)
	scipPath := filepath.Join(root, "index.scip")
	if err := os.WriteFile(scipPath, unmappableScipIndex(t), 0o644); err != nil {
		t.Fatal(err)
	}

	store := filepath.Join(t.TempDir(), "s.sqlite")
	var stdout, stderr bytes.Buffer
	code := cmdIngest([]string{"--service", "scipunmapped", "--path", root, "--store", store, "--scip", scipPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "could not be mapped to a fathom symbol") {
		t.Errorf("expected stderr to disclose the unmapped definition (O115), got:\n%s", stderr.String())
	}
}

// TestCmdIngest_ScipUnmappedDisclosedJSON is the same scenario through
// --json: the machine-readable envelope must carry the count too, not only
// the text warning.
func TestCmdIngest_ScipUnmappedDisclosedJSON(t *testing.T) {
	root := t.TempDir()
	writeTSFixture(t, root, true)
	scipPath := filepath.Join(root, "index.scip")
	if err := os.WriteFile(scipPath, unmappableScipIndex(t), 0o644); err != nil {
		t.Fatal(err)
	}

	store := filepath.Join(t.TempDir(), "s.sqlite")
	var stdout, stderr bytes.Buffer
	code := cmdIngest([]string{"--service", "scipunmappedjson", "--path", root, "--store", store, "--scip", scipPath, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var out ingestJSON
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, stdout.String())
	}
	if len(out.ScipImports) != 1 {
		t.Fatalf("ScipImports = %+v, want exactly one entry", out.ScipImports)
	}
	if got := out.ScipImports[0].Unmapped; got != 1 {
		t.Errorf("ScipImports[0].Unmapped = %d, want 1", got)
	}
}
