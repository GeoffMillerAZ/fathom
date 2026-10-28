package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/langbench"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
	"google.golang.org/protobuf/proto"
)

func gitFixtureCommand(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s: %v", args, b, err)
	}
	return strings.TrimSpace(string(b))
}
func writeFixtureFile(t *testing.T, root, name, value string) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
}
func gitCandidateFixture(t *testing.T) (root, baseline, candidate string) {
	t.Helper()
	root = t.TempDir()
	gitFixtureCommand(t, root, "init", "-q")
	lock := "internal/extractor/tsengine/grammars/grammars.lock.json"
	writeFixtureFile(t, root, lock, `{"core":{},"toolchain":{},"grammars":[{"blob":"c","blob_sha256":"c"},{"blob":"cpp","blob_sha256":"old"}]}`)
	writeFixtureFile(t, root, "internal/extractor/tsengine/grammars/cpp.wasm.gz", "old blob")
	gitFixtureCommand(t, root, "add", ".")
	gitFixtureCommand(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "baseline")
	baseline = gitFixtureCommand(t, root, "rev-parse", "HEAD")
	writeFixtureFile(t, root, lock, `{"core":{},"toolchain":{},"grammars":[{"blob":"c","blob_sha256":"c"},{"blob":"cpp","blob_sha256":"new"}]}`)
	writeFixtureFile(t, root, "internal/extractor/tsengine/grammars/cpp.wasm.gz", "new blob")
	writeFixtureFile(t, root, "internal/extractor/tsengine/grammars/PROVENANCE.md", "new pinned C++ grammar")
	gitFixtureCommand(t, root, "add", ".")
	gitFixtureCommand(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "candidate")
	candidate = gitFixtureCommand(t, root, "rev-parse", "HEAD")
	return
}

func TestCandidateDiffAdmitsOnlyCppGrammarDelta(t *testing.T) {
	root, baseline, candidate := gitCandidateFixture(t)
	if err := candidateDiffAllowed(root, baseline, candidate); err != nil {
		t.Fatalf("refused isolated C++ grammar delta: %v", err)
	}
	writeFixtureFile(t, root, "internal/extractor/tsextract_cppmacros.go", "changed product preparse")
	gitFixtureCommand(t, root, "add", ".")
	gitFixtureCommand(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "unrelated")
	if err := candidateDiffAllowed(root, baseline, gitFixtureCommand(t, root, "rev-parse", "HEAD")); err == nil {
		t.Fatal("accepted changed product preprocessing")
	}
}

func TestExternalCandidateDiffRefusesEmbeddedGrammarDelta(t *testing.T) {
	root, baseline, candidate := gitCandidateFixture(t)
	if err := candidateDiffAllowedForMode(root, baseline, candidate, true); err == nil {
		t.Fatal("external audit accepted a production C++ grammar lock/blob change")
	}
	writeFixtureFile(t, root, "calibration/oracles/scip-clang/cpp-screen/README.md", "external audit helper")
	gitFixtureCommand(t, root, "add", ".")
	gitFixtureCommand(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "helper")
	newHead := gitFixtureCommand(t, root, "rev-parse", "HEAD")
	if err := candidateDiffAllowedForMode(root, candidate, newHead, true); err != nil {
		t.Fatalf("external audit refused helper-only delta: %v", err)
	}
}
func TestCandidateDiffAdmitsHelperOnly(t *testing.T) {
	root, _, baseline := gitCandidateFixture(t)
	writeFixtureFile(t, root, "calibration/oracles/scip-clang/cpp-screen/main.go", "diagnostic helper")
	writeFixtureFile(t, root, "calibration/oracles/scip-clang/cpp-screen/README.md", "parser diagnostic")
	gitFixtureCommand(t, root, "add", ".")
	gitFixtureCommand(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "helper")
	if err := candidateDiffAllowed(root, baseline, gitFixtureCommand(t, root, "rev-parse", "HEAD")); err != nil {
		t.Fatalf("refused narrow diagnostic changes: %v", err)
	}
}
func TestCandidateDiffAuditReceiptPrefix(t *testing.T) {
	for _, tc := range []struct {
		name    string
		allowed bool
	}{
		{"docs/bench/readings/cpp-audit-2026-10-26/replay.json", true},
		{"docs/bench/readings/cpp-audit-2026-10-26/timing/replay.loads.json", true},
		{"docs/bench/readings/lang/x.json", false},
		{"docs/bench/readings/cpp-audit-2026-10-26-other/x.json", false},
		{"docs/bench/readings/cpp-audit-2026-10-26/replay.go", false},
		{"calibration/holdouts/cpp.yaml", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _, baseline := gitCandidateFixture(t)
			writeFixtureFile(t, root, tc.name, "{}")
			gitFixtureCommand(t, root, "add", ".")
			gitFixtureCommand(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "receipt guard")
			err := candidateDiffAllowedForMode(root, baseline, gitFixtureCommand(t, root, "rev-parse", "HEAD"), true)
			if tc.allowed && err != nil {
				t.Fatalf("refused audit receipt: %v", err)
			}
			if !tc.allowed && (err == nil || err.Error() != "candidate_unrelated_change") {
				t.Fatalf("near miss: got %v, want candidate_unrelated_change", err)
			}
		})
	}
}

