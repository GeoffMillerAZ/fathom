package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// A tool's explicit scope asserts that it read every named file, including
// files where it found no calls. Calls without scope retain the older,
// calls-derived coverage contract.
func TestScoreToolOracleCallScope(t *testing.T) {
	rawManifest, err := os.ReadFile(filepath.Join("..", "..", "internal", "extractor", "dialects", "typescript", "dialect.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := langbench.ParseManifest(rawManifest)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{
		{Name: "a", FilePath: "a.ts", Calls: []string{"ok"}},
		{Name: "b", FilePath: "b.ts", Calls: []string{"extra"}},
		{Name: "c", FilePath: "c.ts", Calls: []string{"outside"}},
	}
	measured := map[string]bool{"a.ts": true, "b.ts": true, "c.ts": true}
	for _, tc := range []struct {
		name, oracle                   string
		truth, found, agreed, excluded int
		family, unmatchedB             bool
	}{
		{"explicit scope includes zero-truth file", `{"tool":"ts-calls","tier":"B","scope":["a.ts","b.ts"],"calls":[{"path":"a.ts","from":"a","to":"ok"}]}`, 1, 2, 1, 0, true, true},
		{"explicit empty calls grades all scoped files", `{"tool":"ts-calls","tier":"B","scope":["b.ts"],"calls":[]}`, 0, 1, 0, 0, true, true},
		{"sqlglot fallback empty calls has no call family", `{"tool":"sqlglot","tier":"B","calls":[]}`, 0, 0, 0, 0, false, false},
		{"exclusion removes zero-truth file pair", `{"tool":"ts-calls","tier":"B","scope":["a.ts","b.ts"],"calls":[{"path":"a.ts","from":"a","to":"ok"}],"call_exclusions":[{"path":"b.ts","to":"extra","reason":"test"}]}`, 1, 1, 1, 1, true, false},
		{"legacy calls derive file coverage", `{"tool":"ts-calls","tier":"B","calls":[{"path":"a.ts","from":"a","to":"ok"}]}`, 1, 1, 1, 0, true, false},
		{"scope with omitted calls asserts zero truth", `{"tool":"ts-calls","tier":"B","scope":["b.ts"]}`, 0, 1, 0, 0, true, true},
		{"scope with null calls asserts zero truth", `{"tool":"ts-calls","tier":"B","scope":["b.ts"],"calls":null}`, 0, 1, 0, 0, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oracle := filepath.Join(t.TempDir(), "calls.json")
			if err := os.WriteFile(oracle, []byte(tc.oracle), 0o644); err != nil {
				t.Fatal(err)
			}
			sub := langbench.Subject{Counts: map[string]int{}, Families: map[string]string{}}
			_, _, _, diff, err := scoreToolOracle(oracle, m, nodes, &sub, measured)
			if err != nil {
				t.Fatal(err)
			}
			_, family := sub.Families[langbench.FamilyCalls]
			if family != tc.family || (diff != nil) != tc.family {
				t.Fatalf("call family = %t, diff = %t; want %t", family, diff != nil, tc.family)
			}
			if !tc.family {
				return
			}
			if got := sub.Counts[langbench.CntCallTruth]; got != tc.truth {
				t.Errorf("truth = %d, want %d", got, tc.truth)
			}
			if got := sub.Counts[langbench.CntCallFathom]; got != tc.found {
				t.Errorf("found = %d, want %d", got, tc.found)
			}
			if got := sub.Counts[langbench.CntCallFathomInTrue]; got != tc.agreed {
				t.Errorf("agreed = %d, want %d", got, tc.agreed)
			}
			if got := sub.Counts[cntCallPairsExcludedMine]; got != tc.excluded {
				t.Errorf("excluded = %d, want %d", got, tc.excluded)
			}
			// -unmatched serializes these same diff maps. A call only in b.ts
			// must appear there when b.ts is explicitly in scope.
			bOnly := hasPair(pairsOnlyIn(diff.mine, diff.truth), "b.ts", "b", "extra")
			if bOnly != tc.unmatchedB {
				t.Errorf("-unmatched b.ts extra pair = %t, want %t", bOnly, tc.unmatchedB)
			}
		})
	}
}

