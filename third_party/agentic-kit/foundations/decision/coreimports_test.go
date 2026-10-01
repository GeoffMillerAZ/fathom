package decision_test

import (
	"os/exec"
	"slices"
	"strings"
	"testing"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/agentic-kit/foundations/decision/llm"
	"github.com/geoffmilleraz/agentic-kit/foundations/decision/typesafe"
	llmclient "github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
)

// corePackage is the import path of the port this test guards.
const corePackage = "github.com/geoffmilleraz/agentic-kit/foundations/decision"

// coreForbiddenImports are the packages the core must never reach, directly
// or transitively, outside its tests: the two adapter packages, the generic
// completion client the LLM adapter wraps, and the HTTP transport the
// TypeSafe adapter speaks. A path matches itself and every package below it.
var coreForbiddenImports = []string{
	corePackage + "/llm",
	corePackage + "/typesafe",
	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient",
	"net/http",
}

// TestCoreImportsNoAdapterPackage proves the port's direction (plan row
// DP-14): the adapters import foundations/decision, never the reverse, so the
// core carries no vendor code. It builds an adapter from its own package and
// uses it as a decision.Decider, then walks the core's transitive non-test
// imports with `go list -deps` and fails on any adapter, llmclient or
// net/http package among them.
func TestCoreImportsNoAdapterPackage(t *testing.T) {
	// An adapter plugs into the port from outside the core.
	var d decision.Decider = llm.NewLLM(llmclient.NewMockClient(), "import-graph-model")
	if got := d.Name(); got != "llm" {
		t.Fatalf("llm.NewLLM(...).Name() = %q, want %q", got, "llm")
	}
	var _ decision.Decider = (*typesafe.TypeSafe)(nil)

	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("the go tool is needed to walk the import graph: %v", err)
	}
	// The test runs in the core's own directory; without -test, go list
	// reports only the non-test imports.
	out, err := exec.Command(goTool, "list", "-deps", "-f", "{{.ImportPath}}", ".").Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		t.Fatalf("go list -deps: %v\n%s", err, stderr)
	}
	deps := strings.Fields(string(out))

	// Guard against a vacuous walk: the list must hold the core itself and a
	// standard-library package every version of the core imports.
	for _, want := range []string{corePackage, "context"} {
		if !slices.Contains(deps, want) {
			t.Fatalf("go list -deps did not report %s among %d packages; the walk is not of the core", want, len(deps))
		}
	}

	for _, dep := range deps {
		for _, bad := range coreForbiddenImports {
			if dep == bad || strings.HasPrefix(dep, bad+"/") {
				t.Errorf("%s imports %s (transitively, outside tests); the core must import no adapter or vendor transport", corePackage, dep)
			}
		}
	}
}