func TestCandidateDiffRejectsOtherGrammarAndStaleOrDirtySource(t *testing.T) {
	root, baseline, candidate := gitCandidateFixture(t)
	writeFixtureFile(t, root, "internal/extractor/tsengine/grammars/grammars.lock.json", `{"core":{},"toolchain":{},"grammars":[{"blob":"c","blob_sha256":"changed"},{"blob":"cpp","blob_sha256":"new"}]}`)
	gitFixtureCommand(t, root, "add", ".")
	gitFixtureCommand(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "other grammar")
	if err := candidateDiffAllowed(root, baseline, gitFixtureCommand(t, root, "rev-parse", "HEAD")); err == nil {
		t.Fatal("accepted other grammar lock change")
	}
	if err := candidateDiffAllowed(root, baseline, candidate); err == nil {
		t.Fatal("accepted stale candidate binary revision")
	}
	root2, base2, cand2 := gitCandidateFixture(t)
	writeFixtureFile(t, root2, "internal/extractor/tsengine/grammars/PROVENANCE.md", "dirty")
	if err := candidateDiffAllowed(root2, base2, cand2); err == nil {
		t.Fatal("accepted dirty candidate source")
	}
}

func TestCandidateModeLabelsBaselineAndKeepsPopulationChecks(t *testing.T) {
	root, baseline, candidate := gitCandidateFixture(t)
	cfg := fixture(t, map[string]string{"a.cpp": "int a;\n"})
	cfg.ProductRoot = root
	cfg.ProductRevision = candidate
	b, err := os.ReadFile(cfg.Reading)
	if err != nil {
		t.Fatal(err)
	}
	var reading map[string]any
	if err := json.Unmarshal(b, &reading); err != nil {
		t.Fatal(err)
	}
	reading["fathom_commit"] = baseline
	reading["grammar"] = map[string]any{"blob": "cpp", "blob_sha256": "baseline-grammar-hash"}
	b, err = json.Marshal(reading)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.Reading, b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := measure(cfg); err == nil {
		t.Fatal("strict mode accepted different product revision")
	}
	cfg.Candidate = true
	r, err := measure(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if r.Mode != "candidate-diagnostic" || r.BaselineRevision != baseline || r.ProductRevision != candidate || !r.CountsComplete {
		t.Fatalf("candidate receipt lost provenance/diagnostic label: %+v", r)
	}
	reading["subjects"].([]any)[0].(map[string]any)["bytes"] = 8
	b, err = json.Marshal(reading)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.Reading, b, 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := measure(cfg); err == nil || got.CountsComplete {
		t.Fatal("candidate mode waived reading byte mismatch")
	}
}

func TestParserFailurePrivateDiagnostic(t *testing.T) {
	cause := errors.New("tsengine: parse aborted after 2s budget (pathological input?)")
	e := parserFailure{Code: "parse_fault", Phase: "selected", Ordinal: 7, PeakPages: 93, Cause: cause}
	message := e.Error()
	for _, part := range []string{"phase=selected", "ordinal=7", "peak_pages=93", cause.Error()} {
		if !strings.Contains(message, part) {
			t.Fatalf("lost diagnostic %q: %s", part, message)
		}
	}
	if !errors.Is(e, cause) {
		t.Fatal("lost underlying parser error")
	}
	private := parserFailure{Code: "parse_fault", Phase: "raw", Ordinal: 1, Cause: errors.New("tsengine: /private/subject/a.cpp: secret source")}.Error()
	if strings.Contains(private, "/private/") || strings.Contains(private, "secret source") {
		t.Fatalf("leaked private input: %s", private)
	}
	stack := parserFailure{Code: "parse_fault", Phase: "raw", Ordinal: 2, Cause: errors.New("tsengine: call parse: wasm error: unreachable\nwasm stack trace: private details")}.Error()
	if !strings.Contains(stack, "wasm error: unreachable") || strings.Contains(stack, "wasm stack trace") || strings.Contains(stack, "private details") {
		t.Fatalf("lost class or leaked stack: %s", stack)
	}
}

func fixture(t *testing.T, files map[string]string) config {
	t.Helper()
	root := t.TempDir()
	subject := filepath.Join(root, "subject")
	if err := os.Mkdir(subject, 0700); err != nil {
		t.Fatal(err)
	}
	var names []string
	var bytes int
	for name, src := range files {
		path := filepath.Join(subject, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0600); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
		bytes += len(src)
	}
	// The measured tenancy is sorted in the same way indexbench writes it.
	sortStrings(names)
	reading := filepath.Join(root, "reading.json")
	unmatched := filepath.Join(root, "unmatched.json")
	for path, value := range map[string]any{
		reading:   map[string]any{"dialect": "cpp", "fathom_commit": strings.Repeat("a", 40), "subjects": []any{map[string]any{"name": "public/synthetic", "sha": strings.Repeat("b", 40), "files": len(names), "bytes": bytes}}},
		unmatched: map[string]any{"tenancy_files": names},
	} {
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return config{Subject: subject, SubjectName: "public/synthetic", Reading: reading, Unmatched: unmatched, ProductRevision: strings.Repeat("a", 40), SubjectRevision: strings.Repeat("b", 40)}
}

func TestExactPopulationAndSyntaxPartitions(t *testing.T) {
	cfg := fixture(t, map[string]string{
		"good.cpp":    "int good() { return 1; }\n",
		"bad.cpp":     "int broken( {\n",
		"missing.cpp": "int missing() { return 1;\n",
		"macro.cpp":   "LIB_INLINE int macro() LIB_NOEXCEPT { return 1; }\n",
	})
	r, err := measure(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "complete" || r.Files != 4 || r.Written.Error == 0 || r.Written.Missing == 0 || r.Written.OnlyM == 0 {
		t.Fatalf("missing required distinct syntax evidence: %+v", r)
	}
	for _, p := range []partition{r.Written, r.Selected} {
		if p.Error+p.Missing-p.Both != p.Union || p.OnlyE+p.OnlyM+p.Both != p.Union {
			t.Fatalf("invalid exact partition: %+v", p)
		}
	}
	if !r.Preprocess || r.PreprocessSelected == 0 || r.PopulationSHA256 == "" || r.GrammarBlobSHA256 == "" || r.ProductRevision == "" {
		t.Fatalf("missing identity binding: %+v", r)
	}
	if r.Written.Union != 3 || r.Selected.Union != 2 {
		t.Fatalf("unexpected fixed fixture counts: written=%+v selected=%+v", r.Written, r.Selected)
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), cfg.Subject) || strings.Contains(string(b), "bad.cpp") || strings.Contains(string(b), "public/synthetic") {
		t.Fatal("receipt exposes a path or name")
	}
}

func TestRefusesMismatchedReadingAndChangedInput(t *testing.T) {
	cfg := fixture(t, map[string]string{"a.cpp": "int a;\n"})
	if err := os.WriteFile(filepath.Join(cfg.Subject, "a.cpp"), []byte("int a; int b;\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := measure(cfg); err == nil {
		t.Fatal("accepted changed reading bytes")
	}
	if err := os.WriteFile(cfg.Unmatched, []byte(`{"tenancy_files":["other.cpp"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := measure(cfg); err == nil {
		t.Fatal("accepted mismatched population")
	}
}

func TestRefusesMissingOrMismatchedRevisions(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
	}{
		{"missing product", func(r map[string]any) { delete(r, "fathom_commit") }},
		{"short product", func(r map[string]any) { r["fathom_commit"] = "aabbccdd" }},
		{"mismatched product", func(r map[string]any) { r["fathom_commit"] = strings.Repeat("c", 40) }},
		{"missing subject", func(r map[string]any) { delete(r["subjects"].([]any)[0].(map[string]any), "sha") }},
		{"short subject", func(r map[string]any) { r["subjects"].([]any)[0].(map[string]any)["sha"] = "aabbccdd" }},
		{"mismatched subject", func(r map[string]any) { r["subjects"].([]any)[0].(map[string]any)["sha"] = strings.Repeat("c", 40) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := fixture(t, map[string]string{"a.cpp": "int a;\n"})
			b, err := os.ReadFile(cfg.Reading)
			if err != nil {
				t.Fatal(err)
			}
			var r map[string]any
			if err := json.Unmarshal(b, &r); err != nil {
				t.Fatal(err)
			}
			tc.change(r)
			b, err = json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(cfg.Reading, b, 0600); err != nil {
				t.Fatal(err)
			}
			if got, err := measure(cfg); err == nil || got.Status != "failed" {
				t.Fatalf("accepted revision mismatch: %+v, %v", got, err)
			}
		})
	}
}

func TestRefusesTraversalSymlinkDuplicateAndCaps(t *testing.T) {
	for _, bad := range []string{"../outside.cpp", "/absolute.cpp", "a/../../outside.cpp", "a.cpp/../a.cpp"} {
		cfg := fixture(t, map[string]string{"a.cpp": "int a;\n"})
		if err := os.WriteFile(cfg.Unmatched, []byte(`{"tenancy_files":["`+bad+`"]}`), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := measure(cfg); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	cfg := fixture(t, map[string]string{"a.cpp": "int a;\n"})
	if err := os.Symlink(filepath.Join(cfg.Subject, "a.cpp"), filepath.Join(cfg.Subject, "link.cpp")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.Unmatched, []byte(`{"tenancy_files":["link.cpp"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := measure(cfg); err == nil {
		t.Fatal("accepted symlink")
	}
	if err := os.WriteFile(cfg.Unmatched, []byte(`{"tenancy_files":["a.cpp","a.cpp"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := measure(cfg); err == nil {
		t.Fatal("accepted duplicate")
	}
	cfg = fixture(t, map[string]string{"real/a.cpp": "int a;\n"})
	if err := os.Symlink(filepath.Join(cfg.Subject, "real"), filepath.Join(cfg.Subject, "alias")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.Unmatched, []byte(`{"tenancy_files":["alias/a.cpp"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := measure(cfg); err == nil {
		t.Fatal("accepted symlink parent")
	}
}

func TestCapsAcceptEqualityAndRejectCapPlusOne(t *testing.T) {
	for _, tc := range []struct {
		name       string
		cap, above limits
	}{
		{"json", limits{JSONBytes: 1000, Files: 1, SourceBytes: 1000, TotalBytes: 1000}, limits{JSONBytes: 1, Files: 1, SourceBytes: 1000, TotalBytes: 1000}},
		{"files", limits{JSONBytes: 1000, Files: 1, SourceBytes: 1000, TotalBytes: 1000}, limits{JSONBytes: 1000, Files: 0, SourceBytes: 1000, TotalBytes: 1000}},
		{"source", limits{JSONBytes: 1000, Files: 1, SourceBytes: 7, TotalBytes: 1000}, limits{JSONBytes: 1000, Files: 1, SourceBytes: 6, TotalBytes: 1000}},
		{"total", limits{JSONBytes: 1000, Files: 1, SourceBytes: 1000, TotalBytes: 7}, limits{JSONBytes: 1000, Files: 1, SourceBytes: 1000, TotalBytes: 6}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := fixture(t, map[string]string{"a.cpp": "int a;\n"})
			if tc.name == "json" {
				b, err := os.ReadFile(cfg.Reading)
				if err != nil {
					t.Fatal(err)
				}
				tc.cap.JSONBytes = int64(len(b))
				tc.above.JSONBytes = int64(len(b) - 1)
			}
			cfg.Limits = &tc.cap
			if _, err := measure(cfg); err != nil {
				t.Fatalf("rejected equality: %v", err)
			}
			cfg.Limits = &tc.above
			if _, err := measure(cfg); err == nil {
				t.Fatal("accepted cap + 1")
			}
		})
	}
}

func TestRefusesDuplicateJSONKeyAndReportsFault(t *testing.T) {
	cfg := fixture(t, map[string]string{"a.cpp": "int a;\n"})
	if err := os.WriteFile(cfg.Unmatched, []byte(`{"tenancy_files":["a.cpp"],"tenancy_files":["a.cpp"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := measure(cfg); err == nil {
		t.Fatal("accepted duplicate JSON key")
	}
	cfg = fixture(t, map[string]string{"a.cpp": "int a;\n"})
	if err := os.Remove(filepath.Join(cfg.Subject, "a.cpp")); err != nil {
		t.Fatal(err)
	}
	r, err := measure(cfg)
	if err == nil || r.Status != "failed" || r.Selected.Faults != 1 {
		t.Fatalf("file fault should fail whole invocation with count: %+v, %v", r, err)
	}
}

func TestTenancyCapStopsTokenTraversalBeforeLargeTail(t *testing.T) {
	var payload bytes.Buffer
	payload.WriteString(`{"tenancy_files":[`)
	for i := 0; i < 1000; i++ {
		if i > 0 {
			payload.WriteByte(',')
		}
		fmt.Fprintf(&payload, `"n%04d.cpp"`, i)
	}
	payload.WriteString(`]}`)
	d := json.NewDecoder(bytes.NewReader(payload.Bytes()))
	tokens := 0
	if err := jsonValue(d, 0, 3, 64, maxJSONTokens, &tokens); err == nil {
		t.Fatal("accepted over-cap list")
	}
	// The decoder may buffer ahead, but InputOffset records tokens consumed.
	if d.InputOffset() > int64(bytes.Index(payload.Bytes(), []byte(`"n0004.cpp"`))) {
		t.Fatalf("consumed beyond cap+1 values: offset %d", d.InputOffset())
	}
}

func TestJSONDepthAndTokenBudgetBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, input   string
		depth, tokens int
	}{
		{"depth", `[[0]]`, 2, 3},
		{"tokens", `{"a":1}`, 1, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seen := 0
			if err := jsonValue(json.NewDecoder(strings.NewReader(tc.input)), 0, -1, tc.depth, tc.tokens, &seen); err != nil {
				t.Fatalf("rejected equality: %v", err)
			}
			seen = 0
			depth, tokens := tc.depth, tc.tokens
			if tc.name == "depth" {
				depth--
			} else {
				tokens--
			}
			if err := jsonValue(json.NewDecoder(strings.NewReader(tc.input)), 0, -1, depth, tokens, &seen); err == nil {
				t.Fatal("accepted cap+1")
			}
		})
	}
}

func TestRefusesNonWorkspaceOutput(t *testing.T) {
	if err := validateOutput(filepath.Join(t.TempDir(), "receipt.json")); err == nil {
		t.Fatal("accepted output outside ignored workspace")
	}
}

func TestExistingReceiptNeverClobbered(t *testing.T) {
	cfg := fixture(t, map[string]string{"a.cpp": "int a;\n"})
	repo, err := git(".", "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(repo, "workspace", "tmp")
	if err := os.MkdirAll(base, 0700); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(base, "cpp-screen-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	cfg.Out = filepath.Join(dir, "existing.json")
	if err := os.WriteFile(cfg.Out, []byte("sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(cfg); err == nil {
		t.Fatal("overwrote existing receipt")
	}
	b, err := os.ReadFile(cfg.Out)
	if err != nil || string(b) != "sentinel" {
		t.Fatalf("receipt changed: %q, %v", b, err)
	}
}

// The canonical truth is derived from SCIP and known source bytes. Perturbing
// the extractor nodes must move the real grade without moving that truth.
func TestSCIPTruthIndependentOfExtractorNodes(t *testing.T) {
	source := []byte("int helper() { return 1; }\n")
	index := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-clang", Version: "synthetic"}}, Documents: []*scippb.Document{{RelativePath: "a.cpp", Occurrences: []*scippb.Occurrence{{Symbol: "cxx . . $ helper(aaaaaaaaaaaaaaaa).", Range: []int32{0, 4, 10}, SymbolRoles: int32(scippb.SymbolRole_Definition)}}}}}
	raw, err := proto.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	known := func(string) []byte { return source }
	truth := func(raw []byte) string {
		defs, _, _, err := langbench.SCIPDefs(raw, known)
		if err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(defs)
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf("%x", sha256.Sum256(b))
	}
	originalTruth := truth(raw)
	nodes := []core.SymbolNode{{Service: "s", Name: "helper", Kind: core.KindCppFunc, FilePath: "a.cpp", Line: 1}}
	options := langbench.SCIPOptionsFor("cpp", known)
	graded, err := langbench.ScoreSCIPWith(raw, "s", nodes, options)
	if err != nil {
		t.Fatal(err)
	}
	perturbed, err := langbench.ScoreSCIPWith(raw, "s", nil, options)
	if err != nil {
		t.Fatal(err)
	}
	if graded.Counts[langbench.CntScipDefsMapped] != 1 || perturbed.Counts[langbench.CntScipDefsMapped] != 0 {
		t.Fatalf("grade did not respond to extractor perturbation: %v, %v", graded.Counts, perturbed.Counts)
	}
	if truth(raw) != originalTruth {
		t.Fatal("extractor perturbation changed truth")
	}
	index.Documents[0].Occurrences[0].Symbol = "cxx . . $ changed(aaaaaaaaaaaaaaaa)."
	changed, err := proto.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	if truth(changed) == originalTruth {
		t.Fatal("changed SCIP definition did not change truth")
	}
}
