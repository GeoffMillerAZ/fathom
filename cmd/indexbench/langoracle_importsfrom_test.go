package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// These are scorer controls with the existing synthetic rust-analyzer-shaped
// fixture, not new native Roslyn evidence or a language support reading.
func importsFromArgs(t *testing.T) []string {
	t.Helper()
	return []string{"-dialect", "rust", "-subject", copyFixture(t, "rustfixture"), "-subject-name", "fixture/import-family",
		"-scip", rustFixtureIndex(t), "-ctags", filepath.Join(t.TempDir(), "none"), "-date", "2026-10-19", "-commit", "abc1234",
		"-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
}

func writeImportsFromOracle(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "imports.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func runImportsFrom(t *testing.T, args []string) langbench.Reading {
	t.Helper()
	out := filepath.Join(t.TempDir(), "reading.json")
	var stdout, stderr bytes.Buffer
	if code := cmdLangOracle(append(append([]string{}, args...), "-out", out), &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, _ := readValidReading(t, out)
	return r
}

func refuseImportsFrom(t *testing.T, args []string, want string) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "reading.json")
	var stdout, stderr bytes.Buffer
	code := cmdLangOracle(append(append([]string{}, args...), "-out", out), &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), want) {
		t.Fatalf("exit %d, want refusal containing %q: %s", code, want, stderr.String())
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("refusal published a reading or output status is unknown: %v", err)
	}
}

