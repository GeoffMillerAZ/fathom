package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
)

const familyRulesFile = ".fathom/rules/dependency-boundaries.json"
const familyModule = "github.com/geoffmilleraz/fathom"

// The cases shared with TestClosureChecksFailOnBrokenSets use the same package
// boundaries, with actual native imports. The other cases exercise the allowed
// directions and the direct crossing on a forbidden transitive path.
func TestFamilyDependencyBoundaryNativeParity(t *testing.T) {
	repo := familyPhysicalRoot(t, "../..")
	rules := filepath.Join(repo, familyRulesFile)
	for _, tc := range []struct {
		name, entry, from, target string
		forbidden, transitive     bool
	}{
		{"store", "pkg/lang", "pkg/lang", "internal/store", true, false},
		{"store-child", "pkg/lang", "pkg/lang", "internal/store/sub", true, false},
		{"fusion", "pkg/lang", "pkg/lang", "internal/fusion", true, false},
		{"connector", "pkg/lang", "pkg/lang", "internal/connector", true, false},
		{"mcp", "pkg/lang", "pkg/lang", "internal/mcp", true, false},
		{"http", "pkg/lang/iac", "pkg/lang/iac", "go:net/http", true, false},
		{"cue-from-base", "pkg/lang", "pkg/lang", "go:cuelang.org/go/cue", true, false},
		{"storeless", "pkg/lang", "pkg/lang", "internal/storeless", false, false},
		{"store-transitive", "pkg/lang", "internal/bridge", "internal/store", true, true},
		{"optional-cue", "pkg/lang/cue", "internal/extractor/cuelang", "go:cuelang.org/go/cue", false, true},
		{"cue-adapter-from-base", "pkg/lang", "pkg/lang", "internal/extractor/cuelang", true, false},
		{"cue-door-from-base", "pkg/lang", "pkg/lang", "pkg/lang/cue", true, false},
		{"checks-from-base", "pkg/lang", "pkg/lang", "pkg/lang/deps/boundaries", true, false},
		{"understanding-cli-to-boundaries", "cmd/fathom-lang", "cmd/fathom-lang", "pkg/lang/deps/boundaries", false, false},
		{"product-to-understanding", "cmd/fathom", "cmd/fathom", "pkg/lang", false, false},
		{"product-to-store", "cmd/fathom", "cmd/fathom", "internal/store", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := familyPhysicalRoot(t, t.TempDir())
			for _, file := range []string{"go.mod", "go.sum"} {
				data, err := os.ReadFile(filepath.Join(repo, file))
				if err != nil {
					t.Fatal(err)
				}
				write(t, root, file, string(data))
			}
			operand, external := strings.CutPrefix(tc.target, "go:")
			if !external {
				operand = familyModule + "/" + tc.target
				write(t, root, tc.target+"/subject.go", "package subject\n")
			}
			source := tc.from + "/subject.go"
			write(t, root, source, "package subject\nimport _ \""+operand+"\"\n")
			if tc.transitive {
				write(t, root, tc.entry+"/subject.go", "package subject\nimport _ \""+familyModule+"/"+tc.from+"\"\n")
			}
			write(t, root, tc.entry+"/ignored_test.go", "package subject\nimport _ \"net/http\"\n")

			packages := familyGoList(t, root, "./"+tc.entry)
			entry, from := packages[familyModule+"/"+tc.entry], packages[familyModule+"/"+tc.from]
			if packages[operand].ImportPath != operand || !slices.Contains(from.Imports, operand) {
				t.Fatalf("native graph missed seeded crossing %s -> %s", tc.from, operand)
			}
			if tc.transitive && (slices.Contains(entry.Imports, operand) || !slices.Contains(entry.Imports, from.ImportPath)) {
				t.Fatalf("native entry does not reach target through the intermediary: %v", entry.Imports)
			}
			files := familySourceFiles(t, root, packages)
			if slices.Contains(files, tc.entry+"/ignored_test.go") {
				t.Fatal("native production scope included a test file")
			}
			results, findings, code := runFamilyBoundaries(t, root, rules, files)
			var violations []deps.Edge
			for _, rule := range results {
				violations = append(violations, rule.Violations...)
			}
			if !tc.forbidden {
				if code != 0 || len(violations) != 0 {
					t.Fatalf("allowed native edge was refused: %v, exit %d\n%s", violations, code, findings)
				}
				return
			}
			if code != 1 || len(violations) != 1 || violations[0].From != source ||
				violations[0].To != tc.target || violations[0].Line != 2 {
				t.Fatalf("native crossing = %s:2 -> %s; got %v, exit %d\n%s", source, tc.target, violations, code, findings)
			}
			if external && !strings.Contains(findings, "unverified "+source) {
				t.Fatalf("external violation suppressed uncertainty:\n%s", findings)
			}
			t.Logf("native and boundary agree: %s:2 -> %s", source, tc.target)
		})
	}
}

