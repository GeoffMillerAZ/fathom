package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/langbench"
)

// TestCmdLangReact (LB-F379): lang-react measures a subject's TS/JS files
// through ingest's dispatch and scores the React kinds alone: components and
// props at L1 (a function, a hook and a non-Props interface are out of the
// reading and counted), render and hook pairs at L2 over the names the file
// binds to the oracle's definitions (an imported library component of the same
// name is not one), file pairs and the imports binding one at L3; the level is
// computed (tier C caps each level at good).
func TestCmdLangReact(t *testing.T) {
	subject := t.TempDir()
	for p, c := range map[string]string{
		"src/Button.tsx": `export interface ButtonProps {
  label: string;
}

export function Button({ label }: ButtonProps) {
  return <button>{label}</button>;
}

export function format(n: number) {
  return String(n);
}
`,
		"src/Page.tsx": `import { Button } from './Button';
import { Link } from 'next/link';
import { useCount } from './useCount';

export default function Page() {
  const n = useCount();
  return (
    <main>
      <Button label={String(n)} />
      <Link href="/">home</Link>
    </main>
  );
}
`,
		"src/useCount.ts": `import { useState } from 'react';

export function useCount() {
  const [n] = useState(0);
  return n;
}
`,
		"src/Link.tsx": `export function Link() {
  return <a />;
}
`,
	} {
		full := filepath.Join(subject, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	oracle := reactOracleFile{
		Tool: "react-docgen-typescript (test)", Version: "test", Tier: "C",
		Definitions: []langbench.OracleDef{
			{Path: "src/Button.tsx", Name: "ButtonProps.label", Line: 2, End: 2, Kind: "prop"},
			{Path: "src/Button.tsx", Name: "Button", Line: 5, End: 7, Kind: "component"},
			{Path: "src/Link.tsx", Name: "Link", Line: 1, End: 3, Kind: "component"},
			{Path: "src/Page.tsx", Name: "Page", Line: 5, End: 13, Kind: "component"},
		},
		Hooks: []langbench.OracleDef{{Path: "src/useCount.ts", Name: "useCount", Line: 3, End: 6, Kind: "hook"}},
		Calls: []oraclePair{
			{Path: "src/Page.tsx", From: "Page", To: "Button"},
			{Path: "src/Page.tsx", From: "Page", To: "useCount"},
		},
		FileEdges: [][2]string{{"src/Page.tsx", "src/Button.tsx"}, {"src/Page.tsx", "src/useCount.ts"}},
		Names:     [][2]string{{"src/Page.tsx", "Button"}, {"src/Page.tsx", "useCount"}},
		Imports: []importOracleEntry{
			{Path: "src/Page.tsx", Line: 1, ToPath: "src/Button.tsx"},
			{Path: "src/Page.tsx", Line: 3, ToPath: "src/useCount.ts"},
		},
		Excluded: map[string]int{"lower-case name (React reads a lower-case JSX tag as an HTML element)": 1},
	}
	b, err := json.Marshal(oracle)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	op, out := filepath.Join(dir, "oracle.json"), filepath.Join(dir, "react.json")
	if err := os.WriteFile(op, b, 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := mainRun([]string{"lang-react", "-subject", subject, "-subject-name", "shop", "-oracle", op, "-out", out, "-commit", "test", "-date", "2026-10-10", "-queue", filepath.Join("..", "..", "docs", "plans", "language-breadth.queue.json")}, &stdout, &stderr); code != 0 {
		t.Fatalf("lang-react exit %d: %s", code, stderr.String())
	}
	r, err := loadReading(out)
	if err != nil {
		t.Fatal(err)
	}
	if r.Dialect != "react" || r.Kind != "language" {
		t.Fatalf("dialect %q kind %q", r.Dialect, r.Kind)
	}
	c := r.Counts
	for k, want := range map[string]int{
		langbench.CntOracleDefs: 4, langbench.CntFathomDefs: 4, langbench.CntMatched: 4,
		langbench.CntCallTruth: 2, langbench.CntCallFathom: 2, langbench.CntCallTruthFound: 2,
		langbench.CntFileEdgesTruth: 2, langbench.CntFileEdgesAgree: 2,
		langbench.CntImportsInRepo: 2, langbench.CntImportsAgree: 2,
	} {
		if c[k] != want {
			t.Errorf("count %s = %d, want %d (counts %v)", k, c[k], want, c)
		}
	}
	if r.ExcludedByKind["fathom:JSHook"] != 1 || r.ExcludedByKind["fathom:JSFunc"] != 1 || r.ExcludedByKind["fathom:TSInterface"] != 1 {
		t.Errorf("excluded_by_kind %v: a hook, a function and an interface are out of L1", r.ExcludedByKind)
	}
	// A fixture this small reads under the tenancy floor's throughput, so
	// the grades, not level_met, are what it proves: tier C caps each at good.
	for _, l := range []string{"L1", "L2", "L3"} {
		if g := r.GradeAt(l); g != langbench.GradeGood {
			t.Errorf("%s grade %s, want good (tier C)", l, g)
		}
	}
	if !strings.Contains(strings.Join(r.Notes, "\n"), "the oracle left out of its truth") {
		t.Errorf("notes lack the oracle's exclusions: %q", r.Notes)
	}
}
