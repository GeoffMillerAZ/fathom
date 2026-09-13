package scenariobench

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/contextbench/publicloc"
)

// guards_test.go is SB-24's anchor: TestGuardsCorpusDetectorsExist proves
// testdata/scenariobench/guards-fathom.json is not just prose — every
// detector command it names is something this tree can actually run, and
// every rule citation actually points at the invariant it claims to.

const guardsCorpusPath = "../../testdata/scenariobench/guards-fathom.json"

// moduleRoot is the repo root from this package's location
// (internal/scenariobench), for detectors that walk the tree.
const moduleRoot = "../.."

func loadGuardsCorpus(t *testing.T) Corpus {
	t.Helper()
	data, err := os.ReadFile(guardsCorpusPath)
	if err != nil {
		t.Fatalf("read guards corpus: %v", err)
	}
	var c Corpus
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatalf("parse guards corpus: %v", err)
	}
	return c
}

// allowedDetectorVerbs are the invariant-enforcing command families a
// guards-corpus detector may name (docs/intent/invariants.md's own gate
// vocabulary: go test, task <name>, cue vet, bash scripts/*.sh). Anything
// else must at least resolve on PATH — never a command this repo cannot
// actually run.
var allowedDetectorVerbs = map[string]bool{"go": true, "task": true, "cue": true, "bash": true}

// runDetectorRe extracts a `go test <pkgDir> ... -run <Name>` detector's
// package directory and test name.
func parseGoTestDetector(cmd string) (pkgDir, testName string, ok bool) {
	fields := strings.Fields(cmd)
	if len(fields) < 2 || fields[0] != "go" || fields[1] != "test" {
		return "", "", false
	}
	for i := 2; i < len(fields); i++ {
		f := fields[i]
		if strings.HasPrefix(f, "./") {
			pkgDir = strings.TrimPrefix(f, "./")
		}
		if f == "-run" && i+1 < len(fields) {
			testName = strings.Trim(fields[i+1], `'"`)
		}
	}
	return pkgDir, testName, testName != ""
}

// ruleSourceRe parses a "path/to/file#Lnn" rule source citation.
var ruleSourceRe = regexp.MustCompile(`^(.+)#L(\d+)$`)

// testFuncExistsInTree reports whether some "func <name>(" declaration
// exists among the *_test.go files under root/pkgDir (root's whole tree,
// skipping skipTreeDir, when pkgDir is ""). Unlike findFuncInTree (which
// deliberately excludes _test.go — it locates PRODUCTION symbols for
// FalseDoneRate/ContractDrift), a guards-corpus detector names a Test*
// function, which lives only in a _test.go file.
func testFuncExistsInTree(root, pkgDir, name string) (bool, error) {
	var files []string
	if pkgDir != "" {
		dir := filepath.Join(root, filepath.FromSlash(pkgDir))
		entries, err := os.ReadDir(dir)
		if err != nil {
			return false, err
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), "_test.go") {
				files = append(files, filepath.Join(dir, e.Name()))
			}
		}
	} else {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				if path != root && skipTreeDir(info.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(path, "_test.go") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return false, err
		}
	}

	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		spans, err := publicloc.FuncSpans(src, path)
		if err != nil {
			continue
		}
		for _, sp := range spans {
			if sp.Name == name {
				return true, nil
			}
		}
	}
	return false, nil
}

// TestGuardsCorpusDetectorsExist is SB-24's anchor: every task's detector
// command is runnable in this tree (a go/task/cue/bash verb, or something
// on PATH), every `go test -run TestX` detector names a TestX this tree
// actually declares, and every rule's `source` citation ("file#Lnn") names
// a line that actually carries that rule's id.
func TestGuardsCorpusDetectorsExist(t *testing.T) {
	c := loadGuardsCorpus(t)
	if len(c.Tasks) < 8 {
		t.Fatalf("guards corpus has %d tasks, want >= 8", len(c.Tasks))
	}
	if c.Criterion.ID == "" {
		t.Fatalf("guards corpus has no criterion")
	}

	for _, task := range c.Tasks {
		task := task
		t.Run(task.ID, func(t *testing.T) {
			if task.Rule == nil {
				t.Fatalf("task %s: no rule", task.ID)
			}
			if len(task.Oracle.Detectors) == 0 {
				t.Fatalf("task %s: no detectors", task.ID)
			}
			for _, d := range task.Oracle.Detectors {
				if d.ExpectExit != 0 {
					t.Errorf("task %s: detector %s expect_exit = %d, want 0", task.ID, d.Name, d.ExpectExit)
				}
				fields := strings.Fields(d.Command)
				if len(fields) == 0 {
					t.Fatalf("task %s: detector %s has an empty command", task.ID, d.Name)
				}
				verb := fields[0]
				if !allowedDetectorVerbs[verb] {
					if _, err := exec.LookPath(verb); err != nil {
						t.Errorf("task %s: detector %s's command %q starts with %q, which is none of go/task/cue/bash and is not on PATH", task.ID, d.Name, d.Command, verb)
					}
				}
				if pkgDir, testName, ok := parseGoTestDetector(d.Command); ok {
					if found, err := testFuncExistsInTree(moduleRoot, pkgDir, testName); err != nil {
						t.Errorf("task %s: detector %s: searching for %s in %q: %v", task.ID, d.Name, testName, pkgDir, err)
					} else if !found {
						t.Errorf("task %s: detector %s names test %s, which does not exist under %q", task.ID, d.Name, testName, pkgDir)
					}
				}
			}

			m := ruleSourceRe.FindStringSubmatch(task.Rule.Source)
			if m == nil {
				t.Fatalf("task %s: rule source %q is not \"path#Lnn\"", task.ID, task.Rule.Source)
			}
			path, lineStr := m[1], m[2]
			line, err := strconv.Atoi(lineStr)
			if err != nil {
				t.Fatalf("task %s: rule source line %q: %v", task.ID, lineStr, err)
			}
			data, err := os.ReadFile(moduleRoot + "/" + path)
			if err != nil {
				t.Fatalf("task %s: reading rule source %q: %v", task.ID, path, err)
			}
			lines := strings.Split(string(data), "\n")
			if line < 1 || line > len(lines) {
				t.Fatalf("task %s: rule source %q names line %d, file has %d lines", task.ID, task.Rule.Source, line, len(lines))
			}
			marker := fmt.Sprintf("| %s |", task.Rule.ID)
			if !strings.Contains(lines[line-1], marker) {
				t.Errorf("task %s: %s line %d does not contain %q:\n%s", task.ID, path, line, marker, lines[line-1])
			}
		})
	}
}