func TestCmdLangOracleImportsFromDefaultsPreserveLegacySemantics(t *testing.T) {
	args := importsFromArgs(t)
	baseline := runImportsFrom(t, args)
	explicit := runImportsFrom(t, append(args, "-imports-from", "scip"))
	if !reflect.DeepEqual(baseline.Counts, explicit.Counts) || !reflect.DeepEqual(baseline.Metrics, explicit.Metrics) ||
		!reflect.DeepEqual(baseline.MetricTiers, explicit.MetricTiers) || importNote(baseline) != importNote(explicit) {
		t.Fatal("omitted and explicit SCIP import selection changed semantic counts, metrics, tiers or import disclosure")
	}
	// Ordinary no-SCIP imports still supply LB-F57 dependency truth at their
	// own tier; an explicit default must not require a SCIP index for them.
	ruby := []string{"-dialect", "ruby", "-subject", copyFixture(t, "rubyfixture"), "-subject-name", "fixture/legacy-imports",
		"-ctags", filepath.Join(t.TempDir(), "none"), "-tool-oracle", filepath.Join("testdata", "ruby-require-rubyfixture.json"),
		"-date", "2026-10-19", "-commit", "abc1234", "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	a, b := runImportsFrom(t, ruby), runImportsFrom(t, append(ruby, "-imports-from", "scip"))
	if !reflect.DeepEqual(a.Counts, b.Counts) || !reflect.DeepEqual(a.Metrics, b.Metrics) || !reflect.DeepEqual(a.MetricTiers, b.MetricTiers) ||
		a.Counts[langbench.CntFileEdgesTruth] != 2 || a.MetricTiers[langbench.MFileEdgeRecall] != "B" {
		t.Fatal("ordinary import-dump fallback changed")
	}
}

func TestCmdLangOracleImportsFromToolIsolatesImportFamily(t *testing.T) {
	args := importsFromArgs(t)
	baseline := runImportsFrom(t, args)
	if baseline.Counts[langbench.CntFileEdgesTruth] == 0 {
		t.Fatal("SCIP dependency control has no actual truth")
	}
	oracle := writeImportsFromOracle(t, `{"tool":"native-import-control","tier":"B","imports_semantics":"declaration-membership/v1","imports":[
 {"path":"src/main.rs","line":1,"to_path":"src/net/mod.rs"},
 {"path":"src/main.rs","line":2,"to_path":"src/util.rs"},
 {"path":"src/main.rs","line":4,"to_path":"src/util.rs"}]}`)
	got := runImportsFrom(t, append(args, "-imports-from", "tool", "-tool-oracle", oracle))
	if got.Counts[langbench.CntImportsInRepo] != 3 || got.Counts[langbench.CntImportsAgree] != 3 || got.MetricTiers[langbench.MImportResolution] != "B" {
		t.Fatalf("tool import family not selected: counts %v tiers %v", got.Counts, got.MetricTiers)
	}
	for k, want := range baseline.Counts {
		if strings.HasPrefix(k, "import_") || strings.HasPrefix(k, "imports") {
			continue
		}
		if got.Counts[k] != want {
			t.Errorf("non-import %s = %d, want unchanged %d", k, got.Counts[k], want)
		}
	}
	for _, metric := range []string{langbench.MCallRecall, langbench.MCallPrecision, langbench.MDefMapping, langbench.MTypedRecall,
		langbench.MFileEdgePrecision, langbench.MFileEdgeRecall} {
		if !reflect.DeepEqual(got.Metrics[metric], baseline.Metrics[metric]) || got.MetricTiers[metric] != baseline.MetricTiers[metric] {
			t.Errorf("independent SCIP metric %s changed", metric)
		}
	}
	if got.MetricTiers[langbench.MFileEdgeRecall] != "A" || !strings.Contains(strings.Join(got.Notes, "\n"), "declaration-membership/v1") {
		t.Fatal("SCIP dependency family or membership disclosure missing")
	}
}

func TestCmdLangOracleImportsFromWrongTargetWritesUnmatched(t *testing.T) {
	args := importsFromArgs(t)
	oracle := writeImportsFromOracle(t, `{"tool":"wrong-native-target-control","imports_semantics":"declaration-membership/v1","imports":[{"path":"src/main.rs","line":1,"to_path":"src/util.rs"}]}`)
	unmatched := filepath.Join(t.TempDir(), "unmatched.json")
	got := runImportsFrom(t, append(args, "-imports-from", "tool", "-tool-oracle", oracle, "-unmatched", unmatched))
	if got.Counts[langbench.CntImportsInRepo] != 1 || got.Counts[langbench.CntImportsAgree] != 0 ||
		got.Counts[langbench.CntImportsElsewhere] != 1 || got.Metrics[langbench.MImportResolution] == nil || *got.Metrics[langbench.MImportResolution] != 0 {
		t.Fatalf("wrong target escaped statement disagreement: %v", got.Counts)
	}
	misses := readUnmatched(t, unmatched).Imports
	if len(misses) != 1 || misses[0].ToPath != "src/util.rs" || !reflect.DeepEqual(misses[0].Targets, []string{"src/net/mod.rs"}) {
		t.Fatalf("wrong-target diagnostic lost truth or actual targets: %+v", misses)
	}
}

func TestCmdLangOracleImportsFromExplicitEmptyIsUnknownNotMissingOrPass(t *testing.T) {
	args := importsFromArgs(t)
	for _, semantics := range []string{"", `,"imports_semantics":"declaration-membership/v1"`} {
		oracle := writeImportsFromOracle(t, `{"tool":"native-empty-control","tier":"A","imports":[]`+semantics+`}`)
		got := runImportsFrom(t, append(args, "-imports-from", "tool", "-tool-oracle", oracle))
		if got.Counts[langbench.CntImportsInRepo] != 0 || got.Metrics[langbench.MImportResolution] != nil {
			t.Fatalf("empty imports manufactured a denominator or pass: %v %v", got.Counts, got.Metrics)
		}
		if !strings.Contains(strings.Join(got.Notes, "\n"), "no denominator") || got.MetricTiers[langbench.MFileEdgeRecall] != "A" {
			t.Fatal("explicit empty truth not disclosed separately from SCIP dependencies")
		}
	}
}

func TestCmdLangOracleImportsFromSelectionRefusals(t *testing.T) {
	args := importsFromArgs(t)
	refuseImportsFrom(t, append(args, "-imports-from", "guess"), "-imports-from is scip or tool")
	missing := writeImportsFromOracle(t, `{"tool":"no-imports"}`)
	refuseImportsFrom(t, append(args, "-imports-from", "tool", "-tool-oracle", missing), "exactly one")
	empty := writeImportsFromOracle(t, `{"tool":"empty-imports","imports":[]}`)
	refuseImportsFrom(t, append(args, "-tool-oracle", empty), "imports given alongside another imports oracle")
	refuseImportsFrom(t, append(args, "-imports-from", "tool", "-tool-oracle", empty, "-tool-oracle", empty), "exactly one")
	membership := writeImportsFromOracle(t, `{"tool":"membership","imports_semantics":"declaration-membership/v1","imports":[]}`)
	refuseImportsFrom(t, append(args, "-tool-oracle", membership), "-imports-from tool")
	noSCIP := []string{"-dialect", "rust", "-subject", copyFixture(t, "rustfixture"), "-imports-from", "tool", "-tool-oracle", membership}
	refuseImportsFrom(t, noSCIP, "needs -scip")
	// Merely naming an unreadable/non-SCIP file cannot authorize membership.
	invalid := writeImportsFromOracle(t, "not a SCIP protobuf")
	refuseImportsFrom(t, append(args, "-scip", invalid, "-imports-from", "tool", "-tool-oracle", membership), "decode index")
}

func TestCmdLangOracleMembershipEnvelopeRefusals(t *testing.T) {
	args := importsFromArgs(t)
	for _, tc := range []struct{ body, want string }{
		{`{"tool":"x","imports_semantics":"declaration-membership/v2","imports":[]}`, "unknown imports_semantics"},
		{`{"tool":"x","imports_semantics":"","imports":[]}`, "unknown imports_semantics"},
		{`{"tool":"x","imports_semantics":null,"imports":[]}`, "unknown imports_semantics"},
		{`{"tool":"x","imports_semantics":"declaration-membership/v1"}`, "imports array"},
		{`{"tool":"x","imports_semantics":"declaration-membership/v1","imports":null}`, "imports array"},
		{`{"tool":"x","imports_semantics":"declaration-membership/v1","unexpected":true,"imports":[]}`, "unknown field"},
		{`{"tool":"x","imports_semantics":"declaration-membership/v1","file_edges":[["A.cs","B.cs"]],"imports":[]}`, "not import-family"},
		{`{"tool":"x","imports_semantics":"declaration-membership/v1","calls":[{"path":"A.cs","from":"a","to":"b"}],"imports":[]}`, "not import-family"},
		{`{"tool":"x","imports_semantics":"declaration-membership/v1","imports":[{"path":"src/main.rs","line":1,"to_path":"src/util.rs","unexpected":true}]}`, "unknown field"},
		{`{"tool":"x","imports_semantics":"declaration-membership/v1","imports":[{"path":"src/main.rs","line":1,"to_path":"src/util.rs","same_package":true}]}`, "same_package"},
		{`{"tool":"x","imports_semantics":"declaration-membership/v1","imports":[{"path":"../main.rs","line":1,"to_path":"src/util.rs"}]}`, "relative"},
		{`{"tool":"x","imports_semantics":"declaration-membership/v1","imports":[{"path":"src/main.rs","line":0,"to_path":"src/util.rs"}]}`, "positive line"},
	} {
		t.Run(tc.want+tc.body, func(t *testing.T) {
			oracle := writeImportsFromOracle(t, tc.body)
			refuseImportsFrom(t, append(args, "-imports-from", "tool", "-tool-oracle", oracle), tc.want)
		})
	}
}

func TestMembershipCannotBecomeNoSCIPDependencyTruth(t *testing.T) {
	oracle := writeImportsFromOracle(t, `{"tool":"membership","imports_semantics":"declaration-membership/v1","imports":[{"path":"A.cs","line":1,"to_path":"B.cs"}]}`)
	sub := langbench.Subject{Counts: map[string]int{}, Families: map[string]string{}}
	_, diff, err := scoreImportFileEdges(context.Background(), oracle, "csharp", t.TempDir(), "control", nil, nil, &sub, map[string]bool{"A.cs": true, "B.cs": true})
	if err == nil || !strings.Contains(err.Error(), "not dependency truth") || diff != nil || len(sub.Counts) != 0 || len(sub.Families) != 0 {
		t.Fatalf("membership entered the dependency fallback: err %v, diff %v, subject %+v", err, diff, sub)
	}
}

func TestMembershipTargetSetDiagnosticsKeepWrongPartialAndDuplicateTargets(t *testing.T) {
	entries := []importOracleEntry{{Path: "Use.cs", Line: 1, ToPath: "one/A.cs"}, {Path: "Use.cs", Line: 1, ToPath: "two/B.cs"}}
	refs := []langbench.OracleImport{{Path: "Use.cs", Line: 1, ToPath: "one/A.cs", Explicit: true}, {Path: "Use.cs", Line: 1, ToPath: "two/B.cs", Explicit: true}}
	inScope := map[string]bool{"Use.cs": true, "one/A.cs": true, "two/B.cs": true, "other/Wrong.cs": true}
	for _, tc := range []struct {
		name                                string
		targets                             []string
		statementAgree, targetAgree, misses int
	}{
		{"complete", []string{"one/A.cs", "two/B.cs"}, 1, 2, 0},
		{"partial", []string{"one/A.cs"}, 1, 1, 1},
		{"wrong", []string{"other/Wrong.cs"}, 0, 0, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			si := subjectImports{done: true, ok: true, name: "native-set-control", inScope: inScope,
				sites: []langbench.ImportSite{{Path: "Use.cs", StartLine: 1, EndLine: 1, Targets: tc.targets}}}
			sub := langbench.Subject{Counts: map[string]int{}, Families: map[string]string{}}
			// Duplicate native rows never inflate statement/target truth.
			truth := append(append([]langbench.OracleImport{}, refs...), refs...)
			if err := si.score(truth, "A", "native-set-control", &sub); err != nil {
				t.Fatal(err)
			}
			if sub.Counts[langbench.CntImportsInRepo] != 1 || sub.Counts[langbench.CntImportsAgree] != tc.statementAgree ||
				sub.Counts[langbench.CntImportEdgesTruth] != 2 || sub.Counts[langbench.CntImportEdgesAgree] != tc.targetAgree {
				t.Fatalf("statement agreement hid target-set error: %v", sub.Counts)
			}
			misses := importMisses(entries, si.sites, inScope)
			if len(misses) != tc.misses {
				t.Fatalf("unmatched targets %v, want %d", misses, tc.misses)
			}
			for _, miss := range misses {
				if !reflect.DeepEqual(miss.Targets, tc.targets) {
					t.Fatalf("diagnostic lost actual targets: %+v", miss)
				}
			}
			if _, exists := sub.Families[langbench.FamilyFileEdges]; exists {
				t.Fatal("statement targets became dependencies")
			}
		})
	}
}
