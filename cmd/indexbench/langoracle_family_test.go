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

// O14: unsupported L1 selection is an argument error, before subject/index/
// oracle reads, extraction or output merging. Native files cannot override an
// explicit unsupported -scip-defs request. These controls are not native proof.
func TestCmdLangOracleDefinitionFamilyRefusesUnsupportedBeforeWork(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent-output", true: "existing-output"}[existing], func(t *testing.T) {
			dir := t.TempDir()
			out, unmatched := filepath.Join(dir, "reading.json"), filepath.Join(dir, "unmatched.json")
			const sentinel = "previous bytes must survive\n"
			if existing {
				if err := os.WriteFile(out, []byte(sentinel), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(unmatched, []byte(sentinel), 0o644); err != nil {
				t.Fatal(err)
			}
			args := []string{"-dialect", "csharp", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "csharp", "dialect.json"),
				"-subject", filepath.Join(dir, "missing-subject"), "-scip", filepath.Join(dir, "missing.scip"), "-scip-defs",
				"-tool-oracle", filepath.Join(dir, "missing-native.json"), "-out", out, "-unmatched", unmatched, "-commit", "test"}
			var stdout, stderr bytes.Buffer
			if code := cmdLangOracle(args, &stdout, &stderr); code != 2 {
				t.Fatalf("exit %d, want argument refusal: %s", code, stderr.String())
			}
			for _, want := range []string{"-scip-defs", `dialect "csharp"`, "oracle_kinds.scip", "no scoreable definition kinds", "-tool-oracle", "-imports-from tool"} {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("missing guidance %q: %s", want, stderr.String())
				}
			}
			if stdout.Len() != 0 {
				t.Errorf("refusal wrote stdout: %s", stdout.String())
			}
			if existing {
				if raw, err := os.ReadFile(out); err != nil || string(raw) != sentinel {
					t.Fatalf("existing reading changed: %q, %v", raw, err)
				}
			} else if _, err := os.Stat(out); !os.IsNotExist(err) {
				t.Fatalf("refusal created reading or status unknown: %v", err)
			}
			if raw, err := os.ReadFile(unmatched); err != nil || string(raw) != sentinel {
				t.Fatalf("unmatched changed: %q, %v", raw, err)
			}
		})
	}
}

func TestCmdLangOracleDefinitionFamilyPreservesNeedsIndexError(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := cmdLangOracle([]string{"-dialect", "csharp", "-subject", dir, "-out", filepath.Join(dir, "reading.json"), "-scip-defs"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "-scip-defs needs -scip") {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if strings.Contains(stderr.String(), "unsupported") {
		t.Fatalf("missing index diagnostic lost precedence: %s", stderr.String())
	}
}

// Synthetic scorer contract: supported C# native definitions and imports use
// separate repeatable tool files while genuine SCIP families stay selected.
// No compiler/exporter ran to create this fixture, and no language grade follows.
func TestCmdLangOracleDefinitionFamilyCSharpNativeToolsRemainSupported(t *testing.T) {
	subject, dir := t.TempDir(), t.TempDir()
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(subject, "A.cs"), "using Lib;\nclass A { }\n")
	write(filepath.Join(subject, "B.cs"), "namespace Lib { class B { } }\n")
	const pre = "scip-dotnet nuget . . App/"
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-dotnet", Version: "test"}}, Documents: []*scippb.Document{
		{RelativePath: "A.cs", Occurrences: []*scippb.Occurrence{
			{Symbol: pre + "A#", Range: []int32{1, 6, 7}, SymbolRoles: int32(scippb.SymbolRole_Definition)},
			{Symbol: pre + "Lib/", Range: []int32{0, 6, 9}}}},
		{RelativePath: "B.cs", Occurrences: []*scippb.Occurrence{
			{Symbol: pre + "Lib/", Range: []int32{0, 10, 13}, SymbolRoles: int32(scippb.SymbolRole_Definition)},
			{Symbol: pre + "Lib/B#", Range: []int32{0, 22, 23}, SymbolRoles: int32(scippb.SymbolRole_Definition)}}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	scip := filepath.Join(dir, "control.scip")
	if err := os.WriteFile(scip, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	defs, imports, out := filepath.Join(dir, "definitions.json"), filepath.Join(dir, "imports.json"), filepath.Join(dir, "reading.json")
	write(defs, `{"tool":"native-definition-control","version":"test","tier":"B","definitions":[
 {"path":"A.cs","name":"A","line":2,"end":2,"kind":"class"},
 {"path":"B.cs","name":"Lib","line":1,"end":1,"kind":"namespace"},
 {"path":"B.cs","name":"B","line":1,"end":1,"kind":"class"}]}`)
	write(imports, `{"tool":"native-import-control","version":"test","tier":"B","imports_semantics":"declaration-membership/v1","imports":[{"path":"A.cs","line":1,"to_path":"B.cs"}]}`)
	args := []string{"-dialect", "csharp", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "csharp", "dialect.json"), "-subject", subject, "-scip", scip, "-imports-from", "tool", "-tool-oracle", defs, "-tool-oracle", imports,
		"-ctags", filepath.Join(dir, "absent"), "-commit", "test", "-out", out}
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, _ := readValidReading(t, out)
	for _, metric := range []string{langbench.MSymRecall, langbench.MSymPrecision, langbench.MImportResolution} {
		if r.Metrics[metric] == nil || *r.Metrics[metric] != 1 || r.MetricTiers[metric] != "B" {
			t.Errorf("native %s: value %v tier %q", metric, r.Metrics[metric], r.MetricTiers[metric])
		}
	}
	if r.Counts[langbench.CntImportsInRepo] != 1 || r.Counts[langbench.CntImportsAgree] != 1 {
		t.Errorf("native import counts: %v", r.Counts)
	}
	if r.MetricTiers[langbench.MDefMapping] != "A" {
		t.Errorf("SCIP definition mapping lost: %v", r.MetricTiers)
	}
	for _, oracle := range r.Oracles {
		if oracle.Tool == "universal-ctags" {
			t.Error("native definitions did not suppress ctags")
		}
	}
}
