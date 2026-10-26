package main

import (
	"bytes"
	"os"
	"path/filepath"
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
		{"empty calls without scope grades all measured files", `{"tool":"ts-calls","tier":"B","calls":[]}`, 0, 3, 0, 0, true, true},
		{"exclusion removes zero-truth file pair", `{"tool":"ts-calls","tier":"B","scope":["a.ts","b.ts"],"calls":[{"path":"a.ts","from":"a","to":"ok"}],"call_exclusions":[{"path":"b.ts","to":"extra","reason":"test"}]}`, 1, 1, 1, 1, true, false},
		{"legacy calls derive file coverage", `{"tool":"ts-calls","tier":"B","calls":[{"path":"a.ts","from":"a","to":"ok"}]}`, 1, 1, 1, 0, true, false},
		{"omitted calls have no call family", `{"tool":"ts-calls","tier":"B","scope":["b.ts"]}`, 0, 0, 0, 0, false, false},
		{"null calls have no call family", `{"tool":"ts-calls","tier":"B","scope":["b.ts"],"calls":null}`, 0, 0, 0, 0, false, false},
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
