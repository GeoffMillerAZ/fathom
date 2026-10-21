package mcp_test

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// A controlled typed graph exercises the gate's test-identity seam. These
// generic parser fixtures are not a compiler receipt or an executed test run.
func TestVerifyPlanCSharpAttributedTestsWithoutNamePrefix(t *testing.T) {
	e, err := extractor.NewTSExtractor("csharp")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	nodes, err := e.Extract("svc", "Specs/Cases.cs", []byte(`
namespace Workshop {
    class Cases {
        [Test] public void KeepsStock() { }
        [UnityEngine.TestTools.UnityTest] public IEnumerator EventuallyPlaces() { yield break; }
        [NUnit.Framework.TestCase(1, 2)] public void AcceptsOne(int a, int b) { }
        [TestCaseSource(nameof(Data))] public void SourceCases(int x) { }
        [SetUp] public void Prepare() { }
        [TearDown] public void Cleanup() { }
        [Other.Test] public void OtherAttribute() { }
        [Description("[Test]")] public void AttributeInString() { }
        public void TestWithoutAttribute() { }
    }
    class Worker { public static void Accept() { } public static void Unchecked() { } }
}`))
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]core.SymbolNode{}
	for _, n := range nodes {
		byName[n.Name] = n
	}
	var edges []core.Edge
	for _, name := range []string{"KeepsStock", "EventuallyPlaces", "AcceptsOne", "SourceCases", "Prepare", "Cleanup", "OtherAttribute", "AttributeInString", "TestWithoutAttribute"} {
		from := byName["Cases."+name]
		to := byName["Worker.Unchecked"]
		if name == "KeepsStock" || name == "EventuallyPlaces" || name == "AcceptsOne" || name == "SourceCases" {
			to = byName["Worker.Accept"]
		}
		if from.Name == "" || to.Name == "" {
			t.Fatalf("fixture missing source node %s", name)
		}
		edges = append(edges, core.Edge{FromService: "svc", FromFile: from.FilePath, FromSymbol: from.Key(), ToSymbol: to.Key(), Kind: core.EdgeDirect})
	}
	srv := mcp.NewServer(&verifyFakeBackend{nodes: nodes, typedEdges: edges})
	p := callTool(t, srv, "verify_plan", map[string]any{"service": "svc", "concern": "work", "tasks": []any{
		map[string]any{"id": "N-1", "title": "attributed tests", "symbol": "Worker.Accept"},
		map[string]any{"id": "N-2", "title": "non-test callers", "symbol": "Worker.Unchecked"},
	}})
	tasks := p["tasks"].([]any)
	for i, want := range []string{"pass", "fail"} {
		row := tasks[i].(map[string]any)
		var reach, runtime map[string]any
		for _, raw := range row["criteria"].([]any) {
			c := raw.(map[string]any)
			if c["name"] == "test_reaches" {
				reach = c
			}
			if c["name"] == "runtime_pass" {
				runtime = c
			}
		}
		if reach["status"] != want {
			t.Fatalf("row%d test reach = %v, want %s", i, reach, want)
		}
		if i == 0 && !strings.Contains(reach["detail"].(string), "framework binding and execution unverified") {
			t.Fatalf("missing syntactic-identity disclosure: %v", reach)
		}
		if runtime["status"] != "unknown" {
			t.Fatalf("syntax/typed reach fabricated runtime pass: %v", runtime)
		}
	}
	nav := callTool(t, srv, "symbol_context", map[string]any{"service": "svc", "symbol": "Worker.Accept", "snippet": false})
	if tests, _ := nav["related_tests"].([]any); len(tests) != 4 {
		t.Fatalf("related tests = %v, want all4 attributed methods", nav)
	}
	if nav["related_tests_basis"] == nil {
		t.Fatal("C# candidate identity needs syntax disclosure")
	}
}