type familyGoPackage struct {
	Dir, ImportPath            string
	GoFiles, CgoFiles, Imports []string
	Module                     *struct{ Main bool }
}

func familyPhysicalRoot(t *testing.T, root string) string {
	t.Helper()
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	physical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		t.Fatal(err)
	}
	return physical
}

func familyGoList(t *testing.T, root string, subjects ...string) map[string]familyGoPackage {
	t.Helper()
	args := append([]string{"list", "-deps", "-json=Dir,ImportPath,GoFiles,CgoFiles,Imports,Module"}, subjects...)
	cmd := exec.CommandContext(t.Context(), filepath.Join(runtime.GOROOT(), "bin", "go"), args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOENV=off", "GOFLAGS=-mod=readonly", "GOTOOLCHAIN=local", "GOPROXY=off",
		"GOSUMDB=off", "GOWORK=off", "CGO_ENABLED=0", "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("native go list: %v\n%s", err, &stderr)
	}
	packages := make(map[string]familyGoPackage)
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var p familyGoPackage
		if err := dec.Decode(&p); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		packages[p.ImportPath] = p
	}
	for _, subject := range subjects {
		if p := packages[familyModule+"/"+strings.TrimPrefix(subject, "./")]; p.Module == nil || !p.Module.Main {
			t.Fatalf("native graph did not list family member %s", subject)
		}
	}
	return packages
}

func familySourceFiles(t *testing.T, root string, packages map[string]familyGoPackage) []string {
	t.Helper()
	var names []string
	for _, p := range packages {
		if p.Module == nil || !p.Module.Main {
			continue
		}
		dir, err := filepath.Rel(root, p.Dir)
		if err != nil || !filepath.IsLocal(dir) {
			t.Fatalf("native family package %s is outside the repository", p.ImportPath)
		}
		for _, name := range append(slices.Clone(p.GoFiles), p.CgoFiles...) {
			names = append(names, path.Join(filepath.ToSlash(dir), name))
		}
	}
	if len(names) == 0 {
		t.Fatal("native family scope is empty")
	}
	slices.Sort(names)
	return slices.Compact(names)
}

type familyRuleResult struct {
	ID           string
	Verdict      string
	MatchedFiles int `json:"matched_files"`
	Violations   []deps.Edge
}

func runFamilyBoundaries(t *testing.T, root, rules string, files []string) ([]familyRuleResult, string, int) {
	t.Helper()
	args := append([]string{"boundaries", "--root", root, "--rules", rules}, files...)
	out, findings, code := runBoundariesTest(t, args)
	var result struct{ Rules []familyRuleResult }
	if err := json.Unmarshal([]byte(out), &result); err != nil || len(result.Rules) == 0 {
		t.Fatalf("boundary rule results missing: %v, exit %d\n%s", err, code, findings)
	}
	return result.Rules, findings, code
}
