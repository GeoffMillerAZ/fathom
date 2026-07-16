package extractor_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
)

// The kit convention (and much real Go) imports packages under an explicit
// alias — e.g. `import sig ".../evidence-signature"` — because a hyphenated
// directory name is not a valid Go identifier. Recording the callee under the
// ALIAS ("sig.PreconditionSatisfied") is meaningless to the resolver: the
// alias is file-local. The file's AST import table maps alias → import path,
// a syntactically-present fact, so the extractor must record the callee under
// the canonical package identity — the last import-path segment — instead.

const aliasFixture = `package evidenceladder_test

import (
	"testing"

	sig "example.com/kit/foundations/evidence/evidence-signature"
	"example.com/kit/foundations/llmclient/openai"
)

func TestPrecondition(t *testing.T) {
	if !sig.PreconditionSatisfied() {
		t.Fatal("nope")
	}
	_ = openai.New()
}
`

// TestGoExtractor_AliasedImportCallCanonicalized pins FIX 1: a selector call
// through an explicit import alias is recorded under the import path's last
// segment, never under the file-local alias.
func TestGoExtractor_AliasedImportCallCanonicalized(t *testing.T) {
	x := extractor.NewGoExtractor()
	nodes, err := x.Extract("kit", "foundations/evidence/evidence-ladder/x_test.go", []byte(aliasFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	calls := callsOf(t, nodes, "TestPrecondition")

	if !contains(calls, "evidence-signature.PreconditionSatisfied") {
		t.Errorf("Calls = %v, want %q (alias canonicalized to import-path last segment)",
			calls, "evidence-signature.PreconditionSatisfied")
	}
	if contains(calls, "sig.PreconditionSatisfied") {
		t.Errorf("Calls = %v: file-local alias %q must not be recorded", calls, "sig.PreconditionSatisfied")
	}
	// An unaliased import is referenced by the target package's declared name,
	// which is exactly what byPkgName resolution keys on — leave it unchanged.
	if !contains(calls, "openai.New") {
		t.Errorf("Calls = %v, want %q (unaliased import unchanged)", calls, "openai.New")
	}
}

// callsOf returns the Calls of the node with the given Name, failing the test
// when the node is missing.
func callsOf(t *testing.T, nodes []core.SymbolNode, name string) []string {
	t.Helper()
	for _, n := range nodes {
		if n.Name == name {
			return n.Calls
		}
	}
	t.Fatalf("node %q not found in %d extracted nodes", name, len(nodes))
	return nil
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
