package depgraph_test

import (
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

const (
	familyModule     = "github.com/geoffmilleraz/fathom"
	enginePackage    = familyModule + "/pkg/depgraph"
	companionPackage = familyModule + "/cmd/fathom-lang"
)

func TestDepgraphFamilyBoundary(t *testing.T) {
	// Keep each package's own closure. A merged closure would hide which
	// package imported the engine once the companion is wired.
	closures := nativePackageClosures(t, "../..", "./...")
	for _, member := range []string{
		enginePackage, enginePackage + "/hashw", enginePackage + "/conformance", enginePackage + "/mockresolver",
		familyModule + "/pkg/lang", familyModule + "/pkg/lang/deps", companionPackage,
		familyModule + "/cmd/fathom", familyModule + "/pkg/client", familyModule + "/pkg/platform",
	} {
		if _, ok := closures[member]; !ok {
			t.Errorf("native package list is missing %s", member)
		}
	}
	for _, subject := range slices.Sorted(maps.Keys(closures)) {
		if subject != familyModule && !strings.HasPrefix(subject, familyModule+"/") {
			continue
		}
		if crossings := engineBoundaryCrossings(closures, subject); len(crossings) != 0 {
			t.Errorf("%s reaches forbidden packages: %v", subject, crossings)
		}
	}
}

func TestDepgraphFamilyBoundaryAllowsCompanion(t *testing.T) {
	language := familyModule + "/pkg/lang/deps"
	root := seedFamilyModule(t, map[string]string{
		"cmd/fathom-lang/main.go": "package main\nimport (\n_ " + quoteImport(enginePackage) +
			"_ " + quoteImport(language) + ")\nfunc main() {}\n",
		"pkg/lang/deps/subject.go": "package subject\n",
		"pkg/depgraph/subject.go":  "package subject\n",
	})
	closures := nativePackageClosures(t, root, "./cmd/fathom-lang")
	for _, door := range []string{enginePackage, language} {
		if !slices.Contains(closures[companionPackage], door) {
			t.Fatalf("native companion closure is missing %s", door)
		}
	}
	if crossings := engineBoundaryCrossings(closures, companionPackage); len(crossings) != 0 {
		t.Errorf("companion may compose both doors, got crossings %v", crossings)
	}
}

func TestDepgraphFamilyBoundaryRejectsTransitiveImports(t *testing.T) {
	for _, tc := range []struct {
		name, source, bridge, target string
	}{
		{"engine to language", "pkg/depgraph", "pkg/depgraph/bridge", "pkg/lang/deps"},
		{"engine to proof", "pkg/depgraph", "pkg/depgraph/bridge", "internal/store"},
		{"language to engine", "pkg/lang/deps", "internal/bridge", "pkg/depgraph/hashw"},
		{"proof to engine", "cmd/fathom", "internal/bridge", "pkg/depgraph"},
		{"other package to engine", "pkg/other", "internal/bridge", "pkg/depgraph/conformance"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := familyModule + "/" + tc.target
			root := seedFamilyModule(t, map[string]string{
				tc.source + "/subject.go": "package subject\nimport _ " + quoteImport(familyModule+"/"+tc.bridge),
				tc.bridge + "/subject.go": "package subject\nimport _ " + quoteImport(target),
				tc.target + "/subject.go": "package subject\n",
			})
			closures := nativePackageClosures(t, root, "./"+tc.source)
			if got := engineBoundaryCrossings(closures, familyModule+"/"+tc.source); !slices.Equal(got, []string{target}) {
				t.Fatalf("crossings = %v, want [%s]", got, target)
			}
		})
	}
}

func TestDepgraphFamilyBoundaryRejectsDirectImports(t *testing.T) {
	for _, source := range []string{"cmd/fathom", "cmd/fathom-lang/helper", "cmd/fathom-lang-other"} {
		t.Run(source, func(t *testing.T) {
			root := seedFamilyModule(t, map[string]string{
				source + "/subject.go":    "package subject\nimport _ " + quoteImport(enginePackage),
				"pkg/depgraph/subject.go": "package subject\n",
			})
			closures := nativePackageClosures(t, root, "./"+source)
			if got := engineBoundaryCrossings(closures, familyModule+"/"+source); !slices.Equal(got, []string{enginePackage}) {
				t.Fatalf("crossings = %v, want [%s]", got, enginePackage)
			}
		})
	}
}

func seedFamilyModule(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files["go.mod"] = "module " + familyModule + "\n"
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func quoteImport(path string) string {
	return "\"" + path + "\"\n"
}

func nativePackageClosures(t *testing.T, root string, subjects ...string) map[string][]string {
	t.Helper()
	args := append([]string{"list", "-deps", "-f", "{{if not .Standard}}{{.ImportPath}}{{range .Deps}} {{.}}{{end}}{{end}}"}, subjects...)
	cmd := exec.CommandContext(t.Context(), filepath.Join(runtime.GOROOT(), "bin", "go"), args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=-mod=readonly", "GOWORK=off", "GOTOOLCHAIN=local",
		"GOPROXY=off", "GOSUMDB=off", "CGO_ENABLED=0", "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("native Go closure of %v: %v\n%s", subjects, err, out)
	}
	closures := make(map[string][]string)
	for line := range strings.SplitSeq(string(out), "\n") {
		if fields := strings.Fields(line); len(fields) > 0 {
			closures[fields[0]] = fields[1:]
		}
	}
	return closures
}

func engineBoundaryCrossings(closures map[string][]string, subject string) []string {
	var crossings []string
	for _, dep := range closures[subject] {
		if isEnginePackage(subject) {
			if _, nonStandard := closures[dep]; nonStandard && !isEnginePackage(dep) {
				crossings = append(crossings, dep)
			}
		} else if subject != companionPackage && isEnginePackage(dep) {
			crossings = append(crossings, dep)
		}
	}
	return crossings
}

func isEnginePackage(pkg string) bool {
	return pkg == enginePackage || strings.HasPrefix(pkg, enginePackage+"/")
}
