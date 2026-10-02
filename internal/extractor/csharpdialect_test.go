package extractor

// csharpdialect_test.go — goldens for the csharp dialect (LB-Q-csharp,
// design §13.2 steps 10/12). Mirrors cdialect_test.go's pattern (no generic
// "TestDialectGoldens" runner exists for a tags-engine dialect beyond
// TestTagsDialectParity's five migrated languages).

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var csharpFixtures = []string{"shapes.cs", "namespaces.cs", "imports.cs", "operators.cs", "broken.cs", "conditional.cs", "branches.cs"}

// TestCSharpDialectGoldens runs every named dialects/csharp/testdata/*.cs
// fixture through NewTSExtractor("csharp") and compares symbols + imports
// against its hand-written *.golden.json (written from indexbench
// lang-probe's own capture and tree output, reviewed against the fixtures
// before this test first went green — design §13.2 step 10).
func TestCSharpDialectGoldens(t *testing.T) {
	e, err := NewTSExtractor("csharp")
	if err != nil {
		t.Fatalf("NewTSExtractor(csharp): %v", err)
	}
	defer e.Close()

	dir := filepath.Join("dialects", "csharp", "testdata")
	for _, name := range csharpFixtures {
		fx := filepath.Join(dir, name)
		t.Run(name, func(t *testing.T) {
			got := extractGolden(t, e, fx)
			gp := goldenPathFor(fx)
			if *updateGoldens {
				if err := os.WriteFile(gp, marshalGolden(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			raw, err := os.ReadFile(gp)
			if err != nil {
				t.Fatalf("read golden: %v (run with -update to create)", err)
			}
			var want goldenFile
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s: %v", gp, err)
			}
			gb, wb := marshalGolden(got), marshalGolden(want)
			if !bytes.Equal(gb, wb) {
				t.Errorf("%s: mismatch\n--- got\n%s\n--- want\n%s", fx, gb, wb)
			}
		})
	}
}

// csharpFixtureCalls is calls.cs's calls per definition, sorted.
func csharpFixtureCalls(t *testing.T) map[string][]string {
	t.Helper()
	e, err := NewTSExtractor("csharp")
	if err != nil {
		t.Fatalf("NewTSExtractor(csharp): %v", err)
	}
	defer e.Close()
	got := extractGolden(t, e, filepath.Join("dialects", "csharp", "testdata", "calls.cs"))
	calls := map[string][]string{}
	for _, s := range got.Symbols {
		if len(s.Calls) > 0 {
			calls[s.Name] = append([]string(nil), s.Calls...)
			sort.Strings(calls[s.Name])
		}
	}
	return calls
}

func wantCalls(t *testing.T, calls map[string][]string, want map[string][]string) {
	t.Helper()
	for name, w := range want {
		if strings.Join(calls[name], ",") != strings.Join(w, ",") {
			t.Errorf("%s.Calls = %v, want %v", name, calls[name], w)
		}
	}
}

// TestCSharpDialectGenericCallees (LB-F189): a generic method is named
// without its type arguments, member and static forms included:
// `Make<int>(2)` is Make, `Registry.Pair<System.Exception, string>(5)` is
// Registry.Pair, never `Make<int>` or the `string>` tail a dotted type
// argument left (O271).
func TestCSharpDialectGenericCallees(t *testing.T) {
	calls := csharpFixtureCalls(t)
	wantCalls(t, calls, map[string][]string{
		"Worker.Generic":     {"Make"},
		"Worker.Member":      {"Registry.Create", "Registry.Pair", "reg.Add", "w.Convert"},
		"Worker.Conditional": {"reg.Touch"},
	})
	for name, cs := range calls {
		for _, c := range cs {
			if strings.ContainsAny(c, "<>") && !strings.HasPrefix(name, "Worker.Delegates") {
				t.Errorf("%s: callee %q keeps type arguments", name, c)
			}
		}
	}
}

// TestCSharpDialectNameofIsNotACall (LB-F190): nameof(x) is an operator
// Roslyn and SCIP both treat as one; the call beside it is kept (O272).
func TestCSharpDialectNameofIsNotACall(t *testing.T) {
	calls := csharpFixtureCalls(t)
	wantCalls(t, calls, map[string][]string{"Worker.Names": {"Console.WriteLine"}})
	for name, cs := range calls {
		for _, c := range cs {
			if c == "nameof" {
				t.Errorf("%s records nameof as a call", name)
			}
		}
	}
}

// TestCSharpDialectQualifiedCalls (LB-F191): under calls.qualify a member or
// static call is its receiver and the name (php's convention, LB-F65), this
// and base included; a delegate invocation is its identifier, as written,
// and `new X()` is no call (scip-dotnet writes it as a `.ctor` reference).
func TestCSharpDialectQualifiedCalls(t *testing.T) {
	calls := csharpFixtureCalls(t)
	wantCalls(t, calls, map[string][]string{
		"Worker.Plain":     {"Helper"},
		"Worker.SelfCalls": {"base.ToString", "this.Plain"},
		"Worker.Delegates": {"callback", "map", "new[]{}.Where", "new[]{}.Where().Select"},
	})
	for name, cs := range calls {
		for _, c := range cs {
			if strings.Contains(c, "Registry") && name == "Worker.Delegates" {
				t.Errorf("%s: %q: object creation and typeof are no calls", name, c)
			}
		}
	}
}
