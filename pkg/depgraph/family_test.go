package depgraph_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

const familyModule = "github.com/geoffmilleraz/fathom"
const enginePackage = familyModule + "/pkg/depgraph"

func TestDepgraphFamilyBoundary(t *testing.T) {
	t.Run("engine", func(t *testing.T) {
		packages := nativePackageClosure(t, "../..", "./pkg/depgraph/...")
		for _, member := range []string{enginePackage, enginePackage + "/hashw", enginePackage + "/conformance", enginePackage + "/mockresolver"} {
			if !slices.Contains(packages, member) {
				t.Errorf("engine closure is missing %s", member)
			}
		}
		if crossings := engineBoundaryCrossings(packages, true); len(crossings) != 0 {
			t.Errorf("engine reaches outside its family: %v", crossings)
		}
	})

	t.Run("language and proof layer", func(t *testing.T) {
		roots := []string{"pkg/lang", "cmd/fathom-lang", "cmd/fathom", "pkg/client", "pkg/platform"}
		packages := nativePackageClosure(t, "../..", "./pkg/lang/...", "./cmd/fathom-lang", "./cmd/fathom", "./pkg/client", "./pkg/platform")
		for _, root := range roots {
			if !slices.Contains(packages, familyModule+"/"+root) {
				t.Errorf("reverse closure is missing %s", root)
			}
		}
		if crossings := engineBoundaryCrossings(packages, false); len(crossings) != 0 {
			t.Errorf("language or proof layer reaches the engine: %v", crossings)
		}
	})
}

func TestDepgraphFamilyBoundaryRejectsTransitiveImports(t *testing.T) {
	for _, tc := range []struct {
		name, source, target string
		fromEngine           bool
	}{
		{"engine to language", "pkg/depgraph", "pkg/lang/deps", true},
		{"engine to proof", "pkg/depgraph", "internal/store", true},
		{"language to engine", "pkg/lang/deps", "pkg/depgraph/hashw", false},
		{"proof to engine", "cmd/fathom", "pkg/depgraph", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			bridge := "internal/bridge"
			if tc.fromEngine {
				bridge = "pkg/depgraph/bridge"
			}
			target := familyModule + "/" + tc.target
			for name, content := range map[string]string{
				"go.mod":                  "module " + familyModule + "\n",
				tc.source + "/subject.go": "package subject\nimport _ " + quoteImport(familyModule+"/"+bridge),
				bridge + "/subject.go":    "package subject\nimport _ " + quoteImport(target),
				tc.target + "/subject.go": "package subject\n",
			} {
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			packages := nativePackageClosure(t, root, "./"+tc.source)
			if got := engineBoundaryCrossings(packages, tc.fromEngine); !slices.Equal(got, []string{target}) {
				t.Fatalf("crossings = %v, want [%s]", got, target)
			}
		})
	}
}

func quoteImport(path string) string {
	return "\"" + path + "\"\n"
}

func nativePackageClosure(t *testing.T, root string, subjects ...string) []string {
	t.Helper()
	args := append([]string{"list", "-deps", "-f", "{{if not .Standard}}{{.ImportPath}}{{end}}"}, subjects...)
	cmd := exec.CommandContext(t.Context(), filepath.Join(runtime.GOROOT(), "bin", "go"), args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=-mod=readonly", "GOWORK=off", "GOTOOLCHAIN=local",
		"GOPROXY=off", "GOSUMDB=off", "CGO_ENABLED=0", "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("native Go closure of %v: %v\n%s", subjects, err, out)
	}
	return strings.Fields(string(out))
}

func engineBoundaryCrossings(packages []string, fromEngine bool) []string {
	var crossings []string
	for _, pkg := range packages {
		inEngine := pkg == enginePackage || strings.HasPrefix(pkg, enginePackage+"/")
		if inEngine != fromEngine {
			crossings = append(crossings, pkg)
		}
	}
	return crossings
}
