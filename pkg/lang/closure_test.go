package lang_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// packageBudget is the most non-standard packages the door may link
// (docs/design/lang-library.md section 8, row 1). The bar is 80 for a consumer,
// which adds its own main package, so the door's own budget is 79. The closure
// reads 78 today, with CUE left to pkg/lang/cue.
const packageBudget = 79

// forbiddenClosure are the dependencies the door must never reach: fathom's
// store, fusion, connectors and MCP server, the network client stack and CUE.
// A path matches itself and everything below it.
var forbiddenClosure = []string{
	"github.com/geoffmilleraz/fathom/internal/store",
	"github.com/geoffmilleraz/fathom/internal/fusion",
	"github.com/geoffmilleraz/fathom/internal/connector",
	"github.com/geoffmilleraz/fathom/internal/mcp",
	"net/http",
	"cuelang.org",
}

type listedPackage struct {
	ImportPath string
	Standard   bool
	Error      *struct{ Err string }
}

// closureProblems returns what is wrong with a dependency set: each forbidden
// package reached and a count over the budget. It is separate from the go list
// call so its checks can be held to known inputs.
func closureProblems(pkgs []listedPackage) []string {
	var problems []string
	nonStd := 0
	for _, p := range pkgs {
		if p.Error != nil {
			problems = append(problems, "go list reports an error for "+p.ImportPath+": "+p.Error.Err)
		}
		if !p.Standard {
			nonStd++
		}
		for _, bad := range forbiddenClosure {
			if p.ImportPath == bad || strings.HasPrefix(p.ImportPath, bad+"/") {
				problems = append(problems, "reaches forbidden package "+p.ImportPath)
			}
		}
	}
	if nonStd > packageBudget {
		problems = append(problems, "links "+strconv.Itoa(nonStd)+" non-standard packages, over the budget "+strconv.Itoa(packageBudget))
	}
	return problems
}

// goListDeps runs go list on the package in the working directory. Any
// failure is the caller's to fail on: a closure that cannot be measured is not
// a closure that holds.
func goListDeps(t *testing.T) []listedPackage {
	t.Helper()
	cmd := exec.Command("go", "list", "-e", "-deps", "-json=ImportPath,Standard,Error", ".")
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list -deps failed, so the closure is unmeasured: %v\n%s", err, stderr.String())
	}
	var pkgs []listedPackage
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var p listedPackage
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("decode go list output: %v", err)
		}
		pkgs = append(pkgs, p)
	}
	if len(pkgs) < 10 || !slices.ContainsFunc(pkgs, func(p listedPackage) bool {
		return p.ImportPath == "github.com/geoffmilleraz/fathom/pkg/lang"
	}) {
		t.Fatalf("go list -deps returned %d packages and not pkg/lang itself", len(pkgs))
	}
	return pkgs
}

// LL-20: the door's closure holds no store, fusion, connector, MCP, net/http
// or CUE, and stays inside its package budget. It fails, never skips, when go
// list does not run.
func TestLangClosure(t *testing.T) {
	for _, problem := range closureProblems(goListDeps(t)) {
		t.Error(problem)
	}
}

// The checker itself: each property it guards must fail on a set that breaks
// it, and a clean set must pass.
func TestClosureChecksFailOnBrokenSets(t *testing.T) {
	clean := []listedPackage{{ImportPath: "fmt", Standard: true}, {ImportPath: "github.com/x/y"}}
	if p := closureProblems(clean); len(p) != 0 {
		t.Fatalf("clean set: %v", p)
	}
	for _, bad := range []string{
		"github.com/geoffmilleraz/fathom/internal/store",
		"github.com/geoffmilleraz/fathom/internal/store/sub",
		"github.com/geoffmilleraz/fathom/internal/fusion",
		"github.com/geoffmilleraz/fathom/internal/connector",
		"github.com/geoffmilleraz/fathom/internal/mcp",
		"net/http",
		"cuelang.org/go/cue",
	} {
		if p := closureProblems(append(slices.Clone(clean), listedPackage{ImportPath: bad, Standard: bad == "net/http"})); len(p) == 0 {
			t.Errorf("%s was not caught", bad)
		}
	}
	over := slices.Clone(clean)
	for i := 0; i <= packageBudget; i++ {
		over = append(over, listedPackage{ImportPath: "example.com/p" + strconv.Itoa(i)})
	}
	if p := closureProblems(over); len(p) == 0 {
		t.Error("a set over the budget was not caught")
	}
	if p := closureProblems([]listedPackage{{ImportPath: "x", Error: &struct{ Err string }{"boom"}}}); len(p) == 0 {
		t.Error("a go list error was not caught")
	}
	if p := closureProblems([]listedPackage{{ImportPath: "github.com/geoffmilleraz/fathom/internal/storeless"}}); len(p) != 0 {
		t.Errorf("a name that only shares a prefix was caught: %v", p)
	}
}