// php-parser's definitions-only output emits calls:[] beside scip-php's
// calls family. It must neither replace that family nor trip the duplicate guard.
func TestScoreToolOracleDefinitionsOnlyBesideCalls(t *testing.T) {
	m := langbench.Manifest{}
	sub := langbench.Subject{
		Counts:   map[string]int{langbench.CntCallTruth: 2, langbench.CntCallFathom: 3},
		Families: map[string]string{langbench.FamilyCalls: "A"},
	}
	oracle := filepath.Join(t.TempDir(), "php-parser.json")
	if err := os.WriteFile(oracle, []byte(`{"tool":"php-parser","tier":"B","definitions":[],"calls":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, _, diff, err := scoreToolOracle(oracle, m, nil, &sub, map[string]bool{"a.php": true})
	if err != nil {
		t.Fatal(err)
	}
	if diff != nil || sub.Families[langbench.FamilyCalls] != "A" || sub.Counts[langbench.CntCallTruth] != 2 || sub.Counts[langbench.CntCallFathom] != 3 {
		t.Fatalf("definitions-only oracle changed earlier call family: diff=%v families=%v counts=%v", diff, sub.Families, sub.Counts)
	}
}

// sqlglot may emit an empty call list when a statement falls back to an
// unsupported parse. The empty output must leave the reading's metrics and
// unmatched call output exactly as an oracle with no calls assertion does.
func TestCmdLangOracleUnscopedEmptyCallsIgnored(t *testing.T) {
	subject := tsFixtureSubject(t)
	run := func(raw string) (langbench.Reading, []byte) {
		t.Helper()
		dir := t.TempDir()
		oracle := filepath.Join(dir, "oracle.json")
		if err := os.WriteFile(oracle, []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(dir, "reading.json")
		unmatched := filepath.Join(dir, "unmatched.json")
		var stdout, stderr bytes.Buffer
		args := []string{"-dialect", "typescript", "-subject", subject,
			"-ctags", filepath.Join(dir, "absent"), "-tool-oracle", oracle,
			"-out", out, "-unmatched", unmatched,
			"-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
		if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
			t.Fatalf("exit %d: %s", code, stderr.String())
		}
		r, _ := readValidReading(t, out)
		unmatchedRaw, err := os.ReadFile(unmatched)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		return r, unmatchedRaw
	}
	baseline, baselineUnmatched := run(`{"tool":"sqlglot","tier":"B"}`)
	empty, emptyUnmatched := run(`{"tool":"sqlglot","tier":"B","calls":[]}`)
	if len(empty.Subjects) != 1 {
		t.Fatalf("subjects = %d, want 1", len(empty.Subjects))
	}
	if _, ok := empty.Subjects[0].Families[langbench.FamilyCalls]; ok || empty.Metrics[langbench.MCallPrecision] != nil {
		t.Fatalf("unscoped empty calls asserted a call family: families=%v metrics=%v", empty.Subjects[0].Families, empty.Metrics)
	}
	if !reflect.DeepEqual(empty.Subjects[0].Families, baseline.Subjects[0].Families) ||
		!reflect.DeepEqual(empty.Counts, baseline.Counts) ||
		!reflect.DeepEqual(empty.Metrics, baseline.Metrics) ||
		!bytes.Equal(emptyUnmatched, baselineUnmatched) {
		t.Fatalf("unscoped empty calls changed reading: families %v/%v, counts %v/%v, metrics %v/%v, unmatched %s/%s",
			empty.Subjects[0].Families, baseline.Subjects[0].Families, empty.Counts, baseline.Counts, empty.Metrics, baseline.Metrics, emptyUnmatched, baselineUnmatched)
	}
}

func TestCmdLangOracleDefinitionsOnlyBesideCalls(t *testing.T) {
	subject := tsFixtureSubject(t)
	dir := t.TempDir()
	calls := filepath.Join(dir, "calls.json")
	defs := filepath.Join(dir, "defs.json")
	if err := os.WriteFile(calls, []byte(`{"tool":"calls-oracle","tier":"B","calls":[{"path":"main.ts","from":"run","to":"greet"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(defs, []byte(`{"tool":"php-parser","tier":"B","definitions":[],"calls":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "reading.json")
	var stdout, stderr bytes.Buffer
	args := []string{"-dialect", "typescript", "-subject", subject,
		"-ctags", filepath.Join(dir, "absent"), "-tool-oracle", calls,
		"-tool-oracle", defs, "-out", out,
		"-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("definitions-only oracle after calls exit %d: %s", code, stderr.String())
	}
	r, _ := readValidReading(t, out)
	if len(r.Subjects) != 1 || r.Subjects[0].Families[langbench.FamilyCalls] != "B" || r.Counts[langbench.CntCallTruth] != 1 {
		t.Fatalf("earlier calls lost: subjects=%v counts=%v", r.Subjects, r.Counts)
	}
}

// Recorded-count regrade is a compatibility check for the committed SQL and
// PHP readings. It does not rerun extraction or their independent oracles.
func TestCmdLangRegradeCallScopeUnaffectedReadings(t *testing.T) {
	for _, dialect := range []string{"sql", "php"} {
		t.Run(dialect, func(t *testing.T) {
			source := filepath.Join("..", "..", "docs", "bench", "readings", "lang", dialect+".json")
			raw, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), dialect+".json")
			if err := os.WriteFile(path, raw, 0o644); err != nil {
				t.Fatal(err)
			}
			before, err := loadReading(path)
			if err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			if code := cmdLangRegrade([]string{"-commit", "call-scope-check", path}, &stdout, &stderr); code != 0 {
				t.Fatalf("regrade exit %d: %s", code, stderr.String())
			}
			after, err := loadReading(path)
			if err != nil {
				t.Fatal(err)
			}
			if before.LevelMet != after.LevelMet || !reflect.DeepEqual(before.Grade, after.Grade) ||
				!reflect.DeepEqual(before.Metrics, after.Metrics) || !reflect.DeepEqual(before.Counts, after.Counts) ||
				!reflect.DeepEqual(before.Subjects, after.Subjects) || !reflect.DeepEqual(before.Tenancy, after.Tenancy) {
				t.Fatalf("stored-count regrade changed %s: before level=%s grade=%+v metrics=%v counts=%v; after level=%s grade=%+v metrics=%v counts=%v",
					dialect, before.LevelMet, before.Grade, before.Metrics, before.Counts,
					after.LevelMet, after.Grade, after.Metrics, after.Counts)
			}
		})
	}
}

func TestCmdLangOracleEmptyCallsScopeBlocksL2(t *testing.T) {
	subject := tsFixtureSubject(t)
	oracle := filepath.Join(t.TempDir(), "calls.json")
	if err := os.WriteFile(oracle, []byte(`{"tool":"ts-calls","tier":"B","scope":["greeter.ts"],"calls":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "reading.json")
	unmatched := filepath.Join(t.TempDir(), "unmatched.json")
	var stdout, stderr bytes.Buffer
	args := []string{"-dialect", "typescript", "-subject", subject,
		"-ctags", filepath.Join(t.TempDir(), "absent"), "-tool-oracle", oracle,
		"-out", out, "-unmatched", unmatched,
		"-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, _ := readValidReading(t, out)
	if r.Counts[langbench.CntCallTruth] != 0 || r.Counts[langbench.CntCallFathom] == 0 || r.Counts[langbench.CntCallFathomInTrue] != 0 {
		t.Fatalf("zero-truth scoped calls: %v", r.Counts)
	}
	if p := r.Metrics[langbench.MCallPrecision]; p == nil || *p != 0 {
		t.Errorf("call precision = %v, want 0", p)
	}
	if r.LevelMet == "L2" || r.LevelMet == "L3" || r.LevelMet == "L4" || r.LevelMet == "L5" {
		t.Errorf("level met = %q with zero call precision", r.LevelMet)
	}
	u := readUnmatched(t, unmatched)
	if u.Calls == nil || !hasPair(u.Calls.FathomOnly, "greeter.ts", "greet", "log") {
		t.Errorf("-unmatched calls = %+v, want greeter.ts greet -> log", u.Calls)
	}
}

func TestCmdLangOracleScopeReportsZeroTruthFile(t *testing.T) {
	subject := tsFixtureSubject(t)
	oracle := filepath.Join(t.TempDir(), "calls.json")
	if err := os.WriteFile(oracle, []byte(`{"tool":"ts-calls","tier":"B","scope":["main.ts","greeter.ts"],"calls":[{"path":"main.ts","from":"run","to":"greet"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "reading.json")
	unmatched := filepath.Join(t.TempDir(), "unmatched.json")
	var stdout, stderr bytes.Buffer
	args := []string{"-dialect", "typescript", "-subject", subject,
		"-ctags", filepath.Join(t.TempDir(), "absent"), "-tool-oracle", oracle,
		"-out", out, "-unmatched", unmatched,
		"-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}
	if code := cmdLangOracle(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	r, _ := readValidReading(t, out)
	if r.Counts[langbench.CntCallTruth] != 1 || r.Counts[langbench.CntCallTruthFound] != 1 || r.Counts[langbench.CntCallFathom] < 2 {
		t.Fatalf("scoped calls: %v", r.Counts)
	}
	u := readUnmatched(t, unmatched)
	if u.Calls == nil || !hasPair(u.Calls.FathomOnly, "greeter.ts", "greet", "log") {
		t.Errorf("-unmatched calls = %+v, want greeter.ts greet -> log", u.Calls)
	}
}
