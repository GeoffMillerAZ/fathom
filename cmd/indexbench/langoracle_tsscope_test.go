package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/langbench"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// TestCmdLangOracleTSXScopeAndDefScope (owner ruling L5, K19, O749; LB-F467,
// LB-F468): the TypeScript reading measures its tsx variant's .tsx files, and
// lang-oracle hands the measured files to def_mapping, so a definition the
// index writes in a document the reading does not measure (here a script
// -only leaves out) is out of its denominator, counted and disclosed.
func TestCmdLangOracleTSXScopeAndDefScope(t *testing.T) {
	subject := t.TempDir()
	files := map[string]string{
		"src/math.ts":    "export function add(a: number, b: number) {\n  return a + b;\n}\n",
		"src/Button.tsx": "import { add } from \"./math\";\nexport function Button() {\n  return <b>{add(1, 2)}</b>;\n}\n",
		"scripts/gen.ts": "export function gen() {\n  return 1;\n}\n",
	}
	for p, src := range files {
		full := filepath.Join(subject, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	def := func(sym string, enc []int32, rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: sym, Range: rng, EnclosingRange: enc, SymbolRoles: int32(scippb.SymbolRole_Definition)}
	}
	ref := func(sym string, rng ...int32) *scippb.Occurrence { return &scippb.Occurrence{Symbol: sym, Range: rng} }
	const math, button, gen = "scip-typescript npm fx 1.0.0 src/`math.ts`/", "scip-typescript npm fx 1.0.0 src/`Button.tsx`/", "scip-typescript npm fx 1.0.0 scripts/`gen.ts`/"
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-typescript", Version: "0.4.0"}},
		Documents: []*scippb.Document{
			{RelativePath: "src/math.ts", Occurrences: []*scippb.Occurrence{def(math+"add().", []int32{0, 0, 2, 1}, 0, 16, 19)}},
			{RelativePath: "src/Button.tsx", Occurrences: []*scippb.Occurrence{
				ref(math+"add().", 0, 9, 12),
				def(button+"Button().", []int32{1, 0, 3, 1}, 1, 16, 22),
				ref(math+"add().", 2, 14, 17),
			}},
			{RelativePath: "scripts/gen.ts", Occurrences: []*scippb.Occurrence{def(gen+"gen().", []int32{0, 0, 2, 1}, 0, 16, 19)}},
		}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	scip := filepath.Join(t.TempDir(), "typescript.scip")
	if err := os.WriteFile(scip, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join("..", "..", "internal", "extractor", "dialects", "typescript", "dialect.json")
	queue := filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")
	out := filepath.Join(t.TempDir(), "typescript.json")
	var stdout, stderr bytes.Buffer
	args := []string{"-dialect", "typescript", "-manifest", manifest, "-subject", subject, "-subject-name", "fx",
		"-ctags", filepath.Join(t.TempDir(), "absent"), "-scip", scip, "-only", "src/**", "-out", out, "-queue", queue}
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r := readReading(t, out)
	if len(r.Subjects) != 1 || r.Subjects[0].Files != 2 {
		t.Fatalf("measured files = %+v, want src/math.ts and src/Button.tsx", r.Subjects)
	}
	c := r.Subjects[0].Counts
	if c[langbench.CntScipDefs] != 2 || c[langbench.CntScipDefsMapped] != 2 || c[langbench.CntScipDefsOutOfScope] != 1 {
		t.Errorf("def_mapping: defs %d mapped %d out of scope %d, want 2 2 1 (Button.tsx in, scripts/gen.ts out)", c[langbench.CntScipDefs], c[langbench.CntScipDefsMapped], c[langbench.CntScipDefsOutOfScope])
	}
	found := false
	for _, n := range r.Notes {
		found = found || strings.Contains(n, "fx: 1 definitions the index writes in documents outside") && strings.Contains(n, "L5")
	}
	if !found {
		t.Errorf("notes %q do not disclose ruling L5's count", r.Notes)
	}
}
