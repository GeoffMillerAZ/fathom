package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/langbench"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// TestCUDADriverOffline keeps the driver regressions in the Go test gate.
// Its compiler and oracle processes are local stubs.
func TestCUDADriverOffline(t *testing.T) {
	cmd := exec.Command("python3", filepath.Join("..", "..", "calibration", "oracles", "scip-clang", "cuda-samples_test.py"))
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("offline CUDA driver regressions: %v\n%s", err, out)
	}
}

// cudaTiersSubject writes a two-file CUDA subject (k.cu includes u.cuh and
// calls its helper), its scip-clang index, a tier-A preprocessor file with
// the include, and a tier-A clang-calls file, and returns the directory the
// oracle files are in and lang-oracle's arguments over them; rangesTier, when
// not "", writes preproc.ranges.json, the ranges companion, at that tier.
func cudaTiersSubject(t *testing.T, rangesTier string, extra ...string) (string, []string) {
	t.Helper()
	subject := t.TempDir()
	for p, src := range map[string]string{
		"k.cu":  "#include \"u.cuh\"\nextern \"C\" void launch(int n) {\n  helper(n);\n}\n",
		"u.cuh": "inline int helper(int n) {\n  return n;\n}\n",
	} {
		if err := os.WriteFile(filepath.Join(subject, p), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-clang", Version: "0.4.0"}},
		Documents: []*scippb.Document{
			{RelativePath: "k.cu", Occurrences: []*scippb.Occurrence{
				{Symbol: "cxx . . $ helper(0000000000000002).", Range: []int32{2, 2, 8}},
			}},
			{RelativePath: "u.cuh", Occurrences: []*scippb.Occurrence{
				{Symbol: "cxx . . $ helper(0000000000000002).", Range: []int32{0, 11, 17}, SymbolRoles: int32(scippb.SymbolRole_Definition)},
			}},
		}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	scip := filepath.Join(dir, "cuda.scip")
	files := map[string]string{
		"preproc.json": `{"tool": "clang-preprocessor", "version": "test", "tier": "A", "imports": [{"path": "k.cu", "line": 1, "to_path": "u.cuh"}]}`,
		"clang-calls.json": `{"tool": "clang-ast", "version": "test clang", "tier": "A", "call_sites": [
 {"path": "k.cu", "line": 3, "col": 3, "name": "helper", "class": "resolved"}]}`,
	}
	if rangesTier != "" {
		files["preproc.ranges.json"] = `{"tool": "clang-ast", "version": "test clang", "tier": "` + rangesTier + `", "ranges": [
 {"path": "k.cu", "name": "launch", "kind": "function", "line": 2, "end": 4, "extern_c": true},
 {"path": "u.cuh", "name": "helper", "kind": "function", "line": 1, "end": 3}]}`
		files["preproc.units.json"] = `{"tool": "cmake-ninja-link-lines", "tier": "A", "link_units": {"k": ["k.cu"]}}`
	}
	for p, body := range files {
		if err := os.WriteFile(filepath.Join(dir, p), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(scip, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, append([]string{"-dialect", "cuda", "-manifest", filepath.Join("..", "..", "internal", "extractor", "dialects", "cuda", "dialect.json"), "-subject", subject,
		"-ctags", filepath.Join(t.TempDir(), "none"), "-scip", scip, "-scip-tier", "A",
		"-tool-oracle", filepath.Join(dir, "preproc.json"), "-tool-oracle", filepath.Join(dir, "clang-calls.json"),
		"-out", filepath.Join(t.TempDir(), "cuda.json"), "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}, extra...)
}

// TestCmdLangOracleCUDACompanionTiers (round 5 C5-R4): with the index and the
// call sites at tier A, a tier-B ranges companion lowers the families it
// shapes, file_edges (the linkage rule) and, under CUDA, calls (the compiler
// stand-ins), to B; the reading names clang-ast-defs and the link-unit
// companion as oracles of their own.
func TestCmdLangOracleCUDACompanionTiers(t *testing.T) {
	_, args := cudaTiersSubject(t, "B")
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, raw := readValidReading(t, args[len(args)-3])
	if len(r.Subjects) != 1 {
		t.Fatalf("subjects = %d, want 1", len(r.Subjects))
	}
	for f, want := range map[string]string{langbench.FamilyFileEdges: "B", langbench.FamilyCalls: "B"} {
		if got := r.Subjects[0].Families[f]; got != want {
			t.Errorf("families.%s = %q, want %q\n%s", f, got, want, raw)
		}
	}
	tiers := map[string]string{}
	for _, o := range r.Oracles {
		tiers[o.Tool] = o.Tier
	}
	for tool, want := range map[string]string{"clang-ast-defs": "B", "cmake-ninja-link-lines": "A", "clang-ast": "A", "scip-clang": "A"} {
		if tiers[tool] != want {
			t.Errorf("oracles[%s] tier %q, want %q: %+v", tool, tiers[tool], want, r.Oracles)
		}
	}
	// With no companion the same families stay at A.
	_, args = cudaTiersSubject(t, "")
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("no companion: exit %d: %s", code, stderr.String())
	}
	r, raw = readValidReading(t, args[len(args)-3])
	for _, f := range []string{langbench.FamilyFileEdges, langbench.FamilyCalls} {
		if got := r.Subjects[0].Families[f]; got != "A" {
			t.Errorf("no companion: families.%s = %q, want A\n%s", f, got, raw)
		}
	}
}

// TestCmdLangOracleCUDAUntieredImportDump (round 5 C5-R4): an imports
// dump naming no tier is refused both as import-resolution truth (with an
// index) and file-edge truth (without one). The latter diagnostic proves
// scoreImportFileEdges runs, rather than the earlier imports check hiding it.
func TestCmdLangOracleCUDAUntieredImportDump(t *testing.T) {
	for _, tc := range []struct {
		name      string
		withIndex bool
		want      string
	}{
		{"imports", true, "names no tier A-D"},
		{"file-edges", false, "file-edge truth"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, args := cudaTiersSubject(t, "")
			pre := filepath.Join(dir, "preproc.json")
			if err := os.WriteFile(pre, []byte(`{"tool": "clang-preprocessor", "version": "test", "imports": [{"path": "k.cu", "line": 1, "to_path": "u.cuh"}]}`), 0o644); err != nil {
				t.Fatal(err)
			}
			var kept []string
			for i := 0; i < len(args); i++ {
				switch args[i] {
				case "-scip", "-scip-tier":
					if !tc.withIndex {
						i++
						continue
					}
				case "-tool-oracle":
					if args[i+1] != pre {
						i++
						continue
					}
				}
				kept = append(kept, args[i])
			}
			var stdout, stderr bytes.Buffer
			if code := cmdLangOracle(kept, &stdout, &stderr); code == 0 || !strings.Contains(stderr.String(), pre) || !strings.Contains(stderr.String(), tc.want) {
				t.Errorf("untiered imports dump: exit %d, want a refusal naming it and %q: %s", code, tc.want, stderr.String())
			}
		})
	}
}

// TestCmdLangParitySCIPClangTier (round 5 C5-R4): lang-parity, like
// lang-oracle, refuses a scip-clang index that names no tier.
func TestCmdLangParitySCIPClangTier(t *testing.T) {
	_, args := cudaTiersSubject(t, "")
	var parity []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-scip-tier":
			i++
			continue
		}
		parity = append(parity, args[i])
	}
	var stdout, stderr bytes.Buffer
	if code := cmdLangParity(parity, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "-scip-tier") {
		t.Errorf("a scip-clang index without a tier: exit %d, want 2 naming -scip-tier: %s", code, stderr.String())
	}
}
